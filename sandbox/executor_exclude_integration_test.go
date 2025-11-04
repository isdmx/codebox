package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestExecutorExcludePatternsIntegration tests the integration of exclude patterns with the executors
func TestExecutorExcludePatternsIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create test directory structure with files that should be excluded
	tempDir := t.TempDir()

	// Create files that should be excluded (based on defaults)
	filesToCreate := map[string]string{
		"main.py":                         "print('hello world')",
		"__pycache__/main.cpython-39.pyc": "cache data",
		"test.py":                         "import unittest",
		"cache_file.pyc":                  "another cache",
		"node_modules/package.json":       "node module",
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
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(fullPath, []byte(content), 0600)
		require.NoError(t, err)
	}

	t.Run("DockerExecutor with exclude patterns", func(t *testing.T) {
		config := &Config{
			TimeoutSec:        30,
			MemoryMB:          128,
			NetworkEnabled:    false,
			MaxArtifactSizeMB: 5,
		}

		langEnvs := &LanguageEnvironments{
			Python: map[string]string{"PYTHONPATH": "/workdir"},
		}

		// Create executor with exclude patterns
		langConfigs := &LanguageConfigs{
			Python: LanguageConfig{
				ExcludePatterns: []string{"__pycache__/", "*.pyc", "*.pyo", ".pytest_cache/", "node_modules/", "build/", "dist/"},
			},
		}

		executor := NewDockerExecutor(
			logger,
			config,
			langEnvs,
			WithDockerLanguageConfigs(langConfigs),
		)

		// Test that the exclude patterns are correctly applied
		assert.NotNil(t, executor)
		assert.Equal(t, langConfigs, executor.langConfigs)
	})

	t.Run("PodmanExecutor with exclude patterns", func(t *testing.T) {
		config := &Config{
			TimeoutSec:        30,
			MemoryMB:          128,
			NetworkEnabled:    false,
			MaxArtifactSizeMB: 5,
		}

		langEnvs := &LanguageEnvironments{
			Python: map[string]string{"PYTHONPATH": "/workdir"},
		}

		// Create executor with exclude patterns
		langConfigs := &LanguageConfigs{
			Python: LanguageConfig{
				ExcludePatterns: []string{"__pycache__/", "*.pyc", "*.pyo", ".pytest_cache/", "node_modules/", "build/", "dist/"},
			},
		}

		executor := NewPodmanExecutor(
			logger,
			config,
			langEnvs,
			WithPodmanLanguageConfigs(langConfigs),
		)

		// Test that the exclude patterns are correctly applied
		assert.NotNil(t, executor)
		assert.Equal(t, langConfigs, executor.langConfigs)
	})

	t.Run("LocalExecutor with exclude patterns", func(t *testing.T) {
		config := &Config{
			TimeoutSec:        30,
			MemoryMB:          128,
			NetworkEnabled:    false,
			MaxArtifactSizeMB: 5,
		}

		langEnvs := &LanguageEnvironments{
			Python: map[string]string{"PYTHONPATH": "/workdir"},
		}

		// Create executor with exclude patterns
		langConfigs := &LanguageConfigs{
			Python: LanguageConfig{
				ExcludePatterns: []string{"__pycache__/", "*.pyc", "*.pyo", ".pytest_cache/", "node_modules/", "build/", "dist/"},
			},
		}

		executor := NewLocalExecutor(
			logger,
			config,
			langEnvs,
			WithLocalLanguageConfigs(langConfigs),
		)

		// Test that the exclude patterns are correctly applied
		assert.NotNil(t, executor)
		assert.Equal(t, langConfigs, executor.langConfigs)
	})
}

// TestFactoryWithExcludePatternsIntegration tests the factory integration with exclude patterns
func TestFactoryWithExcludePatternsIntegration(t *testing.T) {
	// This test requires a config with exclude patterns, which may not be possible
	// to create directly for testing factory functions. Let's test the option functions.

	logger := zaptest.NewLogger(t)

	config := &Config{
		TimeoutSec:        30,
		MemoryMB:          128,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 5,
	}

	langEnvs := &LanguageEnvironments{
		Python: map[string]string{"PYTHONPATH": "/workdir"},
		NodeJS: map[string]string{"NODE_PATH": "/workdir"},
		Go:     map[string]string{"GOCACHE": "/tmp/go-build"},
		CPP:    map[string]string{"LANG": "C.UTF-8"},
	}

	// Test all executors with language configs
	t.Run("Factory-style creation with exclude patterns", func(t *testing.T) {
		langConfigs := &LanguageConfigs{
			Python: LanguageConfig{
				ExcludePatterns: []string{"__pycache__/", "*.pyc", "main.py"}, // exclude source
			},
			NodeJS: LanguageConfig{
				ExcludePatterns: []string{"node_modules/", "index.js"}, // exclude source
			},
			Go: LanguageConfig{
				ExcludePatterns: []string{"main.go", "app"}, // exclude source and binary
			},
			CPP: LanguageConfig{
				ExcludePatterns: []string{"main.cpp", "app"}, // exclude source and binary
			},
		}

		// Test Docker executor creation with options
		dockerExec := NewDockerExecutor(
			logger,
			config,
			langEnvs,
			WithDockerLanguageConfigs(langConfigs),
		)
		assert.NotNil(t, dockerExec)
		assert.Equal(t, langConfigs, dockerExec.langConfigs)

		// Test Podman executor creation with options
		podmanExec := NewPodmanExecutor(
			logger,
			config,
			langEnvs,
			WithPodmanLanguageConfigs(langConfigs),
		)
		assert.NotNil(t, podmanExec)
		assert.Equal(t, langConfigs, podmanExec.langConfigs)

		// Test Local executor creation with options
		localExec := NewLocalExecutor(
			logger,
			config,
			langEnvs,
			WithLocalLanguageConfigs(langConfigs),
		)
		assert.NotNil(t, localExec)
		assert.Equal(t, langConfigs, localExec.langConfigs)
	})
}

// TestExecutorWithVariousLanguagesExcludePatterns tests exclude patterns for different languages
func TestExecutorWithVariousLanguagesExcludePatterns(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := &Config{
		TimeoutSec:        30,
		MemoryMB:          128,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 5,
	}

	langEnvs := &LanguageEnvironments{
		Python: map[string]string{"PYTHONPATH": "/workdir"},
		NodeJS: map[string]string{"NODE_PATH": "/workdir"},
		Go:     map[string]string{"GOCACHE": "/tmp/go-build"},
		CPP:    map[string]string{"LANG": "C.UTF-8"},
	}

	t.Run("Python exclude patterns", func(t *testing.T) {
		langConfigs := &LanguageConfigs{
			Python: LanguageConfig{
				ExcludePatterns: []string{"__pycache__/", "*.pyc", "*.pyo", "*.egg-info/", "main.py"},
			},
		}

		_ = NewDockerExecutor(
			logger,
			config,
			langEnvs,
			WithDockerLanguageConfigs(langConfigs),
		)

		// Simulate the exclude pattern matching logic
		shouldExcludeMainPy := shouldExcludeFile("main.py", langConfigs.Python.ExcludePatterns)
		assert.True(t, shouldExcludeMainPy)

		shouldExcludeCache := shouldExcludeFile("__pycache__/file.pyc", langConfigs.Python.ExcludePatterns)
		assert.True(t, shouldExcludeCache)

		shouldNotExcludeTxt := shouldExcludeFile("data.txt", langConfigs.Python.ExcludePatterns)
		assert.False(t, shouldNotExcludeTxt)
	})

	t.Run("NodeJS exclude patterns", func(t *testing.T) {
		langConfigs := &LanguageConfigs{
			NodeJS: LanguageConfig{
				ExcludePatterns: []string{"node_modules/", "*.js.map", "npm-debug.log*", "index.js"},
			},
		}

		_ = NewPodmanExecutor(
			logger,
			config,
			langEnvs,
			WithPodmanLanguageConfigs(langConfigs),
		)

		shouldExcludeNodeModules := shouldExcludeFile("node_modules/package.json", langConfigs.NodeJS.ExcludePatterns)
		assert.True(t, shouldExcludeNodeModules)

		shouldExcludeSourceMap := shouldExcludeFile("app.js.map", langConfigs.NodeJS.ExcludePatterns)
		assert.True(t, shouldExcludeSourceMap)

		shouldNotExcludeRegularJs := shouldExcludeFile("other.js", langConfigs.NodeJS.ExcludePatterns)
		assert.False(t, shouldNotExcludeRegularJs)
	})

	t.Run("Go exclude patterns", func(t *testing.T) {
		langConfigs := &LanguageConfigs{
			Go: LanguageConfig{
				ExcludePatterns: []string{"*.o", "*.a", "*.so", "main.go", "app", "go.sum", "go.mod"},
			},
		}

		_ = NewLocalExecutor(
			logger,
			config,
			langEnvs,
			WithLocalLanguageConfigs(langConfigs),
		)

		shouldExcludeObject := shouldExcludeFile("util.o", langConfigs.Go.ExcludePatterns)
		assert.True(t, shouldExcludeObject)

		shouldExcludeBinary := shouldExcludeFile("app", langConfigs.Go.ExcludePatterns)
		assert.True(t, shouldExcludeBinary)

		shouldExcludeSource := shouldExcludeFile("main.go", langConfigs.Go.ExcludePatterns)
		assert.True(t, shouldExcludeSource)

		shouldNotExcludeOther := shouldExcludeFile("config.yaml", langConfigs.Go.ExcludePatterns)
		assert.False(t, shouldNotExcludeOther)
	})

	t.Run("C++ exclude patterns", func(t *testing.T) {
		langConfigs := &LanguageConfigs{
			CPP: LanguageConfig{
				ExcludePatterns: []string{"*.o", "*.a", "*.so", "*.dll", "*.exe", "main.cpp", "app", "a.out", "a.exe"},
			},
		}

		_ = NewDockerExecutor(
			logger,
			config,
			langEnvs,
			WithDockerLanguageConfigs(langConfigs),
		)

		shouldExcludeObj := shouldExcludeFile("util.o", langConfigs.CPP.ExcludePatterns)
		assert.True(t, shouldExcludeObj)

		shouldExcludeSource := shouldExcludeFile("main.cpp", langConfigs.CPP.ExcludePatterns)
		assert.True(t, shouldExcludeSource)

		shouldExcludeBinary := shouldExcludeFile("app", langConfigs.CPP.ExcludePatterns)
		assert.True(t, shouldExcludeBinary)

		shouldNotExcludeHeader := shouldExcludeFile("header.h", langConfigs.CPP.ExcludePatterns)
		assert.False(t, shouldNotExcludeHeader)
	})
}

// TestExecutorExecuteWithExcludes tests the execution flow with exclude patterns
// Note: This test doesn't actually execute code, just verifies the setup
func TestExecutorExecuteWithExcludes(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := &Config{
		TimeoutSec:        30,
		MemoryMB:          128,
		NetworkEnabled:    false,
		MaxArtifactSizeMB: 5,
	}

	langEnvs := &LanguageEnvironments{
		Python: map[string]string{"PYTHONPATH": "/workdir"},
	}

	langConfigs := &LanguageConfigs{
		Python: LanguageConfig{
			ExcludePatterns: []string{"__pycache__/", "*.pyc", "temp/*", "*.tmp", "main.py"},
		},
	}

	executor := NewLocalExecutor(
		logger,
		config,
		langEnvs,
		WithLocalLanguageConfigs(langConfigs),
	)

	// Test that the executor is properly configured
	assert.NotNil(t, executor)
	assert.Equal(t, langConfigs, executor.langConfigs)

	// Verify that the exclude patterns are accessible and correct
	require.NotNil(t, executor.langConfigs)
	assert.Equal(t, []string{"__pycache__/", "*.pyc", "temp/*", "*.tmp", "main.py"}, executor.langConfigs.Python.ExcludePatterns)

	// Test individual pattern matching
	shouldExcludeCache := shouldExcludeFile("some/path/__pycache__/file.pyc", executor.langConfigs.Python.ExcludePatterns)
	assert.True(t, shouldExcludeCache)

	shouldExcludeSource := shouldExcludeFile("main.py", executor.langConfigs.Python.ExcludePatterns)
	assert.True(t, shouldExcludeSource)

	shouldNotExcludeOther := shouldExcludeFile("requirements.txt", executor.langConfigs.Python.ExcludePatterns)
	assert.False(t, shouldNotExcludeOther)
}
