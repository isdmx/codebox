package sandbox

import (
	"context"
)

// ExecuteRequest represents the parameters for code execution
type ExecuteRequest struct {
	Language     string
	Code         string
	WorkdirTar   []byte // decoded base64
	TimeoutSec   int
	MemoryMB     int
	Network      bool
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