// Package config provides application configuration management.
//
// The config package handles loading and validation of the application's
// configuration from YAML files. It supports configuration for server
// settings, sandbox execution parameters, and language-specific settings.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultHTTPPort        = 8080
	DefaultTimeoutSec      = 10
	DefaultMemoryMB        = 512
	DefaultMaxArtifactSize = 20
)

// Config represents the application configuration.
type Config struct {
	Server    ServerConfig        `mapstructure:"server"`
	Sandbox   SandboxConfig       `mapstructure:"sandbox"`
	Languages map[string]Language `mapstructure:"languages"`
	Logging   LoggingConfig       `mapstructure:"logging"`
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	Transport string `mapstructure:"transport"`
	HTTPPort  int    `mapstructure:"http_port"`
}

// SandboxConfig holds sandbox configuration.
type SandboxConfig struct {
	Backend            string `mapstructure:"backend"`
	TimeoutSec         int    `mapstructure:"timeout_sec"`
	MemoryMB           int    `mapstructure:"memory_mb"`
	MaxArtifactSizeMB  int    `mapstructure:"max_artifact_size_mb"`
	NetworkEnabled     bool   `mapstructure:"network_enabled"`
	EnableLocalBackend bool   `mapstructure:"enable_local_backend"`
}

// Language holds language-specific configurations.
type Language struct {
	Image           string            `mapstructure:"image"`
	BuildCmd        string            `mapstructure:"build_cmd"`
	RunCmd          string            `mapstructure:"run_cmd"`
	PrefixCode      string            `mapstructure:"prefix_code"`
	PostfixCode     string            `mapstructure:"postfix_code"`
	Environment     map[string]string `mapstructure:"environment"`
	ExcludePatterns []string          `mapstructure:"exclude_patterns"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Mode  string `mapstructure:"mode"`
	Level string `mapstructure:"level"`
}

// New loads and validates the application configuration.
func New() (*Config, error) {
	v := viper.New()

	// Set configuration file details
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Set default values
	setDefaults(v)

	// Read configuration from file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal the configuration
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Manually load environment variables to preserve case
	if err := loadEnvironmentVariables(&cfg); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Validate the configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets the default configuration values.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.transport", "stdio")
	v.SetDefault("server.http_port", DefaultHTTPPort)

	// Sandbox defaults
	v.SetDefault("sandbox.backend", "docker")
	v.SetDefault("sandbox.timeout_sec", DefaultTimeoutSec)
	v.SetDefault("sandbox.memory_mb", DefaultMemoryMB)
	v.SetDefault("sandbox.max_artifact_size_mb", DefaultMaxArtifactSize)
	v.SetDefault("sandbox.network_enabled", false)
	v.SetDefault("sandbox.enable_local_backend", false)

	// Logging defaults
	v.SetDefault("logging.mode", "production")
	v.SetDefault("logging.level", "info")
}

// loadEnvironmentVariables loads environment variables from the config file preserving case
func loadEnvironmentVariables(config *Config) error {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		data, err = os.ReadFile("config/config.yaml")
		if err != nil {
			return nil // No config file found, use defaults
		}
	}

	var rawConfig map[string]any
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return fmt.Errorf("error unmarshaling raw config: %w", err)
	}

	return loadLanguageEnvironments(rawConfig, config)
}

func loadLanguageEnvironments(rawConfig map[string]any, config *Config) error {
	languages, ok := rawConfig["languages"].(map[string]any)
	if !ok {
		return nil
	}

	for langName, langData := range languages {
		if langMap, ok := langData.(map[string]any); ok {
			if env, ok := langMap["environment"]; ok {
				if envRaw, ok := env.(map[string]any); ok {
					envMap := make(map[string]string)
					for k, v := range envRaw {
						if vStr, ok := v.(string); ok {
							envMap[k] = vStr
						}
					}
					if lang, ok := config.Languages[langName]; ok {
						lang.Environment = envMap
						config.Languages[langName] = lang
					}
				}
			}
		}
	}

	return nil
}

// validate ensures the configuration is valid.
func (c *Config) validate() error {
	if t := c.Server.Transport; t != "stdio" && t != "http" {
		return fmt.Errorf("invalid server.transport: %s, must be 'stdio' or 'http'", t)
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
		"local":  c.Sandbox.EnableLocalBackend,
	}
	if !supportedBackends[c.Sandbox.Backend] {
		return fmt.Errorf("unsupported sandbox.backend: %s", c.Sandbox.Backend)
	}

	if m := c.Logging.Mode; m != "production" && m != "development" {
		return fmt.Errorf("invalid logging.mode: %s, must be 'production' or 'development'", m)
	}

	logLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "dpanic": true, "panic": true, "fatal": true}
	if !logLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging.level: %s", c.Logging.Level)
	}

	return nil
}

// GetTimeout returns the execution timeout as a duration.
func (c *Config) GetTimeout() time.Duration {
	return time.Duration(c.Sandbox.TimeoutSec) * time.Second
}
