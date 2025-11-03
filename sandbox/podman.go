// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. The PodmanExecutor runs code in Podman containers
// with security constraints similar to the Docker executor.
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

// PodmanExecutor implements SandboxExecutor using Podman
type PodmanExecutor struct {
	logger   *zap.Logger
	config   *Config
	langEnvs *LanguageEnvironments
}

// NewPodmanExecutor creates a new PodmanExecutor
func NewPodmanExecutor(logger *zap.Logger, config *Config, langEnvs *LanguageEnvironments) *PodmanExecutor {
	return &PodmanExecutor{
		logger:   logger,
		config:   config,
		langEnvs: langEnvs,
	}
}

// Execute runs the code in a Podman container
//
//nolint:gocyclo,funlen,gocritic // Complex function intentionally handles multiple languages with large request struct
func (p *PodmanExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
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
		if extractErr := p.extractTarToDir(req.WorkdirTar, workdirPath); extractErr != nil {
			return ExecuteResult{}, fmt.Errorf("failed to extract workdir_tar: %w", extractErr)
		}
	}

	// Write the user code to the appropriate file based on language
	codeFileName, getErr := p.getCodeFileName(req.Language)
	if getErr != nil {
		return ExecuteResult{}, fmt.Errorf("invalid language: %w", getErr)
	}

	codeFilePath := filepath.Join(workdirPath, codeFileName)
	if writeErr := p.writeUserCode(req.Language, req.Code, codeFilePath); writeErr != nil {
		return ExecuteResult{}, fmt.Errorf("failed to write user code: %w", writeErr)
	}

	// Build the Podman command
	imageName := p.getLanguageImage(req.Language)
	containerName := fmt.Sprintf("codebox-exec-%d", time.Now().UnixNano())

	// Prepare Podman run command with security restrictions
	cmdArgs := []string{
		"podman", "run",
		"--name", containerName,
		"--rm", // Remove container after execution
		"-v", fmt.Sprintf("%s:/workdir", workdirPath),
		"--workdir", "/workdir",
		"--memory", fmt.Sprintf("%dm", p.config.MemoryMB),
		"--network", "none", // Disable network by default
		"--ulimit", "fsize=100000000", // Limit file size to 100MB
		"--ulimit", "cpu=10", // Limit CPU time (10 seconds)
		"--security-opt", "no-new-privileges:true",
		"--user", "nobody", // Run as non-privileged user
		"--cap-drop", "ALL", // Drop all capabilities
	}

	// Enable network if configured
	if p.config.NetworkEnabled {
		cmdArgs = append(cmdArgs, "--network", "bridge")
	}

	// Add the image and command to execute
	cmdArgs = append(cmdArgs, imageName)

	// Add environment variables based on language
	envVars, err := p.getEnvironmentVariables(req.Language)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to get environment variables: %w", err)
	}

	for key, value := range envVars {
		cmdArgs = append(cmdArgs, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Determine the command to run based on language
	runCmd, err := p.getRunCommand(req.Language)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to get run command: %w", err)
	}

	cmdArgs = append(cmdArgs, "sh", "-c", runCmd)

	// Execute with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(p.config.TimeoutSec)*time.Second)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctxWithTimeout, cmdArgs[0], cmdArgs[1:]...) //nolint:gosec // Command is constructed from validated inputs

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Start the command
	err = cmd.Run()

	// If the context timed out, handle it explicitly
	if ctxWithTimeout.Err() == context.DeadlineExceeded {
		// Try to stop the container if it's still running
		_ = exec.CommandContext(ctx, "podman", "stop", containerName).Run()

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
	artifactsTar, err := p.createTarFromDir(workdirPath)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create artifacts tar: %w", err)
	}

	// Check artifact size
	if len(artifactsTar) > p.config.MaxArtifactSizeMB*1024*1024 {
		return ExecuteResult{}, fmt.Errorf("artifacts size exceeds limit: %d bytes > %d bytes",
			len(artifactsTar), p.config.MaxArtifactSizeMB*MaxArtifactSizeMul)
	}

	return ExecuteResult{
		Stdout:       stdoutBuf.String(),
		Stderr:       stderrBuf.String(),
		ExitCode:     exitCode,
		ArtifactsTar: artifactsTar,
	}, nil
}

// Helper functions (same as Docker implementation)
func (*PodmanExecutor) getCodeFileName(language string) (string, error) {
	return GetCodeFileName(language)
}

func (*PodmanExecutor) writeUserCode(language, code, filePath string) error {
	// Apply hooks for interpreted languages
	finalCode := ApplyHooks(language, code)

	return os.WriteFile(filePath, []byte(finalCode), FilePermission)
}

func (*PodmanExecutor) getLanguageImage(language string) string {
	// In a real implementation, this would come from config
	switch language {
	case LanguagePython:
		return "python:3.11-slim"
	case LanguageNodeJS:
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
func (*PodmanExecutor) getRunCommand(language string) (string, error) {
	return GetRunCommand(language)
}

func (p *PodmanExecutor) extractTarToDir(tarData []byte, destDir string) error {
	return ExtractTarToDir(p.logger, tarData, destDir)
}

func (*PodmanExecutor) createTarFromDir(srcDir string) ([]byte, error) {
	return CreateTarFromDir(srcDir)
}

func (p *PodmanExecutor) getEnvironmentVariables(language string) (map[string]string, error) {
	return GetEnvironmentVariables(p.langEnvs, language)
}
