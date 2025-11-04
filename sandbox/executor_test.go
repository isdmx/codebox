package sandbox

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockCommandRunner implements CommandRunner for testing
type MockCommandRunner struct {
	commandResults map[string]struct {
		stdout   string
		stderr   string
		exitCode int
		err      error
	}
	defaultResult struct {
		stdout   string
		stderr   string
		exitCode int
		err      error
	}
}

func (m *MockCommandRunner) RunCommand(_ context.Context, args []string) (stdout, stderr string, exitCode int, err error) {
	cmdKey := ""
	for _, arg := range args {
		cmdKey += arg + " "
	}

	if result, exists := m.commandResults[cmdKey]; exists {
		return result.stdout, result.stderr, result.exitCode, result.err
	}

	return m.defaultResult.stdout, m.defaultResult.stderr, m.defaultResult.exitCode, m.defaultResult.err
}

// MockFileSystem implements FileSystem for testing
type MockFileSystem struct {
	mkdirTempResults map[string]string
	mkdirAllErrors   map[string]error
	writeFileErrors  map[string]error
	writeFileData    map[string][]byte
	readFileResults  map[string][]byte
	removeAllErrors  map[string]error
	results          map[string]any
}

func (m *MockFileSystem) MkdirTemp(dir, pattern string) (string, error) {
	key := dir + ":" + pattern
	if result, exists := m.mkdirTempResults[key]; exists {
		return result, nil
	}
	return "/tmp/test", nil
}

func (m *MockFileSystem) MkdirAll(path string, _ os.FileMode) error {
	if err, exists := m.mkdirAllErrors[path]; exists {
		return err
	}
	return nil
}

func (m *MockFileSystem) WriteFile(filename string, data []byte, _ os.FileMode) error {
	if err, exists := m.writeFileErrors[filename]; exists {
		return err
	}
	// Store the data as part of the mock behavior
	if m.writeFileData == nil {
		m.writeFileData = make(map[string][]byte)
	}
	m.writeFileData[filename] = data
	return nil
}

func (m *MockFileSystem) ReadFile(filename string) ([]byte, error) {
	if result, exists := m.readFileResults[filename]; exists {
		return result, nil
	}
	return []byte{}, nil
}

func (m *MockFileSystem) RemoveAll(path string) error {
	if err, exists := m.removeAllErrors[path]; exists {
		return err
	}
	return nil
}

func (m *MockFileSystem) FileExists(path string) (bool, error) {
	if result, exists := m.results[path]; exists {
		if b, ok := result.(bool); ok {
			return b, nil
		}
	}
	return true, nil
}

func TestDockerExecutorConstructors(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &Config{
		TimeoutSec:        30,
		MemoryMB:          512,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 20,
	}
	langEnvs := &LanguageEnvironments{
		Python: map[string]string{"PYTHONPATH": "/workdir"},
	}

	t.Run("DefaultConstructor", func(t *testing.T) {
		executor := NewDockerExecutor(logger, config, langEnvs)
		require.NotNil(t, executor)
		assert.Equal(t, logger, executor.logger)
		assert.Equal(t, config, executor.config)
		assert.Equal(t, langEnvs, executor.langEnvs)
		// Default implementations should be set
		assert.NotNil(t, executor.cmdRunner)
		assert.NotNil(t, executor.fs)
	})

	t.Run("ConstructorWithOptions", func(t *testing.T) {
		mockRunner := &MockCommandRunner{}
		mockFS := &TarTestMockFileSystem{}

		executor := NewDockerExecutor(
			logger,
			config,
			langEnvs,
			WithDockerCommandRunner(mockRunner),
			WithDockerFileSystem(mockFS),
		)
		require.NotNil(t, executor)
		assert.Equal(t, logger, executor.logger)
		assert.Equal(t, config, executor.config)
		assert.Equal(t, langEnvs, executor.langEnvs)
		assert.Equal(t, mockRunner, executor.cmdRunner)
		assert.Equal(t, mockFS, executor.fs)
	})
}

func TestPodmanExecutorConstructors(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &Config{
		TimeoutSec:        30,
		MemoryMB:          512,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 20,
	}
	langEnvs := &LanguageEnvironments{
		Python: map[string]string{"PYTHONPATH": "/workdir"},
	}

	t.Run("DefaultConstructor", func(t *testing.T) {
		executor := NewPodmanExecutor(logger, config, langEnvs)
		require.NotNil(t, executor)
		assert.Equal(t, logger, executor.logger)
		assert.Equal(t, config, executor.config)
		assert.Equal(t, langEnvs, executor.langEnvs)
		// Default implementations should be set
		assert.NotNil(t, executor.cmdRunner)
		assert.NotNil(t, executor.fs)
	})

	t.Run("ConstructorWithOptions", func(t *testing.T) {
		mockRunner := &MockCommandRunner{}
		mockFS := &TarTestMockFileSystem{}

		executor := NewPodmanExecutor(
			logger,
			config,
			langEnvs,
			WithPodmanCommandRunner(mockRunner),
			WithPodmanFileSystem(mockFS),
		)
		require.NotNil(t, executor)
		assert.Equal(t, logger, executor.logger)
		assert.Equal(t, config, executor.config)
		assert.Equal(t, langEnvs, executor.langEnvs)
		assert.Equal(t, mockRunner, executor.cmdRunner)
		assert.Equal(t, mockFS, executor.fs)
	})
}

func TestLocalExecutorConstructors(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &Config{
		TimeoutSec:        30,
		MemoryMB:          512,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 20,
	}
	langEnvs := &LanguageEnvironments{
		Python: map[string]string{"PYTHONPATH": "/workdir"},
	}

	t.Run("DefaultConstructor", func(t *testing.T) {
		executor := NewLocalExecutor(logger, config, langEnvs)
		require.NotNil(t, executor)
		assert.Equal(t, logger, executor.logger)
		assert.Equal(t, config, executor.config)
		assert.Equal(t, langEnvs, executor.langEnvs)
		// Default implementations should be set
		assert.NotNil(t, executor.cmdRunner)
		assert.NotNil(t, executor.fs)
	})

	t.Run("ConstructorWithOptions", func(t *testing.T) {
		mockRunner := &MockCommandRunner{}
		mockFS := &TarTestMockFileSystem{}

		executor := NewLocalExecutor(
			logger,
			config,
			langEnvs,
			WithLocalCommandRunner(mockRunner),
			WithLocalFileSystem(mockFS),
		)
		require.NotNil(t, executor)
		assert.Equal(t, logger, executor.logger)
		assert.Equal(t, config, executor.config)
		assert.Equal(t, langEnvs, executor.langEnvs)
		assert.Equal(t, mockRunner, executor.cmdRunner)
		assert.Equal(t, mockFS, executor.fs)
	})
}
