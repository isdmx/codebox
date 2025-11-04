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

import (
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"

	"github.com/isdmx/codebox/config"
	"github.com/isdmx/codebox/logger"
	"github.com/isdmx/codebox/mcpserver"
	"github.com/isdmx/codebox/sandbox"
)

func main() {
	app := fx.New(
		// Provide dependencies
		fx.Provide(
			// Config
			config.New,

			// Logger with configuration
			logger.NewFromConfig,

			// Sandbox executor based on config
			sandbox.NewExecutor,

			// MCP Server
			mcpserver.New,
		),

		// Start the appropriate transport based on config
		fx.Invoke(
			func(cfg *config.Config, server *mcpserver.MCPServer) {
				switch cfg.Server.Transport {
				case "stdio":
					// Use fx to run this as a background task
					go func() {
						if err := server.ServeStdio(); err != nil {
							panic(err)
						}
					}()
				case "http":
					go func() {
						if err := server.ServeHTTP(); err != nil {
							panic(err)
						}
					}()
				default:
					panic("unsupported transport: " + cfg.Server.Transport)
				}
			},
		),

		// Use the application logger for fx logs
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
	)

	// Start the application
	app.Run()
}
