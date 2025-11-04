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

	// Create language environments from config
	langEnvs := &LanguageEnvironments{
		Python: cfg.Languages.Python.Environment,
		NodeJS: cfg.Languages.NodeJS.Environment,
		Go:     cfg.Languages.Go.Environment,
		CPP:    cfg.Languages.CPP.Environment,
	}

	// Create language configs with exclude patterns
	langConfigs := &LanguageConfigs{
		Python: LanguageConfig{
			ExcludePatterns: cfg.Languages.Python.ExcludePatterns,
		},
		NodeJS: LanguageConfig{
			ExcludePatterns: cfg.Languages.NodeJS.ExcludePatterns,
		},
		Go: LanguageConfig{
			ExcludePatterns: cfg.Languages.Go.ExcludePatterns,
		},
		CPP: LanguageConfig{
			ExcludePatterns: cfg.Languages.CPP.ExcludePatterns,
		},
	}

	switch backend := cfg.Sandbox.Backend; backend {
	case "docker":
		return NewDockerExecutor(
			logger,
			&executorConfig,
			langEnvs,
			WithDockerLanguageConfigs(langConfigs),
		), nil
	case "podman":
		return NewPodmanExecutor(
			logger,
			&executorConfig,
			langEnvs,
			WithPodmanLanguageConfigs(langConfigs),
		), nil
	case "local":
		return NewLocalExecutor(
			logger,
			&executorConfig,
			langEnvs,
			WithLocalLanguageConfigs(langConfigs),
		), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}
