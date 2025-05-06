package inspector

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"mcpcompose/internal/config"
)

//go:embed assets/*
var assetFiles embed.FS

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	processKey contextKey = "process"
	stdinKey   contextKey = "stdin"
	stdoutKey  contextKey = "stdout"
)

// ActiveSession represents an active MCP server session
type ActiveSession struct {
	Process *exec.Cmd
	Stdin   io.Writer
	Stdout  io.Reader
}

// Global session store - in a production app, you'd use a proper session management system
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

	// Create a temporary directory for inspector files
	inspectorDir := filepath.Join(os.TempDir(), "mcp-inspector")
	if err := os.MkdirAll(inspectorDir, 0755); err != nil {
		return fmt.Errorf("failed to create inspector directory: %w", err)
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

		server, exists := cfg.Servers[request.Server]
		if !exists {
			http.Error(w, "Server not found", http.StatusNotFound)
			return
		}

		// Connect to the server
		proc, stdin, stdout, err := connectToServer(server)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "Failed to connect: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Generate a session ID
		sessionID := fmt.Sprintf("%s-%d", request.Server, time.Now().UnixNano())

		// Store the session
		activeSessionsLock.Lock()
		activeSessions[sessionID] = &ActiveSession{
			Process: proc,
			Stdin:   stdin,
			Stdout:  stdout,
		}
		activeSessionsLock.Unlock()

		// Initialize the connection
		result, err := initializeConnection(stdin, stdout, server.Capabilities)
		if err != nil {
			activeSessionsLock.Lock()
			delete(activeSessions, sessionID)
			activeSessionsLock.Unlock()
			http.Error(w, fmt.Sprintf(`{"error": "Failed to initialize: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Add session ID to the result
		resultMap := make(map[string]interface{})
		if err := json.Unmarshal(result, &resultMap); err != nil {
			resultMap = map[string]interface{}{
				"result": result,
			}
		}
		resultMap["sessionId"] = sessionID

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

	// Start the server in a goroutine
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	go func() {
		fmt.Printf("MCP Inspector started on http://localhost:%d\n", port)
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

	tmpl, err := template.New("inspector").Parse(string(tmplContent))
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Config":     cfg,
		"ServerName": serverName,
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

// connectToServer connects to an MCP server process
func connectToServer(server config.ServerConfig) (*exec.Cmd, io.Writer, io.Reader, error) {
	if server.Command == "" {
		return nil, nil, nil, fmt.Errorf("server has no command specified")
	}

	// Create command
	cmd := exec.Command(server.Command, server.Args...)

	// Create pipes for stdin/stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to start process: %v", err)
	}

	return cmd, stdin, stdout, nil
}

// initializeConnection sends an initialize request to the server
func initializeConnection(stdin io.Writer, stdout io.Reader, capabilities []string) (json.RawMessage, error) {
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
			"protocolVersion": "2024-11-05",
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

	// Read the response
	reader := bufio.NewReader(stdout)
	responseLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read initialize response: %v", err)
	}

	// Return the raw JSON response
	return json.RawMessage(responseLine), nil
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
