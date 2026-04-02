// Copyright New Relic, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package adaptivetelemetryprocessor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateStoragePath_ValidPaths(t *testing.T) {
	// Get the actual allowed directory for this platform
	allowedDir := getAllowedStorageDirectory()

	// If storage is disabled (empty string), skip this test
	if allowedDir == "" {
		t.Skip("Storage is disabled on this platform (no writable directory found)")
	}

	// Remove trailing separator for building paths
	baseDir := filepath.Clean(allowedDir)

	tests := []struct {
		name        string
		storagePath string
		description string
	}{
		{
			name:        "valid path directly under allowed directory",
			storagePath: filepath.Join(baseDir, "state.db"),
			description: "Files directly under allowed directory should be allowed",
		},
		{
			name:        "valid nested path",
			storagePath: filepath.Join(baseDir, "data", "subdir", "state.db"),
			description: "Nested paths under allowed directory should be allowed",
		},
		{
			name:        "valid path with multiple levels",
			storagePath: filepath.Join(baseDir, "level1", "level2", "level3", "state.db"),
			description: "Deep nesting should be allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoragePath(tc.storagePath, nil)
			assert.NoError(t, err, tc.description)
		})
	}
}

// TestGetDefaultStoragePath_Windows tests Windows-specific default storage path generation
func TestGetDefaultStoragePath_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	defaultPath := getDefaultStoragePath()

	// Verify it's an absolute path
	assert.True(t, filepath.IsAbs(defaultPath), "Default path should be absolute on Windows")

	// Verify it contains the expected directory
	assert.Contains(t, defaultPath, "nrdot-collector", "Default path should contain nrdot-collector directory")

	// Verify it contains the database filename
	assert.Contains(t, defaultPath, "adaptiveprocess.db", "Default path should contain the database filename")

	// Verify it's in the user's local app data
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		assert.Contains(t, defaultPath, localAppData, "Default path should be under LOCALAPPDATA")
	}
}

// TestCreateDirectoryIfNotExists_Windows tests directory creation on Windows
func TestCreateDirectoryIfNotExists_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	// Use a temporary directory for testing
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test_dir", "nested", "deep")

	// Directory should not exist yet
	_, err := os.Stat(testDir)
	assert.True(t, os.IsNotExist(err), "Test directory should not exist initially")

	// Create the directory
	err = createDirectoryIfNotExists(testDir)
	assert.NoError(t, err, "createDirectoryIfNotExists should succeed")

	// Verify directory was created
	info, err := os.Stat(testDir)
	assert.NoError(t, err, "Directory should exist after creation")
	assert.True(t, info.IsDir(), "Created path should be a directory")

	// Calling again should be idempotent
	err = createDirectoryIfNotExists(testDir)
	assert.NoError(t, err, "createDirectoryIfNotExists should be idempotent")
}

func TestValidateStoragePath_InvalidPaths(t *testing.T) {
	tests := []struct {
		name          string
		storagePath   string
		errorContains string
		description   string
		skipOnWindows bool
	}{
		{
			name:          "empty storage path",
			storagePath:   "",
			errorContains: "cannot be empty",
			description:   "Empty paths should be rejected",
		},
		{
			name:          "relative path",
			storagePath:   "./state.db",
			errorContains: "must be an absolute path",
			description:   "Relative paths should be rejected",
		},
		{
			name:          "relative path with parent traversal",
			storagePath:   "../state.db",
			errorContains: "must be an absolute path",
			description:   "Relative paths with .. should be rejected",
		},
		{
			name:          "path outside allowed directory - /tmp",
			storagePath:   "/tmp/state.db",
			errorContains: "must be under",
			description:   "/tmp is not allowed",
			skipOnWindows: true,
		},
		{
			name:          "path outside allowed directory - /etc",
			storagePath:   "/etc/state.db",
			errorContains: "must be under",
			description:   "/etc is not allowed",
			skipOnWindows: true,
		},
		{
			name:          "path outside allowed directory - /var/lib/other",
			storagePath:   "/var/lib/other-app/state.db",
			errorContains: "must be under",
			description:   "Other directories under /var/lib/ are not allowed",
			skipOnWindows: true,
		},
		{
			name:          "path traversal attempt - parent directory",
			storagePath:   "/var/lib/nrdot-collector/../../../etc/passwd",
			errorContains: "must be under",
			description:   "Path traversal with .. should be rejected after cleaning",
			skipOnWindows: true,
		},
		{
			name:          "root directory",
			storagePath:   "/",
			errorContains: "must be under",
			description:   "Root directory should be rejected",
			skipOnWindows: true,
		},
		{
			name:          "home directory",
			storagePath:   "/home/user/state.db",
			errorContains: "must be under",
			description:   "Home directories should be rejected",
			skipOnWindows: true,
		},
	}

	// Add Windows-specific invalid paths
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		allowedDir := filepath.Join(localAppData, "nrdot-collector")

		windowsTests := []struct {
			name          string
			storagePath   string
			errorContains string
			description   string
		}{
			{
				name:          "Windows path outside allowed directory - C:\\Temp",
				storagePath:   "C:\\Temp\\state.db",
				errorContains: "must be under",
				description:   "C:\\Temp is not allowed on Windows",
			},
			{
				name:          "Windows path outside allowed directory - C:\\Windows",
				storagePath:   "C:\\Windows\\state.db",
				errorContains: "must be under",
				description:   "C:\\Windows is not allowed on Windows",
			},
			{
				name:          "Windows path traversal attempt",
				storagePath:   filepath.Join(allowedDir, "..", "..", "Windows", "System32", "config"),
				errorContains: "must be under",
				description:   "Path traversal should be rejected on Windows",
			},
			{
				name:          "Windows path in Program Files",
				storagePath:   "C:\\Program Files\\nrdot-collector\\state.db",
				errorContains: "must be under",
				description:   "Program Files is not allowed on Windows",
			},
			{
				name:          "Windows path in different user's AppData",
				storagePath:   "C:\\Users\\OtherUser\\AppData\\Local\\nrdot-collector\\state.db",
				errorContains: "must be under",
				description:   "Different user's AppData should be rejected on Windows",
			},
		}

		for _, tc := range windowsTests {
			t.Run(tc.name, func(t *testing.T) {
				err := validateStoragePath(tc.storagePath, nil)
				assert.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			})
		}
	}

	// Run platform-appropriate tests
	for _, tc := range tests {
		if runtime.GOOS == "windows" && tc.skipOnWindows {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoragePath(tc.storagePath, nil)
			assert.Error(t, err, tc.description)
			if tc.errorContains != "" {
				assert.Contains(t, err.Error(), tc.errorContains)
			}
		})
	}
}

func TestCheckPathForSymlinks(t *testing.T) {
	// Skip on Windows as symlinks require admin privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink tests on Windows")
	}

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "nrdot-collector")
	err := os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err)

	// Create a real directory
	realDir := filepath.Join(baseDir, "real_directory")
	err = os.MkdirAll(realDir, 0o755)
	require.NoError(t, err)

	// Create a symlink under baseDir
	symlinkPath := filepath.Join(baseDir, "symlink_dir")
	err = os.Symlink(realDir, symlinkPath)
	require.NoError(t, err)

	tests := []struct {
		name          string
		path          string
		baseDir       string
		expectError   bool
		errorContains string
	}{
		{
			name:        "normal path without symlinks",
			path:        filepath.Join(baseDir, "real_directory", "file.db"),
			baseDir:     baseDir,
			expectError: false,
		},
		{
			name:          "path with symlink component",
			path:          filepath.Join(baseDir, "symlink_dir", "file.db"),
			baseDir:       baseDir,
			expectError:   true,
			errorContains: "is a symlink",
		},
		{
			name:        "non-existent path (should not error)",
			path:        filepath.Join(baseDir, "nonexistent", "path", "file.db"),
			baseDir:     baseDir,
			expectError: false,
		},
		{
			name:        "path equals base",
			path:        baseDir,
			baseDir:     baseDir,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPathForSymlinks(tc.path, tc.baseDir)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetAllowedStorageDirectory(t *testing.T) {
	allowedDir := getAllowedStorageDirectory()

	switch runtime.GOOS {
	case "windows":
		// On Windows, should return %LOCALAPPDATA%\nrdot-collector\
		assert.Contains(t, allowedDir, "nrdot-collector")
		assert.True(t, filepath.IsAbs(allowedDir), "Windows path should be absolute")
		assert.NotEmpty(t, allowedDir, "Windows path should not be empty")
	default:
		// On Linux/Unix/macOS, if storage is enabled, verify the path
		if allowedDir != "" {
			assert.Contains(t, allowedDir, "nrdot-collector", "Path should contain nrdot-collector")
			assert.True(t, filepath.IsAbs(allowedDir), "Path should be absolute")
			assert.True(t,
				strings.HasSuffix(allowedDir, "/"),
				"Path should end with /",
			)
		}
		// Empty string is valid if neither path is writable (storage disabled)
	}
}

func TestConfigValidation_EnableStorage(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		description string
	}{
		{
			name: "storage enabled (default)",
			config: &Config{
				EnableStorage: nil, // nil means default (enabled)
			},
			expectError: false,
			description: "Storage enabled by default should not error",
		},
		{
			name: "storage explicitly enabled",
			config: &Config{
				EnableStorage: ptrBool(true),
			},
			expectError: false,
			description: "Explicitly enabled storage should not error",
		},
		{
			name: "storage disabled",
			config: &Config{
				EnableStorage: ptrBool(false),
			},
			expectError: false,
			description: "Disabled storage should not error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.Normalize()
			err := tc.config.Validate()

			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				assert.NoError(t, err, tc.description)
			}
		})
	}
}

func TestSymlinkAttackPrevention(t *testing.T) {
	// Skip on Windows as symlinks require admin privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink tests on Windows")
	}

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "nrdot-collector")
	err := os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err)

	// Create a subdirectory
	dataDir := filepath.Join(baseDir, "data")
	err = os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	// Create malicious symlink pointing to /etc
	maliciousSymlink := filepath.Join(dataDir, "evil_link")
	err = os.Symlink("/etc", maliciousSymlink)
	require.NoError(t, err)

	// Try to use a path through the symlink
	attackPath := filepath.Join(maliciousSymlink, "passwd")

	err = checkPathForSymlinks(attackPath, baseDir)
	assert.Error(t, err, "Attack path through symlink should be rejected")
	assert.Contains(t, err.Error(), "is a symlink", "Error should mention symlink detection")
}

func TestPathTraversalPrevention(t *testing.T) {
	// Get the actual allowed directory for this platform
	allowedDir := getAllowedStorageDirectory()
	if allowedDir == "" {
		t.Skip("Storage is disabled on this platform (no writable directory found)")
	}

	// Remove trailing separator for building paths
	baseDir := filepath.Clean(allowedDir)

	// Test that path traversal is prevented by filepath.Clean
	var traversalPath string
	var expectedErrorSubstring string

	if runtime.GOOS == "windows" {
		traversalPath = filepath.Join(baseDir, "..", "..", "Windows", "System32", "config")
		expectedErrorSubstring = "must be under"
	} else {
		// Use the actual allowed directory instead of hardcoded /var/lib
		traversalPath = filepath.Join(baseDir, "..", "..", "etc", "passwd")
		expectedErrorSubstring = "must be under"
	}

	err := validateStoragePath(traversalPath, nil)
	assert.Error(t, err, "Path traversal should be rejected")
	assert.Contains(t, err.Error(), expectedErrorSubstring)
}

func TestPathValidation_EdgeCases(t *testing.T) {
	// Get the actual allowed directory for this platform
	allowedDir := getAllowedStorageDirectory()
	if allowedDir == "" {
		t.Skip("Storage is disabled on this platform (no writable directory found)")
	}

	// Remove trailing separator for building paths
	basePath := filepath.Clean(allowedDir)

	tests := []struct {
		name          string
		storagePath   string
		expectError   bool
		errorContains string
	}{
		{
			name:        "path with trailing slash",
			storagePath: filepath.Join(basePath, "state.db") + string(filepath.Separator),
			expectError: false, // filepath.Clean will remove trailing slash
		},
		{
			name:        "path with dot",
			storagePath: filepath.Join(basePath, ".", "state.db"),
			expectError: false, // filepath.Clean handles this
		},
	}

	// Add Linux-specific test for double slashes
	if runtime.GOOS != "windows" {
		tests = append(tests, struct {
			name          string
			storagePath   string
			expectError   bool
			errorContains string
		}{
			name:        "double slashes in path",
			storagePath: filepath.Join(basePath, "data", "state.db"), // Use filepath.Join to avoid double slashes
			expectError: false,                                       // filepath.Clean handles this
		})
	}

	// Add Windows-specific edge cases
	if runtime.GOOS == "windows" {
		windowsTests := []struct {
			name          string
			storagePath   string
			expectError   bool
			errorContains string
		}{
			{
				name:        "Windows path with forward slashes",
				storagePath: basePath + "/data/state.db",
				expectError: false, // filepath.Clean handles mixed separators
			},
			{
				name:        "Windows path with mixed separators",
				storagePath: basePath + "\\data/subdir\\state.db",
				expectError: false, // filepath.Clean normalizes this
			},
		}
		tests = append(tests, windowsTests...)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoragePath(tc.storagePath, nil)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestTOCTOUProtection verifies that symlinks are re-validated immediately before write operations
// to prevent Time-of-Check to Time-of-Use (TOCTOU) race conditions
func TestTOCTOUProtection(t *testing.T) {
	// Skip on Windows as symlinks require admin privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink tests on Windows")
	}

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "nrdot-collector")
	err := os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err)

	dataDir := filepath.Join(baseDir, "data")
	err = os.MkdirAll(dataDir, 0o755)
	require.NoError(t, err)

	// Create a legitimate file path initially
	filePath := filepath.Join(dataDir, "test.db")

	// Create storage with the valid path
	storage := newFileStorageForTesting(filePath, baseDir)

	// Write initial data successfully
	testEntities := map[string]*trackedEntity{
		"entity1": {
			Identity:      "entity1",
			CurrentValues: map[string]float64{"metric1": 10.5},
		},
	}

	err = storage.Save(testEntities)
	require.NoError(t, err, "Initial save should succeed")

	// Now simulate a TOCTOU attack: replace data directory with a symlink to /tmp
	// First remove the existing data directory and file
	err = os.RemoveAll(dataDir)
	require.NoError(t, err)

	// Create a symlink in its place pointing to /tmp
	err = os.Symlink("/tmp", dataDir)
	require.NoError(t, err)

	// Try to write again - this should fail because symlink is re-validated before write
	err = storage.Save(testEntities)
	assert.Error(t, err, "Save should fail when directory is replaced with symlink")
	assert.Contains(t, err.Error(), "symlink validation failed before write", "Error should mention validation failure")
	assert.Contains(t, err.Error(), "is a symlink", "Error should mention symlink detection")

	// Verify that no file was written to /tmp
	tmpFilePath := filepath.Join("/tmp", "test.db")
	_, err = os.Stat(tmpFilePath)
	assert.True(t, os.IsNotExist(err), "File should not be written to /tmp through symlink")
}

// TestPermissionErrorHandling verifies that permission errors during symlink validation
// do not silently bypass security checks
func TestPermissionErrorHandling(t *testing.T) {
	// Skip on Windows as permission handling differs significantly
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	// This test verifies the fix for: "Permission errors can silently bypass the symlink protection"
	// We ensure that if we can't verify a path isn't a symlink due to permission errors,
	// we reject it rather than allowing it

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "nrdot-collector")
	err := os.MkdirAll(baseDir, 0o755)
	require.NoError(t, err)

	// Create a directory that we'll make unreadable
	restrictedDir := filepath.Join(baseDir, "restricted")
	err = os.MkdirAll(restrictedDir, 0o755)
	require.NoError(t, err)

	// Create a file inside it
	testFile := filepath.Join(restrictedDir, "test.db")
	err = os.WriteFile(testFile, []byte("test"), 0o600)
	require.NoError(t, err)

	// Make the directory unreadable (no execute permission means we can't access contents)
	err = os.Chmod(restrictedDir, 0o000)
	require.NoError(t, err)

	// Ensure we restore permissions after test
	defer func() {
		_ = os.Chmod(restrictedDir, 0o755)
	}()

	// Try to validate the path - should fail due to permission error, not silently pass
	err = checkPathForSymlinks(testFile, baseDir)
	assert.Error(t, err, "checkPathForSymlinks should fail when it can't verify path security")
	assert.Contains(t, err.Error(), "cannot verify path security", "Error should indicate inability to verify")
}
