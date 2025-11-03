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
  # ... other language configurations
```

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