package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TarTestMockFileSystem implements the FileSystem interface for tar testing
type TarTestMockFileSystem struct {
	mkdirAllCalls   []string
	writeFileCalls  map[string][]byte
	exists          map[string]bool
	errorOnMkdirAll string
}

func (TarTestMockFileSystem) MkdirTemp(dir, _ string) (string, error) {
	return filepath.Join(dir, "temp"), nil
}

func (m *TarTestMockFileSystem) MkdirAll(path string, _ os.FileMode) error {
	if m.errorOnMkdirAll != "" && path == m.errorOnMkdirAll {
		return fmt.Errorf("mock mkdir error for %s", path)
	}
	if m.mkdirAllCalls == nil {
		m.mkdirAllCalls = []string{}
	}
	m.mkdirAllCalls = append(m.mkdirAllCalls, path)
	return nil
}

func (m *TarTestMockFileSystem) WriteFile(filename string, data []byte, _ os.FileMode) error {
	if m.writeFileCalls == nil {
		m.writeFileCalls = make(map[string][]byte)
	}
	m.writeFileCalls[filename] = data
	return nil
}

func (TarTestMockFileSystem) ReadFile(_ string) ([]byte, error) {
	// This is a basic implementation for testing
	return nil, nil
}

func (TarTestMockFileSystem) RemoveAll(_ string) error {
	return nil
}

func (m *TarTestMockFileSystem) FileExists(path string) (bool, error) {
	if m.exists != nil {
		return m.exists[path], nil
	}
	return true, nil
}

func createTestTar(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		err := tw.WriteHeader(hdr)
		require.NoError(t, err)
		_, err = tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestExtractTarToDir(t *testing.T) {
	t.Run("ValidTarExtraction", func(t *testing.T) {
		mockFS := &TarTestMockFileSystem{}
		tarData := createTestTar(t, map[string]string{
			"file1.txt": "content1",
			"file2.txt": "content2",
		})

		err := ExtractTarToDir(mockFS, tarData, "/dest")
		require.NoError(t, err)

		// Check that files were written
		require.NotNil(t, mockFS.writeFileCalls)
		assert.Equal(t, []byte("content1"), mockFS.writeFileCalls["/dest/file1.txt"])
		assert.Equal(t, []byte("content2"), mockFS.writeFileCalls["/dest/file2.txt"])
	})

	t.Run("DirectoryExtraction", func(t *testing.T) {
		mockFS := &TarTestMockFileSystem{}
		tarData := createTestTar(t, map[string]string{
			"dir/":         "", // This creates a directory
			"dir/file.txt": "content",
		})

		err := ExtractTarToDir(mockFS, tarData, "/dest")
		require.NoError(t, err)

		// Check that directory was created
		assert.Contains(t, mockFS.mkdirAllCalls, "/dest/dir")
	})

	t.Run("PathTraversalPrevention", func(t *testing.T) {
		mockFS := &TarTestMockFileSystem{}
		tarData := createTestTar(t, map[string]string{
			"../dangerous.txt": "should not allow",
		})

		err := ExtractTarToDir(mockFS, tarData, "/dest")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsafe relative path")
	})

	t.Run("AbsolutePathPrevention", func(t *testing.T) {
		mockFS := &TarTestMockFileSystem{}
		tarData := createTestTar(t, map[string]string{
			"/absolute/path.txt": "should not allow",
		})

		err := ExtractTarToDir(mockFS, tarData, "/dest")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "absolute path not allowed")
	})

	t.Run("InvalidTarData", func(t *testing.T) {
		mockFS := &TarTestMockFileSystem{}
		err := ExtractTarToDir(mockFS, []byte("invalid tar data"), "/dest")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create gzip reader")
	})

	t.Run("MkdirError", func(t *testing.T) {
		mockFS := &TarTestMockFileSystem{
			errorOnMkdirAll: "/dest/dangerous_dir",
		}
		tarData := createTestTar(t, map[string]string{
			"dangerous_dir/file.txt": "content",
		})

		err := ExtractTarToDir(mockFS, tarData, "/dest")
		require.Error(t, err)
		// The error could be for either "parent directories" or "directory"
		// Check for either message to make the test more robust
		assert.Contains(t, err.Error(), "failed to create")
	})
}
