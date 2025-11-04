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
)

// New creates a new logger instance based on configuration
func New(mode, level string) (*zap.Logger, error) {
	var config zap.Config

	switch mode {
	case "development":
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	case "production":
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	default:
		return nil, fmt.Errorf("invalid logging mode: %s, must be 'production' or 'development'", mode)
	}

	// Set the log level
	logLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid logging level: %s, must be one of 'debug', 'info', 'warn', 'error', 'dpanic', 'panic', 'fatal'", level)
	}
	config.Level = zap.NewAtomicLevelAt(logLevel)

	return config.Build()
}
