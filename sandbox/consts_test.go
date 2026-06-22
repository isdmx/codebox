package sandbox

// Shared string constants used across the sandbox test suite. They exist to
// keep repeated literals in table-driven tests in a single place.
const (
	testFilePy         = "test.py"
	testFileMainJS     = "main.js"
	testFileGitConfig  = ".git/config"
	testFileNodePkg    = "node_modules/package.json"
	testFileApp        = "app"
	testPatternPy      = "*.py"
	testPatternPyc     = "*.pyc"
	testPatternPyo     = "*.pyo"
	testPatternObj     = "*.o"
	testDirNodeModules = "node_modules/"
	testDirGit         = ".git/"
	testDirPycache     = "__pycache__/"
	testDirBuild       = "build/"
	testDirDist        = "dist/"
	testDirPytestCache = ".pytest_cache/"
	testEnvPythonPath  = "PYTHONPATH"
)
