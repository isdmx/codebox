package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/isdmx/codebox/config"
)

// TestExecutorExcludePatternsIntegration tests the integration of exclude patterns with the executors
func TestExecutorExcludePatternsIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create test directory structure with files that should be excluded
	tempDir := t.TempDir()

	// Create files that should be excluded (based on defaults)
	filesToCreate := map[string]string{
		FilenamePython:                    "print('hello world')",
		"__pycache__/main.cpython-39.pyc": "cache data",
		testFilePy:                        "import unittest",
		"cache_file.pyc":                  "another cache",
		testFileNodePkg:                   "node module",
		"src/nested/__pycache__/file.pyc": "nested cache",
		".pytest_cache/v/cache.py":        "pytest cache",
		"build/output.so":                 "built output",
		"dist/bundle.js":                  "dist file",
		"normal_file.txt":                 "should remain",
		"data.json":                       "should remain",
	}

	for relPath, content := range filesToCreate {
		fullPath := filepath.Join(tempDir, relPath)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0o755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, []byte(content), 0o600)
		require.NoError(t, err)
	}

	t.Run("DockerExecutor with exclude patterns", func(t *testing.T) {
		executorConfig := &Config{
			TimeoutSec:        30,
			MemoryMB:          128,
			NetworkEnabled:    false,
			MaxArtifactSizeMB: 5,
		}

		// Create a mock config for testing
		mockConfig := &config.Config{
			Languages: map[string]config.Language{
				LanguagePython: {
					Environment:     map[string]string{testEnvPythonPath: WorkDirPath},
					ExcludePatterns: []string{testDirPycache, testPatternPyc, testPatternPyo, testDirPytestCache, testDirNodeModules, testDirBuild, testDirDist},
				},
			},
		}

		executor := NewDockerExecutor(
			logger,
			executorConfig,
			mockConfig,
		)

		// Test that the executor is created successfully
		assert.NotNil(t, executor)
	})

	t.Run("PodmanExecutor with exclude patterns", func(t *testing.T) {
		executorConfig := &Config{
			TimeoutSec:        30,
			MemoryMB:          128,
			NetworkEnabled:    false,
			MaxArtifactSizeMB: 5,
		}

		// Create a mock config for testing
		mockConfig := &config.Config{
			Languages: map[string]config.Language{
				LanguagePython: {
					Environment:     map[string]string{testEnvPythonPath: WorkDirPath},
					ExcludePatterns: []string{testDirPycache, testPatternPyc, testPatternPyo, testDirPytestCache, testDirNodeModules, testDirBuild, testDirDist},
				},
			},
		}

		executor := NewPodmanExecutor(
			logger,
			executorConfig,
			mockConfig,
		)

		// Test that the executor is created successfully
		assert.NotNil(t, executor)
	})

	t.Run("LocalExecutor with exclude patterns", func(t *testing.T) {
		executorConfig := &Config{
			TimeoutSec:        30,
			MemoryMB:          128,
			NetworkEnabled:    false,
			MaxArtifactSizeMB: 5,
		}

		// Create a mock config for testing
		mockConfig := &config.Config{
			Languages: map[string]config.Language{
				LanguagePython: {
					Environment:     map[string]string{testEnvPythonPath: WorkDirPath},
					ExcludePatterns: []string{testDirPycache, testPatternPyc, testPatternPyo, testDirPytestCache, testDirNodeModules, testDirBuild, testDirDist},
				},
			},
		}

		executor := NewLocalExecutor(
			logger,
			executorConfig,
			mockConfig,
		)

		// Test that the executor is created successfully
		assert.NotNil(t, executor)
	})
}

// TestFactoryWithExcludePatternsIntegration tests the factory integration with exclude patterns
func TestFactoryWithExcludePatternsIntegration(t *testing.T) {
	// This test requires a config with exclude patterns, which may not be possible
	// to create directly for testing factory functions. Let's test the option functions.

	logger := zaptest.NewLogger(t)

	testConfig := &Config{
		TimeoutSec:        30,
		MemoryMB:          128,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 5,
	}

	// Test all executors with language configs
	t.Run("Factory-style creation with exclude patterns", func(t *testing.T) {
		mockConfig := &config.Config{
			Languages: map[string]config.Language{
				LanguagePython: {
					Environment:     map[string]string{testEnvPythonPath: WorkDirPath},
					ExcludePatterns: []string{testDirPycache, testPatternPyc, FilenamePython}, // exclude source
				},
				LanguageNodeJS: {
					Environment:     map[string]string{"NODE_PATH": WorkDirPath},
					ExcludePatterns: []string{testDirNodeModules, FilenameNodeJS}, // exclude source
				},
				LanguageGo: {
					Environment:     map[string]string{"GOCACHE": "/tmp/go-build"},
					ExcludePatterns: []string{FilenameGo, testFileApp}, // exclude source and binary
				},
				LanguageCPP: {
					Environment:     map[string]string{"LANG": "C.UTF-8"},
					ExcludePatterns: []string{FilenameCPP, testFileApp}, // exclude source and binary
				},
			},
		}

		// Test Docker executor creation with options
		dockerExec := NewDockerExecutor(
			logger,
			testConfig,
			mockConfig,
		)
		assert.NotNil(t, dockerExec)

		// Test Podman executor creation with options
		podmanExec := NewPodmanExecutor(
			logger,
			testConfig,
			mockConfig,
		)
		assert.NotNil(t, podmanExec)

		// Test Local executor creation with options
		localExec := NewLocalExecutor(
			logger,
			testConfig,
			mockConfig,
		)
		assert.NotNil(t, localExec)
	})
}

// TestExecutorWithVariousLanguagesExcludePatterns tests exclude patterns for different languages
func TestExecutorWithVariousLanguagesExcludePatterns(t *testing.T) {
	t.Run("Python exclude patterns", func(t *testing.T) {
		pythonExcludePatterns := []string{testDirPycache, testPatternPyc, testPatternPyo, "*.egg-info/", FilenamePython}

		// Simulate the exclude pattern matching logic
		shouldExcludeMainPy := shouldExcludeFile(FilenamePython, pythonExcludePatterns)
		assert.True(t, shouldExcludeMainPy)

		shouldExcludeCache := shouldExcludeFile("__pycache__/file.pyc", pythonExcludePatterns)
		assert.True(t, shouldExcludeCache)

		shouldNotExcludeTxt := shouldExcludeFile("data.txt", pythonExcludePatterns)
		assert.False(t, shouldNotExcludeTxt)
	})

	t.Run("NodeJS exclude patterns", func(t *testing.T) {
		nodejsExcludePatterns := []string{testDirNodeModules, "*.js.map", "npm-debug.log*", FilenameNodeJS}

		shouldExcludeNodeModules := shouldExcludeFile(testFileNodePkg, nodejsExcludePatterns)
		assert.True(t, shouldExcludeNodeModules)

		shouldExcludeSourceMap := shouldExcludeFile("app.js.map", nodejsExcludePatterns)
		assert.True(t, shouldExcludeSourceMap)

		shouldNotExcludeRegularJs := shouldExcludeFile("other.js", nodejsExcludePatterns)
		assert.False(t, shouldNotExcludeRegularJs)
	})

	t.Run("Go exclude patterns", func(t *testing.T) {
		goExcludePatterns := []string{testPatternObj, "*.a", "*.so", FilenameGo, testFileApp, "go.sum", "go.mod"}

		shouldExcludeObject := shouldExcludeFile("util.o", goExcludePatterns)
		assert.True(t, shouldExcludeObject)

		shouldExcludeBinary := shouldExcludeFile(testFileApp, goExcludePatterns)
		assert.True(t, shouldExcludeBinary)

		shouldExcludeSource := shouldExcludeFile(FilenameGo, goExcludePatterns)
		assert.True(t, shouldExcludeSource)

		shouldNotExcludeOther := shouldExcludeFile("config.yaml", goExcludePatterns)
		assert.False(t, shouldNotExcludeOther)
	})

	t.Run("C++ exclude patterns", func(t *testing.T) {
		cppExcludePatterns := []string{testPatternObj, "*.a", "*.so", "*.dll", "*.exe", FilenameCPP, testFileApp, "a.out", "a.exe"}

		shouldExcludeObj := shouldExcludeFile("util.o", cppExcludePatterns)
		assert.True(t, shouldExcludeObj)

		shouldExcludeSource := shouldExcludeFile(FilenameCPP, cppExcludePatterns)
		assert.True(t, shouldExcludeSource)

		shouldExcludeBinary := shouldExcludeFile(testFileApp, cppExcludePatterns)
		assert.True(t, shouldExcludeBinary)

		shouldNotExcludeHeader := shouldExcludeFile("header.h", cppExcludePatterns)
		assert.False(t, shouldNotExcludeHeader)
	})
}

// TestExecutorExecuteWithExcludes tests the execution flow with exclude patterns
// Note: This test doesn't actually execute code, just verifies the setup
func TestExecutorExecuteWithExcludes(t *testing.T) {
	logger := zaptest.NewLogger(t)

	executorConfig := &Config{
		TimeoutSec:        30,
		MemoryMB:          128,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 5,
	}

	// Create a mock config for testing
	mockConfig := &config.Config{
		Languages: map[string]config.Language{
			LanguagePython: {
				Environment:     map[string]string{testEnvPythonPath: WorkDirPath},
				ExcludePatterns: []string{testDirPycache, testPatternPyc, "temp/*", "*.tmp", FilenamePython},
			},
		},
	}

	executor := NewLocalExecutor(
		logger,
		executorConfig,
		mockConfig,
	)

	// Test that the executor is properly configured
	assert.NotNil(t, executor)

	// Test individual pattern matching with the mock config values directly
	pythonExcludePatterns := mockConfig.Languages[LanguagePython].ExcludePatterns
	shouldExcludeCache := shouldExcludeFile("some/path/__pycache__/file.pyc", pythonExcludePatterns)
	assert.True(t, shouldExcludeCache)

	shouldExcludeSource := shouldExcludeFile(FilenamePython, pythonExcludePatterns)
	assert.True(t, shouldExcludeSource)

	shouldNotExcludeOther := shouldExcludeFile("requirements.txt", pythonExcludePatterns)
	assert.False(t, shouldNotExcludeOther)
}
