// Package main is the entry point for the Codebox MCP server.
//
// The Codebox server implements a secure, configurable Model Context Protocol (MCP)
// server that executes untrusted user code (Python, Node.js, Go, C++) in isolated
// sandboxes. The server supports both stdio and HTTP transports and provides
// comprehensive security features including resource limits, network isolation,
// and path traversal protection.
//
// The application uses Uber's fx framework for dependency injection and lifecycle
// management, with zap for structured logging and viper for configuration.
package main