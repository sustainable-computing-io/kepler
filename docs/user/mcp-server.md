# Using Kepler as an MCP Server

This guide explains how to use Kepler as a Model Context Protocol (MCP) server,
enabling AI assistants and other MCP clients to query power consumption data
using natural language.

## Overview

Kepler's MCP server provides three main capabilities:

- **Query top power consumers** by resource type (processes, containers, VMs, pods)
- **Get detailed power data** for specific resources
- **Search resources** with filters (power thresholds, name patterns)

> **⚠️ Important: Root Access Required**
>
> Kepler requires **root/sudo privileges** to read hardware power sensors (Intel RAPL) located in `/sys/class/powercap/intel-rapl/`.
>
> **For production**: Run with `sudo ./bin/kepler` or configure appropriate sudoers rules
> **For development/testing**: Use the fake CPU meter (`--config.file=test-mcp-config.yaml`) - no sudo required

## Prerequisites

- Kepler binary built with MCP support
- MCP-compatible client (Claude Code, etc.)
- **Root/sudo access**: Required for reading hardware power sensors (Intel RAPL)
  - Alternative: Use fake CPU meter for development/testing (no sudo required)
- Hardware with power measurement capabilities (Intel RAPL) or fake meter for testing

## Configuration

Kepler's MCP server supports multiple transport methods:

- **stdio** (default): Communication via standard input/output (for command-line tools)
- **sse**: Server-Sent Events over HTTP (for web-based clients)
- **streamable**: Streamable HTTP transport (for HTTP-based clients)

### Basic Configuration

Enable the MCP server in your Kepler configuration:

#### Via Configuration File

Create or edit your `kepler.yaml`:

**Stdio Transport (Default)**:

```yaml
log:
  level: info
  format: text

exporter:
  mcp:
    enabled: true
    transport: "stdio"   # Default - for command-line clients
  prometheus:
    enabled: true        # Can run alongside Prometheus exporter

# For testing without hardware sensors
dev:
  fake-cpu-meter:
    enabled: true
    zones: ["package", "core", "uncore", "dram"]
```

**HTTP Transport (SSE)**:

```yaml
log:
  level: info
  format: text

exporter:
  mcp:
    enabled: true
    transport: "sse"     # Server-Sent Events over HTTP
    httpPath: "/mcp"     # HTTP endpoint path
  prometheus:
    enabled: true

web:
  listenAddresses:
    - ":28282"           # HTTP server required for HTTP transport

dev:
  fake-cpu-meter:
    enabled: true
    zones: ["package", "core", "uncore", "dram"]
```

**Streamable HTTP Transport**:

```yaml
log:
  level: info
  format: text

exporter:
  mcp:
    enabled: true
    transport: "streamable"  # Streamable HTTP transport
    httpPath: "/mcp"         # HTTP endpoint path
  prometheus:
    enabled: true

web:
  listenAddresses:
    - ":28282"

dev:
  fake-cpu-meter:
    enabled: true
    zones: ["package", "core", "uncore", "dram"]
```

#### Via Command Line Flags

```bash
# Enable MCP server (stdio transport by default) - REQUIRES SUDO
sudo ./bin/kepler --exporter.mcp

# Enable MCP with HTTP transport (SSE) - REQUIRES SUDO
sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --exporter.mcp.http-path=/mcp

# Enable MCP with streamable HTTP transport - REQUIRES SUDO
sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=streamable --exporter.mcp.http-path=/mcp

# Enable MCP with debug logging - REQUIRES SUDO
sudo ./bin/kepler --exporter.mcp --log.level=debug

# Use fake meter for testing (NO SUDO REQUIRED)
./bin/kepler --exporter.mcp --config.file=test-config.yaml
```

### Production Configuration

For production environments with real hardware:

```yaml
log:
  level: info
  format: json

host:
  sysfs: "/sys"
  procfs: "/proc"

monitor:
  interval: 5s
  staleness: 500ms

exporter:
  mcp:
    enabled: true
  prometheus:
    enabled: true

web:
  listenAddresses:
    - ":28282"
```

## Quick Start with Claude Code

For the fastest setup with Claude Code:

1. **Build Kepler** (if not already done):

   ```bash
   make build
   # This creates the kepler binary at ./bin/kepler
   ```

2. **Add Kepler as an MCP server**:

   **For stdio transport (command-line)**:

   ```bash
   # Development/testing (no sudo required - uses fake CPU meter)
   claude mcp add kepler -- ./bin/kepler --exporter.mcp --config.file=test-mcp-config.yaml

   # Production (requires sudo for hardware access)
   claude mcp add kepler -- sudo ./bin/kepler --exporter.mcp --config.file=kepler.yaml
   ```

   **For HTTP transport (persistent server)**:

   ```bash
   # Start Kepler with HTTP transport in background
   ./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=test-mcp-http-sse-config.yaml &

   # Add MCP client that connects to HTTP endpoint
   claude mcp add --transport sse -s project kepler_http http://localhost:28282/mcp
   ```

3. **Check MCP server status**:

   ```
   /mcp status
   ```

4. **Test with a query**:
   > "What are the top 3 processes consuming power?"

That's it! Claude Code will automatically start Kepler when you make queries.

**Important Notes:**

- Use `claude mcp add` command, not manual JSON files
- The file is `.mcp.json` (with dot), not `mcp.json`
- For project-specific setup, use `-s project` flag

## Running Kepler MCP Server

### Local Development

**Stdio Transport (for command-line clients)**:

1. **Start Kepler with MCP enabled:**

```bash
sudo ./bin/kepler --config.file=kepler.yaml --exporter.mcp
```

2. **The MCP server uses stdio transport** - it communicates via standard input/output

   **Important**: When using stdio transport, Kepler automatically redirects all logs to stderr to prevent interference with the MCP JSON-RPC protocol over stdout/stdin. This ensures clean MCP communication.

**HTTP Transport (for persistent web-based clients)**:

1. **Start Kepler with HTTP transport:**

```bash
# Using SSE transport (fake meter - NO SUDO)
./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=test-mcp-http-sse-config.yaml

# Using streamable HTTP transport (fake meter - NO SUDO)
./bin/kepler --exporter.mcp --exporter.mcp.transport=streamable --config.file=test-mcp-http-streamable-config.yaml

# With hardware sensors (REQUIRES SUDO)
sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=kepler.yaml
```

2. **Kepler will output startup logs:**

```
INFO[...] Kepler version information version=...
INFO[...] Starting Kepler
INFO[...] Initializing MCP server transport=sse http_enabled=true http_path=/mcp
INFO[...] Registered MCP HTTP handler path=/mcp transport=sse
INFO[...] Starting MCP server transport=sse http_enabled=true
INFO[...] MCP server running via HTTP transport path=/mcp
```

3. **The MCP server is now available via HTTP at** `http://localhost:28282/mcp`

4. **Test the HTTP endpoint:**

```bash
# Check if MCP endpoint is accessible
curl -I http://localhost:28282/mcp

# The server will be listed on the API server landing page
curl http://localhost:28282/
```

### Docker Development

Use the development Docker Compose setup:

```bash
cd compose/dev
# Edit docker-compose.yml to add --exporter.mcp flag
docker-compose up -d
```

### Kubernetes Deployment

Add MCP configuration to your Kepler DaemonSet:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kepler
spec:
  template:
    spec:
      containers:
      - name: kepler
        image: kepler:latest
        command:
          - /usr/bin/kepler
          - --exporter.mcp
          - --config.file=/etc/kepler/kepler.yaml
        # ... rest of container spec
```

## MCP Tools Reference

### 1. list_top_consumers

Lists the top power consumers by resource type.

**Parameters:**

- `resource_type` (required): "node", "process", "container", "vm", or "pod"
- `limit` (optional): Maximum results (default: 5)
- `sort_by` (optional): "power" or "energy" (default: "power")

**Example Query:**
> "What are the top 3 processes consuming the most power?"

**MCP Tool Call:**

```json
{
  "resource_type": "process",
  "limit": 3,
  "sort_by": "power"
}
```

**Response:**

```
Top 3 process consumers:

1. Process: 1234, Name: firefox, Power: 15.30W, Energy: 45600J
2. Process: 5678, Name: chrome, Power: 12.75W, Energy: 38200J
3. Process: 9012, Name: python3, Power: 8.90W, Energy: 26700J
```

### 2. get_resource_power

Gets detailed power information for a specific resource.

**Parameters:**

- `resource_type` (required): "process", "container", "vm", or "pod"
- `resource_id` (required): Resource identifier (PID for processes, ID for others)

**Example Query:**
> "Show me detailed power data for container abc123"

**MCP Tool Call:**

```json
{
  "resource_type": "container",
  "resource_id": "abc123"
}
```

**Response:**

```text
Container Details:
ID: abc123
Name: webapp
Total Power: 18.45W
Total Energy: 55350J

Power by Zone:
  package: 12.30W
  core: 4.15W
  uncore: 1.50W
  dram: 0.50W

Metadata:
  runtime: containerd
  cpu_total_time: 142.50
  pod_id: pod-uuid-456
```

### 3. search_resources

Searches for resources matching specific criteria.

**Parameters:**

- `resource_type` (required): "process", "container", "vm", or "pod"
- `power_min` (optional): Minimum power consumption in watts
- `power_max` (optional): Maximum power consumption in watts
- `name_pattern` (optional): Name pattern to match (substring search)
- `limit` (optional): Maximum results (default: 10)

**Example Query:**
> "Find all containers using more than 10W of power"

**MCP Tool Call:**

```json
{
  "resource_type": "container",
  "power_min": 10.0,
  "limit": 5
}
```

**Response:**

```
Found 3 container resources matching criteria:

1. abc123: webapp, Power: 18.45W
2. def456: database, Power: 14.20W
3. ghi789: worker, Power: 11.85W
```

## Using HTTP MCP Endpoint

Kepler's HTTP MCP endpoint enables persistent server operation where MCP clients can connect over HTTP rather than spawning new processes. This is ideal for:

- **Web-based AI assistants** that need HTTP connectivity
- **Long-running sessions** where you want Kepler to stay running
- **Multiple concurrent clients** connecting to the same Kepler instance
- **Production deployments** where process spawning is not desirable

### HTTP Transport Types

Kepler supports two HTTP transport protocols:

1. **SSE (Server-Sent Events)**: Best for web browsers and clients that support EventSource
2. **Streamable HTTP**: General-purpose HTTP transport for any HTTP client

### Starting Kepler with HTTP Transport

**Using SSE Transport:**

```bash
# Start Kepler as persistent HTTP MCP server (with hardware sensors - REQUIRES SUDO)
sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=kepler.yaml

# Or with fake meter for testing (NO SUDO REQUIRED)
./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=test-mcp-http-sse-config.yaml

# Or with custom HTTP path (REQUIRES SUDO)
sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --exporter.mcp.http-path=/api/mcp
```

**Using Streamable HTTP Transport:**

```bash
# Start Kepler with streamable HTTP transport (with hardware sensors - REQUIRES SUDO)
sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=streamable --config.file=kepler.yaml

# Or with fake meter for testing (NO SUDO REQUIRED)
./bin/kepler --exporter.mcp --exporter.mcp.transport=streamable --config.file=test-mcp-http-streamable-config.yaml
```

### Verifying HTTP MCP Server

Once started, verify the HTTP MCP endpoint is accessible:

```bash
# Check if MCP endpoint responds
curl -I http://localhost:28282/mcp
# Should return: HTTP/1.1 200 OK

# Check server info page (lists available endpoints)
curl http://localhost:28282/
# Should show MCP server in the endpoint list

# For SSE transport, check Server-Sent Events headers
curl -H "Accept: text/event-stream" http://localhost:28282/mcp
```

### Connecting MCP Clients to HTTP Endpoint

**Claude Code with HTTP Transport:**

```bash
# Start Kepler HTTP server in background (with fake meter for testing - NO SUDO)
./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=test-mcp-http-sse-config.yaml &

# Or for production with real hardware sensors (REQUIRES SUDO)
# sudo ./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=kepler.yaml &

# Add HTTP MCP server to Claude Code
claude mcp add --transport sse -s project kepler_http http://localhost:28282/mcp

# Check connection
/mcp status
```

**Custom HTTP MCP Client (Python example):**

```python
import asyncio
import aiohttp
from mcp import ClientSession, SSEServerParameters

async def query_kepler_http():
    # Connect to Kepler HTTP MCP server
    async with ClientSession(
        SSEServerParameters(url="http://localhost:28282/mcp")
    ) as session:

        # Initialize connection
        await session.initialize()

        # List available tools
        tools = await session.list_tools()
        print("Available tools:", [tool.name for tool in tools])

        # Query top processes
        result = await session.call_tool(
            "list_top_consumers",
            {"resource_type": "process", "limit": 3}
        )
        print(result.content[0].text)

# Run the client
asyncio.run(query_kepler_http())
```

**Testing with curl (for development):**

```bash
# Example MCP JSON-RPC call over HTTP
curl -X POST http://localhost:28282/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "list_top_consumers",
      "arguments": {
        "resource_type": "process",
        "limit": 5
      }
    }
  }'
```

### HTTP Transport Configuration Options

**Complete HTTP Configuration Example:**

```yaml
log:
  level: debug  # Enable debug logging to see HTTP requests
  format: text

exporter:
  mcp:
    enabled: true
    transport: "sse"        # or "streamable"
    httpPath: "/mcp"        # Custom endpoint path
  prometheus:
    enabled: false          # Disable if only using MCP

web:
  listenAddresses:
    - ":28282"              # HTTP server port
  # Optional: Add TLS for secure connections
  # configFile: "/path/to/web-config.yml"

dev:
  fake-cpu-meter:
    enabled: true           # For testing without hardware
    zones: ["package", "core", "uncore", "dram"]
```

### Multiple Clients with HTTP Transport

HTTP transport supports multiple concurrent MCP clients:

```bash
# Start one Kepler HTTP server
./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=test-mcp-http-sse-config.yaml &

# Connect multiple Claude Code instances
claude mcp add --transport sse -s project kepler_http1 http://localhost:28282/mcp
claude mcp add --transport sse -s global kepler_http2 http://localhost:28282/mcp

# All clients share the same Kepler data source
```

### Production HTTP Deployment

**Systemd Service for HTTP MCP Server:**

```ini
[Unit]
Description=Kepler MCP HTTP Server
After=network.target

[Service]
Type=simple
User=kepler
Group=kepler
ExecStart=/usr/local/bin/kepler --exporter.mcp --exporter.mcp.transport=sse --config.file=/etc/kepler/mcp-http.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

**Docker HTTP MCP Server:**

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bin/kepler .
EXPOSE 28282
CMD ["./kepler", "--exporter.mcp", "--exporter.mcp.transport=sse", "--config.file=/etc/kepler/mcp-http.yaml"]
```

### HTTP Transport Troubleshooting

**Connection Issues:**

```bash
# Test basic connectivity
curl -v http://localhost:28282/mcp

# Check if port is open
netstat -tlnp | grep :28282

# Verify Kepler HTTP server logs
./bin/kepler --exporter.mcp --exporter.mcp.transport=sse --log.level=debug
```

**MCP Protocol Issues:**

```bash
# Test MCP initialization over HTTP
curl -X POST http://localhost:28282/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}}}'
```

**Performance Monitoring:**

```bash
# Monitor HTTP requests in Kepler logs
tail -f kepler.log | grep "HTTP"

# Check concurrent connections
ss -tnp | grep :28282
```

## Root Access and Hardware Requirements

### Why Kepler Needs Root Access

Kepler requires **root/sudo privileges** because it needs to read hardware power sensors:

```bash
# Intel RAPL (Running Average Power Limit) sensors are located in:
ls -la /sys/class/powercap/intel-rapl/
# These files are typically readable only by root:
# -r--r----- 1 root root ... intel-rapl:0:energy_uj
```

### Production Deployment Options

**Option 1: Run with sudo (simplest)**

```bash
sudo ./bin/kepler --exporter.mcp --config.file=kepler.yaml
```

**Option 2: Configure sudoers (recommended for automation)**

```bash
# Create /etc/sudoers.d/kepler
echo '%kepler ALL=(ALL) NOPASSWD: /usr/local/bin/kepler' | sudo tee /etc/sudoers.d/kepler

# Create kepler user and group
sudo useradd -r -s /bin/false kepler
sudo usermod -a -G kepler $USER

# Now you can run without typing password
sudo -u kepler kepler --exporter.mcp
```

**Option 3: Set capabilities (Linux-specific)**

```bash
# Give the binary permission to read hardware sensors
sudo setcap 'cap_sys_rawio=ep' ./bin/kepler

# Now kepler can read RAPL without full root
./bin/kepler --exporter.mcp
```

**Option 4: Privileged container**

```bash
# Docker with privileged access
docker run --privileged -v /sys:/sys -v /proc:/proc kepler:latest --exporter.mcp

# Or with specific capabilities
docker run --cap-add=SYS_RAWIO -v /sys:/sys -v /proc:/proc kepler:latest --exporter.mcp
```

### Development and Testing (No Root Required)

For development and testing, use the **fake CPU meter**:

```bash
# No sudo needed with fake meter
./bin/kepler --exporter.mcp --config.file=test-mcp-config.yaml

# The test config enables fake CPU meter:
# dev:
#   fake-cpu-meter:
#     enabled: true
#     zones: ["package", "core", "uncore", "dram"]
```

### Verifying Hardware Access

```bash
# Check if you can read RAPL sensors
sudo cat /sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj

# Check if intel_rapl module is loaded
lsmod | grep intel_rapl

# Check RAPL domains
sudo find /sys/class/powercap/intel-rapl -name "energy_uj" -exec ls -la {} \;
```

### Common Permission Errors

**Error**: `open /sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj: permission denied`
**Solution**: Run with sudo or configure proper permissions

**Error**: `intel_rapl: no such file or directory`
**Solution**: RAPL not available (VM/container) or intel_rapl module not loaded

**Error**: `failed to initialize CPU power meter`
**Solution**: Use fake CPU meter for testing or check hardware compatibility

## Integrating with MCP Clients

### Claude Code Integration

#### Step 1: Add Kepler as MCP Server

Use the `claude mcp add` command to register Kepler as an MCP server:

**For Development/Testing** (`mcp.json`):

```json
{
  "mcpServers": {
    "kepler": {
      "command": "sudo ./bin/kepler",
      "args": ["--exporter.mcp", "--config.file=test-mcp-config.yaml"],
      "env": {
        "PATH": "/usr/local/bin:/usr/bin:/bin"
      },
      "description": "Kepler power monitoring MCP server - provides energy consumption data for processes, containers, VMs, and pods"
    }
  }
}
```

**For Production** (`mcp-production.json`):

```json
{
  "mcpServers": {
    "kepler": {
      "command": "/usr/local/bin/kepler",
      "args": ["--exporter.mcp", "--config.file=/etc/kepler/kepler.yaml"],
      "env": {
        "PATH": "/usr/local/bin:/usr/bin:/bin",
        "KEPLER_LOG_LEVEL": "info"
      },
      "description": "Kepler power monitoring MCP server (production)"
    }
  }
}
```

**Important Configuration Notes:**

- **command**: Path to your Kepler binary (use absolute path for production)
- **args**: Must include `--exporter.mcp` flag and any config file
- **env**: Ensure PATH includes necessary directories
- **Working Directory**: Claude Code will run the command from the directory containing `mcp.json`

#### Step 2: Configure Claude Code Settings

Add the MCP server to your Claude Code configuration:

```bash
# Initialize MCP configuration (if not already done)
claude mcp install

# Or manually add to your Claude settings
```

#### Step 3: Test the Integration

1. **Start Claude Code** in the directory with your `mcp.json`
2. **Check MCP status:**

   ```
   /mcp status
   ```

3. **List available servers:**

   ```
   /mcp list
   ```

4. **Use natural language** to query power data:
   - "What processes are using the most power right now?"
   - "Show me power consumption for all containers"
   - "Find VMs with power usage above 20 watts"

#### Example Queries for Claude Code

Once configured, you can ask Claude Code:

- **"What are the top 5 processes consuming power?"**
  - This will call `list_top_consumers` with resource_type="process" and limit=5

- **"Show me detailed power information for process 1234"**
  - This will call `get_resource_power` with resource_type="process" and resource_id="1234"

- **"Find all containers using more than 15 watts of power"**
  - This will call `search_resources` with resource_type="container" and power_min=15

#### Troubleshooting Claude Code Integration

**MCP Server Not Found:**

```bash
# Check if mcp.json is in the current directory
ls -la mcp.json

# Verify Kepler binary path
which kepler
./bin/kepler --version
```

**Permission Issues:**

```bash
# Ensure Kepler has proper permissions
chmod +x ./bin/kepler

# For hardware access, may need sudo (see Security Notes below)
```

**Connection Errors:**

```bash
# Test Kepler MCP server manually
./bin/kepler --exporter.mcp --config.file=test-mcp-config.yaml --log.level=debug

# Check for error messages in logs
```

#### Security Note for Claude Code

Since Kepler requires hardware access (sudo), you have options:

1. **Development/Testing**: Use fake CPU meter (no sudo required)

   ```json
   "args": ["--exporter.mcp", "--config.file=test-mcp-config.yaml"]
   ```

2. **Production**: Configure sudoers for the kepler command

   ```bash
   # Add to /etc/sudoers.d/kepler
   %kepler ALL=(ALL) NOPASSWD: /path/to/kepler
   ```

3. **Container**: Run Kepler in privileged container

   ```json
   "command": "docker",
   "args": ["run", "--rm", "--privileged", "-i", "kepler:latest", "--exporter.mcp"]
   ```

### Custom MCP Client

To build your own MCP client:

```python
import asyncio
from mcp import ClientSession, StdioServerParameters

async def query_kepler():
    # Connect to Kepler MCP server
    async with ClientSession(
        StdioServerParameters(command=["./kepler", "--exporter.mcp"])
    ) as session:

        # List available tools
        tools = await session.list_tools()
        print("Available tools:", [tool.name for tool in tools])

        # Query top processes
        result = await session.call_tool(
            "list_top_consumers",
            {"resource_type": "process", "limit": 5}
        )
        print(result.content[0].text)

# Run the client
asyncio.run(query_kepler())
```

## Troubleshooting

### Common Issues

**1. MCP server not starting:**

```bash
# Check if flag is correctly set
./bin/kepler --exporter.mcp --log.level=debug

# Verify in logs
grep "MCP" kepler.log
```

**2. No power data available:**

```bash
# Enable fake CPU meter for testing
./bin/kepler --exporter.mcp --config.file=test-config.yaml

# Check hardware sensor access
sudo ls -la /sys/class/powercap/intel-rapl/
```

**3. Permission errors:**

```bash
# Kepler needs sudo for hardware access
sudo ./bin/kepler --exporter.mcp

# Or run in container with privileged access
docker run --privileged kepler:latest --exporter.mcp
```

### Debug Logging

Enable debug logging to troubleshoot MCP interactions:

```bash
./bin/kepler --exporter.mcp --log.level=debug
```

Debug logs will show:

- MCP tool registrations
- Incoming tool calls
- Data transformation steps
- Response generation

### Verification

Test that the MCP server is working:

1. **Check startup logs** for MCP initialization messages
2. **Verify tool registration** in debug logs
3. **Test with simple MCP client** or development tools
4. **Monitor resource consumption** to ensure data is available

## Performance Considerations

- **Memory Usage**: MCP server adds minimal overhead (~1-2MB)
- **CPU Impact**: Data transformation is lightweight, < 1ms per query
- **Concurrent Queries**: Supports multiple simultaneous MCP clients
- **Data Freshness**: Uses same snapshot mechanism as Prometheus exporter

## Security Notes

### Transport Security

- **Stdio Transport**: Uses standard input/output only, no network exposure
- **HTTP Transport**: Exposes network endpoint, consider these implications:
  - HTTP traffic is unencrypted (use reverse proxy with TLS for production)
  - No authentication required (suitable for trusted networks only)
  - Consider firewall rules to restrict access to authorized clients
  - For production, use TLS termination with reverse proxy (nginx, traefik)

### General Security

- **Read-Only**: MCP tools only read power data, cannot modify system
- **Resource Limits**: Built-in limits prevent resource exhaustion
- **Privileged Access**: Kepler requires sudo for hardware sensors (use fake meter for testing)

### Production HTTP Security Recommendations

```yaml
# Example secure HTTP configuration
web:
  listenAddresses:
    - "127.0.0.1:28282"  # Bind to localhost only
  configFile: "/etc/kepler/web-config.yml"  # Use TLS config

# web-config.yml example:
tls_server_config:
  cert_file: "/etc/ssl/certs/kepler.pem"
  key_file: "/etc/ssl/private/kepler.key"
```

**Reverse Proxy Example (nginx):**

```nginx
server {
    listen 443 ssl;
    server_name kepler.example.com;

    ssl_certificate /etc/ssl/certs/kepler.crt;
    ssl_certificate_key /etc/ssl/private/kepler.key;

    location /mcp {
        proxy_pass http://127.0.0.1:28282/mcp;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Next Steps

- Explore the [Configuration Guide](configuration.md) for advanced settings
- Review [Metrics Documentation](metrics.md) to understand power data
- Check the [Installation Guide](installation.md) for deployment options
- Try the MCP server with your preferred AI assistant or MCP client
