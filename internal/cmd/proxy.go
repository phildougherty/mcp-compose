// internal/cmd/proxy.go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mcpcompose/internal/auth"
	"mcpcompose/internal/config"
	"mcpcompose/internal/openapi"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// NewProxyCommand creates a command to run an MCP proxy server
func NewProxyCommand() *cobra.Command {
	var port int
	var generateConfig bool
	var clientType string
	var outputDir string
	var detach bool
	var containerized bool
	var apiKey string

	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run an MCP proxy server for all services",
		Long: `Run a proxy server that exposes all your MCP services through a unified HTTP endpoint.
This makes it easy to use your MCP servers with various clients like Claude Desktop,
OpenAI tools, or any other MCP-compatible client.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			// Load the configuration
			cfg, err := config.LoadConfig(file)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			// Get project name from config for container names
			projectName := filepath.Base(strings.TrimSuffix(file, filepath.Ext(file)))
			if projectName == "." || projectName == "" {
				cwd, err := os.Getwd()
				if err == nil {
					projectName = filepath.Base(cwd)
				} else {
					projectName = "mcp-compose"
				}
			}
			// If only generating config, do that and exit
			if generateConfig {
				return generateProxyClientConfig(cfg, projectName, port, clientType, outputDir)
			}
			// Run as a container if requested
			if containerized {
				return startContainerizedProxy(cfg, projectName, port, outputDir, apiKey)
			}
			// Start the proxy server
			if detach {
				return startDetachedProxyServer(file, port, projectName, apiKey)
			}
			// Run the proxy server in the foreground
			return startProxyServer(cfg, projectName, port, apiKey)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 9876, "Port to run the proxy server on")
	cmd.Flags().BoolVarP(&generateConfig, "generate-config", "g", false, "Generate client configuration file only")
	cmd.Flags().StringVarP(&clientType, "client", "t", "claude", "Client type (claude, openai, anthropic, all)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "client-config", "Output directory for client configuration")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run proxy server in the background")
	cmd.Flags().BoolVarP(&containerized, "container", "C", true, "Run proxy server as a container")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for securing the proxy server")

	return cmd
}

// startContainerizedProxy starts the proxy server as a Docker container
func startContainerizedProxy(cfg *config.ComposeConfig, projectName string, port int, outputDir string, apiKey string) error {
	// Create a temporary directory for the proxy files
	tempDir, err := os.MkdirTemp("", "mcp-proxy")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the proxy script
	proxyScriptPath := filepath.Join(tempDir, "proxy.py")
	err = createProxyScript(proxyScriptPath)
	if err != nil {
		return fmt.Errorf("failed to create proxy script: %w", err)
	}

	// Create Dockerfile for the proxy
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	err = createProxyDockerfile(dockerfilePath)
	if err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	// Build the proxy image
	fmt.Println("Building proxy container image...")
	buildCmd := exec.Command("docker", "build", "-t", "mcp-compose-proxy:latest", tempDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build proxy image: %w", err)
	}

	// Ensure proxy container is removed if it already exists
	exec.Command("docker", "rm", "-f", "mcp-compose-proxy").Run()

	// Start the proxy container
	fmt.Printf("Starting proxy container on port %d...\n", port)

	// Prepare docker run command
	runArgs := []string{
		"run", "-d",
		"--name", "mcp-compose-proxy",
		"-p", fmt.Sprintf("%d:%d", port, port),
		"--network", "mcp-net",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-e", fmt.Sprintf("MCP_PROXY_PORT=%d", port),
	}

	// Add API key if provided
	if apiKey != "" {
		runArgs = append(runArgs, "-e", fmt.Sprintf("MCP_API_KEY=%s", apiKey))
	}

	// Add the image name
	runArgs = append(runArgs, "mcp-compose-proxy:latest")

	proxyCmd := exec.Command("docker", runArgs...)

	output, err := proxyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start proxy container: %w, output: %s", err, string(output))
	}

	containerId := strings.TrimSpace(string(output))
	fmt.Printf("Proxy container started with ID: %s\n", containerId[:12])

	// Generate client configuration
	fmt.Println("Generating client configuration...")
	if err := generateProxyClientConfig(cfg, projectName, port, "claude", outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to generate client config: %v\n", err)
	}

	// Show proxy status after a brief pause to ensure it's running
	time.Sleep(1 * time.Second)
	statusCmd := exec.Command("docker", "ps", "--filter", "name=mcp-compose-proxy", "--format", "{{.Status}}")
	statusOutput, _ := statusCmd.Output()
	fmt.Printf("Proxy status: %s\n", strings.TrimSpace(string(statusOutput)))

	fmt.Printf("MCP Proxy is running and accessible at http://localhost:%d\n", port)
	if apiKey != "" {
		fmt.Printf("API key authentication is enabled. Use 'Bearer %s' in the Authorization header.\n", apiKey)
	} else {
		fmt.Println("API key authentication is disabled.")
	}
	fmt.Printf("Client configuration generated in %s/\n", outputDir)
	fmt.Println("To stop the proxy: docker stop mcp-compose-proxy")

	return nil
}

// createProxyScript creates the Python proxy server script
func createProxyScript(scriptPath string) error {
	scriptContent := `#!/usr/bin/env python3
import os
import sys
import json
import http.server
import socketserver
import subprocess
from urllib.parse import urlparse
import traceback
import re
import time
from typing import Dict, List, Any, Optional

PORT = int(os.environ.get('MCP_PROXY_PORT', '9876'))
API_KEY = os.environ.get('MCP_API_KEY', '')

class MCPProxyHandler(http.server.BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'  # Force HTTP/1.1 instead of HTTP/0.9
    # Cache for discovered tools
    _tools_cache = {}
    _tools_cache_time = 0
    _tools_cache_ttl = 60  # Cache TTL in seconds
    def get_mcp_servers(self):
        """Dynamically discover MCP servers using Docker"""
        try:
            # Get all running containers with the mcp-compose prefix
            result = subprocess.run(
                ["docker", "ps", "--filter", "name=mcp-compose-", "--format", "{{.Names}}"],
                capture_output=True,
                text=True,
                check=True
            )
            # Parse the output to get container names
            container_names = result.stdout.strip().split('\n')
            container_names = [name for name in container_names if name]  # Remove empty strings
            # Create a mapping of server names to container names
            servers = {}
            for container_name in container_names:
                # Extract server name from container name (assuming format: mcp-compose-{server_name})
                match = re.match(r'mcp-compose-(.*)', container_name)
                if match:
                    server_name = match.group(1)
                    # Skip the proxy container itself
                    if server_name == "proxy" or container_name == "mcp-compose-proxy":
                        self.log_message(f"Skipping proxy container: {container_name}")
                        continue
                    # Check if the container is actually running and not restarting
                    try:
                        status_result = subprocess.run(
                            ["docker", "inspect", "--format", "{{.State.Status}}", container_name],
                            capture_output=True,
                            text=True,
                            check=True
                        )
                        status = status_result.stdout.strip()
                        if status != "running":
                            self.log_message(f"Skipping server {server_name} because container is in state: {status}")
                            continue
                    except Exception as e:
                        self.log_message(f"Error checking container status for {server_name}: {str(e)}")
                        continue
                    servers[server_name] = container_name
            return servers
        except Exception as e:
            self.log_message("Error discovering MCP servers: %s", str(e))
            return {}
    def discover_tools(self, force_refresh=False):
        """Discover all tools from all MCP servers with caching"""
        current_time = time.time()
        # Return cached tools if available and not expired
        if not force_refresh and self._tools_cache and (current_time - self._tools_cache_time) < self._tools_cache_ttl:
            return self._tools_cache
        servers = self.get_mcp_servers()
        all_tools = {}
        server_info = {}
        for server_name, container_name in servers.items():
            try:
                # Initialize the server to get its tools
                init_request = {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "initialize",
                    "params": {
                        "protocolVersion": "2024-01-01",
                        "capabilities": {},
                        "clientInfo": {
                            "name": "MCP Proxy",
                            "version": "1.0.0"
                        }
                    }
                }
                # Execute the initialize request
                cmd = ["docker", "exec", "-i", container_name, "npx", "-y", f"@modelcontextprotocol/server-{server_name}"]
                if server_name == "filesystem":
                    cmd.append("/tmp")
                self.log_message(f"Initializing server {server_name} with command: {' '.join(cmd)}")
                process = subprocess.Popen(
                    cmd,
                    stdin=subprocess.PIPE,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True
                )
                stdout, stderr = process.communicate(input=json.dumps(init_request) + "\n", timeout=10)
                if process.returncode != 0:
                    self.log_message(f"Error initializing server {server_name}: {stderr}")
                    continue
                try:
                    # Check if the response is in SSE format (starts with "data: ")
                    if stdout.startswith("data: "):
                        # Extract the JSON part from the SSE format
                        json_part = stdout.replace("data: ", "").strip()
                        init_response = json.loads(json_part)
                    else:
                        # Regular JSON response
                        init_response = json.loads(stdout)
                    if "result" in init_response and "serverInfo" in init_response["result"]:
                        server_info[server_name] = init_response["result"]["serverInfo"]
                except json.JSONDecodeError:
                    self.log_message(f"Error parsing initialize response for {server_name}: {stdout}")
                # Now get the tools
                tools_request = {
                    "jsonrpc": "2.0",
                    "id": 2,
                    "method": "tools/list",
                    "params": {}
                }
                process = subprocess.Popen(
                    cmd,
                    stdin=subprocess.PIPE,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    text=True
                )
                stdout, stderr = process.communicate(input=json.dumps(tools_request) + "\n", timeout=10)
                if process.returncode != 0:
                    self.log_message(f"Error listing tools for server {server_name}: {stderr}")
                    continue
                try:
                    # Check if the response is in SSE format (starts with "data: ")
                    if stdout.startswith("data: "):
                        # Extract the JSON part from the SSE format
                        json_part = stdout.replace("data: ", "").strip()
                        response = json.loads(json_part)
                    else:
                        # Regular JSON response
                        response = json.loads(stdout)
                    if "result" in response and "tools" in response["result"]:
                        server_tools = response["result"]["tools"]
                        for tool in server_tools:
                            tool_name = tool.get("name")
                            if tool_name:
                                # Store the tool with its server
                                all_tools[f"{server_name}/{tool_name}"] = {
                                    "server": server_name,
                                    "tool": tool,
                                    "server_info": server_info.get(server_name, {})
                                }
                                self.log_message(f"Discovered tool: {server_name}/{tool_name}")
                except json.JSONDecodeError:
                    self.log_message(f"Error parsing tools response for {server_name}: {stdout}")
            except Exception as e:
                self.log_message(f"Error discovering tools for {server_name}: {str(e)}")
                self.log_message(traceback.format_exc())
        # Update cache
        self._tools_cache = all_tools
        self._tools_cache_time = current_time
        return all_tools

    def check_api_key(self):
        """Check if the API key is valid"""
        if not API_KEY:
            return True  # No API key configured, allow all requests
        # Skip API key check for OPTIONS requests
        if self.command == "OPTIONS":
            return True
        auth_header = self.headers.get('Authorization', '')
        if not auth_header.startswith('Bearer '):
            self.send_error(401, "Unauthorized: Missing or invalid Authorization header")
            return False
        token = auth_header[7:]  # Remove 'Bearer ' prefix
        if token != API_KEY:
            self.send_error(403, "Forbidden: Invalid API key")
            return False
        return True

    def add_cors_headers(self):
        """Add CORS headers to the response"""
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type, Authorization')
        self.send_header('Access-Control-Max-Age', '86400')  # 24 hours

    def do_OPTIONS(self):
        """Handle preflight CORS requests"""
        self.send_response(200)
        self.add_cors_headers()
        self.end_headers()

    def do_GET(self):
        # Check API key if configured
        if not self.check_api_key():
            return
        # Handle GET request - show available servers or OpenAPI schema
        if self.path == "/" or self.path == "":
            # Discover tools
            all_tools = self.discover_tools()
            # Group tools by server
            tools_by_server = {}
            for tool_path, tool_info in all_tools.items():
                server_name = tool_info["server"]
                if server_name not in tools_by_server:
                    tools_by_server[server_name] = []
                tools_by_server[server_name].append(tool_info["tool"]["name"])
            response = "<html><body><h1>MCP Proxy</h1>"
            # Add OpenAPI schema link
            response += "<p><a href='/openapi.json'>OpenAPI Schema</a> - <a href='/docs'>API Documentation</a></p>"
            # List servers and their tools
            response += "<h2>Available MCP Servers and Tools:</h2><ul>"
            for server_name, tools in tools_by_server.items():
                response += f"<li><strong>{server_name}</strong>: <ul>"
                for tool_name in tools:
                    response += f"<li>{tool_name}</li>"
                response += "</ul></li>"
            response += "</ul></body></html>"
            response_bytes = response.encode("utf-8")
            self.send_response(200)
            self.add_cors_headers()
            self.send_header("Content-type", "text/html")
            self.send_header("Content-Length", str(len(response_bytes)))
            self.end_headers()
            self.wfile.write(response_bytes)
        elif self.path == "/openapi.json":
            # Generate and serve the OpenAPI schema
            self.serve_openapi_schema()
        elif self.path == "/docs":
            # Serve Swagger UI
            self.serve_swagger_ui()
        elif self.path.endswith("/openapi.json"):
            # Server-specific OpenAPI schema
            server_name = self.path.split("/")[1]
            self.serve_server_openapi_schema(server_name)
        elif self.path.endswith("/docs"):
            # Server-specific Swagger UI
            server_name = self.path.split("/")[1]
            self.serve_server_swagger_ui(server_name)
        else:
            self.send_error(404, "Not found")

    def serve_openapi_schema(self):
        """Generate and serve the OpenAPI schema with individual tools"""
        # Discover all tools from all servers
        all_tools = self.discover_tools()
        # Create a basic OpenAPI schema
        schema = {
            "openapi": "3.0.0",
            "info": {
                "title": "MCP Tools API",
                "description": "API for MCP tools",
                "version": "1.0.0"
            },
            "servers": [
                {
                    "url": "/",
                    "description": "MCP Proxy Server"
                }
            ],
            "paths": {},
            "components": {
                "securitySchemes": {
                    "ApiKeyAuth": {
                        "type": "http",
                        "scheme": "bearer"
                    }
                },
                "schemas": {}
            },
            "security": [
                {
                    "ApiKeyAuth": []
                }
            ]
        }
        # Add paths for each tool
        for tool_path, tool_info in all_tools.items():
            server_name = tool_info["server"]
            tool = tool_info["tool"]
            tool_name = tool["name"]
            # Create path for this specific tool
            schema["paths"][f"/{server_name}/{tool_name}"] = {
                "post": {
                    "summary": tool.get("description", f"Call {tool_name}"),
                    "description": tool.get("description", f"Call the {tool_name} tool on {server_name} server"),
                    "operationId": f"{server_name}_{tool_name}".replace("-", "_"),
                    "requestBody": {
                        "required": True,
                        "content": {
                            "application/json": {
                                "schema": tool.get("inputSchema", {"type": "object"})
                            }
                        }
                    },
                    "responses": {
                        "200": {
                            "description": "Successful response",
                            "content": {
                                "application/json": {
                                    "schema": tool.get("outputSchema", {"type": "object"})
                                }
                            }
                        }
                    }
                }
            }
            # Add input schema to components if it has a title
            input_schema = tool.get("inputSchema", {})
            if "title" in input_schema:
                schema_name = input_schema["title"].replace(" ", "")
                schema["components"]["schemas"][schema_name] = input_schema
                # Reference the schema instead of embedding it
                schema["paths"][f"/{server_name}/{tool_name}"]["post"]["requestBody"]["content"]["application/json"]["schema"] = {
                    "$ref": f"#/components/schemas/{schema_name}"
                }
            # Add output schema to components if it has a title
            output_schema = tool.get("outputSchema", {})
            if "title" in output_schema:
                schema_name = output_schema["title"].replace(" ", "")
                schema["components"]["schemas"][schema_name] = output_schema
                # Reference the schema instead of embedding it
                schema["paths"][f"/{server_name}/{tool_name}"]["post"]["responses"]["200"]["content"]["application/json"]["schema"] = {
                    "$ref": f"#/components/schemas/{schema_name}"
                }
        # Convert schema to JSON
        schema_json = json.dumps(schema)
        self.send_response(200)
        self.add_cors_headers()
        self.send_header("Content-type", "application/json")
        self.send_header("Content-Length", str(len(schema_json)))
        self.end_headers()
        self.wfile.write(schema_json.encode("utf-8"))

    def serve_server_openapi_schema(self, server_name):
        """Generate and serve the OpenAPI schema for a specific server"""
        # Discover all tools from all servers
        all_tools = self.discover_tools()
        # Filter tools for this server
        server_tools = {k: v for k, v in all_tools.items() if v["server"] == server_name}
        if not server_tools:
            self.send_error(404, f"No tools found for server {server_name}")
            return
        # Get server info
        server_info = next(iter(server_tools.values()))["server_info"]
        # Create a basic OpenAPI schema
        schema = {
            "openapi": "3.0.0",
            "info": {
                "title": f"{server_name} API",
                "description": f"API for {server_name} MCP server",
                "version": server_info.get("version", "1.0.0")
            },
            "servers": [
                {
                    "url": f"/{server_name}",
                    "description": f"{server_name} MCP Server"
                }
            ],
            "paths": {},
            "components": {
                "securitySchemes": {
                    "ApiKeyAuth": {
                        "type": "http",
                        "scheme": "bearer"
                    }
                },
                "schemas": {}
            },
            "security": [
                {
                    "ApiKeyAuth": []
                }
            ]
        }
        # Add paths for each tool
        for tool_path, tool_info in server_tools.items():
            tool = tool_info["tool"]
            tool_name = tool["name"]
            # Create path for this specific tool
            schema["paths"][f"/{tool_name}"] = {
                "post": {
                    "summary": tool.get("description", f"Call {tool_name}"),
                    "description": tool.get("description", f"Call the {tool_name} tool"),
                    "operationId": f"{tool_name}".replace("-", "_"),
                    "requestBody": {
                        "required": True,
                        "content": {
                            "application/json": {
                                "schema": tool.get("inputSchema", {"type": "object"})
                            }
                        }
                    },
                    "responses": {
                        "200": {
                            "description": "Successful response",
                            "content": {
                                "application/json": {
                                    "schema": tool.get("outputSchema", {"type": "object"})
                                }
                            }
                        }
                    }
                }
            }
            # Add input schema to components if it has a title
            input_schema = tool.get("inputSchema", {})
            if "title" in input_schema:
                schema_name = input_schema["title"].replace(" ", "")
                schema["components"]["schemas"][schema_name] = input_schema
                # Reference the schema instead of embedding it
                schema["paths"][f"/{tool_name}"]["post"]["requestBody"]["content"]["application/json"]["schema"] = {
                    "$ref": f"#/components/schemas/{schema_name}"
                }
            # Add output schema to components if it has a title
            output_schema = tool.get("outputSchema", {})
            if "title" in output_schema:
                schema_name = output_schema["title"].replace(" ", "")
                schema["components"]["schemas"][schema_name] = output_schema
                # Reference the schema instead of embedding it
                schema["paths"][f"/{tool_name}"]["post"]["responses"]["200"]["content"]["application/json"]["schema"] = {
                    "$ref": f"#/components/schemas/{schema_name}"
                }
        # Convert schema to JSON
        schema_json = json.dumps(schema)
        self.send_response(200)
        self.add_cors_headers()
        self.send_header("Content-type", "application/json")
        self.send_header("Content-Length", str(len(schema_json)))
        self.end_headers()
        self.wfile.write(schema_json.encode("utf-8"))

    def serve_swagger_ui(self):
        """Serve Swagger UI for all tools"""
        html = """
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>MCP Tools API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      const ui = SwaggerUIBundle({
        url: "./openapi.json",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout",
        requestInterceptor: (req) => {
          // Add API key to all requests if it exists in localStorage
          const apiKey = localStorage.getItem('swagger_api_key');
          if (apiKey) {
            req.headers['Authorization'] = 'Bearer ' + apiKey;
          }
          return req;
        }
      });
      // Add API key input field
      const topbarEl = document.querySelector('.topbar');
      if (topbarEl) {
        const apiKeyContainer = document.createElement('div');
        apiKeyContainer.className = 'swagger-ui topbar-wrapper';
        apiKeyContainer.style.display = 'flex';
        apiKeyContainer.style.alignItems = 'center';
        apiKeyContainer.style.marginRight = '1em';
        const apiKeyLabel = document.createElement('label');
        apiKeyLabel.innerText = 'API Key: ';
        apiKeyLabel.style.marginRight = '0.5em';
        const apiKeyInput = document.createElement('input');
        apiKeyInput.type = 'text';
        apiKeyInput.value = localStorage.getItem('swagger_api_key') || '';
        apiKeyInput.placeholder = 'Enter API key';
        apiKeyInput.addEventListener('change', (e) => {
          localStorage.setItem('swagger_api_key', e.target.value);
          // Reload to apply the new API key
          window.location.reload();
        });
        apiKeyContainer.appendChild(apiKeyLabel);
        apiKeyContainer.appendChild(apiKeyInput);
        topbarEl.appendChild(apiKeyContainer);
      }
    };
  </script>
</body>
</html>
        """
        self.send_response(200)
        self.add_cors_headers()
        self.send_header("Content-type", "text/html")
        self.send_header("Content-Length", str(len(html)))
        self.end_headers()
        self.wfile.write(html.encode("utf-8"))

    def serve_server_swagger_ui(self, server_name):
        """Serve Swagger UI for a specific server"""
        html = f"""
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{server_name} API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {{
      const ui = SwaggerUIBundle({{
        url: "/{server_name}/openapi.json",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout",
        requestInterceptor: (req) => {{
          // Add API key to all requests if it exists in localStorage
          const apiKey = localStorage.getItem('swagger_api_key');
          if (apiKey) {{
            req.headers['Authorization'] = 'Bearer ' + apiKey;
          }}
          return req;
        }}
      }});
      // Add API key input field
      const topbarEl = document.querySelector('.topbar');
      if (topbarEl) {{
        const apiKeyContainer = document.createElement('div');
        apiKeyContainer.className = 'swagger-ui topbar-wrapper';
        apiKeyContainer.style.display = 'flex';
        apiKeyContainer.style.alignItems = 'center';
        apiKeyContainer.style.marginRight = '1em';
        const apiKeyLabel = document.createElement('label');
        apiKeyLabel.innerText = 'API Key: ';
        apiKeyLabel.style.marginRight = '0.5em';
        const apiKeyInput = document.createElement('input');
        apiKeyInput.type = 'text';
        apiKeyInput.value = localStorage.getItem('swagger_api_key') || '';
        apiKeyInput.placeholder = 'Enter API key';
        apiKeyInput.addEventListener('change', (e) => {{
          localStorage.setItem('swagger_api_key', e.target.value);
          // Reload to apply the new API key
          window.location.reload();
        }});
        apiKeyContainer.appendChild(apiKeyLabel);
        apiKeyContainer.appendChild(apiKeyInput);
        topbarEl.appendChild(apiKeyContainer);
      }}
    }};
  </script>
</body>
</html>
        """
        self.send_response(200)
        self.add_cors_headers()
        self.send_header("Content-type", "text/html")
        self.send_header("Content-Length", str(len(html)))
        self.end_headers()
        self.wfile.write(html.encode("utf-8"))

    def do_POST(self):
        # Check API key if configured
        if not self.check_api_key():
            return
        # Parse the path to get server name and possibly tool name
        path_parts = urlparse(self.path).path.strip("/").split("/")
        if not path_parts:
            self.send_error(404, "No server specified")
            return
        server_name = path_parts[0]
        tool_name = path_parts[1] if len(path_parts) > 1 else None
        # Get the current list of MCP servers
        servers = self.get_mcp_servers()
        if server_name not in servers:
            self.send_error(404, f"Unknown server: {server_name}")
            return
        # Get the container name
        container_name = servers[server_name]
        # Read the request body
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length).decode("utf-8")
        # If a specific tool is requested, create a tools/call request
        if tool_name:
            try:
                # Parse the request body as the tool arguments
                arguments = json.loads(body)
                # Create a tools/call request
                mcp_request = {
                    "jsonrpc": "2.0",
                    "id": 1,
                    "method": "tools/call",
                    "params": {
                        "name": tool_name,
                        "arguments": arguments
                    }
                }
                # Convert to JSON
                body = json.dumps(mcp_request) + "\n"
            except json.JSONDecodeError:
                self.send_error(400, "Invalid JSON in request body")
                return
        else:
            # Ensure the request ends with a newline for direct server requests
            if not body.endswith('\n'):
                body += '\n'
        self.log_message("Request to %s: %s", server_name, body[:100] + "..." if len(body) > 100 else body)
        try:
            # Use the container-native approach to communicate with the MCP server
            cmd = ["docker", "exec", "-i", container_name, "npx", "-y", f"@modelcontextprotocol/server-{server_name}"]
            # Add additional arguments for specific servers
            if server_name == "filesystem":
                cmd.append("/tmp")
            self.log_message("Executing command: %s", " ".join(cmd))
            # Execute the command
            process = subprocess.Popen(
                cmd,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=False  # Use binary mode for stdin/stdout
            )
            # Send the request to the container
            stdout, stderr = process.communicate(input=body.encode("utf-8"), timeout=10)
            stderr_text = stderr.decode("utf-8", errors="ignore")
            if stderr_text and "Secure MCP Filesystem Server running on stdio" not in stderr_text:
                self.log_message("STDERR: %s", stderr_text)
            self.log_message("Process return code: %d", process.returncode)
            if process.returncode != 0:
                self.send_error(500, f"Error communicating with container (code {process.returncode}): {stderr_text}")
                return
            # Get the response
            response_data = stdout.decode("utf-8", errors="ignore")
            
            # Always remove SSE format if present before further processing
            if response_data.startswith("data: "):
                response_data = response_data.replace("data: ", "").strip()
                stdout = response_data.encode("utf-8")
                
            # Log a snippet of the response
            self.log_message("Response from %s: %s", server_name, 
                             response_data[:100] + "..." if len(response_data) > 100 else response_data)
            # Process the response if it's a tool call
            if tool_name:
                try:
                    # Parse the JSON response (already cleaned from SSE format)
                    response_json = json.loads(response_data)
                        
                    # Check for error
                    if "error" in response_json:
                        error_code = response_json["error"].get("code", 500)
                        error_message = response_json["error"].get("message", "Unknown error")
                        self.send_error(500, f"MCP error: {error_message} (code {error_code})")
                        return
                    # Extract result from tools/call response
                    if "result" in response_json and "content" in response_json["result"]:
                        content = response_json["result"]["content"]
                        processed_response = []
                        for item in content:
                            if "text" in item:
                                # Try to parse as JSON
                                try:
                                    text_json = json.loads(item["text"])
                                    processed_response.append(text_json)
                                except json.JSONDecodeError:
                                    processed_response.append(item["text"])
                            elif "data" in item and "mimeType" in item:
                                # Handle image data
                                processed_response.append({
                                    "type": "image",
                                    "mimeType": item["mimeType"],
                                    "data": item["data"]
                                })
                        # If there's only one item, return it directly
                        if len(processed_response) == 1:
                            processed_response = processed_response[0]
                        # Convert to JSON
                        response_data = json.dumps(processed_response)
                        stdout = response_data.encode("utf-8")
                except json.JSONDecodeError:
                    # If we can't parse the response, just return it as-is
                    pass
                    
            # Send the response
            self.send_response(200)
            self.send_header("Content-type", "application/json")
            self.add_cors_headers()
            # Add Content-Length header
            self.send_header("Content-Length", str(len(stdout)))
            self.end_headers()
            self.wfile.write(stdout)
        except subprocess.TimeoutExpired:
            self.send_error(504, "Timeout communicating with container")
        except Exception as e:
            self.log_message("Error: %s", str(e))
            self.log_message("Traceback: %s", traceback.format_exc())
            self.send_error(500, f"Error: {str(e)}")

def main():
    # Get initial list of servers for logging
    try:
        result = subprocess.run(
            ["docker", "ps", "--filter", "name=mcp-compose-", "--format", "{{.Names}}"],
            capture_output=True,
            text=True,
            check=True
        )
        container_names = result.stdout.strip().split('\n')
        container_names = [name for name in container_names if name]
        servers = {}
        for container_name in container_names:
            match = re.match(r'mcp-compose-(.*)', container_name)
            if match:
                server_name = match.group(1)
                # Skip the proxy container itself
                if server_name == "proxy" or container_name == "mcp-compose-proxy":
                    print(f"Skipping proxy container: {container_name}")
                    continue
                servers[server_name] = container_name
        server_list = list(servers.keys())
    except Exception as e:
        print(f"Warning: Error discovering initial MCP servers: {str(e)}")
        server_list = []
    print(f"Starting MCP proxy server at http://0.0.0.0:{PORT}")
    if API_KEY:
        print("API key authentication is enabled")
    else:
        print("API key authentication is disabled")
    print(f"Available endpoints: {', '.join('/' + s for s in server_list)}")
    # Test Docker connectivity
    try:
        result = subprocess.run(["docker", "ps"], capture_output=True, text=True)
        print("Docker connectivity test:")
        print(f"Return code: {result.returncode}")
        if result.stdout:
            print(f"Output: {result.stdout[:200]}...")
        if result.stderr:
            print(f"Error: {result.stderr}")
    except Exception as e:
        print(f"Docker connectivity test failed: {str(e)}")
    server_class = socketserver.ThreadingTCPServer
    server_class.allow_reuse_address = True
    try:
        with server_class(("", PORT), MCPProxyHandler) as httpd:
            httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nProxy server stopped")

if __name__ == "__main__":
    main()
`
	return os.WriteFile(scriptPath, []byte(scriptContent), 0755)
}

// createProxyDockerfile creates the Dockerfile for the proxy container
func createProxyDockerfile(dockerfilePath string) error {
	dockerfileContent := `FROM python:3.10-slim

# Install basic dependencies
RUN apt-get update && \
    apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release && \
    apt-get clean

# Set up Docker repository with architecture detection
RUN ARCH=$(dpkg --print-architecture) && \
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg && \
    echo "deb [arch=${ARCH} signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

# Try to install Docker CLI, with fallbacks for different architectures and package names
RUN apt-get update && \
    (apt-get install -y docker-ce-cli || \
     apt-get install -y docker-ce || \
     (curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh && rm get-docker.sh)) && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY proxy.py .

# Set environment variables
ENV MCP_PROXY_PORT=9876
ENV MCP_API_KEY=""

EXPOSE 9876
CMD ["python", "proxy.py"]
`
	return os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644)
}

// startProxyServer starts the MCP proxy server in the foreground
func startProxyServer(cfg *config.ComposeConfig, projectName string, port int, apiKey string) error {
	fmt.Printf("Starting MCP Proxy Server on port %d...\n", port)

	// List available servers
	servers := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		servers = append(servers, name)
	}
	fmt.Printf("Available MCP servers: %s\n", strings.Join(servers, ", "))

	// Create HTTP server
	mux := http.NewServeMux()

	// Handler for root path - show available servers
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><h1>MCP Proxy Server</h1><p>Available endpoints:</p><ul>")
		for name := range cfg.Servers {
			fmt.Fprintf(w, "<li>/%s - %s MCP Server (<a href='/%s/docs'>API Docs</a>)</li>", name, name, name)
		}
		fmt.Fprintf(w, "<li><a href='/openapi.json'>OpenAPI Schema</a> - <a href='/docs'>API Documentation</a></li>")
		fmt.Fprintf(w, "</ul></body></html>")
	})

	// Add OpenAPI schema endpoint for all servers
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Convert MCP tools to OpenAPI tools
		tools := []openapi.Tool{}
		for name, server := range cfg.Servers {
			// Fetch tools for each server
			serverTools, err := fetchToolsForServer(name, server, projectName)
			if err != nil {
				log.Printf("Warning: Failed to fetch tools for server %s: %v", name, err)
				continue
			}
			tools = append(tools, serverTools...)
		}

		// Generate OpenAPI schema
		schema, err := openapi.GenerateOpenAPISchema(projectName, tools)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate OpenAPI schema: %v", err), http.StatusInternalServerError)
			return
		}

		// Set content type before writing response
		w.Header().Set("Content-Type", "application/json")

		// Use proper HTTP response
		w.WriteHeader(http.StatusOK)

		// Write the response
		if err := json.NewEncoder(w).Encode(schema); err != nil {
			log.Printf("Error encoding OpenAPI schema: %v", err)
		}
	})

	// Add Swagger UI endpoint
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		html := `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>MCP OpenAPI Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "./openapi.json",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout",
      });
    };
  </script>
</body>
</html>
`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	// Add server-specific endpoints
	for name, server := range cfg.Servers {
		serverName := name
		containerName := fmt.Sprintf("%s-%s", projectName, serverName)

		// Add server-specific OpenAPI schema endpoint
		mux.HandleFunc("/"+serverName+"/openapi.json", func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Fetch tools for this specific server
			tools, err := fetchToolsForServer(serverName, server, projectName)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to fetch tools for server %s: %v", serverName, err), http.StatusInternalServerError)
				return
			}

			// Generate OpenAPI schema
			schema, err := openapi.GenerateOpenAPISchema(serverName, tools)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to generate OpenAPI schema: %v", err), http.StatusInternalServerError)
				return
			}

			// Set content type before writing response
			w.Header().Set("Content-Type", "application/json")

			// Use proper HTTP response
			w.WriteHeader(http.StatusOK)

			// Write the response
			if err := json.NewEncoder(w).Encode(schema); err != nil {
				log.Printf("Error encoding OpenAPI schema: %v", err)
			}
		})

		// Add server-specific Swagger UI endpoint
		mux.HandleFunc("/"+serverName+"/docs", func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s MCP OpenAPI Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/%s/openapi.json",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout",
      });
    };
  </script>
</body>
</html>
`, serverName, serverName)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
		})

		// Add handler for MCP server
		mux.HandleFunc("/"+serverName, func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			if r.Method != "POST" {
				http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
				return
			}

			// Read request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}

			// Forward to container
			cmd := exec.Command("docker", "exec", "-i", containerName, "npx", "-y", fmt.Sprintf("@modelcontextprotocol/server-%s", serverName))

			// Add additional arguments for specific servers
			if serverName == "filesystem" {
				cmd.Args = append(cmd.Args, "/tmp")
			}

			cmd.Stdin = strings.NewReader(string(body))
			output, err := cmd.CombinedOutput()
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to communicate with container: %v", err), http.StatusInternalServerError)
				return
			}

			// Set content type and return the response
			w.Header().Set("Content-Type", "application/json")
			w.Write(output)
		})

		// Add handlers for tool endpoints
		tools, err := fetchToolsForServer(serverName, server, projectName)
		if err != nil {
			log.Printf("Warning: Failed to fetch tools for server %s: %v", serverName, err)
		} else {
			for _, tool := range tools {
				toolName := tool.Name
				mux.HandleFunc("/"+serverName+"/"+toolName, func(w http.ResponseWriter, r *http.Request) {
					// Set CORS headers
					w.Header().Set("Access-Control-Allow-Origin", "*")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

					if r.Method == "OPTIONS" {
						w.WriteHeader(http.StatusOK)
						return
					}

					if r.Method != "POST" {
						http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
						return
					}

					// Read request body
					body, err := io.ReadAll(r.Body)
					if err != nil {
						http.Error(w, "Failed to read request body", http.StatusBadRequest)
						return
					}

					// Parse request body
					var requestData map[string]interface{}
					if err := json.Unmarshal(body, &requestData); err != nil {
						http.Error(w, "Invalid JSON", http.StatusBadRequest)
						return
					}

					// Create MCP request
					mcpRequest := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      1,
						"method":  "tools/call",
						"params": map[string]interface{}{
							"name":      toolName,
							"arguments": requestData,
						},
					}

					// Convert to JSON
					mcpRequestJSON, err := json.Marshal(mcpRequest)
					if err != nil {
						http.Error(w, "Failed to marshal MCP request", http.StatusInternalServerError)
						return
					}

					// Forward to container
					cmd := exec.Command("docker", "exec", "-i", containerName, "npx", "-y", fmt.Sprintf("@modelcontextprotocol/server-%s", serverName))

					// Add additional arguments for specific servers
					if serverName == "filesystem" {
						cmd.Args = append(cmd.Args, "/tmp")
					}

					cmd.Stdin = strings.NewReader(string(mcpRequestJSON) + "\n")
					output, err := cmd.CombinedOutput()
					if err != nil {
						http.Error(w, fmt.Sprintf("Failed to communicate with container: %v", err), http.StatusInternalServerError)
						return
					}

					// Parse MCP response
					var mcpResponse map[string]interface{}
					if err := json.Unmarshal(output, &mcpResponse); err != nil {
						http.Error(w, "Failed to parse MCP response", http.StatusInternalServerError)
						return
					}

					// Extract result
					if result, ok := mcpResponse["result"].(map[string]interface{}); ok {
						if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
							// Extract text content
							if textContent, ok := content[0].(map[string]interface{}); ok {
								if text, ok := textContent["text"].(string); ok {
									// Try to parse as JSON
									var jsonResult interface{}
									if err := json.Unmarshal([]byte(text), &jsonResult); err == nil {
										// Return parsed JSON
										w.Header().Set("Content-Type", "application/json")
										json.NewEncoder(w).Encode(jsonResult)
										return
									}

									// Return as plain text
									w.Header().Set("Content-Type", "application/json")
									json.NewEncoder(w).Encode(map[string]interface{}{
										"result": text,
									})
									return
								}
							}
						}

						// Return the result as is
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(result)
						return
					}

					// Check for error
					if errorObj, ok := mcpResponse["error"].(map[string]interface{}); ok {
						errorMessage := "Unknown error"
						if msg, ok := errorObj["message"].(string); ok {
							errorMessage = msg
						}
						http.Error(w, errorMessage, http.StatusInternalServerError)
						return
					}

					// Set content type and return the response
					w.Header().Set("Content-Type", "application/json")
					w.Write(output)
				})
			}
		}
	}

	// Create the HTTP server with API key middleware if provided
	var handler http.Handler = mux
	if apiKey != "" {
		fmt.Printf("API key authentication is enabled\n")
		handler = auth.NewAPIKeyMiddleware(apiKey, mux)
	} else {
		fmt.Printf("API key authentication is disabled\n")
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	fmt.Printf("MCP Proxy Server listening on http://localhost:%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("proxy server error: %w", err)
	}

	return nil
}

func fetchToolsForServer(serverName string, server config.ServerConfig, projectName string) ([]openapi.Tool, error) {
	containerName := fmt.Sprintf("%s-%s", projectName, serverName)

	// Check if the container is running
	cmd := exec.Command("docker", "inspect", "--format", "{{.State.Running}}", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to check container status: %w", err)
	}

	if strings.TrimSpace(string(output)) != "true" {
		return nil, fmt.Errorf("container %s is not running", containerName)
	}

	// Initialize the server
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-01-01",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcp-proxy",
				"version": "1.0.0",
			},
		},
	}

	initRequestJSON, err := json.Marshal(initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initialize request: %w", err)
	}

	// Create command to initialize the server
	initCmd := exec.Command("docker", "exec", "-i", containerName, "npx", "-y", fmt.Sprintf("@modelcontextprotocol/server-%s", serverName))

	// Add additional arguments for specific servers
	if serverName == "filesystem" {
		initCmd.Args = append(initCmd.Args, "/tmp")
	}

	// Set up pipes for stdin and stdout
	initCmd.Stdin = strings.NewReader(string(initRequestJSON) + "\n")
	initOutput, err := initCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize server: %w", err)
	}

	// Parse the initialize response
	var initResponse map[string]interface{}
	if err := json.Unmarshal(initOutput, &initResponse); err != nil {
		return nil, fmt.Errorf("failed to parse initialize response: %w", err)
	}

	// Check for error in initialize response
	if errorObj, ok := initResponse["error"].(map[string]interface{}); ok {
		errorMessage := "Unknown error"
		if msg, ok := errorObj["message"].(string); ok {
			errorMessage = msg
		}
		return nil, fmt.Errorf("server initialization error: %s", errorMessage)
	}

	// Get the tools
	toolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	toolsRequestJSON, err := json.Marshal(toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tools request: %w", err)
	}

	// Create command to get the tools
	toolsCmd := exec.Command("docker", "exec", "-i", containerName, "npx", "-y", fmt.Sprintf("@modelcontextprotocol/server-%s", serverName))

	// Add additional arguments for specific servers
	if serverName == "filesystem" {
		toolsCmd.Args = append(toolsCmd.Args, "/tmp")
	}

	// Set up pipes for stdin and stdout
	toolsCmd.Stdin = strings.NewReader(string(toolsRequestJSON) + "\n")
	toolsOutput, err := toolsCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	// Parse the tools response
	var toolsResponse map[string]interface{}
	if err := json.Unmarshal(toolsOutput, &toolsResponse); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	// Check for error in tools response
	if errorObj, ok := toolsResponse["error"].(map[string]interface{}); ok {
		errorMessage := "Unknown error"
		if msg, ok := errorObj["message"].(string); ok {
			errorMessage = msg
		}
		return nil, fmt.Errorf("tools list error: %s", errorMessage)
	}

	// Extract the tools
	result, ok := toolsResponse["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools response format: missing result")
	}

	toolsList, ok := result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid tools response format: missing tools array")
	}

	// Convert to openapi.Tool
	tools := make([]openapi.Tool, 0, len(toolsList))
	for _, toolInterface := range toolsList {
		toolMap, ok := toolInterface.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := toolMap["name"].(string)
		if !ok {
			continue
		}

		description, _ := toolMap["description"].(string)

		inputSchema, ok := toolMap["inputSchema"].(map[string]interface{})
		if !ok {
			inputSchema = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		tool := openapi.Tool{
			Name:        name,
			Description: description,
			InputSchema: inputSchema,
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// startDetachedProxyServer starts the proxy server in the background
// startDetachedProxyServer starts the proxy server in the background
func startDetachedProxyServer(configFile string, port int, projectName string, apiKey string) error {
	// Add this line to declare a default output directory
	outputDir := "client-config"
	// Get the path to the current executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	// Create command to start the proxy
	// Create command to start the proxy
	cmd := exec.Command(exe, "proxy", "-f", configFile, "-p", fmt.Sprintf("%d", port))

	// Add API key flag if provided
	if apiKey != "" {
		cmd.Args = append(cmd.Args, "--api-key", apiKey)
	}
	// Run the command in the background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start detached proxy: %w", err)
	}
	// Don't wait for it to finish
	fmt.Printf("Proxy server started in background (PID: %d) on http://localhost:%d\n", cmd.Process.Pid, port)
	fmt.Println("To stop the proxy, use: kill", cmd.Process.Pid)
	// Write PID to file for easier management
	pidFile := filepath.Join(os.TempDir(), "mcp-compose-proxy.pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		fmt.Printf("Warning: Failed to write PID file: %v\n", err)
	} else {
		fmt.Printf("PID file written to: %s\n", pidFile)
	}
	// Generate client config
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config for client generation: %w", err)
	}
	return generateProxyClientConfig(cfg, projectName, port, "claude", outputDir)
}

// generateProxyClientConfig generates configuration files for MCP clients
func generateProxyClientConfig(cfg *config.ComposeConfig, projectName string, port int, clientType string, outputDir string) error {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Handle "all" case separately
	if strings.ToLower(clientType) == "all" {
		// Generate all client configurations
		if err := generateProxyClaudeConfig(cfg, port, outputDir); err != nil {
			return fmt.Errorf("failed to generate Claude config: %w", err)
		}

		if err := generateProxyOpenAIConfig(cfg, port, outputDir); err != nil {
			return fmt.Errorf("failed to generate OpenAI config: %w", err)
		}

		if err := generateProxyAnthropicConfig(cfg, port, outputDir); err != nil {
			return fmt.Errorf("failed to generate Anthropic config: %w", err)
		}

		fmt.Println("Successfully generated all client configurations")
		return nil
	}

	// Handle individual client types
	switch strings.ToLower(clientType) {
	case "claude":
		return generateProxyClaudeConfig(cfg, port, outputDir)
	case "openai":
		return generateProxyOpenAIConfig(cfg, port, outputDir)
	case "anthropic":
		return generateProxyAnthropicConfig(cfg, port, outputDir)
	default:
		return fmt.Errorf("unknown client type: %s", clientType)
	}
}

// generateProxyClaudeConfig generates configuration for Claude Desktop
func generateProxyClaudeConfig(cfg *config.ComposeConfig, port int, outputDir string) error {
	// Create array of servers
	serversList := make([]map[string]interface{}, 0, len(cfg.Servers))

	for name, server := range cfg.Servers {
		serverConfig := map[string]interface{}{
			"name":         name,
			"httpEndpoint": fmt.Sprintf("http://localhost:%d/%s", port, name),
			"capabilities": server.Capabilities,
			"description":  fmt.Sprintf("MCP %s server", name),
		}

		serversList = append(serversList, serverConfig)
	}

	// Wrap the servers array in a root object with "servers" property
	configObject := map[string]interface{}{
		"servers": serversList,
	}

	// Write configuration file
	configPath := filepath.Join(outputDir, "claude-desktop-servers.json")
	configData, err := json.MarshalIndent(configObject, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Claude Desktop config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write Claude Desktop config file: %w", err)
	}

	fmt.Printf("Claude Desktop configuration created at %s\n", configPath)
	fmt.Println("To use with Claude Desktop:")
	fmt.Println("1. Open Claude Desktop")
	fmt.Println("2. Go to Settings > MCP Servers")
	fmt.Println("3. Click 'Import Servers' and select the generated file")

	return nil
}

// generateProxyOpenAIConfig generates configuration for OpenAI tools
func generateProxyOpenAIConfig(cfg *config.ComposeConfig, port int, outputDir string) error {
	// Create template for JavaScript file
	proxyUrl := fmt.Sprintf("http://localhost:%d", port)
	serverNames := getServerNames(cfg)
	serverListJSON, _ := json.Marshal(serverNames)

	// Create a JavaScript file for OpenAI tools setup
	jsCode := fmt.Sprintf(`/**
 * OpenAI MCP Tools Configuration
 * Generated by MCP-Compose
 */
const { OpenAI } = require('openai');

// Initialize the OpenAI client
const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
});

// MCP Server Proxy Configuration
const MCP_PROXY_URL = '%s';
const MCP_SERVERS = %s;

// Example function to call OpenAI with MCP tools
async function callOpenAIWithMCP(prompt, serverName) {
  if (serverName && !MCP_SERVERS.includes(serverName)) {
    throw new Error('Unknown MCP server: ' + serverName);
  }

  const tools = serverName ? [{
    type: "function",
    function: {
      name: "mcp_" + serverName,
      description: "MCP " + serverName + " server",
      parameters: {
        type: "object",
        properties: {
          message: {
            type: "string",
            description: "The JSON-RPC message to send to the MCP server",
          },
        },
        required: ["message"],
      },
    }
  }] : [];

  const response = await openai.chat.completions.create({
    model: "gpt-4-0125-preview",
    messages: [
      { role: "system", content: "You have access to external tools via MCP." },
      { role: "user", content: prompt }
    ],
    tools: tools,
    tool_choice: serverName ? "auto" : "none",
  });

  // Extract the response
  return response.choices[0].message;
}

// Function to call the MCP proxy
async function callMCPProxy(server, message) {
  const response = await fetch(MCP_PROXY_URL + '/' + server, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: message,
  });

  if (!response.ok) {
    throw new Error("Error calling MCP server: " + response.statusText);
  }

  return await response.json();
}

// Example usage
async function main() {
  try {
    // Example without MCP
    const basicResponse = await callOpenAIWithMCP(
      "What is the capital of France?",
    );
    console.log("Basic response:", basicResponse.content);

    // Example with MCP server
    const mcpResponse = await callOpenAIWithMCP(
      "Check if there's a file called example.txt in the filesystem",
      "filesystem"
    );
    console.log("MCP response:", mcpResponse);
  } catch (error) {
    console.error("Error:", error);
  }
}

// Export API
module.exports = {
  callOpenAIWithMCP,
  callMCPProxy,
  MCP_SERVERS,
};

// Run if executed directly
if (require.main === module) {
  main();
}
`, proxyUrl, string(serverListJSON))

	// Write the file
	outputFile := filepath.Join(outputDir, "openai-mcp-tools.js")
	if err := os.WriteFile(outputFile, []byte(jsCode), 0644); err != nil {
		return fmt.Errorf("failed to write OpenAI config: %w", err)
	}

	fmt.Printf("OpenAI tools configuration created at %s\n", outputFile)
	fmt.Println("To use with OpenAI:")
	fmt.Println("1. Install Node.js dependencies: npm install openai")
	fmt.Println("2. Import the generated module in your code")
	fmt.Println("3. Use callOpenAIWithMCP function to make requests")

	return nil
}

// generateProxyAnthropicConfig generates configuration for Anthropic API
func generateProxyAnthropicConfig(cfg *config.ComposeConfig, port int, outputDir string) error {
	// Create template for Python file
	proxyUrl := fmt.Sprintf("http://localhost:%d", port)
	serverNames := getServerNames(cfg)
	serverListJSON, _ := json.Marshal(serverNames)

	// Create a Python file for Anthropic API setup
	pythonCode := fmt.Sprintf(`"""
Anthropic MCP Tools Configuration
Generated by MCP-Compose
"""
import os
import json
import requests
from anthropic import Anthropic

# Initialize the Anthropic client
client = Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY", ""))

# MCP Server Proxy Configuration
MCP_PROXY_URL = '%s'
MCP_SERVERS = %s

def call_anthropic_with_mcp(prompt, server_name=None):
    """Call Claude with optional MCP tools"""
    if server_name and server_name not in MCP_SERVERS:
        raise ValueError(f"Unknown MCP server: {server_name}")
    
    # Create the messages
    messages = [
        {"role": "user", "content": prompt}
    ]
    
    # Set up the request parameters
    params = {
        "model": "claude-3-opus-20240229",
        "max_tokens": 1000,
        "messages": messages,
    }
    
    # Add tool if server specified
    if server_name:
        params["tools"] = [{
            "name": f"mcp_{server_name}",
            "description": f"MCP {server_name} server",
            "input_schema": {
                "type": "object",
                "properties": {
                    "message": {
                        "type": "string", 
                        "description": "The JSON-RPC message to send to the MCP server"
                    }
                },
                "required": ["message"]
            }
        }]
    
    # Make the API call
    response = client.messages.create(**params)
    return response

def call_mcp_proxy(server, message):
    """Call the MCP proxy server with a message"""
    response = requests.post(
        f"{MCP_PROXY_URL}/{server}",
        headers={"Content-Type": "application/json"},
        data=message
    )
    
    if not response.ok:
        raise Exception(f"Error calling MCP server: {response.status_code}, {response.text}")
    
    return response.json()

# Example usage
def main():
    try:
        # Example without MCP
        basic_response = call_anthropic_with_mcp("What is the capital of France?")
        print("Basic response:", basic_response.content[0].text)
        
        # Example with MCP server
        mcp_response = call_anthropic_with_mcp(
            "Check if there's a file called example.txt in the filesystem", 
            server_name="filesystem"
        )
        print("MCP response:", mcp_response.content[0].text)
    except Exception as e:
        print("Error:", str(e))

if __name__ == "__main__":
    main()
`, proxyUrl, string(serverListJSON))

	// Write the file
	outputFile := filepath.Join(outputDir, "anthropic-mcp-tools.py")
	if err := os.WriteFile(outputFile, []byte(pythonCode), 0644); err != nil {
		return fmt.Errorf("failed to write Anthropic config: %w", err)
	}

	fmt.Printf("Anthropic API configuration created at %s\n", outputFile)
	fmt.Println("To use with Anthropic API:")
	fmt.Println("1. Install Python dependencies: pip install anthropic requests")
	fmt.Println("2. Set ANTHROPIC_API_KEY environment variable")
	fmt.Println("3. Import the generated module in your code")

	return nil
}

// getServerNames extracts the names of all servers from the config
func getServerNames(cfg *config.ComposeConfig) []string {
	names := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		names = append(names, name)
	}
	return names
}

func handleOpenAPISchema(w http.ResponseWriter, r *http.Request, serverName string, tools []openapi.Tool) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Generate OpenAPI schema
	schema, err := openapi.GenerateOpenAPISchema(serverName, tools)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate OpenAPI schema: %v", err), http.StatusInternalServerError)
		return
	}

	// Set content type before writing response
	w.Header().Set("Content-Type", "application/json")

	// Use proper HTTP response
	w.WriteHeader(http.StatusOK)

	// Write the response
	if err := json.NewEncoder(w).Encode(schema); err != nil {
		log.Printf("Error encoding OpenAPI schema: %v", err)
	}
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	html := `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>MCP OpenAPI Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@4.5.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "./openapi.json",
        dom_id: '#swagger-ui',
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout",
      });
    };
  </script>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
