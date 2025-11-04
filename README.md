# Codebox - MCP Sandboxed Code Execution Server

A secure, configurable MCP server that executes untrusted user code in isolated sandboxes.

## Features

- Executes code in Python, Node.js, Go, and C++
- Sandboxing via Docker, Podman, or local execution
- Configurable resource limits (time, memory)
- Network isolation by default
- Base64-encoded tar for initial file system state
- Full stdout/stderr capture with exit codes
- Base64-encoded artifact tar of final working directory
- MCP protocol compliant with stdio and HTTP transports

## Architecture

- **MCP Protocol**: [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- **Logging**: [uber-go/zap](https://github.com/uber-go/zap)
- **Dependency Injection**: [uber-go/fx](https://github.com/uber-go/fx)
- **Configuration**: [spf13/viper](https://github.com/spf13/viper)

## Configuration

The server is configured via `config.yaml`:

```yaml
server:
  transport: "stdio"  # or "http"
  http_port: 8080

sandbox:
  backend: "docker"   # or "podman", "local"
  timeout_sec: 10
  memory_mb: 512
  max_artifact_size_mb: 20
  network_enabled: false
  enable_local_backend: false

languages:
  python:
    image: "python:3.11-slim"
    prefix_code: "..."
    postfix_code: "..."
    environment:  # Optional environment variables
      PYTHONPATH: "/workdir"
      PYTHONIOENCODING: "utf-8"
  # ... other language configurations with environment variables
```

Each language supports an optional `environment` section to set custom environment variables for the execution environment. These variables are passed to the execution runtime and can be used to control language-specific behavior.

## Usage

### Stdio Transport (Default)
```bash
go run cmd/server/main.go
```

### HTTP Transport
Update config.yaml:
```yaml
server:
  transport: "http"
  http_port: 8080
```

Then run:
```bash
go run cmd/server/main.go
```

## MCP Tool

The server exposes a single tool: `execute_sandboxed_code`

### Input
```json
{
  "code": "print('Hello, World!')",
  "language": "python",
  "workdir_tar": "base64-encoded-tar-optional"
}
```

### Output
```json
{
  "stdout": "Hello, World!\n",
  "stderr": "",
  "exit_code": 0,
  "artifacts_tar": "base64-encoded-tar-of-workdir"
}
```

## Security

- Code runs in isolated containers
- Resource limits (time, memory)
- Network disabled by default
- File system access restricted
- Non-root execution
- Path traversal protection

## Building

```bash
# Build the binary
go build -o codebox-server cmd/server/main.go

# Build Docker image
docker build -t codebox .
```

## Development

To run with local executor (not recommended for production):
```yaml
sandbox:
  backend: "local"
  enable_local_backend: true
```

Note: Local executor is insecure and only for development purposes.

## Development Tools

### Pre-commit Hooks

This repository uses pre-commit hooks to ensure code quality and consistency. To set up the pre-commit hooks:

1. Install pre-commit:
   ```bash
   pip install pre-commit
   ```

2. Install the git hooks:
   ```bash
   ./install-hooks.sh
   ```
   
   Or install manually:
   ```bash
   pre-commit install
   ```

The hooks will automatically run before each commit to:
- Format Go code with `gofmt`
- Format imports with `goimports`
- Run `go vet` for error checking
- Run `golangci-lint` for code linting
- Run all tests with `go test`
- Ensure the code builds successfully

To run the hooks manually on all files:
```bash
pre-commit run --all-files
```

### Code Quality and Testing

Before submitting pull requests, please ensure:

1. All tests pass:
   ```bash
   go test ./...
   ```

2. The code builds successfully:
   ```bash
   go build ./cmd/server
   ```

3. Code is properly formatted:
   ```bash
   gofmt -s -w .
   goimports -w -local github.com/isdmx/codebox .
   ```

4. Linting passes:
   ```bash
   golangci-lint run
   ```

### Dependency Management

Dependencies are managed with Go modules. To add a new dependency:
```bash
go get github.com/username/package@version
go mod tidy
```

To update dependencies:
```bash
go get -u
go mod tidy
```

Automatic dependency updates are handled by Dependabot.