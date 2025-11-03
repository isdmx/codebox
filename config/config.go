package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Sandbox SandboxConfig `mapstructure:"sandbox"`
	Languages LanguageConfig `mapstructure:"languages"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Transport string `mapstructure:"transport"`
	HTTPPort  int    `mapstructure:"http_port"`
}

// SandboxConfig holds sandbox configuration
type SandboxConfig struct {
	Backend            string        `mapstructure:"backend"`
	TimeoutSec         int           `mapstructure:"timeout_sec"`
	MemoryMB           int           `mapstructure:"memory_mb"`
	MaxArtifactSizeMB  int           `mapstructure:"max_artifact_size_mb"`
	NetworkEnabled     bool          `mapstructure:"network_enabled"`
	EnableLocalBackend bool          `mapstructure:"enable_local_backend"`
}

// LanguageConfig holds language-specific configurations
type LanguageConfig struct {
	Python PythonConfig `mapstructure:"python"`
	NodeJS NodeJSConfig `mapstructure:"nodejs"`
	Go     GoConfig     `mapstructure:"go"`
	CPP    CPPConfig    `mapstructure:"cpp"`
}

// PythonConfig holds Python-specific configuration
type PythonConfig struct {
	Image       string `mapstructure:"image"`
	PrefixCode  string `mapstructure:"prefix_code"`
	PostfixCode string `mapstructure:"postfix_code"`
}

// NodeJSConfig holds Node.js-specific configuration
type NodeJSConfig struct {
	Image       string `mapstructure:"image"`
	PrefixCode  string `mapstructure:"prefix_code"`
	PostfixCode string `mapstructure:"postfix_code"`
}

// GoConfig holds Go-specific configuration
type GoConfig struct {
	Image    string `mapstructure:"image"`
	BuildCmd string `mapstructure:"build_cmd"`
	RunCmd   string `mapstructure:"run_cmd"`
}

// CPPConfig holds C++-specific configuration
type CPPConfig struct {
	Image    string `mapstructure:"image"`
	BuildCmd string `mapstructure:"build_cmd"`
	RunCmd   string `mapstructure:"run_cmd"`
}

// New loads and validates the application configuration
func New() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Set default values
	viper.SetDefault("server.transport", "stdio")
	viper.SetDefault("server.http_port", 8080)
	viper.SetDefault("sandbox.backend", "docker")
	viper.SetDefault("sandbox.timeout_sec", 10)
	viper.SetDefault("sandbox.memory_mb", 512)
	viper.SetDefault("sandbox.max_artifact_size_mb", 20)
	viper.SetDefault("sandbox.network_enabled", false)
	viper.SetDefault("sandbox.enable_local_backend", false)

	// Python defaults
	viper.SetDefault("languages.python.image", "python:3.11-slim")
	viper.SetDefault("languages.python.prefix_code", "import signal, sys\n\ndef timeout_handler(signum, frame):\n    print('Execution timeout!')\n    sys.exit(1)\n\nsignal.signal(signal.SIGALRM, timeout_handler)\nsignal.alarm(10)\n")
	viper.SetDefault("languages.python.postfix_code", "\nsignal.alarm(0)")

	// Node.js defaults
	viper.SetDefault("languages.nodejs.image", "node:20-alpine")
	viper.SetDefault("languages.nodejs.prefix_code", "// Timeout logic would be implemented here\n")
	viper.SetDefault("languages.nodejs.postfix_code", "")

	// Go defaults
	viper.SetDefault("languages.go.image", "golang:1.23-alpine")
	viper.SetDefault("languages.go.build_cmd", "go build -o /workdir/app /workdir/main.go")
	viper.SetDefault("languages.go.run_cmd", "/workdir/app")

	// C++ defaults
	viper.SetDefault("languages.cpp.image", "gcc:13")
	viper.SetDefault("languages.cpp.build_cmd", "g++ -std=c++17 -O2 -o /workdir/app /workdir/main.cpp")
	viper.SetDefault("languages.cpp.run_cmd", "/workdir/app")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// If config file not found, continue with defaults
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &config, nil
}

// validate ensures the configuration is valid
func (c *Config) validate() error {
	if c.Server.Transport != "stdio" && c.Server.Transport != "http" {
		return fmt.Errorf("invalid server.transport: %s, must be 'stdio' or 'http'", c.Server.Transport)
	}

	if c.Sandbox.TimeoutSec <= 0 {
		return fmt.Errorf("sandbox.timeout_sec must be positive, got: %d", c.Sandbox.TimeoutSec)
	}

	if c.Sandbox.MemoryMB <= 0 {
		return fmt.Errorf("sandbox.memory_mb must be positive, got: %d", c.Sandbox.MemoryMB)
	}

	if c.Sandbox.MaxArtifactSizeMB <= 0 {
		return fmt.Errorf("sandbox.max_artifact_size_mb must be positive, got: %d", c.Sandbox.MaxArtifactSizeMB)
	}

	supportedBackends := map[string]bool{
		"docker": true,
		"podman": true,
		"kubernetes": true,
		"local": c.Sandbox.EnableLocalBackend, // local only enabled if specifically allowed
	}

	if !supportedBackends[c.Sandbox.Backend] {
		return fmt.Errorf("unsupported sandbox.backend: %s", c.Sandbox.Backend)
	}

	return nil
}

// GetTimeout returns the execution timeout as a duration
func (c *Config) GetTimeout() time.Duration {
	return time.Duration(c.Sandbox.TimeoutSec) * time.Second
}