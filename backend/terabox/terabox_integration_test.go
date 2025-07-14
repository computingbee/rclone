//go:build integration
// +build integration

package terabox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/configmap"
)

// TestTeraBoxUploadIntegration tests the upload functionality with real credentials
func TestTeraBoxUploadIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Install config file handler
	configfile.Install()

	// Set config path
	configPath := os.Getenv("RCLONE_CONFIG")
	if configPath == "" {
		configPath = filepath.Join(os.Getenv("HOME"), ".config", "rclone", "rclone.conf")
	}

	// Set the config path
	if err := config.SetConfigPath(configPath); err != nil {
		t.Skipf("Could not set config path: %v", err)
	}

	// Load the config data
	configData := config.LoadedData()

	// Check if terabox section exists
	if !configData.HasSection("terabox") {
		t.Skip("No [terabox] section found in rclone config")
	}

	// Create config map
	m := configmap.New()
	for _, key := range configData.GetKeyList("terabox") {
		value, found := configData.GetValue("terabox", key)
		if found {
			m.Set(key, value)
			fmt.Printf("DEBUG: Loaded config key '%s' = '%s'\n", key, value)
		}
	}

	// Debug: Check what keys we found
	fmt.Printf("DEBUG: Found %d config keys for terabox section\n", len(configData.GetKeyList("terabox")))
	fmt.Printf("DEBUG: Config keys: %v\n", configData.GetKeyList("terabox"))

	// Create filesystem
	ctx := context.Background()
	f, err := NewFs(ctx, "terabox", "/test-terabox", m)
	if err != nil {
		t.Fatalf("Failed to create TeraBox filesystem: %v", err)
	}

	// Test sync functionality
	t.Run("SyncLocalToRemote", func(t *testing.T) {
		err := f.(*Fs).SyncLocalToRemote(ctx)
		if err != nil {
			t.Errorf("SyncLocalToRemote failed: %v", err)
		}
	})

	// Test listing files
	t.Run("ListFiles", func(t *testing.T) {
		entries, err := f.List(ctx, "")
		if err != nil {
			t.Errorf("List failed: %v", err)
		}

		fmt.Printf("Found %d entries in remote directory\n", len(entries))
		for _, entry := range entries {
			if obj, ok := entry.(fs.Object); ok {
				fmt.Printf("  File: %s (size: %d)\n", obj.Remote(), obj.Size())
			} else {
				fmt.Printf("  Dir: %s\n", entry.Remote())
			}
		}
	})

	// Test building remote listings
	t.Run("BuildRemoteListings", func(t *testing.T) {
		listings, err := f.(*Fs).BuildRemoteListings(ctx)
		if err != nil {
			t.Errorf("BuildRemoteListings failed: %v", err)
		}

		fmt.Printf("Built remote listings with %d files\n", len(listings))
		for path, size := range listings {
			fmt.Printf("  %s: %d bytes\n", path, size)
		}
	})
}
