// Package logger provides structured logging capabilities.
//
// The logger package sets up and configures the application's logging
// system using zap, providing structured, high-performance logging
// throughout the application.
package logger

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/isdmx/codebox/config"
)

func NewFromConfig(cfg *config.Config) (*zap.Logger, error) {
	return New(cfg.Logging.Mode, cfg.Logging.Level)
}

// New creates a new logger instance based on configuration
func New(mode, level string) (*zap.Logger, error) {
	var cfg zap.Config

	switch mode {
	case "development":
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	case "production":
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	default:
		return nil, fmt.Errorf("invalid logging mode: %s, must be 'production' or 'development'", mode)
	}

	// Set the log level
	logLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid logging level: %s, must be one of 'debug', 'info', 'warn', 'error', 'dpanic', 'panic', 'fatal'", level)
	}
	cfg.Level = zap.NewAtomicLevelAt(logLevel)

	return cfg.Build()
}
