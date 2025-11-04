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

	"github.com/isdmx/codebox/config"
)

// PodmanExecutor implements SandboxExecutor using Podman
type PodmanExecutor struct {
	logger    *zap.Logger
	config    *Config
	cfg       *config.Config // Reference to the full configuration
	cmdRunner CommandRunner
	fs        FileSystem
}

// PodmanExecutorOption defines a functional option for PodmanExecutor
type PodmanExecutorOption func(*PodmanExecutor)

// WithPodmanCommandRunner sets the CommandRunner for PodmanExecutor
func WithPodmanCommandRunner(cmdRunner CommandRunner) PodmanExecutorOption {
	return func(p *PodmanExecutor) {
		p.cmdRunner = cmdRunner
	}
}

// WithPodmanFileSystem sets the FileSystem for PodmanExecutor
func WithPodmanFileSystem(fs FileSystem) PodmanExecutorOption {
	return func(p *PodmanExecutor) {
		p.fs = fs
	}
}

// NewPodmanExecutor creates a new PodmanExecutor with default implementations and optional interfaces
func NewPodmanExecutor(logger *zap.Logger, executorConfig *Config, cfg *config.Config, opts ...PodmanExecutorOption) *PodmanExecutor {
	executor := &PodmanExecutor{
		logger:    logger,
		config:    executorConfig,
		cfg:       cfg,
		cmdRunner: &RealCommandRunner{}, // Default implementation
		fs:        &RealFileSystem{},    // Default implementation
	}

	// Apply options
	for _, opt := range opts {
		opt(executor)
	}

	return executor
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

	// Apply hooks for interpreted languages using config
	finalCode := p.applyHooksFromConfig(req.Language, req.Code)

	codeFilePath := filepath.Join(workdirPath, codeFileName)
	if writeErr := p.fs.WriteFile(codeFilePath, []byte(finalCode), FilePermission); writeErr != nil {
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

	// Add environment variables based on language from config
	envVars := p.getEnvironmentVariables(req.Language)

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

	// Determine exclude patterns based on language from config
	var excludePatterns []string
	if langConfig, exists := p.cfg.Languages[req.Language]; exists {
		excludePatterns = langConfig.ExcludePatterns
	}

	// Create artifacts tar from the workdir with exclude patterns
	artifactsTar, err := p.createTarFromDirWithExcludes(workdirPath, excludePatterns)
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

func (p *PodmanExecutor) getLanguageImage(language string) string {
	if langConfig, exists := p.cfg.Languages[language]; exists {
		if langConfig.Image != "" {
			return langConfig.Image
		}
	}

	// Fallback to defaults if not in config
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
	return ExtractTarToDir(p.fs, tarData, destDir)
}

func (*PodmanExecutor) createTarFromDirWithExcludes(srcDir string, excludePatterns []string) ([]byte, error) {
	return CreateTarFromDirWithExcludes(srcDir, excludePatterns)
}

func (p *PodmanExecutor) getEnvironmentVariables(language string) map[string]string {
	if langConfig, exists := p.cfg.Languages[language]; exists && langConfig.Environment != nil {
		return langConfig.Environment
	}
	return make(map[string]string)
}

// applyHooksFromConfig applies hooks for code execution based on language from config
func (p *PodmanExecutor) applyHooksFromConfig(language, code string) string {
	var prefixCode, postfixCode string

	if langConfig, exists := p.cfg.Languages[language]; exists {
		prefixCode = langConfig.PrefixCode
		postfixCode = langConfig.PostfixCode
	}

	return prefixCode + code + postfixCode
}
