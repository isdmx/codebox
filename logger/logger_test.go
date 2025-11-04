package logger

import (
	"testing"

	"github.com/isdmx/codebox/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerNew(t *testing.T) {
	t.Run("ValidDevelopmentMode", func(t *testing.T) {
		logger, err := New("development", "debug")
		require.NoError(t, err)
		assert.NotNil(t, logger)
		logger.Sync()
	})

	t.Run("ValidProductionMode", func(t *testing.T) {
		logger, err := New("production", "info")
		require.NoError(t, err)
		assert.NotNil(t, logger)
		logger.Sync()
	})

	t.Run("InvalidMode", func(t *testing.T) {
		_, err := New("invalid_mode", "info")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logging mode")
	})

	t.Run("InvalidLevel", func(t *testing.T) {
		_, err := New("production", "invalid_level")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logging level")
	})

	t.Run("ValidLevels", func(t *testing.T) {
		levels := []string{"debug", "info", "warn", "error", "dpanic", "panic", "fatal"}
		for _, level := range levels {
			t.Run(level, func(t *testing.T) {
				logger, err := New("production", level)
				require.NoError(t, err)
				assert.NotNil(t, logger)
				logger.Sync()
			})
		}
	})
}

func TestLoggerNewFromConfig(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := &config.Config{
			Logging: config.LoggingConfig{
				Mode:  "development",
				Level: "debug",
			},
		}
		logger, err := NewFromConfig(cfg)
		require.NoError(t, err)
		assert.NotNil(t, logger)
		logger.Sync()
	})

	t.Run("InvalidConfig", func(t *testing.T) {
		cfg := &config.Config{
			Logging: config.LoggingConfig{
				Mode:  "invalid_mode",
				Level: "info",
			},
		}
		_, err := NewFromConfig(cfg)
		assert.Error(t, err)
	})
}