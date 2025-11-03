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
func NewExecutor(logger *zap.Logger, config *config.Config, backend string) (SandboxExecutor, error) {
	executorConfig := Config{
		TimeoutSec:        config.Sandbox.TimeoutSec,
		MemoryMB:          config.Sandbox.MemoryMB,
		NetworkEnabled:    config.Sandbox.NetworkEnabled,
		MaxArtifactSizeMB: config.Sandbox.MaxArtifactSizeMB,
	}

	// Create language environments from config
	langEnvs := &LanguageEnvironments{
		Python: config.Languages.Python.Environment,
		NodeJS: config.Languages.NodeJS.Environment,
		Go:     config.Languages.Go.Environment,
		CPP:    config.Languages.CPP.Environment,
	}

	switch backend {
	case "docker":
		return NewDockerExecutor(logger, &executorConfig, langEnvs), nil
	case "podman":
		return NewPodmanExecutor(logger, &executorConfig, langEnvs), nil
	case "local":
		return NewLocalExecutor(logger, &executorConfig, langEnvs), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}