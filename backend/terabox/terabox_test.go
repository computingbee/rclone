package terabox

import (
	"context"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config/configmap"
)

func TestNewFs(t *testing.T) {
	ctx := context.Background()

	// Test with valid config
	config := configmap.Simple{
		"jstoken":         "test_jstoken",
		"bdstoken":        "test_bdstoken",
		"cookie":          "test_cookie",
		"local_directory": "/test/local/dir",
		"timeout":         "60",
	}

	f, err := NewFs(ctx, "test", "/test", config)
	if err != nil {
		t.Fatalf("NewFs failed: %v", err)
	}

	fsys, ok := f.(*Fs)
	if !ok {
		t.Fatal("Not a TeraBox Fs")
	}

	if fsys.opt.JSToken != "test_jstoken" {
		t.Errorf("Expected JSToken 'test_jstoken', got '%s'", fsys.opt.JSToken)
	}

	if fsys.opt.BDSToken != "test_bdstoken" {
		t.Errorf("Expected BDSToken 'test_bdstoken', got '%s'", fsys.opt.BDSToken)
	}

	if fsys.opt.Cookie != "test_cookie" {
		t.Errorf("Expected Cookie 'test_cookie', got '%s'", fsys.opt.Cookie)
	}

	if fsys.opt.LocalDirectory != "/test/local/dir" {
		t.Errorf("Expected LocalDirectory '/test/local/dir', got '%s'", fsys.opt.LocalDirectory)
	}

	if fsys.opt.Timeout != 60 {
		t.Errorf("Expected Timeout 60, got %d", fsys.opt.Timeout)
	}
}

func TestNewFsMissingRequired(t *testing.T) {
	ctx := context.Background()

	// Test with missing jstoken
	config := configmap.Simple{
		"bdstoken":        "test_bdstoken",
		"cookie":          "test_cookie",
		"local_directory": "/test/local/dir",
	}

	_, err := NewFs(ctx, "test", "/test", config)
	if err == nil {
		t.Error("Expected error for missing jstoken")
	}

	// Test with missing bdstoken
	config = configmap.Simple{
		"jstoken":         "test_jstoken",
		"cookie":          "test_cookie",
		"local_directory": "/test/local/dir",
	}

	_, err = NewFs(ctx, "test", "/test", config)
	if err == nil {
		t.Error("Expected error for missing bdstoken")
	}

	// Test with missing cookie
	config = configmap.Simple{
		"jstoken":         "test_jstoken",
		"bdstoken":        "test_bdstoken",
		"local_directory": "/test/local/dir",
	}

	_, err = NewFs(ctx, "test", "/test", config)
	if err == nil {
		t.Error("Expected error for missing cookie")
	}

	// Test with missing local_directory
	config = configmap.Simple{
		"jstoken":  "test_jstoken",
		"bdstoken": "test_bdstoken",
		"cookie":   "test_cookie",
	}

	_, err = NewFs(ctx, "test", "/test", config)
	if err == nil {
		t.Error("Expected error for missing local_directory")
	}
}

func TestFsMethods(t *testing.T) {
	ctx := context.Background()

	config := configmap.Simple{
		"jstoken":         "test_jstoken",
		"bdstoken":        "test_bdstoken",
		"cookie":          "test_cookie",
		"local_directory": "/test/local/dir",
	}

	f, err := NewFs(ctx, "test", "/test", config)
	if err != nil {
		t.Fatalf("NewFs failed: %v", err)
	}

	fsys := f.(*Fs)

	// Test Name method
	if fsys.Name() != "test" {
		t.Errorf("Expected Name 'test', got '%s'", fsys.Name())
	}

	// Test Root method
	if fsys.Root() != "/test" {
		t.Errorf("Expected Root '/test', got '%s'", fsys.Root())
	}

	// Test String method
	expected := "TeraBox root '/test'"
	if fsys.String() != expected {
		t.Errorf("Expected String '%s', got '%s'", expected, fsys.String())
	}

	// Test Precision method
	if fsys.Precision() != time.Second {
		t.Errorf("Expected Precision time.Second, got %v", fsys.Precision())
	}

	// Test Hashes method
	hashes := fsys.Hashes()
	if hashes.Count() != 0 {
		t.Errorf("Expected empty hash set, got %d hashes", hashes.Count())
	}

	// Test Features method
	features := fsys.Features()
	if features == nil {
		t.Error("Expected non-nil features")
	}
}

func TestObjectMethods(t *testing.T) {
	ctx := context.Background()

	config := configmap.Simple{
		"jstoken":         "test_jstoken",
		"bdstoken":        "test_bdstoken",
		"cookie":          "test_cookie",
		"local_directory": "/test/local/dir",
	}

	f, err := NewFs(ctx, "test", "/test", config)
	if err != nil {
		t.Fatalf("NewFs failed: %v", err)
	}

	fsys := f.(*Fs)

	obj := &Object{
		fs:      fsys,
		remote:  "test.txt",
		size:    1024,
		modTime: time.Now(),
	}

	// Test Fs method
	if obj.Fs() != fsys {
		t.Error("Fs method returned wrong filesystem")
	}

	// Test Remote method
	if obj.Remote() != "test.txt" {
		t.Errorf("Expected Remote 'test.txt', got '%s'", obj.Remote())
	}

	// Test Size method
	if obj.Size() != 1024 {
		t.Errorf("Expected Size 1024, got %d", obj.Size())
	}

	// Test Storable method
	if !obj.Storable() {
		t.Error("Expected Storable to be true")
	}

	// Test String method
	expected := "test.txt"
	if obj.String() != expected {
		t.Errorf("Expected String '%s', got '%s'", expected, obj.String())
	}
}

func TestObjectInfoMethods(t *testing.T) {
	objInfo := &ObjectInfo{
		remote: "test.txt",
		size:   1024,
	}

	// Test Remote method
	if objInfo.Remote() != "test.txt" {
		t.Errorf("Expected Remote 'test.txt', got '%s'", objInfo.Remote())
	}

	// Test Size method
	if objInfo.Size() != 1024 {
		t.Errorf("Expected Size 1024, got %d", objInfo.Size())
	}

	// Test Storable method
	if !objInfo.Storable() {
		t.Error("Expected Storable to be true")
	}

	// Test String method
	expected := "test.txt"
	if objInfo.String() != expected {
		t.Errorf("Expected String '%s', got '%s'", expected, objInfo.String())
	}

	// Test Fs method
	if objInfo.Fs() != nil {
		t.Error("Expected Fs to return nil")
	}

	// Test Hash method
	ctx := context.Background()
	hash, err := objInfo.Hash(ctx, 0)
	if err != nil {
		t.Errorf("Hash method returned error: %v", err)
	}
	if hash != "" {
		t.Errorf("Expected empty hash, got '%s'", hash)
	}

	// Test MimeType method
	mimeType := objInfo.MimeType(ctx)
	if mimeType != "" {
		t.Errorf("Expected empty mime type, got '%s'", mimeType)
	}
}
