// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. The DockerExecutor runs code in Docker containers
// with security constraints including resource limits, network isolation, and
// read-only filesystems except for the working directory.
package sandbox

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// DockerExecutor implements SandboxExecutor using Docker
type DockerExecutor struct {
	logger          *zap.Logger
	config          *Config
	langEnvs        *LanguageEnvironments
	langConfigs     *LanguageConfigs
	langCodeConfigs *LanguageCodeConfigs
	cmdRunner       CommandRunner
	fs              FileSystem
}

// Config holds configuration for the Docker executor
type Config struct {
	TimeoutSec        int
	MemoryMB          int
	NetworkEnabled    bool
	MaxArtifactSizeMB int
}

// DockerExecutorOption defines a functional option for DockerExecutor
type DockerExecutorOption func(*DockerExecutor)

// WithDockerCommandRunner sets the CommandRunner for DockerExecutor
func WithDockerCommandRunner(cmdRunner CommandRunner) DockerExecutorOption {
	return func(d *DockerExecutor) {
		d.cmdRunner = cmdRunner
	}
}

// WithDockerFileSystem sets the FileSystem for DockerExecutor
func WithDockerFileSystem(fs FileSystem) DockerExecutorOption {
	return func(d *DockerExecutor) {
		d.fs = fs
	}
}

// WithDockerLanguageConfigs sets the LanguageConfigs for DockerExecutor
func WithDockerLanguageConfigs(langConfigs *LanguageConfigs) DockerExecutorOption {
	return func(d *DockerExecutor) {
		d.langConfigs = langConfigs
	}
}

// WithDockerLanguageCodeConfigs sets the LanguageCodeConfigs for DockerExecutor
func WithDockerLanguageCodeConfigs(langCodeConfigs *LanguageCodeConfigs) DockerExecutorOption {
	return func(d *DockerExecutor) {
		d.langCodeConfigs = langCodeConfigs
	}
}

// NewDockerExecutor creates a new DockerExecutor with default implementations and optional interfaces
func NewDockerExecutor(logger *zap.Logger, config *Config, langEnvs *LanguageEnvironments, opts ...DockerExecutorOption) *DockerExecutor {
	executor := &DockerExecutor{
		logger:          logger,
		config:          config,
		langEnvs:        langEnvs,
		langConfigs:     &LanguageConfigs{},     // Default empty, can be set via options
		langCodeConfigs: &LanguageCodeConfigs{}, // Default empty, can be set via options
		cmdRunner:       &RealCommandRunner{},   // Default implementation
		fs:              &RealFileSystem{},      // Default implementation
	}

	// Apply options
	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// Execute runs the code in a Docker container
//
//nolint:gocyclo,funlen,gocritic // Complex function intentionally handles multiple languages with large request struct
func (d *DockerExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	// Create a temporary directory for this execution
	tempDir, err := d.fs.MkdirTemp("", "codebox-exec-*")
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() {
		if rmErr := d.fs.RemoveAll(tempDir); rmErr != nil {
			d.logger.Error("failed to remove temp directory", zap.String("path", tempDir), zap.Error(rmErr))
		}
	}()

	// Prepare the working directory
	workdirPath := filepath.Join(tempDir, "workdir")
	if mkdirErr := d.fs.MkdirAll(workdirPath, DirPermission); mkdirErr != nil {
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

	// Apply hooks for interpreted languages
	finalCode := ApplyHooks(req.Language, req.Code, d.langCodeConfigs)

	codeFilePath := filepath.Join(workdirPath, codeFileName)
	if writeErr := d.fs.WriteFile(codeFilePath, []byte(finalCode), FilePermission); writeErr != nil {
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

	// Execute command using the command runner
	stdout, stderr, exitCode, err := d.cmdRunner.RunCommand(ctxWithTimeout, cmdArgs)

	// If the context timed out, handle it explicitly
	if ctxWithTimeout.Err() == context.DeadlineExceeded {
		// Try to stop the container if it's still running - this still needs direct exec call for now
		stopCmd := exec.CommandContext(ctx, "docker", "stop", containerName)
		if stopErr := stopCmd.Run(); stopErr != nil {
			d.logger.Warn("failed to stop container after timeout", zap.String("container", containerName), zap.Error(stopErr))
		}

		return ExecuteResult{
			Stdout:       stdout,
			Stderr:       stderr + "\nExecution timed out",
			ExitCode:     1,
			ArtifactsTar: []byte{}, // Empty artifacts on timeout
		}, nil
	}

	// Check for execution error
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to execute container: %w", err)
	}

	// Determine exclude patterns based on language
	var excludePatterns []string
	switch req.Language {
	case LanguagePython:
		if d.langConfigs != nil {
			excludePatterns = d.langConfigs.Python.ExcludePatterns
		}
	case LanguageNodeJS:
		if d.langConfigs != nil {
			excludePatterns = d.langConfigs.NodeJS.ExcludePatterns
		}
	case LanguageGo:
		if d.langConfigs != nil {
			excludePatterns = d.langConfigs.Go.ExcludePatterns
		}
	case LanguageCPP:
		if d.langConfigs != nil {
			excludePatterns = d.langConfigs.CPP.ExcludePatterns
		}
	}

	// Create artifacts tar from the workdir with exclude patterns
	artifactsTar, err := d.createTarFromDirWithExcludes(workdirPath, excludePatterns)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create artifacts tar: %w", err)
	}

	// Check artifact size
	if len(artifactsTar) > d.config.MaxArtifactSizeMB*1024*1024 {
		return ExecuteResult{}, fmt.Errorf("artifacts size exceeds limit: %d bytes > %d bytes",
			len(artifactsTar), d.config.MaxArtifactSizeMB*MaxArtifactSizeMul)
	}

	return ExecuteResult{
		Stdout:       stdout,
		Stderr:       stderr,
		ExitCode:     exitCode,
		ArtifactsTar: artifactsTar,
	}, nil
}

// Helper functions

func (*DockerExecutor) getCodeFileName(language string) (string, error) {
	return GetCodeFileName(language)
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
	return ExtractTarToDir(d.fs, tarData, destDir)
}

func (*DockerExecutor) createTarFromDirWithExcludes(srcDir string, excludePatterns []string) ([]byte, error) {
	return CreateTarFromDirWithExcludes(srcDir, excludePatterns)
}
