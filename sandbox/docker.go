// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. The DockerExecutor runs code in Docker containers
// with security constraints including resource limits, network isolation, and
// read-only filesystems except for the working directory.
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
	"strings"
	"time"

	"go.uber.org/zap"
)

// DockerExecutor implements SandboxExecutor using Docker
type DockerExecutor struct {
	logger *zap.Logger
	config *Config
	langEnvs *LanguageEnvironments
}

// Config holds configuration for the Docker executor
type Config struct {
	TimeoutSec        int
	MemoryMB          int
	NetworkEnabled    bool
	MaxArtifactSizeMB int
}

// LanguageEnvironments holds environment variables for different languages
type LanguageEnvironments struct {
	Python map[string]string
	NodeJS map[string]string
	Go     map[string]string
	CPP    map[string]string
}

// NewDockerExecutor creates a new DockerExecutor
func NewDockerExecutor(logger *zap.Logger, config *Config, langEnvs *LanguageEnvironments) *DockerExecutor {
	return &DockerExecutor{
		logger: logger,
		config: config,
		langEnvs: langEnvs,
	}
}

// Execute runs the code in a Docker container
func (d *DockerExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	// Create a temporary directory for this execution
	tempDir, err := os.MkdirTemp("", "codebox-exec-*")
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Prepare the working directory
	workdirPath := filepath.Join(tempDir, "workdir")
	if err := os.MkdirAll(workdirPath, 0755); err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create workdir: %w", err)
	}

	// If workdir_tar is provided, extract it
	if len(req.WorkdirTar) > 0 {
		if err := d.extractTarToDir(req.WorkdirTar, workdirPath); err != nil {
			return ExecuteResult{}, fmt.Errorf("failed to extract workdir_tar: %w", err)
		}
	}

	// Write the user code to the appropriate file based on language
	codeFileName, err := d.getCodeFileName(req.Language)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("invalid language: %w", err)
	}

	codeFilePath := filepath.Join(workdirPath, codeFileName)
	if err := d.writeUserCode(req.Language, req.Code, codeFilePath); err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to write user code: %w", err)
	}

	// Build the Docker command
	imageName := d.getLanguageImage(req.Language)
	containerName := fmt.Sprintf("codebox-exec-%d", time.Now().UnixNano())

	// Prepare Docker run command with security restrictions
	cmdArgs := []string{
		"docker", "run",
		"--name", containerName,
		"--rm", // Remove container after execution
		"-v", fmt.Sprintf("%s:/workdir", workdirPath),
		"--workdir", "/workdir",
		"--memory", fmt.Sprintf("%dm", d.config.MemoryMB),
		"--network", "none", // Disable network by default
		"--ulimit", "fsize=100000000", // Limit file size to 100MB
		"--ulimit", "cpu=10", // Limit CPU time (10 seconds)
		"--security-opt", "no-new-privileges:true",
		"--user", "nobody", // Run as non-privileged user
		"--cap-drop", "ALL", // Drop all capabilities
	}

	// Enable network if configured
	if d.config.NetworkEnabled {
		cmdArgs = append(cmdArgs, "--network", "bridge")
	}

	// Add the image and command to execute
	cmdArgs = append(cmdArgs, imageName)

	// Add environment variables based on language
	envVars, err := d.getEnvironmentVariables(req.Language)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to get environment variables: %w", err)
	}
	
	for key, value := range envVars {
		cmdArgs = append(cmdArgs, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Determine the command to run based on language
	runCmd, err := d.getRunCommand(req.Language)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to get run command: %w", err)
	}

	cmdArgs = append(cmdArgs, "sh", "-c", runCmd)

	// Execute with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(d.config.TimeoutSec)*time.Second)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctxWithTimeout, cmdArgs[0], cmdArgs[1:]...)

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Start the command
	err = cmd.Run()

	// If the context timed out, handle it explicitly
	if ctxWithTimeout.Err() == context.DeadlineExceeded {
		// Try to stop the container if it's still running
		_ = exec.Command("docker", "stop", containerName).Run()
		
		return ExecuteResult{
			Stdout:       stdoutBuf.String(),
			Stderr:       stderrBuf.String() + "\nExecution timed out",
			ExitCode:     1,
			ArtifactsTar: []byte{}, // Empty artifacts on timeout
		}, nil
	}

	// Get the exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return ExecuteResult{}, fmt.Errorf("failed to execute container: %w", err)
		}
	}

	// Create artifacts tar from the workdir
	artifactsTar, err := d.createTarFromDir(workdirPath)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create artifacts tar: %w", err)
	}

	// Check artifact size
	if len(artifactsTar) > d.config.MaxArtifactSizeMB*1024*1024 {
		return ExecuteResult{}, fmt.Errorf("artifacts size exceeds limit: %d bytes > %d bytes", 
			len(artifactsTar), d.config.MaxArtifactSizeMB*1024*1024)
	}

	return ExecuteResult{
		Stdout:       stdoutBuf.String(),
		Stderr:       stderrBuf.String(),
		ExitCode:     exitCode,
		ArtifactsTar: artifactsTar,
	}, nil
}

// Helper functions

func (d *DockerExecutor) getCodeFileName(language string) (string, error) {
	switch language {
	case "python":
		return "main.py", nil
	case "nodejs":
		return "index.js", nil
	case "go":
		return "main.go", nil
	case "cpp":
		return "main.cpp", nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func (d *DockerExecutor) writeUserCode(language, code, filePath string) error {
	// Apply hooks for interpreted languages
	finalCode := d.applyHooks(language, code)

	return os.WriteFile(filePath, []byte(finalCode), 0644)
}

func (d *DockerExecutor) applyHooks(language, code string) string {
	// In a real implementation, these would come from config
	var prefixCode, postfixCode string
	
	switch language {
	case "python":
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
	case "nodejs":
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

func (d *DockerExecutor) getLanguageImage(language string) string {
	// In a real implementation, this would come from config
	switch language {
	case "python":
		return "python:3.11-slim"
	case "nodejs":
		return "node:20-alpine"
	case "go":
		return "golang:1.23-alpine"
	case "cpp":
		return "gcc:13"
	default:
		return "alpine:latest" // fallback
	}
}

func (d *DockerExecutor) getRunCommand(language string) (string, error) {
	switch language {
	case "python":
		return "python main.py", nil
	case "nodejs":
		return "node index.js", nil
	case "go":
		return "go build -o app main.go && ./app", nil
	case "cpp":
		return "g++ -std=c++17 -O2 -o app main.cpp && ./app", nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func (d *DockerExecutor) getEnvironmentVariables(language string) (map[string]string, error) {
	if d.langEnvs == nil {
		return map[string]string{}, nil
	}
	
	switch language {
	case "python":
		if d.langEnvs.Python != nil {
			return d.langEnvs.Python, nil
		}
		return map[string]string{}, nil
	case "nodejs":
		if d.langEnvs.NodeJS != nil {
			return d.langEnvs.NodeJS, nil
		}
		return map[string]string{}, nil
	case "go":
		if d.langEnvs.Go != nil {
			return d.langEnvs.Go, nil
		}
		return map[string]string{}, nil
	case "cpp":
		if d.langEnvs.CPP != nil {
			return d.langEnvs.CPP, nil
		}
		return map[string]string{}, nil
	default:
		return map[string]string{}, fmt.Errorf("unsupported language: %s", language)
	}
}

func (d *DockerExecutor) extractTarToDir(tarData []byte, destDir string) error {
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
		filePath := filepath.Join(destDir, header.Name)
		if !strings.HasPrefix(filePath, destDir) {
			return fmt.Errorf("invalid file path in tar: %s", header.Name)
		}

		// Prevent absolute paths and directory traversal
		if filepath.IsAbs(header.Name) || strings.Contains(header.Name, "..") {
			return fmt.Errorf("unsafe path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories if they don't exist
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
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

			outFile.Close()
		default:
			return fmt.Errorf("unsupported file type in tar: %c", header.Typeflag)
		}
	}

	return nil
}

func (d *DockerExecutor) createTarFromDir(srcDir string) ([]byte, error) {
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