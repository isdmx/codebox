// Package logger provides structured logging capabilities.
//
// The logger package sets up and configures the application's logging
// system using zap, providing structured, high-performance logging
// throughout the application.
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a new logger instance
func New() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)

	return config.Build()
}

// NewDevelopment creates a development logger instance
func NewDevelopment() (*zap.Logger, error) {
	return zap.NewDevelopment()
}
