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

func TestCreateTarFromDirWithExcludes(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()
	
	// Create test files and directories
	files := map[string]string{
		"main.py":                                    "print('hello')",
		"__pycache__/main.cpython-39.pyc":           "cache content",
		"test.py":                                    "test code",
		"data.txt":                                   "data content",
		".git/config":                                "git config",
		"node_modules/package/index.js":              "module code", 
		"build/output.o":                             "compiled object",
		"src/main.go":                                "go code",
	}
	
	for relPath, content := range files {
		fullPath := filepath.Join(tempDir, relPath)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Run("Exclude patterns work correctly", func(t *testing.T) {
		// Test excluding __pycache__, *.pyc, node_modules, etc.
		excludePatterns := []string{"__pycache__/", "*.pyc", "node_modules/", "build/"}
		
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
	})

	t.Run("No exclude patterns (all files included)", func(t *testing.T) {
		archive, err := CreateTarFromDirWithExcludes(tempDir, nil)
		require.NoError(t, err)
		
		// Extract and count all files
		reader := bytes.NewReader(archive)
		gzipReader, err := gzip.NewReader(reader)
		require.NoError(t, err)
		defer gzipReader.Close()
		
		tarReader := tar.NewReader(gzipReader)
		
		includedFiles := []string{}
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			
			if header.Typeflag == tar.TypeReg {
				includedFiles = append(includedFiles, header.Name)
			}
		}
		
		// All files should be included when no exclude patterns are provided
		assert.Contains(t, includedFiles, "main.py")
		assert.Contains(t, includedFiles, "__pycache__/main.cpython-39.pyc")
		assert.Contains(t, includedFiles, "test.py")
		assert.Contains(t, includedFiles, "data.txt")
		assert.Contains(t, includedFiles, ".git/config")
		assert.Contains(t, includedFiles, "node_modules/package/index.js")
		assert.Contains(t, includedFiles, "build/output.o")
		assert.Contains(t, includedFiles, "src/main.go")
	})

	t.Run("Specific file extension pattern", func(t *testing.T) {
		// Exclude all .py files
		excludePatterns := []string{"*.py"}
		
		archive, err := CreateTarFromDirWithExcludes(tempDir, excludePatterns)
		require.NoError(t, err)
		
		reader := bytes.NewReader(archive)
		gzipReader, err := gzip.NewReader(reader)
		require.NoError(t, err)
		defer gzipReader.Close()
		
		tarReader := tar.NewReader(gzipReader)
		
		includedFiles := []string{}
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			
			if header.Typeflag == tar.TypeReg {
				includedFiles = append(includedFiles, header.Name)
			}
		}
		
		// Specific .py files (matching pattern *.py) should be excluded
		// Check that main.py and test.py (which match *.py pattern) are NOT in the archive
		mainPyPresent := false
		testPyPresent := false
		for _, file := range includedFiles {
			if file == "main.py" {
				mainPyPresent = true
			}
			if file == "test.py" {
				testPyPresent = true
			}
		}
		assert.False(t, mainPyPresent, "main.py should be excluded by *.py pattern")
		assert.False(t, testPyPresent, "test.py should be excluded by *.py pattern")
		
		// Non-Python files should be included
		assert.Contains(t, includedFiles, "data.txt")
		assert.Contains(t, includedFiles, ".git/config")
		assert.Contains(t, includedFiles, "node_modules/package/index.js")
		assert.Contains(t, includedFiles, "build/output.o")
		assert.Contains(t, includedFiles, "src/main.go")
		
		// Files that contain ".py" as substring but don't match pattern *.py should still be included
		// E.g., "__pycache__/main.cpython-39.pyc" contains ".py" but doesn't match "*.py" pattern
	})
}

func TestCreateTarFromDirWithExcludes_SkipDir(t *testing.T) {
	// Create a temporary directory structure with nested directories
	tempDir := t.TempDir()
	
	// Create nested directory structure
	nestedDir := filepath.Join(tempDir, "deep", "nested", "dir")
	err := os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)
	
	// Create files in the nested structure
	err = os.WriteFile(filepath.Join(tempDir, "root_file.txt"), []byte("root content"), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(filepath.Join(nestedDir, "deep_file.txt"), []byte("deep content"), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(filepath.Join(tempDir, "other.txt"), []byte("other content"), 0644)
	require.NoError(t, err)

	// Test with excluding a deep directory
	excludePatterns := []string{"deep/"}
	
	archive, err := CreateTarFromDirWithExcludes(tempDir, excludePatterns)
	require.NoError(t, err)
	
	// Extract and verify
	reader := bytes.NewReader(archive)
	gzipReader, err := gzip.NewReader(reader)
	require.NoError(t, err)
	defer gzipReader.Close()
	
	tarReader := tar.NewReader(gzipReader)
	
	includedDirs := []string{}
	includedFiles := []string{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		
		if header.Typeflag == tar.TypeDir {
			includedDirs = append(includedDirs, header.Name)
		} else if header.Typeflag == tar.TypeReg {
			includedFiles = append(includedFiles, header.Name)
		}
	}
	
	// Root files should be included
	assert.Contains(t, includedFiles, "root_file.txt")
	assert.Contains(t, includedFiles, "other.txt")
	
	// Deep nested directory and its files should be excluded
	for _, dir := range includedDirs {
		assert.NotContains(t, dir, "deep", "Deep directory should be excluded")
	}
	
	for _, file := range includedFiles {
		assert.NotContains(t, file, "deep", "Files from deep directory should be excluded")
	}
}