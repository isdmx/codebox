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
	"context"
	"fmt"

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
		fx.Invoke(func(lc fx.Lifecycle, cfg *config.Config, server *mcpserver.MCPServer, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					go func() {
						if err := startServer(cfg, server); err != nil {
							log.Error("Failed to start server", zap.Error(err))
						}
					}()
					return nil
				},
			})
		}),

		// Use the application logger for fx logs
		fx.WithLogger(func(log *zap.Logger) fxevent.Logger {
			return &fxevent.ZapLogger{Logger: log}
		}),
	)

	// Start the application
	app.Run()
}

func startServer(cfg *config.Config, server *mcpserver.MCPServer) error {
	switch cfg.Server.Transport {
	case "stdio":
		return server.ServeStdio()
	case "http":
		return server.ServeHTTP()
	default:
		return fmt.Errorf("unsupported transport: %s", cfg.Server.Transport)
	}
}
