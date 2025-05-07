// internal/inspector/inspector.go
package inspector

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
)

//go:embed assets/*
var assetFiles embed.FS

// ActiveSession represents an active MCP server session
type ActiveSession struct {
	Process     *exec.Cmd
	Stdin       io.Writer
	Stdout      io.Reader
	ContainerID string
}

// Global session store
var (
	activeSessions     = make(map[string]*ActiveSession)
	activeSessionsLock sync.Mutex
)

// LaunchInspector starts the MCP Inspector for the specified server
func LaunchInspector(configFile, serverName string, port int) error {
	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Detect container runtime
	runtime, err := container.DetectRuntime()
	if err != nil {
		fmt.Printf("Warning: Failed to detect container runtime: %v\n", err)
	} else {
		fmt.Printf("Using container runtime: %s\n", runtime.GetRuntimeName())
	}

	// Create the HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			serveInspectorUI(w, cfg, serverName)
		} else {
			http.NotFound(w, r)
		}
	})

	http.HandleFunc("/api/servers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		servers := make(map[string]interface{})

		if serverName != "" {
			// Return only the specified server
			if server, exists := cfg.Servers[serverName]; exists {
				servers[serverName] = map[string]interface{}{
					"name":         serverName,
					"capabilities": server.Capabilities,
					"command":      server.Command,
					"args":         server.Args,
					"image":        server.Image,
				}
			}
		} else {
			// Return all servers
			for name, server := range cfg.Servers {
				servers[name] = map[string]interface{}{
					"name":         name,
					"capabilities": server.Capabilities,
					"command":      server.Command,
					"args":         server.Args,
					"image":        server.Image,
				}
			}
		}

		json.NewEncoder(w).Encode(servers)
	})

	http.HandleFunc("/api/connect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			Server string `json:"server"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		fmt.Printf("Received connection request for server: %s\n", request.Server)
		server, exists := cfg.Servers[request.Server]
		if !exists {
			http.Error(w, "Server not found", http.StatusNotFound)
			return
		}

		// Connect to the server
		cmd, stdin, stdout, containerId, err := connectToServer(server, runtime, request.Server)
		if err != nil {
			errMsg := fmt.Sprintf(`{"error": "Failed to connect: %v"}`, err)
			fmt.Println("Connection error:", err)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		// Generate a session ID
		sessionID := fmt.Sprintf("%s-%d", request.Server, time.Now().UnixNano())

		// Store the session
		activeSessionsLock.Lock()
		activeSessions[sessionID] = &ActiveSession{
			Process:     cmd,
			Stdin:       stdin,
			Stdout:      stdout,
			ContainerID: containerId,
		}
		activeSessionsLock.Unlock()

		fmt.Println("Session created, sending initialize request...")

		// Initialize the connection
		result, err := initializeConnection(stdin, stdout, server.Capabilities)
		if err != nil {
			fmt.Println("Initialization error:", err)
			activeSessionsLock.Lock()
			delete(activeSessions, sessionID)
			activeSessionsLock.Unlock()

			// Clean up container if needed
			if containerId != "" && runtime != nil {
				runtime.StopContainer(containerId)
			}

			http.Error(w, fmt.Sprintf(`{"error": "Failed to initialize: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Add session ID to the result
		resultMap := make(map[string]interface{})
		if err := json.Unmarshal(result, &resultMap); err != nil {
			resultMap = map[string]interface{}{
				"result":    string(result),
				"sessionId": sessionID,
			}
		} else {
			resultMap["sessionId"] = sessionID
		}

		fmt.Printf("Connection successful, sessionId: %s\n", sessionID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resultMap)
	})

	http.HandleFunc("/api/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			SessionID string          `json:"sessionId"`
			JSONRPC   string          `json:"jsonrpc"`
			ID        int             `json:"id"`
			Method    string          `json:"method"`
			Params    json.RawMessage `json:"params,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
			return
		}

		// Get the session
		activeSessionsLock.Lock()
		session, exists := activeSessions[request.SessionID]
		activeSessionsLock.Unlock()
		if !exists {
			http.Error(w, `{"error": "Session not found"}`, http.StatusNotFound)
			return
		}

		// Create the actual JSON-RPC request
		jsonRPCRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"method":  request.Method,
		}
		if request.Params != nil {
			var params interface{}
			if err := json.Unmarshal(request.Params, &params); err == nil {
				jsonRPCRequest["params"] = params
			}
		}

		// Send the request
		requestBytes, err := json.Marshal(jsonRPCRequest)
		if err != nil {
			http.Error(w, `{"error": "Failed to marshal request"}`, http.StatusInternalServerError)
			return
		}

		// Always add a newline to the request
		requestBytes = append(requestBytes, '\n')
		if _, err := session.Stdin.Write(requestBytes); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to send request: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Read the response
		reader := bufio.NewReader(session.Stdout)
		responseLine, err := reader.ReadString('\n')
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to read response: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Parse the response
		var response interface{}
		if err := json.Unmarshal([]byte(responseLine), &response); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Invalid response: %v, received: %s"}`, err, responseLine), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/debug/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Collect active sessions info
		sessions := make(map[string]map[string]string)
		activeSessionsLock.Lock()
		for id, session := range activeSessions {
			sessions[id] = map[string]string{
				"containerID": session.ContainerID,
			}
		}
		activeSessionsLock.Unlock()

		debug := map[string]interface{}{
			"activeSessions": sessions,
			"runtime":        runtime.GetRuntimeName(),
		}
		json.NewEncoder(w).Encode(debug)
	})

	// Start the server in a goroutine
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	go func() {
		fmt.Printf("MCP Inspector started on http://localhost:%d\n", port)
		fmt.Printf("Debug info available at http://localhost:%d/debug/info\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	// Open browser
	time.Sleep(500 * time.Millisecond)
	openBrowser(fmt.Sprintf("http://localhost:%d", port))

	// Block until interrupted
	fmt.Println("MCP Inspector is running. Press Ctrl+C to exit.")
	waitForInterrupt()

	// Clean up sessions when shutting down
	activeSessionsLock.Lock()
	for _, session := range activeSessions {
		if session.Process != nil && session.Process.Process != nil {
			session.Process.Process.Kill()
		}
		if session.ContainerID != "" && runtime != nil {
			runtime.StopContainer(session.ContainerID)
		}
	}
	activeSessionsLock.Unlock()

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

// serveInspectorUI serves the inspector UI
func serveInspectorUI(w http.ResponseWriter, cfg *config.ComposeConfig, serverName string) {
	// Load template from the embedded file
	tmplContent, err := assetFiles.ReadFile("assets/inspector-ui.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		return
	}

	// Serve the template directly without parsing
	w.Header().Set("Content-Type", "text/html")
	w.Write(tmplContent)
}

// connectToServer connects to an MCP server using the appropriate method
func connectToServer(server config.ServerConfig, runtime container.Runtime, serverName string) (*exec.Cmd, io.Writer, io.Reader, string, error) {
	// For container-based servers, create a dedicated inspection container
	if server.Image != "" {
		fmt.Println("Creating inspection container for server")

		// Create a unique container name
		containerName := fmt.Sprintf("mcp-inspector-%s-%d", serverName, time.Now().UnixNano())

		// Prepare container options
		opts := &container.ContainerOptions{
			Name:        containerName,
			Image:       server.Image,
			Command:     server.Command,
			Args:        server.Args,
			Env:         server.Env,
			Volumes:     server.Volumes,
			NetworkMode: server.NetworkMode,
			Networks:    server.Networks,
		}

		fmt.Printf("Starting container with options: %+v\n", opts)

		// Start the container
		containerID, err := runtime.StartContainer(opts)
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("failed to start container: %w", err)
		}

		fmt.Printf("Container started with ID: %s\n", containerID)

		// Create communication with the container
		var execCmd []string
		if runtime.GetRuntimeName() == "docker" {
			execCmd = []string{"docker", "exec", "-i", containerName, "cat"}
		} else if runtime.GetRuntimeName() == "podman" {
			execCmd = []string{"podman", "exec", "-i", containerName, "cat"}
		} else {
			runtime.StopContainer(containerName)
			return nil, nil, nil, "", fmt.Errorf("unsupported container runtime: %s", runtime.GetRuntimeName())
		}

		cmd := exec.Command(execCmd[0], execCmd[1:]...)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			runtime.StopContainer(containerName)
			return nil, nil, nil, "", fmt.Errorf("failed to create stdin pipe: %v", err)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			stdin.Close()
			runtime.StopContainer(containerName)
			return nil, nil, nil, "", fmt.Errorf("failed to create stdout pipe: %v", err)
		}

		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			stdin.Close()
			stdout.Close()
			runtime.StopContainer(containerName)
			return nil, nil, nil, "", fmt.Errorf("failed to start exec command: %v", err)
		}

		// Cleanup when done
		go func() {
			cmd.Wait()
			runtime.StopContainer(containerName)
		}()

		return cmd, stdin, stdout, containerID, nil
	} else if server.Command != "" {
		// For process-based servers, just execute the command
		fmt.Printf("Starting command-based server: %s %v\n", server.Command, server.Args)
		cmd := exec.Command(server.Command, server.Args...)

		// Set environment variables if specified
		if len(server.Env) > 0 {
			env := os.Environ()
			for k, v := range server.Env {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
			cmd.Env = env
		}

		// Set working directory if specified
		if server.WorkDir != "" {
			cmd.Dir = server.WorkDir
		}

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("failed to create stdin pipe: %v", err)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, nil, nil, "", fmt.Errorf("failed to create stdout pipe: %v", err)
		}

		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return nil, nil, nil, "", fmt.Errorf("failed to start process: %v", err)
		}

		return cmd, stdin, stdout, "", nil
	}

	return nil, nil, nil, "", fmt.Errorf("server has neither command nor image specified")
}

// startContainerServer starts a container for an MCP server and returns pipes for communication
func startContainerServer(server config.ServerConfig, runtime container.Runtime, serverName string) (*exec.Cmd, io.Writer, io.Reader, string, error) {
	if runtime == nil {
		return nil, nil, nil, "", fmt.Errorf("no container runtime available")
	}

	// Create a unique container name
	containerName := fmt.Sprintf("mcp-inspector-%s-%d", serverName, time.Now().UnixNano())

	// Determine the actual command to run (not the bash wrapper)
	command := server.Command
	args := server.Args

	// Check for bash wrapper pattern and replace with direct commands
	if command == "bash" && len(args) >= 2 && args[0] == "-c" {
		bashCmd := strings.Join(args[1:], " ")

		// Replace common patterns
		if strings.Contains(bashCmd, "@modelcontextprotocol/server-filesystem") {
			command = "npx"
			args = []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"}
		} else if strings.Contains(bashCmd, "@modelcontextprotocol/server-memory") {
			command = "npx"
			args = []string{"-y", "@modelcontextprotocol/server-memory"}
		} else if strings.Contains(bashCmd, "server.js") {
			// For weather server, use a simplified direct approach
			command = "node"

			// Extract the JavaScript code
			jsCode := extractJavaScriptFromBashCommand(bashCmd)
			if jsCode != "" {
				args = []string{"-e", jsCode}
			} else {
				// Fallback to a simple weather server implementation
				args = []string{"-e", createSimpleWeatherServer()}
			}
		}
	}

	// Prepare container options
	opts := &container.ContainerOptions{
		Name:        containerName,
		Image:       server.Image,
		Command:     command,
		Args:        args,
		Env:         server.Env,
		Ports:       server.Ports,
		Volumes:     server.Volumes,
		Pull:        server.Pull,
		NetworkMode: server.NetworkMode,
		Networks:    server.Networks,
		WorkDir:     server.WorkDir,
	}

	// For resource paths, create volume mappings
	for _, resourcePath := range server.Resources.Paths {
		volumeMapping := fmt.Sprintf("%s:%s", resourcePath.Source, resourcePath.Target)
		if resourcePath.ReadOnly {
			volumeMapping += ":ro"
		}
		opts.Volumes = append(opts.Volumes, volumeMapping)
	}

	fmt.Printf("Starting container with options: %+v\n", opts)

	// Start a detached container
	containerID, err := runtime.StartContainer(opts)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Printf("Container started with ID: %s\n", containerID)

	// Create pipes to communicate with the container
	// We'll use 'docker exec -i' to create an interactive session
	execCmd := []string{}
	if runtime.GetRuntimeName() == "docker" {
		execCmd = []string{"docker", "exec", "-i", containerName, "cat"}
	} else if runtime.GetRuntimeName() == "podman" {
		execCmd = []string{"podman", "exec", "-i", containerName, "cat"}
	} else {
		return nil, nil, nil, "", fmt.Errorf("unsupported container runtime: %s", runtime.GetRuntimeName())
	}

	cmd := exec.Command(execCmd[0], execCmd[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		// Clean up container
		runtime.StopContainer(containerName)
		return nil, nil, nil, "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		// Clean up
		stdin.Close()
		runtime.StopContainer(containerName)
		return nil, nil, nil, "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	cmd.Stderr = os.Stderr

	// Start the exec command
	if err := cmd.Start(); err != nil {
		runtime.StopContainer(containerName)
		return nil, nil, nil, "", fmt.Errorf("failed to start exec command: %v", err)
	}

	return cmd, stdin, stdout, containerID, nil
}

// extractJavaScriptFromBashCommand extracts JavaScript code from a bash command
func extractJavaScriptFromBashCommand(bashCmd string) string {
	// Look for JavaScript between single quotes
	singleQuoteStart := strings.Index(bashCmd, "'const readline")
	if singleQuoteStart >= 0 {
		singleQuoteEnd := strings.LastIndex(bashCmd, "'")
		if singleQuoteEnd > singleQuoteStart {
			return bashCmd[singleQuoteStart+1 : singleQuoteEnd]
		}
	}

	// Look for JavaScript between double quotes
	doubleQuoteStart := strings.Index(bashCmd, "\"const readline")
	if doubleQuoteStart >= 0 {
		doubleQuoteEnd := strings.LastIndex(bashCmd, "\"")
		if doubleQuoteEnd > doubleQuoteStart {
			return bashCmd[doubleQuoteStart+1 : doubleQuoteEnd]
		}
	}

	return ""
}

// createSimpleWeatherServer returns JavaScript code for a simple weather MCP server
func createSimpleWeatherServer() string {
	return `
    const readline = require('readline');
    const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
        terminal: false
    });
    
    rl.on('line', (line) => {
        try {
            const req = JSON.parse(line);
            if (req.method === 'initialize') {
                console.log(JSON.stringify({
                    jsonrpc: '2.0',
                    id: req.id,
                    result: {
                        protocolVersion: '2024-01-01',
                        serverInfo: { 
                            name: 'weather',
                            version: '1.0.0' 
                        },
                        capabilities: { 
                            tools: {} 
                        }
                    }
                }));
            } else if (req.method === 'tools/get') {
                const params = req.params || {};
                const location = params.location || 'Unknown';
                console.log(JSON.stringify({
                    jsonrpc: '2.0',
                    id: req.id,
                    result: {
                        temperature: Math.floor(Math.random()*30)+10,
                        conditions: ['Sunny','Cloudy','Rainy','Snowy'][Math.floor(Math.random()*4)],
                        location
                    }
                }));
            } else {
                console.log(JSON.stringify({
                    jsonrpc: '2.0',
                    id: req.id,
                    error: { 
                        code: -32601, 
                        message: 'Method not found' 
                    }
                }));
            }
        } catch(e) {
            console.log(JSON.stringify({
                jsonrpc: '2.0',
                id: null,
                error: { 
                    code: -32700, 
                    message: 'Parse error' 
                }
            }));
        }
    });
    
    console.error('Simple MCP Weather Server running');
    `
}

// initializeConnection sends an initialize request to the server
func initializeConnection(stdin io.Writer, stdout io.Reader, capabilities []string) (json.RawMessage, error) {
	fmt.Println("Sending initialize request with capabilities:", capabilities)

	// Create capabilities structure
	capabilityOpts := map[string]interface{}{}
	for _, cap := range capabilities {
		switch cap {
		case "resources":
			capabilityOpts["resources"] = map[string]bool{
				"listChanged": true,
				"subscribe":   true,
			}
		case "tools":
			capabilityOpts["tools"] = map[string]bool{
				"listChanged": true,
			}
		case "prompts":
			capabilityOpts["prompts"] = map[string]bool{
				"listChanged": true,
			}
		case "sampling":
			capabilityOpts["sampling"] = struct{}{}
		case "logging":
			capabilityOpts["logging"] = struct{}{}
		}
	}

	// Create initialize request
	initializeRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-01-01",
			"capabilities":    capabilityOpts,
			"clientInfo": map[string]string{
				"name":    "MCP Inspector",
				"version": "1.0.0",
			},
		},
	}

	// Send the request
	requestBytes, err := json.Marshal(initializeRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initialize request: %v", err)
	}

	// Write the request with a newline
	requestBytes = append(requestBytes, '\n')
	if _, err := stdin.Write(requestBytes); err != nil {
		return nil, fmt.Errorf("failed to send initialize request: %v", err)
	}

	fmt.Println("Initialize request sent, waiting for response...")

	// Read the response with timeout
	responseCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(stdout)
		responseLine, err := reader.ReadString('\n')
		if err != nil {
			errCh <- fmt.Errorf("failed to read initialize response: %v", err)
			return
		}
		responseCh <- responseLine
	}()

	// Wait for response with timeout
	select {
	case responseLine := <-responseCh:
		fmt.Println("Received initialize response:", responseLine)
		return json.RawMessage(responseLine), nil
	case err := <-errCh:
		return nil, err
	case <-time.After(30 * time.Second): // 30 second timeout
		return nil, fmt.Errorf("timeout waiting for initialize response")
	}
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Don't wait for the browser to close
	return nil
}

// waitForInterrupt blocks until Ctrl+C is pressed
func waitForInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
