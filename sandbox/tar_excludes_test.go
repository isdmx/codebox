package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test directory structure
func createTestDirectoryStructure(t *testing.T) string {
	tempDir := t.TempDir()

	// Create test files and directories
	files := map[string]string{
		"main.py":                         "print('hello')",
		"__pycache__/main.cpython-39.pyc": "cache content",
		"test.py":                         "test code",
		"data.txt":                        "data content",
		".git/config":                     "git config",
		"node_modules/package/index.js":   "module code",
		"build/output.o":                  "compiled object",
		"src/main.go":                     "go code",
	}

	for relPath, content := range files {
		fullPath := filepath.Join(tempDir, relPath)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, []byte(content), 0600)
		require.NoError(t, err)
	}

	return tempDir
}

// Helper function to extract included files from archive
func extractIncludedFiles(t *testing.T, tempDir string, excludePatterns []string) []string {
	archive, err := CreateTarFromDirWithExcludes(tempDir, excludePatterns)
	require.NoError(t, err)

	// Extract the tar to check what files are included
	reader := bytes.NewReader(archive)
	gzipReader, err := gzip.NewReader(reader)
	require.NoError(t, err)
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	// Collect all file paths from the archive
	includedFiles := []string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		// Only collect regular files, not directories
		if header.Typeflag == tar.TypeReg {
			includedFiles = append(includedFiles, header.Name)
		}
	}

	return includedFiles
}

func testExcludePatternsCorrectly(t *testing.T) {
	tempDir := createTestDirectoryStructure(t)

	// Test excluding __pycache__, *.pyc, node_modules, etc.
	excludePatterns := []string{"__pycache__/", "*.pyc", "node_modules/", "build/"}
	includedFiles := extractIncludedFiles(t, tempDir, excludePatterns)

	// Files that should be included (not excluded)
	assert.Contains(t, includedFiles, "main.py")
	assert.Contains(t, includedFiles, "test.py")
	assert.Contains(t, includedFiles, "data.txt")
	assert.Contains(t, includedFiles, ".git/config")
	assert.Contains(t, includedFiles, "src/main.go")

	// Files that should be excluded
	// The __pycache__ directory should be excluded entirely
	for _, file := range includedFiles {
		assert.NotContains(t, file, "__pycache__", "Files from __pycache__ directory should be excluded")
	}

	// Node modules should be excluded
	for _, file := range includedFiles {
		assert.NotContains(t, file, "node_modules", "Files from node_modules directory should be excluded")
	}

	// Build directory should be excluded
	for _, file := range includedFiles {
		assert.NotContains(t, file, "build", "Files from build directory should be excluded")
	}
}

func testNoExcludePatterns(t *testing.T) {
	tempDir := createTestDirectoryStructure(t)

	includedFiles := extractIncludedFiles(t, tempDir, nil)

	// All files should be included when no exclude patterns are provided
	assert.Contains(t, includedFiles, "main.py")
	assert.Contains(t, includedFiles, "__pycache__/main.cpython-39.pyc")
	assert.Contains(t, includedFiles, "test.py")
	assert.Contains(t, includedFiles, "data.txt")
	assert.Contains(t, includedFiles, ".git/config")
	assert.Contains(t, includedFiles, "node_modules/package/index.js")
	assert.Contains(t, includedFiles, "build/output.o")
	assert.Contains(t, includedFiles, "src/main.go")
}

func testSpecificFileExtensionPattern(t *testing.T) {
	tempDir := createTestDirectoryStructure(t)

	// Exclude all .py files
	excludePatterns := []string{"*.py"}
	includedFiles := extractIncludedFiles(t, tempDir, excludePatterns)

	// Python files should be excluded
	mainPyExcluded := true
	testPyExcluded := true
	for _, file := range includedFiles {
		if file == "main.py" {
			mainPyExcluded = false
		}
		if file == "test.py" {
			testPyExcluded = false
		}
	}
	assert.True(t, mainPyExcluded, "main.py should be excluded by *.py pattern")
	assert.True(t, testPyExcluded, "test.py should be excluded by *.py pattern")

	// Non-Python files should be included
	assert.Contains(t, includedFiles, "data.txt")
	assert.Contains(t, includedFiles, ".git/config")
	assert.Contains(t, includedFiles, "node_modules/package/index.js")
	assert.Contains(t, includedFiles, "build/output.o")
	assert.Contains(t, includedFiles, "src/main.go")
}

func TestCreateTarFromDirWithExcludes(t *testing.T) {
	t.Run("Exclude patterns work correctly", testExcludePatternsCorrectly)
	t.Run("No exclude patterns (all files included)", testNoExcludePatterns)
	t.Run("Specific file extension pattern", testSpecificFileExtensionPattern)
}
