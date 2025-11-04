package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/isdmx/codebox/config"
	"github.com/isdmx/codebox/logger"
	"github.com/isdmx/codebox/mcpserver"
	"github.com/isdmx/codebox/sandbox"
)

// TestIntegrationConfigLoggerSandbox tests the integration between config, logger, and sandbox packages
func TestIntegrationConfigLoggerSandbox(t *testing.T) {
	t.Run("ConfigAndLoggerIntegration", func(t *testing.T) {
		// Test that config validation works properly with logger initialization
		cfg := &config.Config{
			Server: config.ServerConfig{
				Transport: "stdio",
				HTTPPort:  8080,
			},
			Sandbox: config.SandboxConfig{
				Backend:            "docker", // This will fail in sandbox creation if local backend not enabled
				TimeoutSec:         30,
				MemoryMB:           512,
				MaxArtifactSizeMB:  20,
				NetworkEnabled:     false,
				EnableLocalBackend: false,
			},
			Logging: config.LoggingConfig{
				Mode:  "development",
				Level: "debug",
			},
			Languages: map[string]config.Language{},
		}

		// Create logger using config
		testLogger, err := logger.New(cfg.Logging.Mode, cfg.Logging.Level)
		require.NoError(t, err)
		require.NotNil(t, testLogger)

		// Test that logger works
		testLogger.Info("Integration test started")
		_ = testLogger.Sync()
	})

	t.Run("ConfigLoggerSandboxFactoryIntegration", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Transport: "stdio",
				HTTPPort:  8080,
			},
			Sandbox: config.SandboxConfig{
				Backend:            "local", // Use local backend for testing without Docker
				TimeoutSec:         10,      // Short timeout for tests
				MemoryMB:           128,     // Lower memory for faster tests
				MaxArtifactSizeMB:  5,
				NetworkEnabled:     false,
				EnableLocalBackend: true, // Must be enabled for local backend
			},
			Logging: config.LoggingConfig{
				Mode:  "development",
				Level: "info",
			},
			Languages: map[string]config.Language{
				"python": {
					Image:       "python:3.11-slim",
					Environment: map[string]string{"PYTHONPATH": "/workdir"},
				},
			},
		}

		testLogger, err := logger.New(cfg.Logging.Mode, cfg.Logging.Level)
		require.NoError(t, err)

		// Create sandbox executor using config and logger
		executor, err := sandbox.NewExecutor(testLogger, cfg)
		require.NoError(t, err)
		require.NotNil(t, executor)

		// The factory should work and create a proper executor
		assert.NotNil(t, executor)

		// This test mainly verifies that the integration between config/logger/sandbox works
		// without throwing configuration errors
	})

	t.Run("FullMCPIntegration", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Transport: "stdio",
				HTTPPort:  8080,
			},
			Sandbox: config.SandboxConfig{
				Backend:            "local",
				TimeoutSec:         5,
				MemoryMB:           128,
				MaxArtifactSizeMB:  5,
				NetworkEnabled:     false,
				EnableLocalBackend: true,
			},
			Logging: config.LoggingConfig{
				Mode:  "development",
				Level: "info",
			},
			Languages: map[string]config.Language{
				"python": {
					Environment: map[string]string{},
				},
				"nodejs": {
					Environment: map[string]string{},
				},
				"go": {
					Environment: map[string]string{},
				},
				"cpp": {
					Environment: map[string]string{},
				},
			},
		}

		mcpLogger, err := logger.New(cfg.Logging.Mode, cfg.Logging.Level)
		require.NoError(t, err)

		// Create sandbox executor
		executor, err := sandbox.NewExecutor(mcpLogger, cfg)
		require.NoError(t, err)

		// Create MCP server
		server, err := mcpserver.New(cfg, mcpLogger, executor)
		require.NoError(t, err)
		require.NotNil(t, server)

		// Test that the server creates successfully without panicking
		assert.NotNil(t, server)

		// Test that tools are registered
		mcpServer := server.GetMCPServer()
		require.NotNil(t, mcpServer)
		// Note: We can't easily verify tool registration without mcp library internals
	})
}

// TestIntegrationSandboxExecution tests sandbox functionality without expecting specific execution results
func TestIntegrationSandboxExecution(t *testing.T) {
	testLogger := zaptest.NewLogger(t)

	t.Run("LocalExecutorCreation", func(t *testing.T) {
		cfg := &config.Config{
			Sandbox: config.SandboxConfig{
				Backend:            "local",
				TimeoutSec:         5,
				MemoryMB:           128,
				MaxArtifactSizeMB:  5,
				NetworkEnabled:     false,
				EnableLocalBackend: true,
			},
			Languages: map[string]config.Language{
				"python": {
					Environment: map[string]string{},
				},
			},
		}

		executor, err := sandbox.NewExecutor(testLogger, cfg)
		require.NoError(t, err)
		require.NotNil(t, executor)

		// This test verifies that executor can be created properly with config and logger
		assert.NotNil(t, executor)
	})

	t.Run("ExecutorWithDifferentLanguages", func(t *testing.T) {
		cfg := &config.Config{
			Sandbox: config.SandboxConfig{
				Backend:            "local",
				TimeoutSec:         5,
				MemoryMB:           128,
				MaxArtifactSizeMB:  5,
				NetworkEnabled:     false,
				EnableLocalBackend: true,
			},
			Languages: map[string]config.Language{
				"python": {
					Environment: map[string]string{},
				},
				"nodejs": {
					Environment: map[string]string{},
				},
				"go": {
					Environment: map[string]string{},
				},
				"cpp": {
					Environment: map[string]string{},
				},
			},
		}

		executor, err := sandbox.NewExecutor(testLogger, cfg)
		require.NoError(t, err)
		require.NotNil(t, executor)

		// Test that executor can be created with multiple language configurations
		assert.NotNil(t, executor)
	})
}
