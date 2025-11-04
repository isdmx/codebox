# How to Use the Codebox MCP Server

## Installation

### Prerequisites
- Go 1.23+
- Docker or Podman (for sandboxing, optional for local development)
- Node.js (for the MCP inspector tool)

### Installing the MCP Inspector Tool
```bash
npm install -g @modelcontextprotocol/inspector
```

## Running the Server

### Stdio Transport (Default)
```bash
# Build the server
go build -o codebox-server cmd/server/main.go

# Run with default config (stdio transport)
./codebox-server
```

### HTTP Transport
Update your `config.yaml` to:
```yaml
server:
  transport: "http"
  http_port: 8080
```

Then run the server:
```bash
./codebox-server
```

### Using Docker
```bash
# Build the image
docker build -t codebox .

# Run with Docker (requires Docker socket access for nested containers)
docker run -v /var/run/docker.sock:/var/run/docker.sock -p 8080:8080 codebox
```

## Using the MCP Inspector

First, make sure the inspector tool is installed:
```bash
npm install -g @modelcontextprotocol/inspector
```

Check available options:
```bash
mcp-inspector --help
```

Once the server is running, you can connect using the MCP inspector:

### Using the CLI Mode 
The correct way to use the inspector with the Codebox server. Note that the `--cli` flag opens a browser-based interface:

```bash
# List available tools 
mcp-inspector --cli --transport http --method tools/list --server-url --target http://localhost:8080/mcp

# Execute Python code 
mcp-inspector --cli --transport http --method tools/call --server-url --target http://localhost:8080/mcp --tool-name execute_sandboxed_code --tool-arg language=python --tool-arg code="
import os
import sys

print(sys.version)
print(os.uname())
print('\n'.join(f'{k}={v}' for k, v in os.environ.items()))
"

# Execute Node.js code 
mcp-inspector --cli --transport http --method tools/call --server-url --target http://localhost:8080/mcp --tool-name execute_sandboxed_code --tool-arg language=nodejs --tool-arg code="
console.log(process.env);
"

# Execute Go code 
mcp-inspector --cli --transport http --method tools/call --server-url --target http://localhost:8080/mcp --tool-name execute_sandboxed_code --tool-arg language=go --tool-arg code="
package main

import \"fmt\"

func main() {
fmt.Println(\"Hello from a single Go file!\")
}
"

# Execute C++ code 
mcp-inspector --cli --transport http --method tools/call --server-url --target http://localhost:8080/mcp --tool-name execute_sandboxed_code --tool-arg language=cpp --tool-arg code="
#include <iostream>

int main() {
std::cout << \"Hello, World!\" << std::endl;
return 0;
}
"
```

### Using the CLI Mode with files



```bash
mcp-inspector --cli --transport http --method tools/call --server-url --target http://localhost:8080/mcp \
  --tool-name execute_sandboxed_code --tool-arg language=python --tool-arg code='
import os
import sys

f = open("report.txt", "w")
print(sys.version, file=f)
print(os.uname(), file=f)
print("\n".join(f"{k}={v}" for k, v in os.environ.items()), file=f)
f.close()
' | jq -r '.content.[0].text' | jq -r '.artifacts_tar' | base64 -D | tar tf -

```



## Configuration

The server behavior can be controlled via `config.yaml`:

- `server.transport`: "stdio" or "http"
- `server.http_port`: Port for HTTP transport (default: 8080)
- `sandbox.backend`: "docker", "podman", or "local"
- `sandbox.timeout_sec`: Execution timeout in seconds (default: 10)
- `sandbox.memory_mb`: Memory limit in MB (default: 512)
- `sandbox.max_artifact_size_mb`: Max size of returned artifacts (default: 20)
- `sandbox.network_enabled`: Whether to allow network access (default: false)
- `sandbox.enable_local_backend`: Enable local executor (default: false)
- Language-specific settings (container images, hooks, environment variables, etc.)

## Environment Variables

Each language can define custom environment variables in the configuration:

```yaml
languages:
  python:
    environment:
      PYTHONPATH: "/workdir"
      PYTHONIOENCODING: "utf-8"
      # Add any other Python-specific environment variables
  nodejs:
    environment:
      NODE_PATH: "/workdir"
      NODE_ENV: "production"
      # Add any other Node.js-specific environment variables
  go:
    environment:
      GOCACHE: "/tmp/go-build"
      GOMODCACHE: "/tmp/go-mod"
      # Add any other Go-specific environment variables
  cpp:
    environment:
      LANG: "C.UTF-8"
      LC_ALL: "C.UTF-8"
      # Add any other C++-specific environment variables
```

These environment variables will be available to the executed code during runtime.

## Security Features

- Code runs in isolated containers
- Resource limits enforced (time, memory)
- Network disabled by default
- File system access restricted to workdir
- Path traversal protection in tar operations
- Non-root execution in containers
- System call restrictions via container capabilities

## Development Notes

To run with local execution (NOT SECURE, for development only):
```yaml
sandbox:
  backend: "local"
  enable_local_backend: true
```

Warning: The local backend provides no security isolation and should never be used in production.
