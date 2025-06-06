// internal/runtime/process.go
package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ProcessOptions contains options for process start
type ProcessOptions struct {
	Env     map[string]string
	WorkDir string
	Name    string
}

// Process represents a running server process
type Process struct {
	cmd     *exec.Cmd
	pidFile string
	logFile string
	name    string
}

// NewProcess creates a new process
func NewProcess(command string, args []string, opts ProcessOptions) (*Process, error) {
	// Create run directory if not exists
	runDir := filepath.Join(os.TempDir(), "mcp-compose", "run")
	logDir := filepath.Join(os.TempDir(), "mcp-compose", "logs")

	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create run directory: %w", err)
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Set up PID file and log file
	pidFile := filepath.Join(runDir, fmt.Sprintf("%s.pid", opts.Name))
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", opts.Name))

	// Create command
	cmd := exec.Command(command, args...)

	// Setup environment
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Set working directory
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	// Create log file for output
	stdout, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	cmd.Stdout = stdout
	cmd.Stderr = stdout

	// Set process group to detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return &Process{
		cmd:     cmd,
		pidFile: pidFile,
		logFile: logFile,
		name:    opts.Name,
	}, nil
}

// Start starts the process
func (p *Process) Start() error {
	// Start the process
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Write PID to file
	if err := os.WriteFile(p.pidFile, []byte(strconv.Itoa(p.cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Close the file handles in the parent process since child has its own copy
	if closer, ok := p.cmd.Stdout.(interface{ Close() error }); ok {
		closer.Close()
	}

	// Detach process from parent
	p.cmd.Process.Release()

	return nil
}

// Stop stops the process
func (p *Process) Stop() error {
	// Read PID from file
	pidBytes, err := os.ReadFile(p.pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	// Try to find process
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process doesn't exist, clean up PID file
		os.Remove(p.pidFile)
		return nil
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// If process doesn't exist, clean up PID file
		if err.Error() == "os: process already finished" {
			os.Remove(p.pidFile)
			return nil
		}

		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Clean up PID file
	os.Remove(p.pidFile)

	return nil
}

// IsRunning checks if the process is running
func (p *Process) IsRunning() (bool, error) {
	// Read PID from file
	pidBytes, err := os.ReadFile(p.pidFile)
	if err != nil {
		return false, fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return false, fmt.Errorf("invalid PID: %w", err)
	}

	// Try to find process
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))

	return err == nil, nil
}

// FindProcess finds a process by name
func FindProcess(name string) (*Process, error) {
	runDir := filepath.Join(os.TempDir(), "mcp-compose", "run")
	logDir := filepath.Join(os.TempDir(), "mcp-compose", "logs")

	pidFile := filepath.Join(runDir, fmt.Sprintf("%s.pid", name))
	logFile := filepath.Join(logDir, fmt.Sprintf("%s.log", name))

	// Check if PID file exists
	if _, err := os.Stat(pidFile); err != nil {
		return nil, fmt.Errorf("process %s not found", name)
	}

	return &Process{
		pidFile: pidFile,
		logFile: logFile,
		name:    name,
	}, nil
}

// ShowLogs shows logs for a process
func (p *Process) ShowLogs(follow bool) error {
	if _, err := os.Stat(p.logFile); err != nil {
		return fmt.Errorf("log file not found: %w", err)
	}

	if follow {
		// Use tail -f to show logs
		cmd := exec.Command("tail", "-f", p.logFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		// Use cat to show logs
		cmd := exec.Command("cat", p.logFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
}
