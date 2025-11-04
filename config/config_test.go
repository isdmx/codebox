package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		// Test that a valid config does not fail validation
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "docker",
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "info",
			},
			Languages: map[string]Language{
				"python": {
					Image: "python:3.11-slim",
				},
			},
		}

		err := cfg.validate()
		require.NoError(t, err)
	})

	t.Run("InvalidServerTransport", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "invalid", // Invalid transport
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "docker",
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "info",
			},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid server.transport")
	})

	t.Run("InvalidSandboxTimeout", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "docker",
				TimeoutSec:         0, // Invalid: must be positive
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "info",
			},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sandbox.timeout_sec must be positive")
	})

	t.Run("InvalidSandboxMemory", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "docker",
				TimeoutSec:         30,
				MemoryMB:           0, // Invalid: must be positive
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "info",
			},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sandbox.memory_mb must be positive")
	})

	t.Run("InvalidLoggingMode", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "docker",
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: LoggingConfig{
				Mode:  "invalid_mode", // Invalid mode
				Level: "info",
			},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logging.mode")
	})

	t.Run("InvalidLogLevel", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "docker",
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "invalid_level", // Invalid level
			},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logging.level")
	})

	t.Run("ValidBackendWhenLocalEnabled", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "local",
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: true, // Local backend enabled
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "info",
			},
		}

		err := cfg.validate()
		require.NoError(t, err)
	})

	t.Run("InvalidBackendWhenLocalNotEnabled", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{
				Transport: "http",
				HTTPPort:  8080,
			},
			Sandbox: SandboxConfig{
				Backend:            "local",
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false, // Local backend not enabled
			},
			Logging: LoggingConfig{
				Mode:  "production",
				Level: "info",
			},
		}

		err := cfg.validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported sandbox.backend")
	})
}
