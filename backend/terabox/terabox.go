// Package terabox provides an interface to the TeraBox storage system.
package terabox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"crypto/md5"
	"encoding/hex"
	"path/filepath"

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
				Name:     "client_id",
				Help:     "OAuth Client Id (leave blank to use rclone's)",
				Required: false,
			},
			{
				Name:     "client_secret",
				Help:     "OAuth Client Secret (leave blank to use rclone's)",
				Required: false,
			},
			{
				Name:     "token",
				Help:     "OAuth Access Token or API token",
				Required: false,
			},
			{
				Name:     "root_folder_id",
				Help:     "ID of the root folder (leave blank for default root)",
				Required: false,
			},
			{
				Name:    "chunk_size",
				Help:    "Upload chunk size. Must be a multiple of 256k. Default: 8M",
				Default: fs.SizeSuffix(8 << 20),
			},
			{
				Name:    "use_trash",
				Help:    "Send deleted files to trash instead of deleting permanently. Default: true",
				Default: true,
			},
			{
				Name:    "list_chunk",
				Help:    "Size of listing chunk. Default: 1000",
				Default: 1000,
			},
			{
				Name:    "disable_check_sum",
				Help:    "Disable checksum verification if TeraBox supports it. Default: false",
				Default: false,
			},
			{
				Name:    "upload_cutoff",
				Help:    "Cutoff for switching to chunked upload. Default: 8M",
				Default: fs.SizeSuffix(8 << 20),
			},
			{
				Name:    "upload_chunk_size",
				Help:    "Chunk size for uploads. Default: 8M",
				Default: fs.SizeSuffix(8 << 20),
			},
			{
				Name:    "metadata_fields",
				Help:    "Comma separated list of metadata fields to fetch. Default: all",
				Default: "all",
			},
			{
				Name:    "timeout",
				Help:    "Timeout for API requests in seconds. Default: 60",
				Default: 60,
			},
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
				Name:    "delete_delay",
				Help:    "Delay between delete operations in milliseconds. Default: 1000ms",
				Default: 1000 * time.Millisecond,
			},
			{
				Name:    "batch_delete_size",
				Help:    "Number of files to delete in a single batch operation. Default: 10",
				Default: 10,
			},
			{
				Name:    "max_retries",
				Help:    "Maximum number of retries for failed operations. Default: 3",
				Default: 3,
			},
			{
				Name:    "retry_delay",
				Help:    "Delay between retries in milliseconds. Default: 2000ms",
				Default: 2000 * time.Millisecond,
			},
			{
				Name:    "streaming_threshold",
				Help:    "File size threshold for streaming uploads. Files larger than this will use streaming. Default: 10M",
				Default: fs.SizeSuffix(10 << 20),
			},
		},
	})
}

// Options defines the configuration for the TeraBox backend
// Many options are mapped from Google Drive for user familiarity

type Options struct {
	ClientID string `config:"client_id"
		Help:"OAuth Client Id (leave blank to use rclone's)"`
	ClientSecret string `config:"client_secret"
		Help:"OAuth Client Secret (leave blank to use rclone's)"`
	Token string `config:"token"
		Help:"OAuth Access Token or API token"`
	RootFolderID string `config:"root_folder_id"
		Help:"ID of the root folder (leave blank for default root)"`
	ChunkSize fs.SizeSuffix `config:"chunk_size"
		Help:"Upload chunk size. Must be a multiple of 256k. Default: 8M"`
	UseTrash bool `config:"use_trash"
		Help:"Send deleted files to trash instead of deleting permanently. Default: true"`
	ListChunk int `config:"list_chunk"
		Help:"Size of listing chunk. Default: 1000"`
	DisableCheckSum bool `config:"disable_check_sum"
		Help:"Disable checksum verification if TeraBox supports it. Default: false"`
	UploadCutoff fs.SizeSuffix `config:"upload_cutoff"
		Help:"Cutoff for switching to chunked upload. Default: 8M"`
	UploadChunkSize fs.SizeSuffix `config:"upload_chunk_size"
		Help:"Chunk size for uploads. Default: 8M"`
	MetadataFields string `config:"metadata_fields"
		Help:"Comma separated list of metadata fields to fetch. Default: all"`
	Timeout int `config:"timeout"
		Help:"Timeout for API requests in seconds. Default: 60"`
	JSToken  string `config:"jstoken" help:"TeraBox JSTOKEN (required)"`
	BDSToken string `config:"bdstoken" help:"TeraBox BDSTOKEN (required)"`
	Cookie   string `config:"cookie" help:"TeraBox COOKIE (required)"`
	// New options for improved performance and reliability
	DeleteDelay time.Duration `config:"delete_delay"
		Help:"Delay between delete operations in milliseconds. Default: 1000ms"`
	BatchDeleteSize int `config:"batch_delete_size"
		Help:"Number of files to delete in a single batch operation. Default: 10"`
	MaxRetries int `config:"max_retries"
		Help:"Maximum number of retries for failed operations. Default: 3"`
	RetryDelay time.Duration `config:"retry_delay"
		Help:"Delay between retries in milliseconds. Default: 2000ms"`
	StreamingThreshold fs.SizeSuffix `config:"streaming_threshold"
		Help:"File size threshold for streaming uploads. Files larger than this will use streaming. Default: 10M"`
}

// Fs represents a remote TeraBox server
// Implements fs.Fs

type Fs struct {
	name       string       // name of this remote
	root       string       // the path we are working on
	opt        Options      // parsed options
	features   *fs.Features // optional features
	httpClient *http.Client
}

// Implement fs.Info interface
func (f *Fs) Name() string   { return f.name }
func (f *Fs) Root() string   { return f.root }
func (f *Fs) String() string { return "TeraBox root '" + f.root + "'" }

// Object describes a TeraBox file object
// Implements fs.Object

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
		return nil, fmt.Errorf("TeraBox requires JSToken, BDSToken, and Cookie to be set")
	}

	// Create HTTP client with proper timeouts
	timeout := time.Duration(opt.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second // Default 60 seconds
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	fs := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		httpClient: httpClient,
	}

	// Check authentication status
	if err := fs.checkAuthStatus(ctx); err != nil {
		return nil, fmt.Errorf("TeraBox authentication failed: %w", err)
	}

	return fs, nil
}

// retryWithBackoff executes a function with exponential backoff retry logic
func (f *Fs) retryWithBackoff(ctx context.Context, operation string, fn func() error) error {
	maxRetries := f.opt.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Default
	}

	retryDelay := f.opt.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second // Default
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying with exponential backoff
			backoffDelay := retryDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDelay):
			}
		}

		if err := fn(); err != nil {
			lastErr = err
			// Check if error is retryable
			if !isRetryableError(err) {
				return err
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("%s failed after %d attempts: %w", operation, maxRetries+1, lastErr)
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a retry error from fserrors
	if fserrors.IsRetryError(err) {
		return true
	}

	// Network errors, timeouts, and rate limiting are retryable
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "errno=-9") ||
		strings.Contains(errStr, "errno=12")
}

// batchDeleteFiles deletes multiple files in batches to reduce API calls
func (f *Fs) batchDeleteFiles(ctx context.Context, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	batchSize := f.opt.BatchDeleteSize
	if batchSize <= 0 {
		batchSize = 10 // Default
	}

	deleteDelay := f.opt.DeleteDelay
	if deleteDelay <= 0 {
		deleteDelay = 1 * time.Second // Default
	}

	for i := 0; i < len(fileIDs); i += batchSize {
		end := i + batchSize
		if end > len(fileIDs) {
			end = len(fileIDs)
		}

		batch := fileIDs[i:end]

		// Use retry logic for each batch
		if err := f.retryWithBackoff(ctx, fmt.Sprintf("DeleteBatch-%d-%d", i, end-1), func() error {
			return f.deleteBatch(ctx, batch)
		}); err != nil {
			return fmt.Errorf("batch delete failed for batch %d-%d: %w", i, end-1, err)
		}

		// Add delay between batches to avoid rate limiting
		if end < len(fileIDs) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(deleteDelay):
			}
		}
	}
	return nil
}

// deleteBatch deletes a batch of files using their fs_ids
func (f *Fs) deleteBatch(ctx context.Context, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	// Build the filelist JSON array
	var fileList []string
	for _, fsID := range fileIDs {
		fileList = append(fileList, fmt.Sprintf(`{"fs_id":%d}`, fsID))
	}
	fileListJSON := "[" + strings.Join(fileList, ",") + "]"

	removeData := url.Values{}
	removeData.Set("filelist", fileListJSON)

	removeReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.terabox.com/api/filemanager?opera=delete&bdstoken="+f.opt.BDSToken+"&app_id=250528&jsToken="+f.opt.JSToken,
		strings.NewReader(removeData.Encode()))
	if err != nil {
		return err
	}

	removeReq.Header.Set("User-Agent", "okhttp/7.4")
	removeReq.Header.Set("Origin", "https://www.terabox.com")
	removeReq.Header.Set("Referer", "https://www.terabox.com/")
	removeReq.Header.Set("Cookie", f.opt.Cookie)
	removeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := f.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(removeReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("TeraBox API remove failed: %s", resp.Status)
	}

	// Check response for success
	var result struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If we can't decode JSON, check if it's a 200 status
		if resp.StatusCode == 200 {
			return nil
		}
		return fmt.Errorf("TeraBox API remove failed: %s", resp.Status)
	}

	// Handle specific TeraBox API error codes
	switch result.Errno {
	case 0:
		// Success
		return nil
	case 2:
		// File not found - this is actually OK for cleanup operations
		fmt.Printf("DEBUG: File not found during delete (errno=2), assuming already deleted\n")
		return nil
	case -9:
		// Network error or temporary issue - this might be retryable
		return fserrors.RetryError(fmt.Errorf("TeraBox API network error (errno=-9)"))
	case 12:
		// Permission denied or file in use - might be retryable
		return fserrors.RetryError(fmt.Errorf("TeraBox API permission error (errno=12)"))
	default:
		return fmt.Errorf("TeraBox API remove error: errno=%d", result.Errno)
	}
}

// calculateMD5Streaming calculates MD5 hash of a reader without loading entire file into memory
func calculateMD5Streaming(reader io.Reader) (string, error) {
	hash := md5.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// checkAuthStatus verifies that the authentication tokens are valid
func (f *Fs) checkAuthStatus(ctx context.Context) error {
	query := map[string]string{
		"checkexpire": "1",
		"checkfree":   "1",
		"app_id":      "250528",
		"jsToken":     f.opt.JSToken,
	}
	resp, err := f.apiRequest(ctx, "GET", "/api/quota", query, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode quota response: %w", err)
	}

	if result.Errno == 4000023 {
		return fmt.Errorf("TeraBox authentication error (errno=4000023): Verification required. Please update your TeraBox authentication tokens (JSTOKEN, BDSTOKEN, and COOKIE)")
	}

	if result.Errno != 0 {
		return fmt.Errorf("TeraBox API error: errno=%d", result.Errno)
	}

	return nil
}

// Helper to create an authorized HTTP request with all required headers
func (f *Fs) apiRequest(ctx context.Context, method, endpoint string, query map[string]string, body []byte) (*http.Response, error) {
	baseURL := "https://www.terabox.com"
	url := baseURL + endpoint
	if len(query) > 0 {
		q := "?"
		for k, v := range query {
			q += k + "=" + v + "&"
		}
		url += q[:len(q)-1]
	}
	fmt.Printf("DEBUG: API Request - %s %s\n", method, url)
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}
	// Set required headers
	req.Header.Set("User-Agent", "okhttp/7.4")
	req.Header.Set("Origin", baseURL)
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Cookie", f.opt.Cookie)
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := f.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	return client.Do(req)
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	// Handle root directory case - when dir is empty, list the root
	apiDir := dir
	if dir == "" {
		apiDir = "/" // Use "/" for root directory in TeraBox API
	}

	encodedDir := url.QueryEscape(apiDir)
	fmt.Printf("DEBUG: List called with dir='%s', apiDir='%s', encoded='%s'\n", dir, apiDir, encodedDir)

	query := map[string]string{
		"dir":      encodedDir, // URL encode the directory path like the Bash script
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}
	resp, err := f.apiRequest(ctx, "GET", "/api/list", query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TeraBox API list failed: %s", resp.Status)
	}

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
	for i, item := range listResp.List {
		fmt.Printf("DEBUG: API Entry %d - Filename: '%s', Isdir: %d, Path: '%s'\n",
			i, item.Filename, item.Isdir, item.Path)
	}

	if listResp.Errno != 0 {
		// Handle specific error codes
		switch listResp.Errno {
		case 2:
			// Directory not found - return empty list
			fmt.Printf("DEBUG: Directory not found during list (errno=2), returning empty list\n")
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
		// Handle entries with empty Filename but non-empty Path
		var name string
		if item.Filename == "" && item.Path != "" {
			// Extract the name from the path
			name = filepath.Base(item.Path)
			// Skip if it's the root directory itself
			if name == "" || name == "/" {
				continue
			}
		} else if item.Filename == "" {
			// Skip entries with both empty Filename and Path
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
				// Extract the relative path by removing the root prefix
				remotePath = strings.TrimPrefix(item.Path, "/")
			} else {
				remotePath = name
			}

			// Use server_mtime if available, otherwise use local_mtime
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
	// Skip if it's the root directory
	if dir == "/" || dir == "." {
		return nil
	}

	// First check if directory already exists by trying to list it
	query := map[string]string{
		"dir":      dir,
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}
	resp, err := f.apiRequest(ctx, "GET", "/api/list", query, nil)
	if err == nil && resp.StatusCode == 200 {
		// Directory exists, no need to create it
		resp.Body.Close()
		return nil
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Directory doesn't exist, create it
	mkdirData := url.Values{}
	mkdirData.Set("path", dir)
	mkdirData.Set("isdir", "1")
	mkdirData.Set("rtype", "1")

	mkdirReq, err := http.NewRequestWithContext(ctx, "POST", "https://www.terabox.com/api/create?bdstoken="+f.opt.BDSToken+"&app_id=250528&jsToken="+f.opt.JSToken, strings.NewReader(mkdirData.Encode()))
	if err != nil {
		return err
	}
	mkdirReq.Header.Set("User-Agent", "okhttp/7.4")
	mkdirReq.Header.Set("Origin", "https://www.terabox.com")
	mkdirReq.Header.Set("Referer", "https://www.terabox.com/")
	mkdirReq.Header.Set("Cookie", f.opt.Cookie)
	mkdirReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := f.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err = client.Do(mkdirReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check response for success
	var result struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If we can't decode JSON, check if it's a 200 status
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

// Remove deletes a file
func (f *Fs) Remove(ctx context.Context, path string) error {
	return f.retryWithBackoff(ctx, "Remove", func() error {
		return f.removeSingle(ctx, path)
	})
}

// removeSingle performs the actual removal of a single file
func (f *Fs) removeSingle(ctx context.Context, path string) error {
	// Construct full path like other methods
	fullPath := path
	if !strings.HasPrefix(fullPath, "/") {
		fullPath = "/" + fullPath
	}

	// First, try to get the file's fs_id for more reliable deletion
	parentDir := filepath.Dir(fullPath)
	if parentDir == "/" {
		parentDir = ""
	}

	listQuery := map[string]string{
		"dir":      url.QueryEscape(parentDir),
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}

	resp, err := f.apiRequest(ctx, "GET", "/api/list", listQuery, nil)
	if err != nil {
		return fmt.Errorf("failed to list directory for deletion: %w", err)
	}
	defer resp.Body.Close()

	var listResp struct {
		List []struct {
			Path     string `json:"path"`
			FsID     int64  `json:"fs_id"`
			Isdir    int    `json:"isdir"`
			Filename string `json:"server_filename"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return fmt.Errorf("failed to decode list response: %w", err)
	}

	if listResp.Errno != 0 {
		// Handle specific error codes
		switch listResp.Errno {
		case 2:
			// Directory not found - this means the file doesn't exist, which is OK for deletion
			fmt.Printf("DEBUG: Directory not found during remove (errno=2), assuming file already deleted\n")
			return nil
		case -9:
			// Network error - retryable
			return fserrors.RetryError(fmt.Errorf("list API network error (errno=-9)"))
		default:
			return fmt.Errorf("list API error: errno=%d", listResp.Errno)
		}
	}

	// Find the file to get its fs_id
	var fsID int64
	for _, item := range listResp.List {
		if item.Path == fullPath && item.Isdir == 0 {
			fsID = item.FsID
			break
		}
	}

	if fsID == 0 {
		// File not found, consider it already deleted
		return nil
	}

	// Use batch delete for single file (more efficient)
	return f.deleteBatch(ctx, []int64{fsID})
}

// RemoveDir deletes a directory
func (f *Fs) RemoveDir(ctx context.Context, dir string) error {
	return f.Remove(ctx, dir)
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.RemoveDir(ctx, dir)
}

// Put uploads a file (handles both small and chunked uploads)
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if in == nil {
		return nil, fmt.Errorf("input reader is nil")
	}

	remote := src.Remote()
	fileSize := src.Size()

	// Ensure parent directory exists (like Bash script's ensure_remote_dir)
	fullPath := f.root
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += remote

	// Check if file already exists and compare metadata
	existingObj, err := f.NewObject(ctx, remote)
	if err == nil {
		// File exists, check if it needs to be updated
		existingSize := existingObj.Size()
		existingModTime := existingObj.ModTime(ctx)
		srcModTime := src.ModTime(ctx)

		// If size and modification time match, skip upload
		if existingSize == fileSize && existingModTime.Equal(srcModTime) {
			fmt.Printf("DEBUG: File %s unchanged (size: %d, modTime: %s), skipping upload\n",
				remote, fileSize, srcModTime.Format(time.RFC3339))
			return existingObj, nil
		}

		fmt.Printf("DEBUG: File %s changed (existing: size=%d, modTime=%s; new: size=%d, modTime=%s), uploading\n",
			remote, existingSize, existingModTime.Format(time.RFC3339), fileSize, srcModTime.Format(time.RFC3339))

		// Clean up existing files and timestamped versions
		if err := f.cleanupExistingFiles(ctx, remote, fullPath); err != nil {
			// Always fail the upload if cleanup fails to prevent duplicates
			return nil, fmt.Errorf("failed to cleanup existing files for %s: %v (upload aborted to prevent duplicates)", remote, err)
		}
	} else if err == fs.ErrorObjectNotFound {
		// File doesn't exist, proceed with upload
		fmt.Printf("DEBUG: File %s does not exist, uploading\n", remote)
	} else if fserrors.IsRetryError(err) {
		// Retryable error (like network issues), log it but continue with upload
		fmt.Printf("DEBUG: Retryable error checking existing file %s: %v, proceeding with upload\n", remote, err)
	} else {
		// Some other error occurred, log it but continue with upload
		fmt.Printf("DEBUG: Error checking existing file %s: %v, proceeding with upload\n", remote, err)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if parentDir != "/" && parentDir != "." {
		if err := f.Mkdir(ctx, parentDir); err != nil {
			return nil, fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
		}
	}

	// Determine upload strategy based on file size
	streamingThreshold := f.opt.StreamingThreshold
	if streamingThreshold <= 0 {
		streamingThreshold = 10 << 20 // Default 10MB
	}

	if fileSize > int64(streamingThreshold) {
		return f.putStreaming(ctx, in, src, fullPath, remote)
	} else {
		return f.putBuffered(ctx, in, src, fullPath, remote)
	}
}

// cleanupExistingFiles removes existing files and timestamped versions
func (f *Fs) cleanupExistingFiles(ctx context.Context, remote, fullPath string) error {
	// Use more aggressive retries for cleanup to prevent duplicates
	maxRetries := 8 // Increased retries for cleanup
	if f.opt.MaxRetries > 0 && f.opt.MaxRetries > maxRetries {
		maxRetries = f.opt.MaxRetries
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Find all files (original and timestamped versions) to delete
		allFileIDs, err := f.findAllFileVersions(ctx, remote, fullPath)
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to find file versions: %w", err)
			}
			// Retry for network errors
			if fserrors.IsRetryError(err) {
				time.Sleep(f.opt.RetryDelay)
				continue
			}
			return fmt.Errorf("failed to find file versions: %w", err)
		}

		if len(allFileIDs) > 0 {
			fmt.Printf("DEBUG: Found %d file versions to delete (including timestamped)\n", len(allFileIDs))
			if err := f.batchDeleteFiles(ctx, allFileIDs); err != nil {
				if attempt == maxRetries-1 {
					// On final attempt, always fail to prevent duplicates
					return fmt.Errorf("failed to delete file versions after %d attempts: %w", maxRetries, err)
				}

				// For permission errors, use progressive backoff
				if strings.Contains(err.Error(), "errno=12") || strings.Contains(err.Error(), "permission error") {
					waitTime := time.Duration(attempt+1) * 5 * time.Second // Progressive wait: 5s, 10s, 15s...
					fmt.Printf("DEBUG: Permission error during cleanup (attempt %d/%d), waiting %v before retry\n", attempt+1, maxRetries, waitTime)
					time.Sleep(waitTime)
				} else {
					time.Sleep(f.opt.RetryDelay)
				}
				continue
			}
			fmt.Printf("DEBUG: Successfully deleted %d file versions\n", len(allFileIDs))

			// Add a longer delay to ensure deletion is processed
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}

		return nil
	}

	return fmt.Errorf("cleanup failed after %d attempts", maxRetries)
}

// findAllFileVersions finds both the original file and any timestamped versions
func (f *Fs) findAllFileVersions(ctx context.Context, remote, fullPath string) ([]int64, error) {
	// Extract base filename without extension
	baseName := filepath.Base(remote)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	// List the directory to find all versions
	parentDir := filepath.Dir(fullPath)
	if parentDir == "/" {
		parentDir = ""
	}

	listQuery := map[string]string{
		"dir":      url.QueryEscape(parentDir),
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}

	resp, err := f.apiRequest(ctx, "GET", "/api/list", listQuery, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var listResult struct {
		List []struct {
			Path     string `json:"path"`
			FsID     int64  `json:"fs_id"`
			Isdir    int    `json:"isdir"`
			Filename string `json:"server_filename"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		return nil, err
	}

	if listResult.Errno != 0 {
		// Handle specific error codes
		switch listResult.Errno {
		case 2:
			// Directory not found - this is OK for cleanup operations
			fmt.Printf("DEBUG: Directory not found during cleanup (errno=2), assuming no files exist\n")
			return []int64{}, nil
		case -9:
			// Network error - retryable
			return nil, fserrors.RetryError(fmt.Errorf("list API network error (errno=-9)"))
		default:
			return nil, fmt.Errorf("list API error: errno=%d", listResult.Errno)
		}
	}

	var fileIDs []int64
	for _, item := range listResult.List {
		if item.Isdir == 0 {
			itemBaseName := filepath.Base(item.Filename)
			itemExt := filepath.Ext(itemBaseName)
			itemNameWithoutExt := strings.TrimSuffix(itemBaseName, itemExt)

			// Check if this is the original file or a timestamped version
			if (itemNameWithoutExt == nameWithoutExt && itemExt == ext) ||
				(strings.HasPrefix(itemNameWithoutExt, nameWithoutExt+"_") && itemExt == ext) {
				// For timestamped versions, verify the timestamp format
				if strings.HasPrefix(itemNameWithoutExt, nameWithoutExt+"_") {
					timestampPart := strings.TrimPrefix(itemNameWithoutExt, nameWithoutExt+"_")
					if len(timestampPart) == 15 && strings.Contains(timestampPart, "_") {
						// Valid timestamped version
						fileIDs = append(fileIDs, item.FsID)
						fmt.Printf("DEBUG: Found timestamped version to delete: %s (fs_id: %d)\n", item.Filename, item.FsID)
					}
				} else {
					// Original file
					fileIDs = append(fileIDs, item.FsID)
					fmt.Printf("DEBUG: Found original file to delete: %s (fs_id: %d)\n", item.Filename, item.FsID)
				}
			}
		}
	}

	return fileIDs, nil
}

// putBuffered uploads a file by reading it into memory first (for smaller files)
func (f *Fs) putBuffered(ctx context.Context, in io.Reader, src fs.ObjectInfo, fullPath, remote string) (fs.Object, error) {
	// Read the file into memory
	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, err
	}
	fileSize := int64(len(data))
	fileMD5 := md5.Sum(data)
	fileMD5Hex := hex.EncodeToString(fileMD5[:])

	return f.uploadFile(ctx, bytes.NewReader(data), fullPath, remote, fileSize, fileMD5Hex)
}

// putStreaming uploads a file using streaming (for larger files)
func (f *Fs) putStreaming(ctx context.Context, in io.Reader, src fs.ObjectInfo, fullPath, remote string) (fs.Object, error) {
	fileSize := src.Size()

	// Create a temporary file for streaming
	tmpFile, err := ioutil.TempFile("", "terabox_upload_*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Copy data to temp file and calculate MD5
	hash := md5.New()
	teeReader := io.TeeReader(in, hash)

	if _, err := io.Copy(tmpFile, teeReader); err != nil {
		return nil, fmt.Errorf("failed to copy data to temp file: %w", err)
	}

	fileMD5Hex := hex.EncodeToString(hash.Sum(nil))

	// Reset temp file for reading
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	return f.uploadFile(ctx, tmpFile, fullPath, remote, fileSize, fileMD5Hex)
}

// uploadFile performs the actual upload using TeraBox API
func (f *Fs) uploadFile(ctx context.Context, reader io.Reader, fullPath, remote string, fileSize int64, fileMD5Hex string) (fs.Object, error) {
	userAgent := "okhttp/7.4"

	// 1. Precreate to get upload ID
	precreateData := url.Values{}
	precreateData.Set("path", fullPath)
	precreateData.Set("size", fmt.Sprintf("%d", fileSize))
	precreateData.Set("isdir", "0")
	precreateData.Set("block_list", "[\""+fileMD5Hex+"\"]")
	precreateData.Set("autoinit", "1")

	precreateReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.terabox.com/api/precreate?app_id=250528&jsToken="+f.opt.JSToken,
		strings.NewReader(precreateData.Encode()))
	if err != nil {
		return nil, err
	}

	precreateReq.Header.Set("User-Agent", userAgent)
	precreateReq.Header.Set("Origin", "https://www.terabox.com")
	precreateReq.Header.Set("Referer", "https://www.terabox.com/")
	precreateReq.Header.Set("Cookie", f.opt.Cookie)
	precreateReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := f.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	precreateResp, err := client.Do(precreateReq)
	if err != nil {
		return nil, fmt.Errorf("precreate failed: %w", err)
	}
	defer precreateResp.Body.Close()

	var precreateResult struct {
		UploadID string `json:"uploadid"`
		Errno    int    `json:"errno"`
	}
	if err := json.NewDecoder(precreateResp.Body).Decode(&precreateResult); err != nil {
		return nil, fmt.Errorf("precreate decode failed: %w", err)
	}
	if precreateResult.Errno != 0 {
		if precreateResult.Errno == 4000023 {
			return nil, fmt.Errorf("TeraBox authentication error (errno=4000023): Verification required. Please update your TeraBox authentication tokens (JSTOKEN, BDSTOKEN, and COOKIE)")
		}
		return nil, fmt.Errorf("precreate API error: errno=%d", precreateResult.Errno)
	}

	// 2. Upload file using streaming
	uploadURL := "https://c-jp.terabox.com/rest/2.0/pcs/superfile2?method=upload&type=tmpfile&app_id=250528&path=" +
		url.QueryEscape(fullPath) + "&uploadid=" + precreateResult.UploadID + "&partseq=0"

	// Use retry logic for upload
	if err := f.retryWithBackoff(ctx, "Upload", func() error {
		uploadBody := &bytes.Buffer{}
		writer := multipart.NewWriter(uploadBody)
		part, err := writer.CreateFormFile("file", remote)
		if err != nil {
			return err
		}

		// Copy data to multipart form
		if _, err := io.Copy(part, reader); err != nil {
			return err
		}
		writer.Close()

		uploadReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, uploadBody)
		if err != nil {
			return err
		}

		uploadReq.Header.Set("User-Agent", userAgent)
		uploadReq.Header.Set("Origin", "https://www.terabox.com")
		uploadReq.Header.Set("Referer", "https://www.terabox.com/")
		uploadReq.Header.Set("Cookie", f.opt.Cookie)
		uploadReq.Header.Set("Content-Type", writer.FormDataContentType())

		uploadResp, err := client.Do(uploadReq)
		if err != nil {
			return err
		}
		defer uploadResp.Body.Close()

		if uploadResp.StatusCode != 200 {
			return fmt.Errorf("upload HTTP error: %s", uploadResp.Status)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// 3. Create file
	createData := url.Values{}
	createData.Set("path", fullPath)
	createData.Set("size", fmt.Sprintf("%d", fileSize))
	createData.Set("uploadid", precreateResult.UploadID)
	createData.Set("block_list", "[\""+fileMD5Hex+"\"]")

	createReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://www.terabox.com/api/create?app_id=250528&jsToken="+f.opt.JSToken,
		strings.NewReader(createData.Encode()))
	if err != nil {
		return nil, err
	}

	createReq.Header.Set("User-Agent", userAgent)
	createReq.Header.Set("Origin", "https://www.terabox.com")
	createReq.Header.Set("Referer", "https://www.terabox.com/")
	createReq.Header.Set("Cookie", f.opt.Cookie)
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	createResp, err := client.Do(createReq)
	if err != nil {
		return nil, fmt.Errorf("create failed: %w", err)
	}
	defer createResp.Body.Close()

	var createResult struct {
		Errno int `json:"errno"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createResult); err != nil {
		return nil, fmt.Errorf("create decode failed: %w", err)
	}
	if createResult.Errno != 0 {
		return nil, fmt.Errorf("create API error: errno=%d", createResult.Errno)
	}

	// Return a properly constructed Object with metadata
	return &Object{
		fs:      f,
		remote:  remote,
		size:    fileSize,
		modTime: time.Now(), // Use current time since we don't have src context here
		path:    fullPath,
	}, nil
}

// Get file metadata
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Construct full path
	fullPath := f.root
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += remote

	// Get file metadata from the list API
	parentDir := filepath.Dir(fullPath)
	if parentDir == "/" {
		parentDir = ""
	}

	listQuery := map[string]string{
		"dir":      url.QueryEscape(parentDir),
		"bdstoken": f.opt.BDSToken,
		"app_id":   "250528",
		"jsToken":  f.opt.JSToken,
	}

	resp, err := f.apiRequest(ctx, "GET", "/api/list", listQuery, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("TeraBox API error: %s", resp.Status)
	}

	var listResp struct {
		List []struct {
			Path        string `json:"path"`
			FsID        int64  `json:"fs_id"`
			Isdir       int    `json:"isdir"`
			Size        int64  `json:"size"`
			Filename    string `json:"server_filename"`
			ServerMtime int64  `json:"server_mtime"`
			LocalMtime  int64  `json:"local_mtime"`
		} `json:"list"`
		Errno int `json:"errno"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("failed to decode list response: %w", err)
	}

	if listResp.Errno != 0 {
		// Handle specific error codes
		switch listResp.Errno {
		case 2:
			// Directory not found - this means the file doesn't exist
			return nil, fs.ErrorObjectNotFound
		case -9:
			// Network error - retryable
			return nil, fserrors.RetryError(fmt.Errorf("list API network error (errno=-9)"))
		default:
			return nil, fmt.Errorf("list API error: errno=%d", listResp.Errno)
		}
	}

	// Find the file in the list
	// First try exact match
	for _, item := range listResp.List {
		if item.Path == fullPath && item.Isdir == 0 {
			// Use server_mtime if available, otherwise use local_mtime
			mtime := item.ServerMtime
			if mtime == 0 {
				mtime = item.LocalMtime
			}

			return &Object{
				fs:      f,
				remote:  remote,
				size:    item.Size,
				modTime: time.Unix(mtime, 0),
				fsID:    item.FsID,
				path:    item.Path,
			}, nil
		}
	}

	// If exact match not found, look for timestamped versions
	// Extract base filename without extension
	baseName := filepath.Base(remote)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	// Look for files that match the pattern: nameWithoutExt_YYYYMMDD_HHMMSS.ext
	var latestItem *struct {
		Path        string `json:"path"`
		FsID        int64  `json:"fs_id"`
		Isdir       int    `json:"isdir"`
		Size        int64  `json:"size"`
		Filename    string `json:"server_filename"`
		ServerMtime int64  `json:"server_mtime"`
		LocalMtime  int64  `json:"local_mtime"`
	}
	var latestMtime int64

	for _, item := range listResp.List {
		if item.Isdir == 0 {
			itemBaseName := filepath.Base(item.Filename)
			itemExt := filepath.Ext(itemBaseName)
			itemNameWithoutExt := strings.TrimSuffix(itemBaseName, itemExt)

			// Check if this is a timestamped version of our file
			if strings.HasPrefix(itemNameWithoutExt, nameWithoutExt+"_") && itemExt == ext {
				// Extract timestamp from filename (format: YYYYMMDD_HHMMSS)
				timestampPart := strings.TrimPrefix(itemNameWithoutExt, nameWithoutExt+"_")
				if len(timestampPart) == 15 && strings.Contains(timestampPart, "_") {
					// This looks like a timestamped version, check if it's the latest
					mtime := item.ServerMtime
					if mtime == 0 {
						mtime = item.LocalMtime
					}

					if latestItem == nil || mtime > latestMtime {
						latestItem = &item
						latestMtime = mtime
					}
				}
			}
		}
	}

	if latestItem != nil {
		return &Object{
			fs:      f,
			remote:  remote, // Return the original remote name, not the timestamped one
			size:    latestItem.Size,
			modTime: time.Unix(latestMtime, 0),
			fsID:    latestItem.FsID,
			path:    latestItem.Path,
		}, nil
	}

	return nil, fs.ErrorObjectNotFound
}

// Download file contents
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Construct full path
	fullPath := o.fs.root
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += o.remote

	// Retry logic for file availability
	maxRetries := 5
	retryDelay := 2 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retrying
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		// First, get the file metadata from the list API to get the fs_id
		parentDir := filepath.Dir(fullPath)
		if parentDir == "/" {
			parentDir = ""
		}

		listQuery := map[string]string{
			"dir":      url.QueryEscape(parentDir),
			"bdstoken": o.fs.opt.BDSToken,
			"app_id":   "250528",
			"jsToken":  o.fs.opt.JSToken,
		}

		fmt.Printf("DEBUG: Download - getting file metadata from list API, dir: %s\n", parentDir)
		resp, err := o.fs.apiRequest(ctx, "GET", "/api/list", listQuery, nil)
		if err != nil {
			if attempt == maxRetries-1 {
				return nil, err
			}
			continue
		}

		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("TeraBox API request failed: %s", resp.Status)
			}
			continue
		}

		var listResp struct {
			List []struct {
				Path     string `json:"path"`
				FsID     int64  `json:"fs_id"`
				Isdir    int    `json:"isdir"`
				Size     int64  `json:"size"`
				Filename string `json:"server_filename"`
			} `json:"list"`
			Errno int `json:"errno"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("list decode failed: %w", err)
			}
			continue
		}

		if listResp.Errno != 0 {
			// Handle specific error codes
			switch listResp.Errno {
			case 2:
				// Directory not found - this means the file doesn't exist
				if attempt == maxRetries-1 {
					return nil, fs.ErrorObjectNotFound
				}
				continue
			case -9:
				// Network error - retryable
				if attempt == maxRetries-1 {
					return nil, fserrors.RetryError(fmt.Errorf("list API network error (errno=-9)"))
				}
				continue
			default:
				if attempt == maxRetries-1 {
					return nil, fmt.Errorf("list API error: errno=%d", listResp.Errno)
				}
				continue
			}
		}

		// Find the file in the list to get its fs_id
		var fsID int64
		fmt.Printf("DEBUG: Download - looking for file: %s in list response with %d entries\n", fullPath, len(listResp.List))
		for i, item := range listResp.List {
			fmt.Printf("DEBUG: Download - Entry %d: Path='%s', Filename='%s', Isdir=%d, FsID=%d\n",
				i, item.Path, item.Filename, item.Isdir, item.FsID)
			if item.Path == fullPath && item.Isdir == 0 {
				fsID = item.FsID
				fmt.Printf("DEBUG: Download - Found matching file, fs_id: %d\n", fsID)
				break
			}
		}

		if fsID == 0 {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("file not found in list response")
			}
			continue
		}

		// Try to get download link using the file ID
		// Note: TeraBox API doesn't seem to provide direct download links through standard endpoints
		// This is a limitation of the TeraBox API - we'll return an error explaining this
		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("TeraBox API limitation: direct download links are not available through the standard API endpoints. This is a known limitation of the TeraBox API. Consider using the web interface or external tools for downloads")
		}

		// Try alternative approaches (these are likely to fail but we'll try them)
		// 1. Try download API with fs_id
		downloadQuery := map[string]string{
			"fs_id":    fmt.Sprintf("%d", fsID),
			"bdstoken": o.fs.opt.BDSToken,
			"app_id":   "250528",
			"jsToken":  o.fs.opt.JSToken,
		}

		fmt.Printf("DEBUG: Download - trying download API with fs_id: %d\n", fsID)
		downloadResp, err := o.fs.apiRequest(ctx, "GET", "/api/download", downloadQuery, nil)
		if err != nil {
			continue
		}
		downloadResp.Body.Close()

		// 2. Try filemetas API with fs_id
		filemetasQuery := map[string]string{
			"fs_id":    fmt.Sprintf("%d", fsID),
			"bdstoken": o.fs.opt.BDSToken,
			"app_id":   "250528",
			"jsToken":  o.fs.opt.JSToken,
		}

		fmt.Printf("DEBUG: Download - trying filemetas API with fs_id: %d\n", fsID)
		filemetasResp, err := o.fs.apiRequest(ctx, "GET", "/api/filemetas", filemetasQuery, nil)
		if err != nil {
			continue
		}
		filemetasResp.Body.Close()
	}

	// If we get here, all attempts failed
	return nil, fmt.Errorf("TeraBox API limitation: direct download links are not available through the standard API endpoints. This is a known limitation of the TeraBox API. Consider using the web interface or external tools for downloads")
}

// Update file contents
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// For TeraBox, this may be the same as Put
	_, err := o.fs.Put(ctx, in, src, options...)
	return err
}

// Delete this file
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.Remove(ctx, o.remote)
}

// Hash returns the selected checksum of the object
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	// TeraBox API may not provide hashes
	return "", hash.ErrUnsupported
}

// SetModTime sets the modification time of the object
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	// TeraBox API may not support this
	return fs.ErrorNotImplemented
}

// Precision returns the precision of the mod times in this Fs
func (f *Fs) Precision() time.Duration {
	// TODO: Return the correct precision
	return time.Second
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	// TODO: Return supported hashes
	return hash.NewHashSet()
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Implement required methods for Object (stubs)

// Storable returns whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// String returns a description of the object
func (o *Object) String() string {
	return o.remote
}
