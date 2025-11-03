package main

import (
	"go.uber.org/fx"
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
			
			// Logger
			logger.New,
			
			// Sandbox executor based on config
			func(cfg *config.Config, logger *zap.Logger) (sandbox.SandboxExecutor, error) {
				executorConfig := &sandbox.Config{
					TimeoutSec:        cfg.Sandbox.TimeoutSec,
					MemoryMB:          cfg.Sandbox.MemoryMB,
					NetworkEnabled:    cfg.Sandbox.NetworkEnabled,
					MaxArtifactSizeMB: cfg.Sandbox.MaxArtifactSizeMB,
				}
				
				return sandbox.NewExecutor(logger, executorConfig, cfg.Sandbox.Backend)
			},
			
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
	)

	// Start the application
	app.Run()
}