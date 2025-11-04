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
		Python: cfg.Languages["python"].Environment,
		NodeJS: cfg.Languages["nodejs"].Environment,
		Go:     cfg.Languages["go"].Environment,
		CPP:    cfg.Languages["cpp"].Environment,
	}

	// Create language configs with exclude patterns
	langConfigs := &LanguageConfigs{
		Python: LanguageConfig{
			ExcludePatterns: cfg.Languages["python"].ExcludePatterns,
		},
		NodeJS: LanguageConfig{
			ExcludePatterns: cfg.Languages["nodejs"].ExcludePatterns,
		},
		Go: LanguageConfig{
			ExcludePatterns: cfg.Languages["go"].ExcludePatterns,
		},
		CPP: LanguageConfig{
			ExcludePatterns: cfg.Languages["cpp"].ExcludePatterns,
		},
	}

	// Create language code configs with prefix and postfix code
	langCodeConfigs := &LanguageCodeConfigs{
		Python: LanguageCodeConfig{
			PrefixCode:  cfg.Languages["python"].PrefixCode,
			PostfixCode: cfg.Languages["python"].PostfixCode,
		},
		NodeJS: LanguageCodeConfig{
			PrefixCode:  cfg.Languages["nodejs"].PrefixCode,
			PostfixCode: cfg.Languages["nodejs"].PostfixCode,
		},
		Go: LanguageCodeConfig{
			PrefixCode:  cfg.Languages["go"].PrefixCode,
			PostfixCode: cfg.Languages["go"].PostfixCode,
		},
		CPP: LanguageCodeConfig{
			PrefixCode:  cfg.Languages["cpp"].PrefixCode,
			PostfixCode: cfg.Languages["cpp"].PostfixCode,
		},
	}

	switch backend := cfg.Sandbox.Backend; backend {
	case "docker":
		return NewDockerExecutor(
			logger,
			&executorConfig,
			langEnvs,
			WithDockerLanguageConfigs(langConfigs),
			WithDockerLanguageCodeConfigs(langCodeConfigs),
		), nil
	case "podman":
		return NewPodmanExecutor(
			logger,
			&executorConfig,
			langEnvs,
			WithPodmanLanguageConfigs(langConfigs),
			WithPodmanLanguageCodeConfigs(langCodeConfigs),
		), nil
	case "local":
		return NewLocalExecutor(
			logger,
			&executorConfig,
			langEnvs,
			WithLocalLanguageConfigs(langConfigs),
			WithLocalLanguageCodeConfigs(langCodeConfigs),
		), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}
