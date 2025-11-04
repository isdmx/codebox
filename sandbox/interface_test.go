package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCodeFileName(t *testing.T) {
	tests := []struct {
		language string
		expected string
		hasError bool
	}{
		{"python", "main.py", false},
		{"nodejs", "index.js", false},
		{"go", "main.go", false},
		{"cpp", "main.cpp", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			result, err := GetCodeFileName(tt.language)
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetRunCommand(t *testing.T) {
	tests := []struct {
		language string
		expected string
		hasError bool
	}{
		{"python", "python main.py", false},
		{"nodejs", "node index.js", false},
		{"go", "go build -o app main.go && ./app", false},
		{"cpp", "g++ -std=c++17 -O2 -o app main.cpp && ./app", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			result, err := GetRunCommand(tt.language)
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestApplyHooks(t *testing.T) {
	t.Run("PythonHooks", func(t *testing.T) {
		code := "print('hello')"
		result := ApplyHooks("python", code)
		assert.Contains(t, result, "timeout_handler")
		assert.Contains(t, result, "print('hello')")
	})

	t.Run("NodeJSHooks", func(t *testing.T) {
		code := "console.log('hello')"
		result := ApplyHooks("nodejs", code)
		assert.Contains(t, result, "Set timeout for Node.js execution")
		assert.Contains(t, result, "console.log('hello')")
	})

	t.Run("GoNoHooks", func(t *testing.T) {
		code := "package main\nfunc main() { println(\"hello\") }"
		result := ApplyHooks("go", code)
		assert.Equal(t, code, result) // Go should have no hooks
	})

	t.Run("CppNoHooks", func(t *testing.T) {
		code := "#include <iostream>\nint main() { std::cout << \"hello\"; }"
		result := ApplyHooks("cpp", code)
		assert.Equal(t, code, result) // C++ should have no hooks
	})

	t.Run("InvalidLanguage", func(t *testing.T) {
		code := "some code"
		result := ApplyHooks("invalid", code)
		assert.Equal(t, code, result) // Invalid should return code unchanged
	})
}

func TestGetEnvironmentVariables(t *testing.T) {
	langEnvs := &LanguageEnvironments{
		Python: map[string]string{
			"PYTHONPATH": "/workdir",
		},
		NodeJS: map[string]string{
			"NODE_ENV": "production",
		},
		Go: map[string]string{
			"GOCACHE": "/tmp/go-build",
		},
		CPP: map[string]string{
			"LANG": "C.UTF-8",
		},
	}

	t.Run("PythonEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(langEnvs, "python")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"PYTHONPATH": "/workdir"}, env)
	})

	t.Run("NodeJSEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(langEnvs, "nodejs")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"NODE_ENV": "production"}, env)
	})

	t.Run("GoEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(langEnvs, "go")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"GOCACHE": "/tmp/go-build"}, env)
	})

	t.Run("CppEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(langEnvs, "cpp")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"LANG": "C.UTF-8"}, env)
	})

	t.Run("InvalidLanguage", func(t *testing.T) {
		env, err := GetEnvironmentVariables(langEnvs, "invalid")
		require.Error(t, err)
		// Function returns an empty map and an error
		// So the map is not nil but empty
		assert.Equal(t, map[string]string{}, env)
	})

	t.Run("NilLangEnvs", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "python")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{}, env)
	})

	t.Run("MissingLanguageEnv", func(t *testing.T) {
		emptyLangEnvs := &LanguageEnvironments{}
		env, err := GetEnvironmentVariables(emptyLangEnvs, "python")
		require.NoError(t, err)
		assert.Equal(t, map[string]string{}, env)
	})
}

func TestLanguageConstants(t *testing.T) {
	assert.Equal(t, "python", LanguagePython)
	assert.Equal(t, "nodejs", LanguageNodeJS)
	assert.Equal(t, "go", LanguageGo)
	assert.Equal(t, "cpp", LanguageCPP)
}

func TestFilePermissionAndSizeConstants(t *testing.T) {
	assert.Equal(t, 0755, int(DirPermission))
	assert.Equal(t, 0600, int(FilePermission))
	assert.Equal(t, 1024, BytesPerKB)
	assert.Equal(t, 1024*1024, MaxArtifactSizeMul)
}
