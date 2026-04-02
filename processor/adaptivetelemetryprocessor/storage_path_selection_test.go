// Copyright New Relic, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package adaptivetelemetryprocessor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsPathWritableOrCreatable tests the isPathWritableOrCreatable function
func TestIsPathWritableOrCreatable(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(t *testing.T) string
		expectedResult bool
		description    string
	}{
		{
			name: "writable directory",
			setupFunc: func(t *testing.T) string {
				// Create a temporary directory that should be writable
				tmpDir := t.TempDir()
				testDir := filepath.Join(tmpDir, "writable-test")
				return testDir
			},
			expectedResult: true,
			description:    "Should return true for a writable directory",
		},
		{
			name: "existing writable directory",
			setupFunc: func(t *testing.T) string {
				// Create a temporary directory that already exists
				tmpDir := t.TempDir()
				testDir := filepath.Join(tmpDir, "existing-writable")
				err := os.MkdirAll(testDir, 0o700)
				require.NoError(t, err)
				return testDir
			},
			expectedResult: true,
			description:    "Should return true for an existing writable directory",
		},
		{
			name: "non-writable directory",
			setupFunc: func(t *testing.T) string {
				if runtime.GOOS == "windows" {
					// On Windows, it's harder to create non-writable directories
					// Skip this test case on Windows
					t.Skip("Skipping non-writable directory test on Windows")
				}
				// Try to use a system directory that regular users can't write to
				return "/root/test-dir-should-not-exist"
			},
			expectedResult: false,
			description:    "Should return false for a non-writable directory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.setupFunc(t)
			result := isPathWritableOrCreatable(path)
			assert.Equal(t, tc.expectedResult, result, tc.description)
		})
	}
}

// TestIsPathWritableOrCreatable_CreatesDirectory tests that the function creates the directory
func TestIsPathWritableOrCreatable_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "new-dir", "nested", "path")

	// Verify directory doesn't exist
	_, err := os.Stat(testDir)
	assert.True(t, os.IsNotExist(err), "Directory should not exist initially")

	// Call the function
	result := isPathWritableOrCreatable(testDir)
	assert.True(t, result, "Should be able to create and write to the directory")

	// Verify directory was created
	stat, err := os.Stat(testDir)
	assert.NoError(t, err, "Directory should exist after the check")
	assert.True(t, stat.IsDir(), "Path should be a directory")
}

// TestIsPathWritableOrCreatable_CleanupTestFile tests that test file is cleaned up
func TestIsPathWritableOrCreatable_CleanupTestFile(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "cleanup-test")

	// Call the function
	result := isPathWritableOrCreatable(testDir)
	assert.True(t, result, "Should be able to create and write to the directory")

	// Verify test file was cleaned up
	testFile := filepath.Join(testDir, ".write_test")
	_, err := os.Stat(testFile)
	assert.True(t, os.IsNotExist(err), "Test file should be cleaned up")
}

// TestGetAllowedStorageDirectory_XDGPathPrecedence tests XDG path precedence
func TestGetAllowedStorageDirectory_XDGPathPrecedence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping XDG path test on Windows")
	}

	// This test verifies the precedence logic
	allowedDir := getAllowedStorageDirectory()

	// On macOS or Linux with a home directory, it should prefer XDG if writable
	home, err := os.UserHomeDir()
	if err == nil && home != "" && home != "/" {
		xdgPath := filepath.Join(home, ".local", "state", "nrdot-collector") + string(filepath.Separator)
		varLibPath := "/var/lib/nrdot-collector/"

		// The allowed directory should be either XDG or /var/lib
		assert.True(t,
			allowedDir == xdgPath || allowedDir == varLibPath || allowedDir == "",
			"Allowed directory should be XDG path, /var/lib path, or empty string",
		)

		// If XDG path is writable, it should be preferred
		if isPathWritableOrCreatable(filepath.Join(home, ".local", "state", "nrdot-collector")) {
			assert.Equal(t, xdgPath, allowedDir, "XDG path should be preferred when writable")
		}
	}
}

// TestGetAllowedStorageDirectory_FallbackToVarLib tests fallback behavior
func TestGetAllowedStorageDirectory_FallbackToVarLib(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping /var/lib fallback test on Windows")
	}

	// This test verifies that if storage is enabled, it uses a valid path
	allowedDir := getAllowedStorageDirectory()

	if allowedDir != "" {
		// Verify it's a valid absolute path
		assert.True(t, filepath.IsAbs(allowedDir), "Allowed directory should be absolute")
		assert.Contains(t, allowedDir, "nrdot-collector", "Path should contain nrdot-collector")
		assert.True(t,
			allowedDir == "/var/lib/nrdot-collector/" ||
				filepath.Dir(filepath.Clean(allowedDir)) != "/var/lib",
			"Should be /var/lib/nrdot-collector/ or an XDG path",
		)
	}
}

// TestGetAllowedStorageDirectory_DisabledWhenNoWritablePath tests storage disabled behavior
func TestGetAllowedStorageDirectory_DisabledWhenNoWritablePath(t *testing.T) {
	// This test documents the behavior when neither XDG nor /var/lib is writable
	// In such cases, getAllowedStorageDirectory() returns an empty string

	allowedDir := getAllowedStorageDirectory()

	// The function should return either:
	// 1. A valid path (storage enabled)
	// 2. Empty string (storage disabled)
	if allowedDir != "" {
		assert.True(t, filepath.IsAbs(allowedDir), "If enabled, path should be absolute")
		assert.Contains(t, allowedDir, "nrdot-collector", "If enabled, path should contain nrdot-collector")
	}
	// Empty string is valid and indicates storage is disabled
}

// TestGetAllowedStorageDirectory_WindowsBehavior tests Windows-specific behavior
func TestGetAllowedStorageDirectory_WindowsBehavior(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	allowedDir := getAllowedStorageDirectory()

	// On Windows, should always return a path (never empty)
	assert.NotEmpty(t, allowedDir, "Windows should always return a path")
	assert.Contains(t, allowedDir, "nrdot-collector", "Path should contain nrdot-collector")
	assert.True(t, filepath.IsAbs(allowedDir), "Path should be absolute")

	// Verify it's using LOCALAPPDATA
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	expectedPath := filepath.Join(localAppData, "nrdot-collector") + string(filepath.Separator)
	assert.Equal(t, expectedPath, allowedDir, "Should use LOCALAPPDATA path on Windows")
}

// TestGetDefaultStoragePath_UsesAllowedDirectory tests that default path uses allowed directory
func TestGetDefaultStoragePath_UsesAllowedDirectory(t *testing.T) {
	allowedDir := getAllowedStorageDirectory()
	defaultPath := getDefaultStoragePath()

	if allowedDir != "" {
		// If storage is enabled, default path should be under allowed directory
		expectedPath := filepath.Join(allowedDir, "adaptiveprocess.db")
		assert.Equal(t, expectedPath, defaultPath, "Default path should be under allowed directory")
	} else {
		// If storage is disabled (empty allowed dir), default path will be just the filename
		assert.Equal(t, "adaptiveprocess.db", defaultPath, "Default path should be just filename when storage disabled")
	}
}

// TestValidateStoragePath_WithDynamicAllowedDirectory tests validation with dynamic path
func TestValidateStoragePath_WithDynamicAllowedDirectory(t *testing.T) {
	allowedDir := getAllowedStorageDirectory()
	if allowedDir == "" {
		t.Skip("Storage is disabled on this platform (no writable directory found)")
	}

	baseDir := filepath.Clean(allowedDir)

	// Test that validation works with the actual allowed directory
	validPath := filepath.Join(baseDir, "test.db")
	err := validateStoragePath(validPath, nil)
	assert.NoError(t, err, "Should accept path under actual allowed directory")

	// Test that paths outside allowed directory are rejected
	invalidPath := "/tmp/test.db"
	if runtime.GOOS == "windows" {
		invalidPath = "C:\\Windows\\test.db"
	}
	err = validateStoragePath(invalidPath, nil)
	assert.Error(t, err, "Should reject path outside allowed directory")
	assert.Contains(t, err.Error(), "must be under", "Error should mention path restriction")
}
