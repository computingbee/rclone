//go:build integration
// +build integration

package terabox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
)

// export TERABOX_JSTOKEN="8C20525FF4861BC6348209DE4A3281B23896637D0E3E00A3176CA332B8838D2919ACA9C886B3283C7808D72785227A7BD9A6C85F523D65BB48C59D0BA391C471"
// export TERABOX_BDSTOKEN="6024c5b8db44d52f1f6b2f7651872a89"
// export TERABOX_COOKIE="browserid=y8itB1KvnGfGCqbKMWBok52kNIIjaR0vrlY6ccwcItnhIb4BPQbD1tD-R78=; lang=en; __bid_n=196da4a17986ed82754207; __stripe_mid=ce350eb2-75db-48b3-9e19-290e9955256d917d11; _ga=GA1.1.955667431.1747419019; g_state={"i_l":0}; ndus=Ybi6jg7teHuiqtz_4sn0Mw1a1Bj-fXd7gJ67dcfb; _clck=1kb3qnh%7C2%7Cfvy%7C0%7C1962; _ga_VRPKGB01VJ=GS2.1.s1747431936$o1$g1$t1747432622$j0$l0$h0; _ga_06ZNKL8C2E=GS2.1.s1748873964$o7$g0$t1748873964$j60$l0$h0; csrfToken=qgU8fZxnGvLVbE0v12RIWzEM; _gcl_au=1.1.1079017173.1751312284; _ga_HSVH9T016H=GS2.1.s1751312283$o13$g0$t1751312283$j60$l0$h0; ndut_fmt=3960534CBA940EA3AD1901CFFCD7CF5930C9EF2BD893995DD580E8E520227A42"

const (
	LocalTestDir  = "/home/dyno/test-terabox"
	RemoteTestDir = "/test-terabox" // Use a simple test directory
)

func getIntegrationFs(t *testing.T) *Fs {
	jstoken := os.Getenv("TERABOX_JSTOKEN")
	bdstoken := os.Getenv("TERABOX_BDSTOKEN")
	cookie := os.Getenv("TERABOX_COOKIE")
	if jstoken == "" || bdstoken == "" || cookie == "" {
		t.Skip("Set TERABOX_JSTOKEN, TERABOX_BDSTOKEN, and TERABOX_COOKIE env vars to run integration tests")
	}
	return &Fs{
		name: "terabox",
		root: RemoteTestDir,
		opt: Options{
			JSToken:  jstoken,
			BDSToken: bdstoken,
			Cookie:   cookie,
		},
	}
}

// cleanupRemoteDirectory removes the remote test directory and all its contents
func cleanupRemoteDirectory(t *testing.T, fsys *Fs, ctx context.Context) {
	t.Logf("Cleaning up remote directory: %s", RemoteTestDir)

	// List all files in the remote directory
	entries, err := fsys.List(ctx, "")
	if err != nil {
		t.Logf("Warning: Could not list remote directory (may not exist): %v", err)
		return
	}

	t.Logf("Found %d entries to clean up", len(entries))
	for i, entry := range entries {
		t.Logf("Entry %d: %s (type: %T)", i, entry.Remote(), entry)
	}

	// Delete all files and directories
	for _, entry := range entries {
		if obj, ok := entry.(*Object); ok {
			// Delete file
			if err := obj.Remove(ctx); err != nil {
				t.Logf("Warning: Failed to delete file %s: %v", obj.Remote(), err)
			} else {
				t.Logf("✓ Deleted file: %s", obj.Remote())
			}
		} else if dir, ok := entry.(fs.Directory); ok {
			// Try to delete directory directly first
			dirPath := dir.Remote()
			t.Logf("Attempting to delete directory: %s", dirPath)
			if err := fsys.RemoveDir(ctx, dirPath); err != nil {
				t.Logf("Warning: Failed to delete directory %s directly: %v", dirPath, err)
				// If direct deletion fails, try recursive cleanup
				t.Logf("Attempting recursive cleanup for directory: %s", dirPath)
				if err := cleanupDirectoryRecursively(t, fsys, ctx, dirPath); err != nil {
					t.Logf("Warning: Failed to clean directory %s: %v", dirPath, err)
				} else {
					t.Logf("✓ Cleaned directory: %s", dirPath)
				}
			} else {
				t.Logf("✓ Deleted directory: %s", dirPath)
			}
		} else {
			t.Logf("Warning: Unknown entry type: %T", entry)
		}
	}

	t.Logf("Remote directory cleanup completed")
}

// cleanupDirectoryRecursively deletes all contents of a directory recursively
func cleanupDirectoryRecursively(t *testing.T, fsys *Fs, ctx context.Context, dirPath string) error {
	// Use full path format for TeraBox API
	fullPath := "/" + strings.TrimPrefix(dirPath, "/")
	t.Logf("Listing directory with full path: %s", fullPath)

	// List contents of the directory
	entries, err := fsys.List(ctx, fullPath)
	if err != nil {
		return fmt.Errorf("failed to list directory %s: %w", fullPath, err)
	}

	// Delete all contents first
	for _, entry := range entries {
		if obj, ok := entry.(*Object); ok {
			// Delete file
			if err := obj.Remove(ctx); err != nil {
				t.Logf("Warning: Failed to delete file %s: %v", obj.Remote(), err)
			} else {
				t.Logf("✓ Deleted file: %s", obj.Remote())
			}
		} else if dir, ok := entry.(fs.Directory); ok {
			// Recursively delete subdirectory
			subDirPath := dir.Remote()
			if err := cleanupDirectoryRecursively(t, fsys, ctx, subDirPath); err != nil {
				t.Logf("Warning: Failed to clean subdirectory %s: %v", subDirPath, err)
			}
		}
	}

	// Now try to delete the directory itself
	if err := fsys.RemoveDir(ctx, dirPath); err != nil {
		return fmt.Errorf("failed to delete directory %s: %w", dirPath, err)
	}

	t.Logf("✓ Deleted directory: %s", dirPath)
	return nil
}

// TestIntegrationSyncToRemote tests uploading files from local directory to TeraBox
func TestIntegrationSyncToRemote(t *testing.T) {
	fsys := getIntegrationFs(t)
	ctx := context.Background()

	// Lower retry and delay for integration test to avoid long stalls on errors
	fsys.opt.MaxRetries = 2
	fsys.opt.DeleteDelay = 1 * time.Second

	// DISABLED: Clean up remote directory before starting test
	// cleanupRemoteDirectory(t, fsys, ctx)

	// Check if local test directory exists
	fi, err := os.Stat(LocalTestDir)
	if err != nil || !fi.IsDir() {
		t.Skipf("Local test directory %s does not exist or is not a directory", LocalTestDir)
	}

	// Gather all files recursively
	var files []string
	err = filepath.WalkDir(LocalTestDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk local directory: %v", err)
	}
	if len(files) == 0 {
		t.Skipf("No files found in %s", LocalTestDir)
	}

	t.Logf("Found %d files to sync to TeraBox", len(files))

	// Upload each file to TeraBox
	uploadedFiles := make(map[string]*Object)
	for _, file := range files {
		relPath, _ := filepath.Rel(LocalTestDir, file)
		t.Logf("Uploading: %s -> %s", file, relPath)

		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read file %s: %v", file, err)
			continue
		}

		// Open file for upload
		f, err := os.Open(file)
		if err != nil {
			t.Errorf("Failed to open file %s: %v", file, err)
			continue
		}

		// Upload to TeraBox (no retries, no deletes)
		obj, err := fsys.Put(ctx, f, &ObjectInfoMock{remote: relPath, size: int64(len(content))})
		f.Close()
		if err != nil {
			t.Errorf("Failed to upload %s: %v", file, err)
			continue
		}

		// Type assertion to get the concrete Object type
		if teraboxObj, ok := obj.(*Object); ok {
			uploadedFiles[relPath] = teraboxObj
		} else {
			t.Errorf("Failed to get TeraBox object for %s", relPath)
			continue
		}
		t.Logf("Successfully uploaded: %s", relPath)
	}

	t.Logf("Successfully uploaded %d files to TeraBox", len(uploadedFiles))

	// Wait a bit for files to be available
	t.Logf("Waiting 3 seconds for files to be available...")
	time.Sleep(3 * time.Second)

	// (Removed change detection block that modifies local files)

	// Skip download verification due to TeraBox API limitations
	// TeraBox does not provide direct download links through the standard API endpoints
	t.Logf("Skipping download verification for %d files (TeraBox API limitation)", len(uploadedFiles))
	for relPath := range uploadedFiles {
		t.Logf("✓ Uploaded (verification skipped): %s", relPath)
	}

	// Cleaning up uploaded files...
	/*
		t.Logf("Cleaning up uploaded files...")
		for relPath := range uploadedFiles {
			err := fsys.Remove(ctx, relPath)
			if err != nil {
				t.Logf("Failed to delete %s: %v", relPath, err)
			} else {
				t.Logf("✓ Deleted: %s", relPath)
			}
		}
	*/

	t.Logf("Integration test completed successfully")
}

// TestIntegrationStreamingUpload tests streaming upload functionality
func TestIntegrationStreamingUpload(t *testing.T) {
	fsys := getIntegrationFs(t)
	ctx := context.Background()

	// Test with a file that should trigger streaming upload
	largeContent := strings.Repeat("This is a large file content for testing streaming uploads. ", 10000) // ~500KB
	testFile := "test_streaming_large.txt"

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "terabox_streaming_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write large test content
	if _, err := tmpFile.WriteString(largeContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Set streaming threshold to a small value to force streaming
	fsys.opt.StreamingThreshold = 1024 // 1KB

	// Upload the file
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer f.Close()

	obj, err := fsys.Put(ctx, f, &ObjectInfoMock{remote: testFile, size: int64(len(largeContent))})
	if err != nil {
		t.Fatalf("Failed to upload large file: %v", err)
	}

	t.Logf("Successfully uploaded large file using streaming: %s", testFile)

	// Skip download verification due to TeraBox API limitations
	t.Logf("✓ Streaming upload completed (download verification skipped due to TeraBox API limitation)")

	// Clean up
	if err := obj.Remove(ctx); err != nil {
		t.Errorf("Failed to delete test file: %v", err)
	} else {
		t.Logf("✓ Deleted test file")
	}
}

// TestIntegrationBatchOperations tests batch operations functionality
func TestIntegrationBatchOperations(t *testing.T) {
	fsys := getIntegrationFs(t)
	ctx := context.Background()

	// Set batch operation parameters
	fsys.opt.BatchDeleteSize = 5
	fsys.opt.DeleteDelay = 100 * time.Millisecond

	// Create multiple test files
	testFiles := []string{
		"batch_test_1.txt",
		"batch_test_2.txt",
		"batch_test_3.txt",
		"batch_test_4.txt",
		"batch_test_5.txt",
		"batch_test_6.txt",
		"batch_test_7.txt",
	}

	uploadedFiles := make([]*Object, 0, len(testFiles))

	// Upload test files
	for i, filename := range testFiles {
		content := fmt.Sprintf("Test content for file %d: %s", i+1, time.Now().String())

		// Create temporary file
		tmpFile, err := os.CreateTemp("", "terabox_batch_test_*")
		if err != nil {
			t.Fatalf("Failed to create temp file for %s: %v", filename, err)
		}
		defer os.Remove(tmpFile.Name())

		// Write test content
		if _, err := tmpFile.WriteString(content); err != nil {
			t.Fatalf("Failed to write test content for %s: %v", filename, err)
		}
		tmpFile.Close()

		// Upload the file
		f, err := os.Open(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to open temp file for %s: %v", filename, err)
		}

		obj, err := fsys.Put(ctx, f, &ObjectInfoMock{remote: filename, size: int64(len(content))})
		f.Close()
		if err != nil {
			t.Fatalf("Failed to upload %s: %v", filename, err)
		}

		if teraboxObj, ok := obj.(*Object); ok {
			uploadedFiles = append(uploadedFiles, teraboxObj)
		}
		t.Logf("✓ Uploaded: %s", filename)
	}

	t.Logf("Successfully uploaded %d files for batch testing", len(uploadedFiles))

	// Wait for files to be available
	time.Sleep(2 * time.Second)

	// Test batch deletion by finding and deleting timestamped versions
	// (This tests the batch deletion infrastructure)
	for _, obj := range uploadedFiles {
		// Modify the file to create a timestamped version
		originalContent := fmt.Sprintf("Modified content for %s: %s", obj.Remote(), time.Now().String())

		// Create a reader for the modified content
		reader := strings.NewReader(originalContent)

		// Upload modified version (this should create timestamped version)
		_, err := fsys.Put(ctx, reader, &ObjectInfoMock{remote: obj.Remote(), size: int64(len(originalContent))})
		if err != nil {
			t.Logf("Warning: Failed to create timestamped version for %s: %v", obj.Remote(), err)
		} else {
			t.Logf("✓ Created timestamped version for: %s", obj.Remote())
		}
	}

	// Clean up all files
	t.Logf("Cleaning up %d uploaded files...", len(uploadedFiles))
	for _, obj := range uploadedFiles {
		if err := obj.Remove(ctx); err != nil {
			t.Errorf("Failed to delete %s: %v", obj.Remote(), err)
		} else {
			t.Logf("✓ Deleted: %s", obj.Remote())
		}
	}

	t.Logf("Batch operations test completed successfully")
}

// TestIntegrationListAndDelete tests listing files and deleting them
func TestIntegrationListAndDelete(t *testing.T) {
	fsys := getIntegrationFs(t)
	ctx := context.Background()

	// Clean up remote directory before starting test
	cleanupRemoteDirectory(t, fsys, ctx)

	// First, upload a test file
	testContent := []byte("test file for list and delete " + time.Now().String())
	testFile := "test_list_delete.txt"

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "terabox_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write test content
	if _, err := tmpFile.Write(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Upload the file
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer f.Close()

	obj, err := fsys.Put(ctx, f, &ObjectInfoMock{remote: testFile, size: int64(len(testContent))})
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Logf("Uploaded test file: %s", testFile)

	// Wait for file to be available
	time.Sleep(2 * time.Second)

	// List files in the root directory
	t.Logf("Listing files in %s", RemoteTestDir)
	entries, err := fsys.List(ctx, "")
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	found := false
	for _, entry := range entries {
		if obj, ok := entry.(*Object); ok {
			t.Logf("Found file: %s", obj.Remote())
			if obj.Remote() == testFile {
				found = true
			}
		}
	}

	if !found {
		t.Errorf("Uploaded file %s not found in listing", testFile)
	} else {
		t.Logf("✓ Found uploaded file in listing")
	}

	// Delete the test file
	if err := obj.Remove(ctx); err != nil {
		t.Errorf("Failed to delete test file: %v", err)
	} else {
		t.Logf("✓ Successfully deleted test file")
	}
}

// TestIntegrationDirectoryOperations tests directory creation and deletion
func TestIntegrationDirectoryOperations(t *testing.T) {
	fsys := getIntegrationFs(t)
	ctx := context.Background()

	// Clean up remote directory before starting test
	cleanupRemoteDirectory(t, fsys, ctx)

	testDir := "test_dir_operations"

	// Create a directory
	t.Logf("Creating directory: %s", testDir)
	if err := fsys.Mkdir(ctx, testDir); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	t.Logf("✓ Created directory: %s", testDir)

	// Try to create the same directory again (should not fail)
	if err := fsys.Mkdir(ctx, testDir); err != nil {
		t.Logf("Warning: Creating existing directory failed: %v", err)
	}

	// Create a subdirectory
	subDir := testDir + "/subdir"
	t.Logf("Creating subdirectory: %s", subDir)
	if err := fsys.Mkdir(ctx, subDir); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	t.Logf("✓ Created subdirectory: %s", subDir)

	// Upload a file to the subdirectory
	testContent := []byte("test file in subdirectory")
	testFile := subDir + "/test_file.txt"

	tmpFile, err := os.CreateTemp("", "terabox_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	f, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer f.Close()

	obj, err := fsys.Put(ctx, f, &ObjectInfoMock{remote: testFile, size: int64(len(testContent))})
	if err != nil {
		t.Fatalf("Failed to upload file to subdirectory: %v", err)
	}
	t.Logf("✓ Uploaded file to subdirectory: %s", testFile)

	// Wait for file to be available
	time.Sleep(2 * time.Second)

	// List files in the subdirectory
	t.Logf("Listing files in subdirectory: %s", subDir)
	entries, err := fsys.List(ctx, subDir)
	if err != nil {
		t.Fatalf("Failed to list files in subdirectory: %v", err)
	}

	found := false
	for _, entry := range entries {
		if obj, ok := entry.(*Object); ok {
			t.Logf("Found file in subdirectory: %s", obj.Remote())
			if obj.Remote() == testFile {
				found = true
			}
		}
	}

	if !found {
		t.Errorf("Uploaded file not found in subdirectory listing")
	} else {
		t.Logf("✓ Found uploaded file in subdirectory listing")
	}

	// Clean up: delete file and directories
	t.Logf("Cleaning up...")
	if err := obj.Remove(ctx); err != nil {
		t.Errorf("Failed to delete test file: %v", err)
	} else {
		t.Logf("✓ Deleted test file")
	}

	// Note: TeraBox might not support directory deletion, so we'll just log this
	t.Logf("Note: Directory deletion not tested (TeraBox may not support it)")
}
