// Package config provides application configuration management.
//
// The config package handles loading and validation of the application's
// configuration from YAML files. It supports configuration for server
// settings, sandbox execution parameters, and language-specific settings.
//
// Usage:
//
//	cfg, err := config.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Server transport: %s\n", cfg.Server.Transport)
package config