// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. It supports multiple backends including
// Docker, Podman, and local execution (for development).
package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// ExecuteRequest represents the parameters for code execution
type ExecuteRequest struct {
	Language   string
	Code       string
	WorkdirTar []byte // decoded base64
	TimeoutSec int
	MemoryMB   int
	Network    bool
}

// ExecuteResult represents the result of code execution
type ExecuteResult struct {
	Stdout       string
	Stderr       string
	ExitCode     int
	ArtifactsTar []byte // raw tar.gz
}

// SandboxExecutor defines the interface for sandbox execution
type SandboxExecutor interface {
	Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error)
}

// CommandRunner defines an interface for executing system commands
type CommandRunner interface {
	RunCommand(ctx context.Context, args []string) (stdout, stderr string, exitCode int, err error)
}

// RealCommandRunner implements CommandRunner using actual exec commands
type RealCommandRunner struct{}

// RunCommand executes the given command with arguments
func (RealCommandRunner) RunCommand(ctx context.Context, args []string) (stdout, stderr string, exitCode int, err error) {
	if len(args) < 1 {
		return "", "", 0, fmt.Errorf("no command provided")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // Safe as this is controlled input

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	exitCode = 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return "", "", 0, err
		}
	}

	return stdoutBuf.String(), stderrBuf.String(), exitCode, nil
}

// FileSystem defines an interface for file system operations
type FileSystem interface {
	MkdirTemp(dir, pattern string) (string, error)
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	ReadFile(filename string) ([]byte, error)
	RemoveAll(path string) error
	FileExists(path string) (bool, error)
}

// RealFileSystem implements FileSystem using actual file system operations
type RealFileSystem struct{}

func (RealFileSystem) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func (RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (RealFileSystem) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

func (RealFileSystem) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (RealFileSystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (RealFileSystem) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// LanguageEnvironments holds environment variables for different languages
type LanguageEnvironments struct {
	Python map[string]string
	NodeJS map[string]string
	Go     map[string]string
	CPP    map[string]string
}

// LanguageName constants
const (
	LanguagePython = "python"
	LanguageNodeJS = "nodejs"
	LanguageGo     = "go"
	LanguageCPP    = "cpp"
)

// File permission and size constants
const (
	DirPermission      = 0o755
	FilePermission     = 0o600
	BytesPerKB         = 1024
	MaxArtifactSizeMul = 1024 * 1024 // 1 MB multiplier
)

// Filename constants
const (
	FilenamePython = "main.py"
	FilenameNodeJS = "index.js"
	FilenameGo     = "main.go"
	FilenameCPP    = "main.cpp"
)

// GetCodeFileName returns the appropriate filename based on the language
func GetCodeFileName(language string) (string, error) {
	switch language {
	case LanguagePython:
		return FilenamePython, nil
	case LanguageNodeJS:
		return FilenameNodeJS, nil
	case LanguageGo:
		return FilenameGo, nil
	case LanguageCPP:
		return FilenameCPP, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

// GetRunCommand returns the appropriate run command based on the language
func GetRunCommand(language string) (string, error) {
	switch language {
	case LanguagePython:
		return fmt.Sprintf("python %s", FilenamePython), nil
	case LanguageNodeJS:
		return fmt.Sprintf("node %s", FilenameNodeJS), nil
	case LanguageGo:
		return fmt.Sprintf("go build -o app %s && ./app", FilenameGo), nil
	case LanguageCPP:
		return fmt.Sprintf("g++ -std=c++17 -O2 -o app %s && ./app", FilenameCPP), nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

// ApplyHooks applies hooks for code execution based on language
func ApplyHooks(language, code string, codeConfigs *LanguageCodeConfigs) string {
	var prefixCode, postfixCode string

	if codeConfigs != nil {
		switch language {
		case LanguagePython:
			prefixCode = codeConfigs.Python.PrefixCode
			postfixCode = codeConfigs.Python.PostfixCode
		case LanguageNodeJS:
			prefixCode = codeConfigs.NodeJS.PrefixCode
			postfixCode = codeConfigs.NodeJS.PostfixCode
		case LanguageGo:
			prefixCode = codeConfigs.Go.PrefixCode
			postfixCode = codeConfigs.Go.PostfixCode
		case LanguageCPP:
			prefixCode = codeConfigs.CPP.PrefixCode
			postfixCode = codeConfigs.CPP.PostfixCode
		}
	}

	return prefixCode + code + postfixCode
}

// GetEnvironmentVariables retrieves environment variables for the specified language
func GetEnvironmentVariables(langEnvs *LanguageEnvironments, language string) (map[string]string, error) {
	if langEnvs == nil {
		return make(map[string]string), nil
	}

	switch language {
	case LanguagePython:
		if langEnvs.Python != nil {
			return langEnvs.Python, nil
		}
		return make(map[string]string), nil
	case LanguageNodeJS:
		if langEnvs.NodeJS != nil {
			return langEnvs.NodeJS, nil
		}
		return make(map[string]string), nil
	case LanguageGo:
		if langEnvs.Go != nil {
			return langEnvs.Go, nil
		}
		return make(map[string]string), nil
	case LanguageCPP:
		if langEnvs.CPP != nil {
			return langEnvs.CPP, nil
		}
		return make(map[string]string), nil
	default:
		return make(map[string]string), fmt.Errorf("unsupported language: %s", language)
	}
}

// ExtractTarToDir extracts tar.gz data to the destination directory safely
func ExtractTarToDir(fs FileSystem, tarData []byte, destDir string) error {
	// Decompress the tar.gz data
	gzipReader, err := gzip.NewReader(bytes.NewReader(tarData))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		// Validate the path to prevent directory traversal
		// Clean the path to resolve any relative paths like ../
		cleanName := filepath.Clean(header.Name)
		if strings.Contains(cleanName, "..") {
			return fmt.Errorf("unsafe relative path in tar: %s", header.Name)
		}

		filePath := filepath.Join(destDir, cleanName)
		if !strings.HasPrefix(filePath, destDir) {
			return fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		// Prevent absolute paths
		if filepath.IsAbs(header.Name) {
			return fmt.Errorf("absolute path not allowed in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := fs.MkdirAll(filePath, DirPermission); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories if they don't exist
			if err := fs.MkdirAll(filepath.Dir(filePath), DirPermission); err != nil {
				return fmt.Errorf("failed to create parent directories: %w", err)
			}

			// Read file content
			fileContent := make([]byte, header.Size)
			_, err := io.ReadFull(tarReader, fileContent)
			if err != nil {
				return fmt.Errorf("failed to read file content: %w", err)
			}

			// Write the file content using the file system interface
			if err := fs.WriteFile(filePath, fileContent, FilePermission); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
		default:
			return fmt.Errorf("unsupported file type in tar: %c", header.Typeflag)
		}
	}

	return nil
}

// LanguageCodeConfig holds the prefix and postfix code for a language
type LanguageCodeConfig struct {
	PrefixCode  string
	PostfixCode string
}

// LanguageConfig holds configuration for a specific language including exclude patterns
type LanguageConfig struct {
	ExcludePatterns []string
}

// LanguageCodeConfigs holds code configurations for different languages
type LanguageCodeConfigs struct {
	Python LanguageCodeConfig
	NodeJS LanguageCodeConfig
	Go     LanguageCodeConfig
	CPP    LanguageCodeConfig
}

// LanguageConfigs holds configurations for different languages
type LanguageConfigs struct {
	Python LanguageConfig
	NodeJS LanguageConfig
	Go     LanguageConfig
	CPP    LanguageConfig
}

// CreateTarFromDir creates a tar.gz archive from a directory
func CreateTarFromDir(srcDir string) ([]byte, error) {
	return CreateTarFromDirWithExcludes(srcDir, nil)
}

// CreateTarFromDirWithExcludes creates a tar.gz archive from a directory with excluded patterns
func CreateTarFromDirWithExcludes(srcDir string, excludePatterns []string) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	err := filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path from the source directory
		relPath, err := filepath.Rel(srcDir, file)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		// Check if the file should be excluded based on patterns
		if shouldExcludeFile(relPath, excludePatterns) {
			if fi.IsDir() {
				// If it's a directory and matches an exclude pattern, skip the entire directory
				return filepath.SkipDir
			}
			// If it's a file that matches an exclude pattern, skip it
			return nil
		}

		// Create a header for the file
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's not a directory, write the file content
		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			defer data.Close()

			if _, err := io.Copy(tarWriter, data); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := tarWriter.Close(); err != nil {
		return nil, err
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// shouldExcludeFile checks if a file should be excluded based on the exclude patterns
func shouldExcludeFile(relPath string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		// Special case: if relPath and pattern are exactly the same and pattern doesn't have a trailing slash,
		// skip this pattern for common directory names to avoid matching directory entries with file patterns
		if relPath == pattern && !strings.HasSuffix(pattern, "/") {
			// Check if this might be a directory name (common directory names) and skip in that case
			if isCommonDirectoryName(pattern) {
				continue
			}
		}

		// If pattern ends with '/', it's a directory pattern
		if strings.HasSuffix(pattern, "/") {
			dirPattern := strings.TrimSuffix(pattern, "/")
			// Check if the relative path matches the directory exactly or starts with it followed by a slash
			if relPath == dirPattern || strings.HasPrefix(relPath, dirPattern+"/") {
				return true
			}
			// Also check if the directory appears anywhere in the path (e.g. "frontend/node_modules/file.js" contains "node_modules/")
			// Split the path and check if any directory component matches the pattern
			relPathParts := strings.Split(relPath, "/")
			if slices.Contains(relPathParts, dirPattern) {
				return true
			}
		} else {
			// Regular pattern (not directory-specific) - match against the basename
			if match, err := filepath.Match(pattern, filepath.Base(relPath)); err == nil && match {
				return true
			}
			// Also match if the entire path matches the pattern exactly
			if match, err := filepath.Match(pattern, relPath); err == nil && match {
				return true
			}
		}
	}
	return false
}

// isCommonDirectoryName checks if a name is commonly used for directories
func isCommonDirectoryName(name string) bool {
	// Common directory names that should not be matched by non-directory patterns
	commonDirNames := map[string]bool{
		"node_modules":  true,
		"__pycache__":   true,
		".git":          true,
		".svn":          true,
		".hg":           true,
		"build":         true,
		"dist":          true,
		"target":        true,
		"bin":           true,
		"obj":           true,
		"vendor":        true,
		".pytest_cache": true,
	}

	return commonDirNames[name]
}
