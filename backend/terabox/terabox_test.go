package terabox

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

func newTestFs(serverURL string) *Fs {
	return &Fs{
		name: "terabox",
		root: "/",
		opt: Options{
			JSToken:  "8C20525FF4861BC6348209DE4A3281B2343BE61EE527D9997F15F5FD2FDED1C500F85DB45374D94F66503553C56A15047D1E0B60286EE83B90EC687A678B82A2",
			BDSToken: "6024c5b8db44d52f1f6b2f7651872a89",
			Cookie:   `browserid=y8itB1KvnGfGCqbKMWBok52kNIIjaR0vrlY6ccwcItnhIb4BPQbD1tD-R78=; lang=en; __bid_n=196da4a17986ed82754207; __stripe_mid=ce350eb2-75db-48b3-9e19-290e9955256d917d11; _ga=GA1.1.955667431.1747419019; g_state={"i_l":0}; ndus=Ybi6jg7teHuiqtz_4sn0Mw1a1Bj-fXd7gJ67dcfb; _clck=1kb3qnh%7C2%7Cfvy%7C0%7C1962; _ga_VRPKGB01VJ=GS2.1.s1747431936$o1$g1$t1747432622$j0$l0$h0; _gcl_au=1.1.1079017173.1751312284; _ga_06ZNKL8C2E=GS2.1.s1751849926$o9$g1$t1751849951$j35$l0$h0; csrfToken=vRw8_9vAzK1Ldoj8MxHQOqWe; __stripe_sid=6a2ea91c-34a1-4d33-9489-78c88ae16f70feb790; _ga_HSVH9T016H=GS2.1.s1752105365$o21$g0$t1752105375$j50$l0$h0; ndut_fmt=810D079708EF959FB70FD764F014A377B9C0621966ABE9633ECFEA9DC5FC9DA3`,
		},
		features: nil,
	}
}

func TestTeraboxBasic(t *testing.T) {
	t.Log("TeraBox backend basic test placeholder")
}

func TestTeraboxList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"list":[],"errno":0}`))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	_, err := fsys.List(context.Background(), ".")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTeraboxPut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	_, err := fsys.Put(context.Background(), strings.NewReader("test"), &ObjectInfoMock{remote: "test.txt", size: 4})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTeraboxMkdir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	if err := fsys.Mkdir(context.Background(), "/testdir"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTeraboxRemove(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	if err := fsys.Remove(context.Background(), "/testfile"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTeraboxRemoveDir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	if err := fsys.RemoveDir(context.Background(), "/testdir"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTeraboxNewObject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"name":"testfile","size":123,"mod_time":1234567890}`))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	obj, err := fsys.NewObject(context.Background(), "testfile")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if obj.Remote() != "testfile" {
		t.Errorf("expected remote 'testfile', got %v", obj.Remote())
	}
}

func TestTeraboxObjectOpen(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("file contents"))
	}))
	defer ts.Close()
	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	obj := &Object{fs: fsys, remote: "testfile"}
	rc, err := obj.Open(context.Background())
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 20)
	n, _ := rc.Read(buf)
	if string(buf[:n]) != "file contents" {
		t.Errorf("expected 'file contents', got %q", string(buf[:n]))
	}
}

func TestTeraboxObjectUpdate(t *testing.T) {
	o := &Object{}
	err := o.Update(context.Background(), nil, nil)
	if err != fs.ErrorNotImplemented {
		t.Errorf("expected ErrorNotImplemented, got %v", err)
	}
}

func TestTeraboxObjectRemove(t *testing.T) {
	o := &Object{}
	err := o.Remove(context.Background())
	if err != fs.ErrorNotImplemented {
		t.Errorf("expected ErrorNotImplemented, got %v", err)
	}
}

func TestTeraboxObjectHash(t *testing.T) {
	o := &Object{}
	_, err := o.Hash(context.Background(), hash.MD5)
	if err != fs.ErrorNotImplemented {
		t.Errorf("expected ErrorNotImplemented, got %v", err)
	}
}

func TestTeraboxObjectSetModTime(t *testing.T) {
	o := &Object{}
	err := o.SetModTime(context.Background(), time.Now())
	if err != fs.ErrorNotImplemented {
		t.Errorf("expected ErrorNotImplemented, got %v", err)
	}
}

// TestRetryWithBackoff tests the retry logic
func TestRetryWithBackoff(t *testing.T) {
	fsys := &Fs{
		opt: Options{
			MaxRetries: 2,
			RetryDelay: 10 * time.Millisecond,
		},
	}

	ctx := context.Background()

	// Test successful operation
	attempts := 0
	err := fsys.retryWithBackoff(ctx, "test", func() error {
		attempts++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}

	// Test retryable error
	attempts = 0
	err = fsys.retryWithBackoff(ctx, "test", func() error {
		attempts++
		return fmt.Errorf("timeout error")
	})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if attempts != 3 { // MaxRetries + 1
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Test non-retryable error
	attempts = 0
	err = fsys.retryWithBackoff(ctx, "test", func() error {
		attempts++
		return fmt.Errorf("permanent error")
	})
	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

// TestIsRetryableError tests the retryable error detection
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"timeout error", fmt.Errorf("timeout"), true},
		{"connection error", fmt.Errorf("connection refused"), true},
		{"network error", fmt.Errorf("network unreachable"), true},
		{"rate limit error", fmt.Errorf("rate limit exceeded"), true},
		{"429 error", fmt.Errorf("429 too many requests"), true},
		{"503 error", fmt.Errorf("503 service unavailable"), true},
		{"502 error", fmt.Errorf("502 bad gateway"), true},
		{"permanent error", fmt.Errorf("file not found"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// TestBatchDeleteFiles tests batch deletion functionality
func TestBatchDeleteFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()
	fsys.opt.BatchDeleteSize = 3
	fsys.opt.DeleteDelay = 1 * time.Millisecond

	ctx := context.Background()

	// Test empty list
	if err := fsys.batchDeleteFiles(ctx, []int64{}); err != nil {
		t.Errorf("expected nil error for empty list, got %v", err)
	}

	// Test single batch
	if err := fsys.batchDeleteFiles(ctx, []int64{1, 2, 3}); err != nil {
		t.Errorf("expected nil error for single batch, got %v", err)
	}

	// Test multiple batches
	if err := fsys.batchDeleteFiles(ctx, []int64{1, 2, 3, 4, 5, 6, 7}); err != nil {
		t.Errorf("expected nil error for multiple batches, got %v", err)
	}
}

// TestCalculateMD5Streaming tests streaming MD5 calculation
func TestCalculateMD5Streaming(t *testing.T) {
	testData := []byte("test data for MD5 calculation")
	expectedMD5 := "d8e8fca2dc0f896fd7cb4cb0031ba249" // Pre-calculated MD5

	reader := bytes.NewReader(testData)
	calculatedMD5, err := calculateMD5Streaming(reader)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	if calculatedMD5 != expectedMD5 {
		t.Errorf("expected MD5 %s, got %s", expectedMD5, calculatedMD5)
	}
}

// TestFindTimestampedVersions tests timestamped version detection
func TestFindTimestampedVersions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{
			"list": [
				{"path": "/test/foo.txt", "fs_id": 1, "isdir": 0, "server_filename": "foo.txt"},
				{"path": "/test/foo_20250707_042741.txt", "fs_id": 2, "isdir": 0, "server_filename": "foo_20250707_042741.txt"},
				{"path": "/test/foo_20250708_123456.txt", "fs_id": 3, "isdir": 0, "server_filename": "foo_20250708_123456.txt"},
				{"path": "/test/bar.txt", "fs_id": 4, "isdir": 0, "server_filename": "bar.txt"},
				{"path": "/test/dir", "fs_id": 5, "isdir": 1, "server_filename": "dir"}
			],
			"errno": 0
		}`))
	}))
	defer ts.Close()

	fsys := newTestFs(ts.URL)
	fsys.httpClient = ts.Client()

	ctx := context.Background()
	fileIDs, err := fsys.findAllFileVersions(ctx, "foo.txt", "/test/foo.txt")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	expectedIDs := []int64{2, 3}
	if len(fileIDs) != len(expectedIDs) {
		t.Errorf("expected %d file IDs, got %d", len(expectedIDs), len(fileIDs))
	}

	for i, expectedID := range expectedIDs {
		if i >= len(fileIDs) || fileIDs[i] != expectedID {
			t.Errorf("expected file ID %d at position %d, got %d", expectedID, i, fileIDs[i])
		}
	}
}

// ObjectInfoMock is a mock implementation of fs.ObjectInfo for testing
type ObjectInfoMock struct {
	remote string
	size   int64
}

func (o *ObjectInfoMock) Remote() string                                        { return o.remote }
func (o *ObjectInfoMock) ModTime(ctx context.Context) time.Time                 { return time.Now() }
func (o *ObjectInfoMock) Size() int64                                           { return o.size }
func (o *ObjectInfoMock) Fs() fs.Info                                           { return nil }
func (o *ObjectInfoMock) Hash(ctx context.Context, t hash.Type) (string, error) { return "", nil }
func (o *ObjectInfoMock) Storable() bool                                        { return true }
func (o *ObjectInfoMock) String() string                                        { return o.remote }
