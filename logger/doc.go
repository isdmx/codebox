// Package logger provides structured logging capabilities.
//
// The logger package sets up and configures the application's logging
// system using zap, providing structured, high-performance logging
// throughout the application.
//
// Usage:
//
//	logger, err := logger.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	logger.Info("Application started")
//	logger.Error("An error occurred", zap.Error(err))
package logger