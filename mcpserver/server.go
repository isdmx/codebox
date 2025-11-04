// Package mcpserver provides the Model Context Protocol (MCP) server implementation.
//
// The mcpserver package implements an MCP-compliant server that exposes tools
// for code execution. It uses the mark3labs/mcp-go library to handle the
// protocol details and provides the execute_sandboxed_code tool as the primary
// interface for secure code execution.
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

// ExecuteRequest represents the input parameters for code execution
type ExecuteRequest struct {
	Code       string `json:"code" jsonschema_description:"User-provided source code" jsonschema:"required"`
	Language   string `json:"language" jsonschema:"enum=python,enum=nodejs,enum=go,enum=cpp,required"`
	WorkdirTar string `json:"workdir_tar,omitempty" jsonschema_description:"Base64-encoded tar.gz of initial working directory (optional)"`
}

// ExecuteResponse represents the structured response from code execution
type ExecuteResponse struct {
	Stdout       string `json:"stdout" jsonschema_description:"Standard output from execution"`
	Stderr       string `json:"stderr" jsonschema_description:"Standard error from execution"`
	ExitCode     int    `json:"exit_code" jsonschema_description:"Exit code of the process"`
	ArtifactsTar string `json:"artifacts_tar,omitempty" jsonschema_description:"Base64-encoded tar.gz of working directory after execution"`
	Error        string `json:"error,omitempty" jsonschema_description:"Error message if execution failed"`
	Success      bool   `json:"success" jsonschema_description:"Indicates if execution was successful"`
}

// MCPServer represents the MCP server
type MCPServer struct {
	config      *config.Config
	logger      *zap.Logger
	sandboxExec sandbox.SandboxExecutor
	mcpServer   *server.MCPServer
}

// New creates a new MCPServer
func New(cfg *config.Config, logger *zap.Logger, sandboxExec sandbox.SandboxExecutor) (*MCPServer, error) {
	s := &MCPServer{
		config:      cfg,
		logger:      logger,
		sandboxExec: sandboxExec,
	}

	// Log configuration parameters on startup
	fields := []zap.Field{
		zap.String("server.transport", s.config.Server.Transport),
		zap.Int("server.http_port", s.config.Server.HTTPPort),
		zap.String("sandbox.backend", s.config.Sandbox.Backend),
		zap.Int("sandbox.timeout_sec", s.config.Sandbox.TimeoutSec),
		zap.Int("sandbox.memory_mb", s.config.Sandbox.MemoryMB),
		zap.Int("sandbox.max_artifact_size_mb", s.config.Sandbox.MaxArtifactSizeMB),
		zap.Bool("sandbox.network_enabled", s.config.Sandbox.NetworkEnabled),
		zap.Bool("sandbox.enable_local_backend", s.config.Sandbox.EnableLocalBackend),
	}
	for lang, langCfg := range s.config.Languages {
		fields = append(fields, zap.String(fmt.Sprintf("languages.%s.image", lang), langCfg.Image))
	}
	logger.Info("configuration loaded", fields...)

	// Create the MCP server
	s.mcpServer = server.NewMCPServer("codebox-executor", "A secure code execution server")

	// Register the execute_sandboxed_code tool
	s.registerExecuteSandboxedCodeTool()

	return s, nil
}

// registerExecuteSandboxedCodeTool registers the execute_sandboxed_code tool
func (s *MCPServer) registerExecuteSandboxedCodeTool() {
	tool := mcp.NewTool("execute_sandboxed_code",
		mcp.WithDescription("Execute untrusted code in a sandboxed environment"),
		mcp.WithInputSchema[ExecuteRequest](),
		mcp.WithOutputSchema[ExecuteResponse](),
	)

	s.mcpServer.AddTool(tool, mcp.NewStructuredToolHandler(s.handleExecuteSandboxedCodeStructured))
}

// handleExecuteSandboxedCodeStructured handles the execute_sandboxed_code tool with structured input/output
func (s *MCPServer) handleExecuteSandboxedCodeStructured(
	ctx context.Context,
	_ mcp.CallToolRequest,
	args ExecuteRequest,
) (ExecuteResponse, error) {
	s.logger.Info("code execution requested")

	// Validate language
	if _, ok := s.config.Languages[args.Language]; !ok {
		return ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid language: %s", args.Language),
		}, nil
	}

	// Get optional workdir_tar
	var workdirTar []byte
	if args.WorkdirTar != "" {
		decodedWorkdirTar, decodeErr := base64.StdEncoding.DecodeString(args.WorkdirTar)
		if decodeErr != nil {
			return ExecuteResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to decode workdir_tar: %v", decodeErr),
			}, nil
		}
		workdirTar = decodedWorkdirTar
	}

	// Log execution
	s.logger.Info("executing code in sandbox",
		zap.String("language", args.Language),
		zap.Bool("has_workdir", len(workdirTar) > 0))

	// Prepare the execution request
	execReq := sandbox.ExecuteRequest{
		Language:   args.Language,
		Code:       args.Code,
		WorkdirTar: workdirTar,
		TimeoutSec: s.config.Sandbox.TimeoutSec,
		MemoryMB:   s.config.Sandbox.MemoryMB,
		Network:    s.config.Sandbox.NetworkEnabled,
	}

	// Execute the code
	result, err := s.sandboxExec.Execute(ctx, execReq)
	if err != nil {
		s.logger.Error("sandbox execution failed",
			zap.Error(err),
			zap.String("language", args.Language),
			zap.String("code", args.Code))
		return ExecuteResponse{
			Stdout:   "",
			Stderr:   "",
			ExitCode: 1,
			Error:    fmt.Sprintf("execution failed: %v", err),
			Success:  false,
		}, nil
	}

	// Log execution result
	s.logger.Info("code execution completed",
		zap.String("language", args.Language),
		zap.Int("exit_code", result.ExitCode),
		zap.Int("stdout_len", len(result.Stdout)),
		zap.Int("stderr_len", len(result.Stderr)))

	// Encode artifacts as base64
	artifactsB64 := base64.StdEncoding.EncodeToString(result.ArtifactsTar)

	return ExecuteResponse{
		Stdout:       result.Stdout,
		Stderr:       result.Stderr,
		ExitCode:     result.ExitCode,
		ArtifactsTar: artifactsB64,
		Success:      true,
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
