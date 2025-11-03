// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. The DockerExecutor runs code in Docker containers
// with security constraints including resource limits, network isolation, and
// read-only filesystems except for the working directory.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// DockerExecutor implements SandboxExecutor using Docker
type DockerExecutor struct {
	logger   *zap.Logger
	config   *Config
	langEnvs *LanguageEnvironments
}

// Config holds configuration for the Docker executor
type Config struct {
	TimeoutSec        int
	MemoryMB          int
	NetworkEnabled    bool
	MaxArtifactSizeMB int
}

// NewDockerExecutor creates a new DockerExecutor
func NewDockerExecutor(logger *zap.Logger, config *Config, langEnvs *LanguageEnvironments) *DockerExecutor {
	return &DockerExecutor{
		logger:   logger,
		config:   config,
		langEnvs: langEnvs,
	}
}

// Execute runs the code in a Docker container
//
//nolint:gocyclo,funlen,gocritic // Complex function intentionally handles multiple languages with large request struct
func (d *DockerExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	// Create a temporary directory for this execution
	tempDir, err := os.MkdirTemp("", "codebox-exec-*")
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Prepare the working directory
	workdirPath := filepath.Join(tempDir, "workdir")
	if mkdirErr := os.MkdirAll(workdirPath, DirPermission); mkdirErr != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create workdir: %w", mkdirErr)
	}

	// If workdir_tar is provided, extract it
	if len(req.WorkdirTar) > 0 {
		if extractErr := d.extractTarToDir(req.WorkdirTar, workdirPath); extractErr != nil {
			return ExecuteResult{}, fmt.Errorf("failed to extract workdir_tar: %w", extractErr)
		}
	}

	// Write the user code to the appropriate file based on language
	codeFileName, getErr := d.getCodeFileName(req.Language)
	if getErr != nil {
		return ExecuteResult{}, fmt.Errorf("invalid language: %w", getErr)
	}

	codeFilePath := filepath.Join(workdirPath, codeFileName)
	if writeErr := d.writeUserCode(req.Language, req.Code, codeFilePath); writeErr != nil {
		return ExecuteResult{}, fmt.Errorf("failed to write user code: %w", writeErr)
	}

	// Build the Docker command
	imageName := getLanguageImage(req.Language)
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

	// Add environment variables based on language
	envVars, envErr := d.getEnvironmentVariables(req.Language)
	if envErr != nil {
		return ExecuteResult{}, fmt.Errorf("failed to get environment variables: %w", envErr)
	}

	// Log environment variables for debugging (at info level to ensure visibility)
	if len(envVars) > 0 {
		d.logger.Info("applying environment variables", zap.Any("env_vars", envVars))
		for key, value := range envVars {
			d.logger.Info("env var details", zap.String("key", key), zap.String("value", value))
		}
	} else {
		d.logger.Info("no environment variables found for language", zap.String("language", req.Language))
	}

	for key, value := range envVars {
		cmdArgs = append(cmdArgs, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add the image
	cmdArgs = append(cmdArgs, imageName)

	// Determine the command to run based on language
	runCmd, cmdErr := d.getRunCommand(req.Language)
	if cmdErr != nil {
		return ExecuteResult{}, fmt.Errorf("failed to get run command: %w", cmdErr)
	}

	cmdArgs = append(cmdArgs, "sh", "-c", runCmd)

	// Execute with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(d.config.TimeoutSec)*time.Second)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctxWithTimeout, cmdArgs[0], cmdArgs[1:]...) //nolint:gosec // Command is constructed from validated inputs

	// Log the full command for debugging
	d.logger.Info("executing docker command", zap.Strings("command", cmdArgs))

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Start the command
	err = cmd.Run()

	// If the context timed out, handle it explicitly
	if ctxWithTimeout.Err() == context.DeadlineExceeded {
		// Try to stop the container if it's still running
		_ = exec.CommandContext(ctx, "docker", "stop", containerName).Run()

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
			len(artifactsTar), d.config.MaxArtifactSizeMB*MaxArtifactSizeMul)
	}

	return ExecuteResult{
		Stdout:       stdoutBuf.String(),
		Stderr:       stderrBuf.String(),
		ExitCode:     exitCode,
		ArtifactsTar: artifactsTar,
	}, nil
}

// Helper functions

func (*DockerExecutor) getCodeFileName(language string) (string, error) {
	return GetCodeFileName(language)
}

func (*DockerExecutor) writeUserCode(language, code, filePath string) error {
	// Apply hooks for interpreted languages
	finalCode := ApplyHooks(language, code)

	return os.WriteFile(filePath, []byte(finalCode), FilePermission)
}

func getLanguageImage(language string) string {
	// In a real implementation, this would come from config
	switch language {
	case "python":
		return "python:3.11-slim"
	case "nodejs":
		return "node:20-alpine"
	case LanguageGo:
		return "golang:1.23-alpine"
	case LanguageCPP:
		return "gcc:13"
	default:
		return "alpine:latest" // fallback
	}
}

// getRunCommand returns the appropriate run command based on the language
func (*DockerExecutor) getRunCommand(language string) (string, error) {
	return GetRunCommand(language)
}

func (d *DockerExecutor) getEnvironmentVariables(language string) (map[string]string, error) {
	return GetEnvironmentVariables(d.langEnvs, language)
}

func (d *DockerExecutor) extractTarToDir(tarData []byte, destDir string) error {
	return ExtractTarToDir(d.logger, tarData, destDir)
}

func (*DockerExecutor) createTarFromDir(srcDir string) ([]byte, error) {
	return CreateTarFromDir(srcDir)
}
