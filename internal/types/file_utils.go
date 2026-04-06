package types

import (
	"os"
	"path/filepath"
)

// File system utility functions for file picker and directory handling

// FindClosestExistingDirectory finds the closest parent directory that exists
// Starting from the given path, it walks up the directory tree until it finds
// a directory that exists. If no parent exists, returns user home directory.
func FindClosestExistingDirectory(path string) string {
	// Clean the path to handle any inconsistent separators
	cleanPath := filepath.Clean(path)

	// Check if the current path exists and is a directory
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return cleanPath
	}

	// If it's a file path, get its directory
	if !isDirectoryPath(cleanPath) {
		cleanPath = filepath.Dir(cleanPath)
	}

	// Walk up the directory tree
	for {
		// Check if current directory exists
		if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
			return cleanPath
		}

		// Get parent directory
		parent := filepath.Dir(cleanPath)

		// If we've reached the root or can't go further, fallback to home
		if parent == cleanPath || parent == "." || parent == "/" || (len(parent) == 3 && parent[1:] == ":\\") {
			// Windows drive root (e.g., "C:\") or Unix root "/" reached
			break
		}

		cleanPath = parent
	}

	// Final fallback to user home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		return homeDir
	}

	// If even home directory fails, return current working directory
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}

	// Absolute last resort - return root
	return string(filepath.Separator)
}

// isDirectoryPath checks if a path appears to be a directory (vs file)
// This is a heuristic based on whether the path ends with a separator
// or doesn't have a file extension
func isDirectoryPath(path string) bool {
	// If it ends with a separator, it's likely a directory
	if len(path) > 0 && os.IsPathSeparator(path[len(path)-1]) {
		return true
	}

	// If it has no extension and no separator, might be a directory name
	base := filepath.Base(path)
	ext := filepath.Ext(base)

	// If there's no extension, assume it's a directory
	return ext == ""
}

// ValidateDirectoryExists checks if a directory exists and is accessible
func ValidateDirectoryExists(path string) bool {
	if path == "" {
		return false
	}

	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}
