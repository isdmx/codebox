package mcpserver

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	"github.com/isdmx/codebox/config"
	"github.com/isdmx/codebox/sandbox"
)

// MCPServer represents the MCP server
type MCPServer struct {
	config       *config.Config
	logger       *zap.Logger
	sandboxExec  sandbox.SandboxExecutor
	mcpServer    *server.MCPServer
}

// New creates a new MCPServer
func New(config *config.Config, logger *zap.Logger, sandboxExec sandbox.SandboxExecutor) (*MCPServer, error) {
	s := &MCPServer{
		config:      config,
		logger:      logger,
		sandboxExec: sandboxExec,
	}

	// Create the MCP server
	s.mcpServer = server.NewMCPServer("codebox-executor", "A secure code execution server")

	// Register the execute_sandboxed_code tool
	s.registerExecuteSandboxedCodeTool()

	return s, nil
}

// registerExecuteSandboxedCodeTool registers the execute_sandboxed_code tool
func (s *MCPServer) registerExecuteSandboxedCodeTool() {
	tool := mcp.Tool{
		Name:        "execute_sandboxed_code",
		Description: "Execute untrusted code in a sandboxed environment",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "User-provided source code",
				},
				"language": map[string]any{
					"type":        "string",
					"description": "Runtime language",
					"enum":        []string{"python", "nodejs", "go", "cpp"},
				},
				"workdir_tar": map[string]any{
					"type":        "string",
					"description": "Base64-encoded tar.gz of initial working directory (optional)",
				},
			},
			Required: []string{"code", "language"},
		},
	}

	s.mcpServer.AddTool(tool, s.handleExecuteSandboxedCode)
}

// handleExecuteSandboxedCode handles the execute_sandboxed_code tool
func (s *MCPServer) handleExecuteSandboxedCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logger.Info("code execution requested")

	// Extract parameters
	code, err := request.RequireString("code")
	if err != nil {
		return nil, fmt.Errorf("code parameter is required: %w", err)
	}

	language, err := request.RequireString("language")
	if err != nil {
		return nil, fmt.Errorf("language parameter is required: %w", err)
	}

	// Validate language
	validLanguages := map[string]bool{
		"python": true,
		"nodejs": true,
		"go":     true,
		"cpp":    true,
	}
	if !validLanguages[language] {
		return nil, fmt.Errorf("invalid language: %s, must be one of: python, nodejs, go, cpp", language)
	}

	// Get optional workdir_tar
	var workdirTar []byte
	workdirTarStr := request.GetString("workdir_tar", "")
	if workdirTarStr != "" {
		decoded, err := base64.StdEncoding.DecodeString(workdirTarStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode workdir_tar: %w", err)
		}
		workdirTar = decoded
	}

	// Log execution
	s.logger.Info("executing code in sandbox",
		zap.String("language", language),
		zap.Bool("has_workdir", len(workdirTar) > 0))

	// Prepare the execution request
	req := sandbox.ExecuteRequest{
		Language:   language,
		Code:       code,
		WorkdirTar: workdirTar,
		TimeoutSec: s.config.Sandbox.TimeoutSec,
		MemoryMB:   s.config.Sandbox.MemoryMB,
		Network:    s.config.Sandbox.NetworkEnabled,
	}

	// Execute the code
	result, err := s.sandboxExec.Execute(ctx, req)
	if err != nil {
		s.logger.Error("sandbox execution failed", zap.Error(err))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Execution failed: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Log execution result
	s.logger.Info("code execution completed",
		zap.String("language", language),
		zap.Int("exit_code", result.ExitCode),
		zap.Int("stdout_len", len(result.Stdout)),
		zap.Int("stderr_len", len(result.Stderr)))

	// Encode artifacts as base64
	artifactsB64 := base64.StdEncoding.EncodeToString(result.ArtifactsTar)

	// Convert result to JSON string for content
	resultJSON := fmt.Sprintf(`{"stdout":%q,"stderr":%q,"exit_code":%d,"artifacts_tar":%q}`,
		result.Stdout, result.Stderr, result.ExitCode, artifactsB64)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: resultJSON,
			},
		},
	}, nil
}

// ServeStdio starts the server on stdio
func (s *MCPServer) ServeStdio() error {
	s.logger.Info("starting MCP server on stdio")
	return server.ServeStdio(s.mcpServer)
}

// ServeHTTP starts the server on HTTP
func (s *MCPServer) ServeHTTP() error {
	port := s.config.Server.HTTPPort
	s.logger.Info("starting MCP server on HTTP", zap.Int("port", port))
	
	httpServer := server.NewStreamableHTTPServer(s.mcpServer)
	return httpServer.Start(fmt.Sprintf(":%d", port))
}

// GetMCPServer returns the underlying MCP server for fx
func (s *MCPServer) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}