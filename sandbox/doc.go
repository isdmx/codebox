// Package sandbox provides secure code execution capabilities.
//
// The sandbox package implements the execution engine for running untrusted
// code in isolated environments. It supports multiple backends including
// Docker, Podman, and local execution (for development).
//
// The package defines the SandboxExecutor interface and provides concrete
// implementations for different execution backends. Each executor handles
// the full lifecycle of code execution including setup, execution, and
// cleanup while enforcing security constraints.
//
// Usage:
//
//	executor, err := sandbox.NewExecutor(logger, config, "docker")
//	result, err := executor.Execute(ctx, sandbox.ExecuteRequest{
//	    Language:   "python",
//	    Code:       "print('Hello, World!')",
//	    TimeoutSec: 10,
//	})
package sandbox