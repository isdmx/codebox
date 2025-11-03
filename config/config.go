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

// Config represents the application configuration
type Config struct {
	Server    ServerConfig   `mapstructure:"server"`
	Sandbox   SandboxConfig  `mapstructure:"sandbox"`
	Languages LanguageConfig `mapstructure:"languages"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Transport string `mapstructure:"transport"`
	HTTPPort  int    `mapstructure:"http_port"`
}

// SandboxConfig holds sandbox configuration
type SandboxConfig struct {
	Backend            string `mapstructure:"backend"`
	TimeoutSec         int    `mapstructure:"timeout_sec"`
	MemoryMB           int    `mapstructure:"memory_mb"`
	MaxArtifactSizeMB  int    `mapstructure:"max_artifact_size_mb"`
	NetworkEnabled     bool   `mapstructure:"network_enabled"`
	EnableLocalBackend bool   `mapstructure:"enable_local_backend"`
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
	Image       string            `mapstructure:"image"`
	PrefixCode  string            `mapstructure:"prefix_code"`
	PostfixCode string            `mapstructure:"postfix_code"`
	Environment map[string]string `mapstructure:"environment"`
}

// NodeJSConfig holds Node.js-specific configuration
type NodeJSConfig struct {
	Image       string            `mapstructure:"image"`
	PrefixCode  string            `mapstructure:"prefix_code"`
	PostfixCode string            `mapstructure:"postfix_code"`
	Environment map[string]string `mapstructure:"environment"`
}

// GoConfig holds Go-specific configuration
type GoConfig struct {
	Image       string            `mapstructure:"image"`
	BuildCmd    string            `mapstructure:"build_cmd"`
	RunCmd      string            `mapstructure:"run_cmd"`
	Environment map[string]string `mapstructure:"environment"`
}

// CPPConfig holds C++-specific configuration
type CPPConfig struct {
	Image       string            `mapstructure:"image"`
	BuildCmd    string            `mapstructure:"build_cmd"`
	RunCmd      string            `mapstructure:"run_cmd"`
	Environment map[string]string `mapstructure:"environment"`
}

// New loads and validates the application configuration
func New() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Define default values as constants to avoid magic numbers
	const (
		defaultHTTPPort        = 8080
		defaultTimeoutSec      = 10
		defaultMemoryMB        = 512
		defaultMaxArtifactSize = 20
	)

	// Set default values
	viper.SetDefault("server.transport", "stdio")
	viper.SetDefault("server.http_port", defaultHTTPPort)
	viper.SetDefault("sandbox.backend", "docker")
	viper.SetDefault("sandbox.timeout_sec", defaultTimeoutSec)
	viper.SetDefault("sandbox.memory_mb", defaultMemoryMB)
	viper.SetDefault("sandbox.max_artifact_size_mb", defaultMaxArtifactSize)
	viper.SetDefault("sandbox.network_enabled", false)
	viper.SetDefault("sandbox.enable_local_backend", false)

	// Python defaults
	viper.SetDefault("languages.python.image", "python:3.11-slim")
	pythonPrefixCode := `import signal, sys

def timeout_handler(signum, frame):
    print('Execution timeout!')
    sys.exit(1)

signal.signal(signal.SIGALRM, timeout_handler)
signal.alarm(10)
`
	viper.SetDefault("languages.python.prefix_code", pythonPrefixCode)
	viper.SetDefault("languages.python.postfix_code", "\nsignal.alarm(0)")

	// Node.js defaults
	viper.SetDefault("languages.nodejs.image", "node:20-alpine")
	viper.SetDefault("languages.nodejs.prefix_code", "// Timeout logic would be implemented here\n")
	viper.SetDefault("languages.nodejs.postfix_code", "")

	// Go defaults
	viper.SetDefault("languages.go.image", "golang:1.23-alpine")
	viper.SetDefault("languages.go.run_cmd", "/workdir/app")
	viper.SetDefault("languages.go.build_cmd", "go build -o /workdir/app /workdir/main.go")

	// C++ defaults
	viper.SetDefault("languages.cpp.image", "gcc:13")
	viper.SetDefault("languages.cpp.build_cmd", "g++ -std=c++17 -O2 -o /workdir/app /workdir/main.cpp")
	viper.SetDefault("languages.cpp.run_cmd", "/workdir/app")

	if configReadErr := viper.ReadInConfig(); configReadErr != nil {
		if _, ok := configReadErr.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", configReadErr)
		}
		// If config file not found, continue with defaults
	}

	// Load config without environment variables first
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Manually load environment variables to preserve case
	if err := loadEnvironmentVariables(&config); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}

	return &config, nil
}

// loadEnvironmentVariables loads environment variables from the config file preserving case
func loadEnvironmentVariables(config *Config) error {
	data := readConfigData()
	if data == nil {
		return nil // No config file found, use defaults
	}

	var rawConfig map[string]any
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return fmt.Errorf("error unmarshaling raw config: %w", err)
	}

	return loadLanguageEnvironments(rawConfig, config)
}

func readConfigData() []byte {
	possiblePaths := []string{"./config.yaml", "./config.yml", "./config/config.yaml", "./config/config.yml", "."}

	for _, path := range possiblePaths {
		if path == "." {
			for _, configFile := range []string{"config.yaml", "config.yml"} {
				d, err := os.ReadFile(configFile)
				if err == nil {
					return d
				}
				// Continue to next file if current one doesn't exist
			}
		} else {
			d, err := os.ReadFile(path)
			if err == nil {
				return d
			}
		}
	}

	return nil // No config file found
}

func loadLanguageEnvironments(rawConfig map[string]any, config *Config) error {
	languages, ok := rawConfig["languages"].(map[string]any)
	if !ok {
		return nil
	}

	pythonEnv := getLanguageEnvironment(languages, "python")
	config.Languages.Python.Environment = pythonEnv

	nodejsEnv := getLanguageEnvironment(languages, "nodejs")
	config.Languages.NodeJS.Environment = nodejsEnv

	goEnv := getLanguageEnvironment(languages, "go")
	config.Languages.Go.Environment = goEnv

	cppEnv := getLanguageEnvironment(languages, "cpp")
	config.Languages.CPP.Environment = cppEnv

	return nil
}

func getLanguageEnvironment(languages map[string]any, lang string) map[string]string {
	envMap := make(map[string]string)

	if langData, ok := languages[lang]; ok {
		if langMap, ok := langData.(map[string]any); ok {
			if env, ok := langMap["environment"]; ok {
				if envRaw, ok := env.(map[string]any); ok {
					for k, v := range envRaw {
						if vStr, ok := v.(string); ok {
							envMap[k] = vStr
						}
					}
				}
			}
		}
	}
	return envMap
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
		"docker":     true,
		"podman":     true,
		"kubernetes": true,
		"local":      c.Sandbox.EnableLocalBackend, // local only enabled if specifically allowed
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
