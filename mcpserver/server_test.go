package mcpserver

import (
	"context"
	"testing"

	"github.com/isdmx/codebox/config"
	"github.com/isdmx/codebox/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockSandboxExecutor implements sandbox.SandboxExecutor for testing
type MockSandboxExecutor struct {
	executeResult sandbox.ExecuteResult
	executeError  error
}

func (m *MockSandboxExecutor) Execute(_ context.Context, _ sandbox.ExecuteRequest) (sandbox.ExecuteResult, error) { //nolint:gocritic // Mock implementation requires full parameter signature
	return m.executeResult, m.executeError
}

func TestNewMCPServer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		Server: config.ServerConfig{
			Transport: "http",
			HTTPPort:  8080,
		},
		Sandbox: config.SandboxConfig{
			Backend:            "docker",
			TimeoutSec:         30,
			MemoryMB:           512,
			MaxArtifactSizeMB:  20,
			NetworkEnabled:     false,
			EnableLocalBackend: false,
		},
		Logging: config.LoggingConfig{
			Mode:  "production",
			Level: "info",
		},
		Languages: config.LanguageConfig{},
	}
	mockExecutor := &MockSandboxExecutor{}

	server, err := New(cfg, logger, mockExecutor)
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Equal(t, cfg, server.config)
	assert.Equal(t, logger, server.logger)
	assert.Equal(t, mockExecutor, server.sandboxExec)
}

// Test basic server functionality without needing to create complex request structs
// since we can't easily instantiate external library types in tests
func TestServerCreation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := &config.Config{
		Server:    config.ServerConfig{Transport: "stdio", HTTPPort: 8080},
		Sandbox:   config.SandboxConfig{TimeoutSec: 30, MemoryMB: 512, NetworkEnabled: false, MaxArtifactSizeMB: 20},
		Logging:   config.LoggingConfig{Mode: "production", Level: "info"},
		Languages: config.LanguageConfig{},
	}

	mockExecutor := &MockSandboxExecutor{
		executeResult: sandbox.ExecuteResult{
			Stdout:       "output",
			Stderr:       "error",
			ExitCode:     0,
			ArtifactsTar: []byte{},
		},
	}

	server, err := New(cfg, logger, mockExecutor)
	require.NoError(t, err)
	require.NotNil(t, server)

	// Test that server has proper initialization
	assert.Equal(t, cfg, server.config)
	assert.Equal(t, logger, server.logger)
	assert.Equal(t, mockExecutor, server.sandboxExec)
	assert.NotNil(t, server.mcpServer)
}
