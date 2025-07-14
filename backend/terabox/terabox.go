// Package terabox provides an interface to the TeraBox storage system.
// This backend implements sync functionality with efficient change detection.
package terabox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
)

// Register the backend with Rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "terabox",
		Description: "TeraBox Cloud Storage",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "jstoken",
				Help:     "TeraBox JSTOKEN (required)",
				Required: true,
			},
			{
				Name:     "bdstoken",
				Help:     "TeraBox BDSTOKEN (required)",
				Required: true,
			},
			{
				Name:     "cookie",
				Help:     "TeraBox COOKIE (required)",
				Required: true,
			},
			{
				Name:    "timeout",
				Help:    "Timeout for API requests in seconds. Default: 60",
				Default: 60,
			},
		},
	})
}

// Options defines the configuration for the TeraBox backend
type Options struct {
	JSToken  string `config:"jstoken" help:"TeraBox JSTOKEN (required)"`
	BDSToken string `config:"bdstoken" help:"TeraBox BDSTOKEN (required)"`
	Cookie   string `config:"cookie" help:"TeraBox COOKIE (required)"`
	Timeout  int    `config:"timeout" help:"Timeout for API requests in seconds. Default: 60"`
}

// Fs represents a remote TeraBox server
type Fs struct {
	name       string       // name of this remote
	root       string       // the path we are working on
	opt        Options      // parsed options
	features   *fs.Features // optional features
	httpClient *http.Client
}

// Object describes a TeraBox file object
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
	fsID    int64
	path    string
}

// NewFs constructs an Fs from the path, config and options
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Check required tokens
	if opt.JSToken == "" || opt.BDSToken == "" || opt.Cookie == "" {
		return nil, fmt.Errorf("TeraBox requires JSToken, BDSTOKEN, and Cookie to be set")
	}

	// Create HTTP client with proper timeouts
	timeout := time.Duration(opt.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		httpClient: httpClient,
	}

	// Set features
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		ReadMimeType:            false,
		WriteMimeType:           false,
		ListR:                   f.ListR,
	}).Fill(ctx, f)

	return f, nil
}

// Implement fs.Info interface
func (f *Fs) Name() string   { return f.name }
func (f *Fs) Root() string   { return f.root }
func (f *Fs) String() string { return "TeraBox root '" + f.root + "'" }

// Implement fs.Object interface
func (o *Object) Fs() fs.Info                           { return o.fs }
func (o *Object) Remote() string                        { return o.remote }
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }
func (o *Object) Size() int64                           { return o.size }
func (o *Object) Storable() bool                        { return true }
func (o *Object) String() string                        { return o.remote }

// Implement fs.Fs interface
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Handle root directory case
	apiDir := dir
	if dir == "" {
		apiDir = "/"
	}

	// Ensure all non-root paths start with a leading slash
	if apiDir != "/" && !strings.HasPrefix(apiDir, "/") {
		apiDir = "/" + apiDir
	}

	// DO NOT encode apiDir here; pass it raw to the API
	fmt.Printf("DEBUG: List called with dir='%s', apiDir='%s'\n", dir, apiDir)

	query := map[string]string{
		"dir":      apiDir,
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}
	resp, err := f.apiRequest(ctx, "GET", "/api/list", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var listResp struct {
		List []struct {
			Filename   string `json:"filename"`
			Path       string `json:"path"`
			Isdir      int    `json:"isdir"`
			Size       int64  `json:"size"`
			ModTime    int64  `json:"server_mtime"`
			LocalMtime int64  `json:"local_mtime"`
			FsID       int64  `json:"fs_id"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	fmt.Printf("DEBUG: List response - errno=%d, entries=%d\n", listResp.Errno, len(listResp.List))

	if listResp.Errno != 0 {
		switch listResp.Errno {
		case 2:
			// Directory not found - return empty list
			return fs.DirEntries{}, nil
		case -9:
			// Network error - retryable
			return nil, fserrors.RetryError(fmt.Errorf("list API network error (errno=-9)"))
		default:
			return nil, fmt.Errorf("TeraBox API list error: errno=%d", listResp.Errno)
		}
	}

	entries = make(fs.DirEntries, 0, len(listResp.List))
	for _, item := range listResp.List {
		var name string
		if item.Filename == "" && item.Path != "" {
			name = filepath.Base(item.Path)
			if name == "" || name == "/" {
				continue
			}
		} else if item.Filename == "" {
			continue
		} else {
			name = item.Filename
		}

		if item.Isdir == 1 {
			// Directory
			entries = append(entries, fs.NewDir(name, time.Unix(item.ModTime, 0)))
		} else {
			// File - extract relative path from full path
			var remotePath string
			if item.Path != "" {
				remotePath = strings.TrimPrefix(item.Path, "/")
			} else {
				remotePath = name
			}

			mtime := item.ModTime
			if mtime == 0 {
				mtime = item.LocalMtime
			}

			obj := &Object{
				fs:      f,
				remote:  remotePath,
				size:    item.Size,
				modTime: time.Unix(mtime, 0),
				fsID:    item.FsID,
				path:    item.Path,
			}
			entries = append(entries, obj)
		}
	}

	return entries, nil
}

// Mkdir creates a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	if dir == "/" || dir == "." {
		return nil
	}

	// Check if directory already exists
	query := map[string]string{
		"dir":      dir,
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}
	resp, err := f.apiRequest(ctx, "GET", "/api/list", query, nil)
	if err == nil && resp.StatusCode == 200 {
		resp.Body.Close()
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Create directory
	mkdirData := url.Values{}
	mkdirData.Set("path", dir)
	mkdirData.Set("isdir", "1")
	mkdirData.Set("rtype", "1")

	mkdirReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.terabox.com/api/create?bdstoken="+f.opt.BDSToken+"&app_id=250528&jsToken="+f.opt.JSToken,
		strings.NewReader(mkdirData.Encode()))
	if err != nil {
		return err
	}
	mkdirReq.Header.Set("User-Agent", "okhttp/7.4")
	mkdirReq.Header.Set("Origin", "https://www.terabox.com")
	mkdirReq.Header.Set("Referer", "https://www.terabox.com/")
	mkdirReq.Header.Set("Cookie", f.opt.Cookie)
	mkdirReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err = f.httpClient.Do(mkdirReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if resp.StatusCode == 200 {
			return nil
		}
		return fmt.Errorf("TeraBox API mkdir failed: %s", resp.Status)
	}

	if result.Errno != 0 {
		return fmt.Errorf("TeraBox API mkdir error: errno=%d", result.Errno)
	}

	return nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if in == nil {
		return nil, fmt.Errorf("input reader is nil")
	}

	remote := src.Remote()
	fileSize := src.Size()

	// Ensure parent directory exists
	fullPath := f.root
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += remote

	parentDir := filepath.Dir(fullPath)
	if parentDir != "/" && parentDir != "." {
		if err := f.Mkdir(ctx, parentDir); err != nil {
			return nil, fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
		}
	}

	// Read file data
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}

	// Upload file
	uploadData := url.Values{}
	uploadData.Set("path", fullPath)
	uploadData.Set("file", string(data))
	uploadData.Set("rtype", "1")

	uploadReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.terabox.com/api/upload?bdstoken="+f.opt.BDSToken+"&app_id=250528&jsToken="+f.opt.JSToken,
		strings.NewReader(uploadData.Encode()))
	if err != nil {
		return nil, err
	}
	uploadReq.Header.Set("User-Agent", "okhttp/7.4")
	uploadReq.Header.Set("Origin", "https://www.terabox.com")
	uploadReq.Header.Set("Referer", "https://www.terabox.com/")
	uploadReq.Header.Set("Cookie", f.opt.Cookie)
	uploadReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(uploadReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var uploadResult struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&uploadResult); err != nil {
		return nil, fmt.Errorf("failed to decode upload response: %w", err)
	}

	if uploadResult.Errno != 0 {
		return nil, fmt.Errorf("TeraBox API upload error: errno=%d", uploadResult.Errno)
	}

	obj := &Object{
		fs:      f,
		remote:  remote,
		size:    fileSize,
		modTime: time.Now(),
	}

	return obj, nil
}

// Remove deletes a file
func (f *Fs) Remove(ctx context.Context, path string) error {
	// Construct full path
	fullPath := path
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}

	// Delete file
	deleteData := url.Values{}
	deleteData.Set("filelist", fmt.Sprintf("[{\"path\":\"%s\"}]", fullPath))

	deleteReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.terabox.com/api/filemanager?bdstoken="+f.opt.BDSToken+"&app_id=250528&jsToken="+f.opt.JSToken,
		strings.NewReader(deleteData.Encode()))
	if err != nil {
		return err
	}
	deleteReq.Header.Set("User-Agent", "okhttp/7.4")
	deleteReq.Header.Set("Origin", "https://www.terabox.com")
	deleteReq.Header.Set("Referer", "https://www.terabox.com/")
	deleteReq.Header.Set("Cookie", f.opt.Cookie)
	deleteReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.httpClient.Do(deleteReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var deleteResult struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&deleteResult); err != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}

	if deleteResult.Errno != 0 {
		return fmt.Errorf("TeraBox API delete error: errno=%d", deleteResult.Errno)
	}

	return nil
}

// RemoveDir removes a directory
func (f *Fs) RemoveDir(ctx context.Context, dir string) error {
	return f.Remove(ctx, dir)
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.RemoveDir(ctx, dir)
}

// NewObject creates a new object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Try to get object info
	query := map[string]string{
		"dir":      filepath.Dir(remote),
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}
	resp, err := f.apiRequest(ctx, "GET", "/api/list", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var listResp struct {
		List []struct {
			Filename   string `json:"filename"`
			Path       string `json:"path"`
			Isdir      int    `json:"isdir"`
			Size       int64  `json:"size"`
			ModTime    int64  `json:"server_mtime"`
			LocalMtime int64  `json:"local_mtime"`
			FsID       int64  `json:"fs_id"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	if listResp.Errno != 0 {
		return nil, fs.ErrorObjectNotFound
	}

	// Find the file
	for _, item := range listResp.List {
		if item.Filename == filepath.Base(remote) && item.Isdir == 0 {
			mtime := item.ModTime
			if mtime == 0 {
				mtime = item.LocalMtime
			}

			obj := &Object{
				fs:      f,
				remote:  remote,
				size:    item.Size,
				modTime: time.Unix(mtime, 0),
				fsID:    item.FsID,
				path:    item.Path,
			}
			return obj, nil
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// apiRequest makes an API request to TeraBox
func (f *Fs) apiRequest(ctx context.Context, method, endpoint string, query map[string]string, body []byte) (*http.Response, error) {
	requestURL := "https://www.terabox.com" + endpoint
	if len(query) > 0 {
		values := url.Values{}
		for k, v := range query {
			values.Set(k, v)
		}
		requestURL += "?" + values.Encode()
	}

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, requestURL, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "okhttp/7.4")
	req.Header.Set("Origin", "https://www.terabox.com")
	req.Header.Set("Referer", "https://www.terabox.com/")
	req.Header.Set("Cookie", f.opt.Cookie)

	fmt.Printf("DEBUG: API Request - %s %s\n", method, requestURL)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Implement remaining fs.Fs interface methods
func (f *Fs) Precision() time.Duration { return time.Second }
func (f *Fs) Hashes() hash.Set         { return hash.Set(hash.None) }
func (f *Fs) Features() *fs.Features   { return f.features }

// Implement remaining fs.Object interface methods
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fmt.Errorf("not implemented")
}

func (o *Object) Remove(ctx context.Context) error {
	return o.fs.Remove(ctx, o.remote)
}

func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fmt.Errorf("not implemented")
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	// Simple recursive implementation using List method
	// This avoids infinite recursion that would occur with walk.ListR
	return f.listRRecursive(ctx, dir, callback)
}

// listRRecursive implements recursive listing using the List method
func (f *Fs) listRRecursive(ctx context.Context, dir string, callback fs.ListRCallback) error {
	entries, err := f.List(ctx, dir)
	if err != nil {
		return err
	}

	// Call callback with current entries
	if err := callback(entries); err != nil {
		return err
	}

	// Recursively list subdirectories
	for _, entry := range entries {
		if d, ok := entry.(fs.Directory); ok {
			// Construct the full path for the subdirectory
			var subDir string
			if dir == "" {
				subDir = d.Remote()
			} else {
				subDir = filepath.Join(dir, d.Remote())
			}
			if err := f.listRRecursive(ctx, subDir, callback); err != nil {
				return err
			}
		}
	}

	return nil
}
