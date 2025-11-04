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
		result := ApplyHooks("python", code, nil) // Second parameter ignored for backward compatibility
		assert.Equal(t, code, result)             // Since the function now returns code as-is for backward compatibility
	})

	t.Run("NodeJSHooks", func(t *testing.T) {
		code := "console.log('hello')"
		result := ApplyHooks("nodejs", code, nil) // Second parameter ignored for backward compatibility
		assert.Equal(t, code, result)
	})

	t.Run("GoNoHooks", func(t *testing.T) {
		code := "package main\nfunc main() { println(\"hello\") }"
		result := ApplyHooks("go", code, nil) // Second parameter ignored for backward compatibility
		assert.Equal(t, code, result)
	})

	t.Run("CppNoHooks", func(t *testing.T) {
		code := "#include <iostream>\nint main() { std::cout << \"hello\"; }"
		result := ApplyHooks("cpp", code, nil) // Second parameter ignored for backward compatibility
		assert.Equal(t, code, result)
	})

	t.Run("InvalidLanguage", func(t *testing.T) {
		code := "some code"
		result := ApplyHooks("invalid", code, nil) // Second parameter ignored for backward compatibility
		assert.Equal(t, code, result)
	})
}

func TestGetEnvironmentVariables(t *testing.T) {
	t.Run("PythonEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "python") // First parameter ignored for backward compatibility
		require.NoError(t, err)
		assert.Equal(t, map[string]string{}, env) // Returns empty map for backward compatibility
	})

	t.Run("NodeJSEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "nodejs") // First parameter ignored for backward compatibility
		require.NoError(t, err)
		assert.Equal(t, map[string]string{}, env)
	})

	t.Run("GoEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "go") // First parameter ignored for backward compatibility
		require.NoError(t, err)
		assert.Equal(t, map[string]string{}, env)
	})

	t.Run("CppEnv", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "cpp") // First parameter ignored for backward compatibility
		require.NoError(t, err)
		assert.Equal(t, map[string]string{}, env)
	})

	t.Run("InvalidLanguage", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "invalid") // First parameter ignored for backward compatibility
		require.NoError(t, err)                             // Function now always returns no error for backward compatibility
		assert.Equal(t, map[string]string{}, env)
	})

	t.Run("NilLangEnvs", func(t *testing.T) {
		env, err := GetEnvironmentVariables(nil, "python") // First parameter ignored for backward compatibility
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
