package sandbox

import (
	"fmt"

	"go.uber.org/zap"
)

// NewExecutor creates an appropriate sandbox executor based on the configuration
func NewExecutor(logger *zap.Logger, config *Config, backend string) (SandboxExecutor, error) {
	executorConfig := Config{
		TimeoutSec:        config.TimeoutSec,
		MemoryMB:          config.MemoryMB,
		NetworkEnabled:    config.NetworkEnabled,
		MaxArtifactSizeMB: config.MaxArtifactSizeMB,
	}

	switch backend {
	case "docker":
		return NewDockerExecutor(logger, &executorConfig), nil
	case "podman":
		return NewPodmanExecutor(logger, &executorConfig), nil
	case "local":
		return NewLocalExecutor(logger, &executorConfig), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}
}