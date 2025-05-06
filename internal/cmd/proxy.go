// internal/cmd/proxy.go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mcpcompose/internal/config"
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
				return startContainerizedProxy(cfg, projectName, port, outputDir)
			}

			// Start the proxy server
			if detach {
				return startDetachedProxyServer(file, port, projectName)
			}

			// Run the proxy server in the foreground
			return startProxyServer(cfg, projectName, port)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 9876, "Port to run the proxy server on")
	cmd.Flags().BoolVarP(&generateConfig, "generate-config", "g", false, "Generate client configuration file only")
	cmd.Flags().StringVarP(&clientType, "client", "t", "claude", "Client type (claude, openai, anthropic, all)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "client-config", "Output directory for client configuration")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run proxy server in the background")
	cmd.Flags().BoolVarP(&containerized, "container", "C", true, "Run proxy server as a container")

	return cmd
}

// startContainerizedProxy starts the proxy server as a Docker container
func startContainerizedProxy(cfg *config.ComposeConfig, projectName string, port int, outputDir string) error {
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
	proxyCmd := exec.Command(
		"docker", "run", "-d",
		"--name", "mcp-compose-proxy",
		"-p", fmt.Sprintf("%d:%d", port, port),
		"--network", "mcp-net",
		// Add this line to mount the Docker socket:
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"mcp-compose-proxy:latest",
	)

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

PORT = 9876

# Define your servers and containers - these will be resolved via Docker networking
MCP_SERVERS = {
    "filesystem": "mcp-compose-filesystem",
    "memory": "mcp-compose-memory", 
    "weather": "mcp-compose-weather"
}

class MCPProxyHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        # Handle GET request - just show available servers
        if self.path == "/" or self.path == "":
            self.send_response(200)
            self.send_header("Content-type", "text/html")
            self.end_headers()
            
            response = "<html><body><h1>MCP Proxy</h1><p>Available endpoints:</p><ul>"
            for server in MCP_SERVERS:
                response += f"<li>/{server} - {server} MCP server</li>"
            response += "</ul></body></html>"
            
            self.wfile.write(response.encode("utf-8"))
        else:
            self.send_error(404, "Not found")
    
    def log_message(self, format, *args):
        # Log to stdout with timestamp
        sys.stdout.write("[%s] %s\n" % (self.log_date_time_string(), format % args))
        sys.stdout.flush()
    
    def do_POST(self):
        # Parse the path to get server name
        path_parts = urlparse(self.path).path.strip("/").split("/")
        if not path_parts:
            self.send_error(404, "No server specified")
            return
            
        server_name = path_parts[0]
        
        if server_name not in MCP_SERVERS:
            self.send_error(404, f"Unknown server: {server_name}")
            return
            
        # Get the container name
        container_name = MCP_SERVERS[server_name]
        
        # Read the request body
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length).decode("utf-8")
        
        self.log_message("Request to %s: %s", server_name, body[:100] + "..." if len(body) > 100 else body)
        
        try:
            # Forward to the container using docker exec
            cmd = ["docker", "exec", "-i", container_name, "cat"]
            
            # Now send the actual command
            process = subprocess.Popen(
                cmd,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=False  # Use binary mode for stdin/stdout
            )
            
            # Add a newline to the body if not present
            if not body.endswith('\n'):
                body += '\n'
                
            # Send the request to the container
            stdout, stderr = process.communicate(input=body.encode("utf-8"), timeout=10)
            
            stderr_text = stderr.decode("utf-8", errors="ignore")
            if stderr_text:
                self.log_message("STDERR: %s", stderr_text)
            
            if process.returncode != 0:
                self.send_error(500, f"Error communicating with container (code {process.returncode}): {stderr_text}")
                return
                
            # Get the response
            response_data = stdout.decode("utf-8", errors="ignore")
            
            # Log a snippet of the response
            self.log_message("Response from %s: %s", server_name, 
                             response_data[:100] + "..." if len(response_data) > 100 else response_data)
            
            # Send the response
            self.send_response(200)
            self.send_header("Content-type", "application/json")
            self.end_headers()
            self.wfile.write(stdout)
            
        except subprocess.TimeoutExpired:
            self.send_error(504, "Timeout communicating with container")
        except Exception as e:
            self.log_message("Error: %s", str(e))
            self.send_error(500, f"Error: {str(e)}")

def main():
    print(f"Starting MCP proxy server at http://0.0.0.0:{PORT}")
    print(f"Available endpoints: {', '.join('/' + s for s in MCP_SERVERS)}")
    
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

	return ioutil.WriteFile(scriptPath, []byte(scriptContent), 0755)
}

// createProxyDockerfile creates the Dockerfile for the proxy container
func createProxyDockerfile(dockerfilePath string) error {
	dockerfileContent := `FROM python:3.10-slim

# Install Docker CLI
RUN apt-get update && \
    apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release && \
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg && \
    echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce-cli && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY proxy.py .

EXPOSE 9876

CMD ["python", "proxy.py"]
`

	return os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644)
}

// startProxyServer starts the MCP proxy server in the foreground
func startProxyServer(cfg *config.ComposeConfig, projectName string, port int) error {
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

		fmt.Fprintf(w, "MCP Proxy Server\n")
		fmt.Fprintf(w, "Available endpoints:\n")
		for name := range cfg.Servers {
			fmt.Fprintf(w, "- /%s - %s MCP Server\n", name, name)
		}
	})

	// Add handler for each server
	for name := range cfg.Servers {
		serverName := name
		containerName := fmt.Sprintf("%s-%s", projectName, serverName)

		mux.HandleFunc("/"+serverName, func(w http.ResponseWriter, r *http.Request) {
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
			cmd := exec.Command("docker", "exec", "-i", containerName, "cat")
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
	}

	// Start the HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	fmt.Printf("MCP Proxy Server listening on http://localhost:%d\n", port)
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("proxy server error: %w", err)
	}

	return nil
}

// startDetachedProxyServer starts the proxy server in the background
// startDetachedProxyServer starts the proxy server in the background
func startDetachedProxyServer(configFile string, port int, projectName string) error {
	// Add this line to declare a default output directory
	outputDir := "client-config"

	// Get the path to the current executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create command to start the proxy
	cmd := exec.Command(exe, "proxy", "-f", configFile, "-p", fmt.Sprintf("%d", port))

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
