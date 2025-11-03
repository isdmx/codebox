// Package mcpserver provides the Model Context Protocol (MCP) server implementation.
//
// The mcpserver package implements an MCP-compliant server that exposes tools
// for code execution. It uses the mark3labs/mcp-go library to handle the
// protocol details and provides the execute_sandboxed_code tool as the primary
// interface for secure code execution.
//
// The server supports both stdio and HTTP transports as configured by the
// application configuration.
//
// Usage:
//
//	server, err := mcpserver.New(config, logger, sandboxExecutor)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	err = server.ServeStdio() // or server.ServeHTTP()
package mcpserver