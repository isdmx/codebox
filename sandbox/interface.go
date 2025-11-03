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
	"path/filepath"
	"strings"

	"go.uber.org/zap"
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
	DirPermission      = 0755
	FilePermission     = 0600
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
func ApplyHooks(language, code string) string {
	// In a real implementation, these would come from config
	var prefixCode, postfixCode string

	switch language {
	case LanguagePython:
		// Add timeout and security hooks for Python
		prefixCode = `import signal, sys, resource

def timeout_handler(signum, frame):
    print('Execution timeout!')
    sys.exit(1)

signal.signal(signal.SIGALRM, timeout_handler)
signal.alarm(10)  # Set timeout to 10 seconds

# Limit memory usage
resource.setrlimit(resource.RLIMIT_AS, (512*1024*1024, 512*1024*1024))  # 512MB limit

`
		postfixCode = `
signal.alarm(0)  # Cancel the alarm
sys.stdout.flush()
sys.stderr.flush()
`
	case LanguageNodeJS:
		prefixCode = `// Set timeout for Node.js execution
setTimeout(() => {
  console.log('Execution timeout!');
  process.exit(1);
}, 10000);  // 10 seconds

// Additional security could be added here
`
		postfixCode = ``
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
//
//nolint:gocyclo // Function needs to handle different file types and validation in tar extraction
func ExtractTarToDir(logger *zap.Logger, tarData []byte, destDir string) error {
	// Decompress the tar.gz data
	gzipReader, err := gzip.NewReader(bytes.NewReader(tarData))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil && logger != nil {
			logger.Error("Failed to close gzip reader", zap.Error(closeErr))
		}
	}()

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
			if err := os.MkdirAll(filePath, DirPermission); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories if they don't exist
			if err := os.MkdirAll(filepath.Dir(filePath), DirPermission); err != nil {
				return fmt.Errorf("failed to create parent directories: %w", err)
			}

			// Write the file
			outFile, err := os.Create(filePath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.CopyN(outFile, tarReader, header.Size); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to copy file content: %w", err)
			}

			if err := outFile.Close(); err != nil {
				return fmt.Errorf("failed to close file: %w", err)
			}
		default:
			return fmt.Errorf("unsupported file type in tar: %c", header.Typeflag)
		}
	}

	return nil
}

// CreateTarFromDir creates a tar.gz archive from a directory
func CreateTarFromDir(srcDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzipWriter)

	err := filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create a header for the file
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// Update the name to be relative to the source directory
		relPath, err := filepath.Rel(srcDir, file)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
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
