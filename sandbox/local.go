// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. The LocalExecutor runs code directly on the host
// system (for development only) with security constraints applied through process
// environments and resource limits.
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

// LocalExecutor implements SandboxExecutor using local execution (for development only)
type LocalExecutor struct {
	logger    *zap.Logger
	config    *Config
	cfg       *config.Config // Reference to the full configuration
	cmdRunner CommandRunner
	fs        FileSystem
}

// LocalExecutorOption defines a functional option for LocalExecutor
type LocalExecutorOption func(*LocalExecutor)

// WithLocalCommandRunner sets the CommandRunner for LocalExecutor
func WithLocalCommandRunner(cmdRunner CommandRunner) LocalExecutorOption {
	return func(l *LocalExecutor) {
		l.cmdRunner = cmdRunner
	}
}

// WithLocalFileSystem sets the FileSystem for LocalExecutor
func WithLocalFileSystem(fs FileSystem) LocalExecutorOption {
	return func(l *LocalExecutor) {
		l.fs = fs
	}
}

// NewLocalExecutor creates a new LocalExecutor with default implementations and optional interfaces
func NewLocalExecutor(logger *zap.Logger, executorConfig *Config, cfg *config.Config, opts ...LocalExecutorOption) *LocalExecutor {
	executor := &LocalExecutor{
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

// Execute runs the code locally (WARNING: This is not secure and should only be used for development)
//
//nolint:gocyclo,funlen,gocritic // Complex function intentionally handles multiple languages with large request struct
func (l *LocalExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
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
		if extractErr := l.extractTarToDir(req.WorkdirTar, workdirPath); extractErr != nil {
			return ExecuteResult{}, fmt.Errorf("failed to extract workdir_tar: %w", extractErr)
		}
	}

	// Write the user code to the appropriate file based on language
	codeFileName, getErr := l.getCodeFileName(req.Language)
	if getErr != nil {
		return ExecuteResult{}, fmt.Errorf("invalid language: %w", getErr)
	}

	// Apply hooks for interpreted languages using config
	finalCode := l.applyHooksFromConfig(req.Language, req.Code)

	codeFilePath := filepath.Join(workdirPath, codeFileName)
	if writeErr := l.fs.WriteFile(codeFilePath, []byte(finalCode), FilePermission); writeErr != nil {
		return ExecuteResult{}, fmt.Errorf("failed to write user code: %w", writeErr)
	}

	// Execute with timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(l.config.TimeoutSec)*time.Second)
	defer cancel()

	// Build the command based on language
	var cmd *exec.Cmd
	switch req.Language {
	case LanguagePython:
		cmd = exec.CommandContext(ctxWithTimeout, "python3", codeFilePath)
	case LanguageNodeJS:
		cmd = exec.CommandContext(ctxWithTimeout, "node", codeFilePath)
	case LanguageGo:
		// Build and run Go code
		//nolint:gosec // Building code is intended functionality
		buildCmd := exec.CommandContext(ctxWithTimeout, "go", "build", "-o", filepath.Join(workdirPath, "app"), codeFilePath)
		buildCmd.Dir = workdirPath
		if buildErr := buildCmd.Run(); buildErr != nil {
			return ExecuteResult{
				Stdout:       "",
				Stderr:       fmt.Sprintf("Build error: %v", buildErr),
				ExitCode:     1,
				ArtifactsTar: []byte{},
			}, nil
		}
		//nolint:gosec // Running built app is intended functionality
		cmd = exec.CommandContext(ctxWithTimeout, filepath.Join(workdirPath, "app"))
	case LanguageCPP:
		// Compile and run C++ code
		binaryPath := filepath.Join(workdirPath, "app")
		compileCmd := exec.CommandContext(ctxWithTimeout, "g++", "-std=c++17", "-O2", "-o", binaryPath, codeFilePath)
		compileCmd.Dir = workdirPath
		if compileErr := compileCmd.Run(); compileErr != nil {
			return ExecuteResult{
				Stdout:       "",
				Stderr:       fmt.Sprintf("Compile error: %v", compileErr),
				ExitCode:     1,
				ArtifactsTar: []byte{},
			}, nil
		}
		cmd = exec.CommandContext(ctxWithTimeout, binaryPath)
	default:
		return ExecuteResult{}, fmt.Errorf("unsupported language: %s", req.Language)
	}

	// Set working directory
	cmd.Dir = workdirPath

	// Set environment variables based on language
	envVars := l.getEnvironmentVariables(req.Language)

	// Start with existing environment
	cmd.Env = os.Environ()

	// Add custom environment variables
	for key, value := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Start the command
	err = cmd.Run()

	// If the context timed out, handle it explicitly
	if ctxWithTimeout.Err() == context.DeadlineExceeded {
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
			return ExecuteResult{}, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	// Determine exclude patterns based on language from config
	var excludePatterns []string
	if langConfig, exists := l.cfg.Languages[req.Language]; exists {
		excludePatterns = langConfig.ExcludePatterns
	}

	// Create artifacts tar from the workdir with exclude patterns
	artifactsTar, err := l.createTarFromDirWithExcludes(workdirPath, excludePatterns)
	if err != nil {
		return ExecuteResult{}, fmt.Errorf("failed to create artifacts tar: %w", err)
	}

	// Check artifact size
	if len(artifactsTar) > l.config.MaxArtifactSizeMB*1024*1024 {
		return ExecuteResult{}, fmt.Errorf("artifacts size exceeds limit: %d bytes > %d bytes",
			len(artifactsTar), l.config.MaxArtifactSizeMB*MaxArtifactSizeMul)
	}

	return ExecuteResult{
		Stdout:       stdoutBuf.String(),
		Stderr:       stderrBuf.String(),
		ExitCode:     exitCode,
		ArtifactsTar: artifactsTar,
	}, nil
}

// Helper functions (same as other executors)
func (*LocalExecutor) getCodeFileName(language string) (string, error) {
	return GetCodeFileName(language)
}

func (l *LocalExecutor) extractTarToDir(tarData []byte, destDir string) error {
	return ExtractTarToDir(l.fs, tarData, destDir)
}

func (*LocalExecutor) createTarFromDirWithExcludes(srcDir string, excludePatterns []string) ([]byte, error) {
	return CreateTarFromDirWithExcludes(srcDir, excludePatterns)
}

func (l *LocalExecutor) getEnvironmentVariables(language string) map[string]string {
	if langConfig, exists := l.cfg.Languages[language]; exists && langConfig.Environment != nil {
		return langConfig.Environment
	}
	return make(map[string]string)
}

// applyHooksFromConfig applies hooks for code execution based on language from config
func (l *LocalExecutor) applyHooksFromConfig(language, code string) string {
	var prefixCode, postfixCode string

	if langConfig, exists := l.cfg.Languages[language]; exists {
		prefixCode = langConfig.PrefixCode
		postfixCode = langConfig.PostfixCode
	}

	return prefixCode + code + postfixCode
}
