// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. The factory provides a unified interface
// to create sandbox executors based on the configuration.
package sandbox

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/isdmx/codebox/config"
)

// NewExecutor creates an appropriate sandbox executor based on the configuration
func NewExecutor(logger *zap.Logger, cfg *config.Config) (SandboxExecutor, error) {
	executorConfig := Config{
		TimeoutSec:        cfg.Sandbox.TimeoutSec,
		MemoryMB:          cfg.Sandbox.MemoryMB,
		NetworkEnabled:    cfg.Sandbox.NetworkEnabled,
		MaxArtifactSizeMB: cfg.Sandbox.MaxArtifactSizeMB,
	}

	switch backend := cfg.Sandbox.Backend; backend {
	case "docker":
		return NewDockerExecutor(logger, &executorConfig, cfg), nil
	case "podman":
		return NewPodmanExecutor(logger, &executorConfig, cfg), nil
	case "local":
		return NewLocalExecutor(logger, &executorConfig, cfg), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}
