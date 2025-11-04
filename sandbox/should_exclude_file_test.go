package sandbox

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShouldExcludeFile tests the shouldExcludeFile function
func TestShouldExcludeFile(t *testing.T) {
	t.Run("excludes specific files", func(t *testing.T) {
		testCases := []struct {
			name           string
			relPath        string
			excludePattern string
			shouldExclude  bool
		}{
			{"exact file match", "main.py", "main.py", true},
			{"different file", "test.py", "main.py", false},
			{"wildcard extension", "script.py", "*.py", true},
			{"wildcard non-match", "script.js", "*.py", false},
			{"wildcard multiple chars", "main.pyc", "*.pyc", true},
			{"wildcard with prefix", "cache_main.py", "cache_*.py", true},
			{"wildcard no match", "main.js", "*.py", false},
			{"relative path", "src/main.py", "main.py", true},
			{"relative path wildcard", "src/cache/main.pyc", "*.pyc", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				excludePatterns := []string{tc.excludePattern}
				result := shouldExcludeFile(tc.relPath, excludePatterns)
				assert.Equal(t, tc.shouldExclude, result, "Expected %v for file %s with pattern %s", tc.shouldExclude, tc.relPath, tc.excludePattern)
			})
		}
	})

	t.Run("excludes directories", func(t *testing.T) {
		testCases := []struct {
			name           string
			relPath        string
			excludePattern string
			shouldExclude  bool
		}{
			{"directory pattern exact match", "node_modules/package.json", "node_modules/", true},
			{"directory pattern with subdir", "node_modules/deep/nested/file.js", "node_modules/", true},
			{"directory pattern no match", "src/main.js", "node_modules/", false},
			{"git directory", ".git/config", ".git/", true},
			{"git directory file", ".git/HEAD", ".git/", true},
			{"non-directory pattern on dir", "node_modules", "node_modules", false}, // without trailing slash, shouldn't match directory
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				excludePatterns := []string{tc.excludePattern}
				result := shouldExcludeFile(tc.relPath, excludePatterns)
				assert.Equal(t, tc.shouldExclude, result, "Expected %v for path %s with directory pattern %s", tc.shouldExclude, tc.relPath, tc.excludePattern)
			})
		}
	})

	t.Run("handles multiple patterns", func(t *testing.T) {
		excludePatterns := []string{"__pycache__/", "*.pyc", "node_modules/", ".git/"}

		testCases := []struct {
			name          string
			relPath       string
			shouldExclude bool
		}{
			{"matches first pattern", "__pycache__/file.pyc", true},
			{"matches second pattern", "cache.pyc", true},
			{"matches third pattern", "node_modules/package.json", true},
			{"matches fourth pattern", ".git/config", true},
			{"no match", "main.py", false},
			{"no match different extension", "main.js", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := shouldExcludeFile(tc.relPath, excludePatterns)
				assert.Equal(t, tc.shouldExclude, result, "Expected %v for path %s", tc.shouldExclude, tc.relPath)
			})
		}
	})

	t.Run("handles complex directory structures", func(t *testing.T) {
		excludePatterns := []string{"build/", "node_modules/", "*.o", "dist/"}

		testCases := []struct {
			name          string
			relPath       string
			shouldExclude bool
		}{
			{"nested build directory", "src/build/output.o", true},
			{"deep in node_modules", "frontend/node_modules/react/index.js", true},
			{"object files in different dirs", "lib/util.o", true},
			{"object file in deep dir", "src/subdir/module.o", true},
			{"dist directory", "dist/bundle.js", true},
			{"not excluded", "src/main.go", false},
			{"similar name not excluded", "building/tool.py", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := shouldExcludeFile(tc.relPath, excludePatterns)
				assert.Equal(t, tc.shouldExclude, result, "Expected %v for path %s", tc.shouldExclude, tc.relPath)
			})
		}
	})

	t.Run("filepath.Match error handling", func(t *testing.T) {
		// Test with an invalid pattern - should not crash but return false
		invalidPatterns := []string{"[invalid-pattern"}
		result := shouldExcludeFile("main.py", invalidPatterns)
		// If there's a pattern match error, it should return false (not exclude)
		assert.False(t, result)
	})

	t.Run("path separators work across platforms", func(t *testing.T) {
		// Test that the pattern matching works with different path separators
		excludePatterns := []string{"node_modules/"}

		testCases := []struct {
			name          string
			relPath       string
			shouldExclude bool
		}{
			{"forward slash", "node_modules/package.json", true},
			{"nested with forward slashes", "frontend/node_modules/react.js", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := shouldExcludeFile(tc.relPath, excludePatterns)
				assert.Equal(t, tc.shouldExclude, result, "Expected %v for path %s", tc.shouldExclude, tc.relPath)
			})
		}
	})
}

// TestFilePathMatchBehavior tests how filepath.Match behaves in our context
func TestFilePathMatchBehavior(t *testing.T) {
	t.Run("basic glob patterns", func(t *testing.T) {
		patterns := []struct {
			pattern string
			matches []string
			misses  []string
		}{
			{
				pattern: "*.txt",
				matches: []string{"file.txt", "main.txt", "config.txt"},
				misses:  []string{"file.html", "script.js", "README.md", "dir/file.txt"},
			},
			{
				pattern: "**/*.txt",                  // This won't work as expected with filepath.Match - it only supports basic glob; ** is treated as two * wildcards
				matches: []string{"subdir/file.txt"}, // matches because first * matches "subdir", second * matches "", and *.txt matches "file.txt"
				misses:  []string{"file.txt"},        // doesn't match because there's no directory component to match the first *
			},
			{
				pattern: "main.*",
				matches: []string{"main.py", "main.js", "main.go", "main.exe"},
				misses:  []string{"test.main.py", "py.main"},
			},
		}

		for _, p := range patterns {
			for _, match := range p.matches {
				t.Run(p.pattern+" matches "+match, func(t *testing.T) {
					matched, err := filepath.Match(p.pattern, match)
					require.NoError(t, err)
					assert.True(t, matched, "%s should match pattern %s", match, p.pattern)
				})
			}
			for _, miss := range p.misses {
				t.Run(p.pattern+" misses "+miss, func(t *testing.T) {
					matched, err := filepath.Match(p.pattern, miss)
					require.NoError(t, err)
					assert.False(t, matched, "%s should not match pattern %s", miss, p.pattern)
				})
			}
		}
	})
}
