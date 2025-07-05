package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProcessOptions(t *testing.T) {
	opts := ProcessOptions{
		Env: map[string]string{
			"TEST_VAR": "test_value",
			"PATH":     "/usr/bin:/bin",
		},
		WorkDir: "/tmp",
		Name:    "test-process",
	}

	if opts.Env["TEST_VAR"] != "test_value" {
		t.Errorf("Expected TEST_VAR to be 'test_value', got %s", opts.Env["TEST_VAR"])
	}

	if opts.WorkDir != "/tmp" {
		t.Errorf("Expected WorkDir to be '/tmp', got %s", opts.WorkDir)
	}

	if opts.Name != "test-process" {
		t.Errorf("Expected Name to be 'test-process', got %s", opts.Name)
	}
}

func TestNewProcess(t *testing.T) {
	tempDir := t.TempDir()
	
	opts := ProcessOptions{
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		WorkDir: tempDir,
		Name:    "test-process",
	}

	// Test with a simple command that exists on most systems
	process, err := NewProcess("echo", []string{"hello"}, opts)
	if err != nil {
		t.Fatalf("Expected no error creating process, got: %v", err)
	}

	if process == nil {
		t.Fatal("Expected process to be created")
	}

	if process.name != opts.Name {
		t.Errorf("Expected process name to be %s, got %s", opts.Name, process.name)
	}

	if process.cmd == nil {
		t.Error("Expected command to be set")
	}

	if process.pidFile == "" {
		t.Error("Expected PID file path to be set")
	}

	if process.logFile == "" {
		t.Error("Expected log file path to be set")
	}

	// Test command setup
	if process.cmd.Path == "" {
		t.Error("Expected command path to be set")
	}

	if len(process.cmd.Args) < 2 || process.cmd.Args[1] != "hello" {
		t.Errorf("Expected command args to include 'hello', got: %v", process.cmd.Args)
	}

	// Test environment setup
	envFound := false
	for _, env := range process.cmd.Env {
		if env == "TEST_VAR=test_value" {
			envFound = true
			break
		}
	}
	if !envFound {
		t.Error("Expected TEST_VAR environment variable to be set")
	}

	// Test working directory
	if process.cmd.Dir != opts.WorkDir {
		t.Errorf("Expected working directory to be %s, got %s", opts.WorkDir, process.cmd.Dir)
	}
}

func TestNewProcessInvalidCommand(t *testing.T) {
	opts := ProcessOptions{
		Name: "invalid-process",
	}

	// Test with non-existent command
	_, err := NewProcess("nonexistent_command_12345", []string{}, opts)
	// This should not error during NewProcess, only during Start
	if err != nil {
		t.Errorf("NewProcess should not fail for non-existent command, got: %v", err)
	}
}

func TestProcessStartAndStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	tempDir := t.TempDir()
	
	opts := ProcessOptions{
		WorkDir: tempDir,
		Name:    "test-sleep-process",
		Env: map[string]string{
			"TEST_ENV": "test_value",
		},
	}

	// Use sleep command for testing
	process, err := NewProcess("sleep", []string{"2"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	// Test starting the process
	err = process.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Verify PID file was created
	if _, err := os.Stat(process.pidFile); err != nil {
		t.Errorf("Expected PID file to be created at %s", process.pidFile)
	}

	// Read and verify PID
	pidBytes, err := os.ReadFile(process.pidFile)
	if err != nil {
		t.Errorf("Failed to read PID file: %v", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Errorf("Invalid PID in file: %v", err)
	}

	if pid <= 0 {
		t.Errorf("Expected positive PID, got %d", pid)
	}

	// Test that process is running
	running, err := process.IsRunning()
	if err != nil {
		t.Errorf("Failed to check if process is running: %v", err)
	}
	if !running {
		t.Error("Expected process to be running")
	}

	// Test stopping the process
	err = process.Stop()
	if err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}

	// Verify PID file was cleaned up
	if _, err := os.Stat(process.pidFile); err == nil {
		t.Error("Expected PID file to be removed after stopping")
	}

	// Wait a bit for process to fully stop
	time.Sleep(100 * time.Millisecond)

	// Test that process is no longer running
	running, err = process.IsRunning()
	if err == nil && running {
		t.Error("Expected process to not be running after stop")
	}
}

func TestProcessIsRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	tempDir := t.TempDir()
	
	opts := ProcessOptions{
		WorkDir: tempDir,
		Name:    "test-running-process",
	}

	process, err := NewProcess("sleep", []string{"1"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	// Test IsRunning before starting (should return error)
	running, err := process.IsRunning()
	if err == nil {
		t.Error("Expected error when checking non-existent process")
	}
	if running {
		t.Error("Expected process to not be running before start")
	}

	// Start the process
	err = process.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Test IsRunning after starting
	running, err = process.IsRunning()
	if err != nil {
		t.Errorf("Failed to check if process is running: %v", err)
	}
	if !running {
		t.Error("Expected process to be running after start")
	}

	// Clean up
	process.Stop()
}

func TestProcessShowLogs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	tempDir := t.TempDir()
	
	opts := ProcessOptions{
		WorkDir: tempDir,
		Name:    "test-log-process",
	}

	// Use echo to generate some output
	process, err := NewProcess("echo", []string{"test log message"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	// Start and let it complete
	err = process.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Wait for process to complete and write to log
	time.Sleep(500 * time.Millisecond)

	// Test showing logs (non-follow mode)
	// Note: This will output to stdout in test, which is expected
	err = process.ShowLogs(false)
	if err != nil {
		t.Errorf("Failed to show logs: %v", err)
	}

	// Verify log file exists and has content
	if _, err := os.Stat(process.logFile); err != nil {
		t.Errorf("Expected log file to exist at %s", process.logFile)
	}

	logContent, err := os.ReadFile(process.logFile)
	if err != nil {
		t.Errorf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(logContent), "test log message") {
		t.Errorf("Expected log file to contain 'test log message', got: %s", string(logContent))
	}
}

func TestProcessShowLogsNonExistentFile(t *testing.T) {
	process := &Process{
		logFile: "/nonexistent/path/test.log",
		name:    "test",
	}

	err := process.ShowLogs(false)
	if err == nil {
		t.Error("Expected error when showing logs for non-existent file")
	}
}

func TestFindProcess(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	tempDir := t.TempDir()
	
	opts := ProcessOptions{
		WorkDir: tempDir,
		Name:    "test-find-process",
	}

	// Create and start a process
	originalProcess, err := NewProcess("sleep", []string{"2"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	err = originalProcess.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer originalProcess.Stop()

	// Test finding the process
	foundProcess, err := FindProcess("test-find-process")
	if err != nil {
		t.Errorf("Failed to find process: %v", err)
	}

	if foundProcess == nil {
		t.Fatal("Expected to find process")
	}

	if foundProcess.name != "test-find-process" {
		t.Errorf("Expected found process name to be 'test-find-process', got %s", foundProcess.name)
	}

	if foundProcess.pidFile != originalProcess.pidFile {
		t.Errorf("Expected found process PID file to match original")
	}

	if foundProcess.logFile != originalProcess.logFile {
		t.Errorf("Expected found process log file to match original")
	}

	// Test that the found process is running
	running, err := foundProcess.IsRunning()
	if err != nil {
		t.Errorf("Failed to check if found process is running: %v", err)
	}
	if !running {
		t.Error("Expected found process to be running")
	}
}

func TestFindProcessNonExistent(t *testing.T) {
	_, err := FindProcess("non-existent-process")
	if err == nil {
		t.Error("Expected error when finding non-existent process")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected error message to contain 'not found', got: %s", err.Error())
	}
}

func TestProcessStopNonExistentPIDFile(t *testing.T) {
	process := &Process{
		pidFile: "/nonexistent/path/test.pid",
		name:    "test",
	}

	err := process.Stop()
	if err == nil {
		t.Error("Expected error when stopping process with non-existent PID file")
	}
}

func TestProcessStopInvalidPID(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "invalid.pid")

	// Write invalid PID
	err := os.WriteFile(pidFile, []byte("invalid_pid"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid PID file: %v", err)
	}

	process := &Process{
		pidFile: pidFile,
		name:    "test",
	}

	err = process.Stop()
	if err == nil {
		t.Error("Expected error when stopping process with invalid PID")
	}
}

func TestProcessStopNonExistentProcess(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "nonexistent.pid")

	// Write PID of non-existent process (use a very high number that's unlikely to exist)
	err := os.WriteFile(pidFile, []byte("999999"), 0644)
	if err != nil {
		t.Fatalf("Failed to create PID file: %v", err)
	}

	process := &Process{
		pidFile: pidFile,
		name:    "test",
	}

	// This should not return an error, it should clean up gracefully
	err = process.Stop()
	if err != nil {
		t.Errorf("Expected no error when stopping non-existent process, got: %v", err)
	}

	// Verify PID file was cleaned up
	if _, err := os.Stat(pidFile); err == nil {
		t.Error("Expected PID file to be removed")
	}
}

func TestProcessEnvironmentInheritance(t *testing.T) {
	// Set a test environment variable
	originalValue := os.Getenv("TEST_INHERITANCE")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("TEST_INHERITANCE")
		} else {
			os.Setenv("TEST_INHERITANCE", originalValue)
		}
	}()

	os.Setenv("TEST_INHERITANCE", "parent_value")

	opts := ProcessOptions{
		Name: "test-env-process",
		Env: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	process, err := NewProcess("echo", []string{"test"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	// Check that both inherited and custom environment variables are present
	envMap := make(map[string]string)
	for _, env := range process.cmd.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Check inherited environment
	if envMap["TEST_INHERITANCE"] != "parent_value" {
		t.Errorf("Expected inherited TEST_INHERITANCE to be 'parent_value', got %s", envMap["TEST_INHERITANCE"])
	}

	// Check custom environment
	if envMap["CUSTOM_VAR"] != "custom_value" {
		t.Errorf("Expected custom CUSTOM_VAR to be 'custom_value', got %s", envMap["CUSTOM_VAR"])
	}

	// Check that standard environment variables are still present
	if envMap["PATH"] == "" {
		t.Error("Expected PATH environment variable to be inherited")
	}
}

func TestProcessDirectoryCreation(t *testing.T) {
	opts := ProcessOptions{
		Name: "test-dir-process",
	}

	_, err := NewProcess("echo", []string{"test"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	// Check that run and log directories were created
	runDir := filepath.Join(os.TempDir(), "mcp-compose", "run")
	logDir := filepath.Join(os.TempDir(), "mcp-compose", "logs")

	if _, err := os.Stat(runDir); err != nil {
		t.Errorf("Expected run directory to be created at %s", runDir)
	}

	if _, err := os.Stat(logDir); err != nil {
		t.Errorf("Expected log directory to be created at %s", logDir)
	}
}

func TestProcessSignalHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	tempDir := t.TempDir()
	
	opts := ProcessOptions{
		WorkDir: tempDir,
		Name:    "test-signal-process",
	}

	// Start a long-running process
	process, err := NewProcess("sleep", []string{"10"}, opts)
	if err != nil {
		t.Fatalf("Failed to create process: %v", err)
	}

	err = process.Start()
	if err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}

	// Verify process is running
	running, err := process.IsRunning()
	if err != nil {
		t.Fatalf("Failed to check if process is running: %v", err)
	}
	if !running {
		t.Fatal("Expected process to be running")
	}

	// Stop the process (sends SIGTERM)
	err = process.Stop()
	if err != nil {
		t.Errorf("Failed to stop process: %v", err)
	}

	// Wait a bit for signal to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify process is no longer running
	running, err = process.IsRunning()
	if err == nil && running {
		t.Error("Expected process to be stopped after SIGTERM")
	}
}

func TestProcessConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping process test in short mode")
	}

	tempDir := t.TempDir()
	
	// Test concurrent process creation and management
	done := make(chan error, 5)
	
	for i := 0; i < 5; i++ {
		go func(id int) {
			opts := ProcessOptions{
				WorkDir: tempDir,
				Name:    fmt.Sprintf("concurrent-process-%d", id),
			}

			process, err := NewProcess("echo", []string{fmt.Sprintf("message-%d", id)}, opts)
			if err != nil {
				done <- fmt.Errorf("failed to create process %d: %v", id, err)
				return
			}

			err = process.Start()
			if err != nil {
				done <- fmt.Errorf("failed to start process %d: %v", id, err)
				return
			}

			// Wait a bit for process to complete
			time.Sleep(100 * time.Millisecond)

			// Try to find the process
			_, err = FindProcess(fmt.Sprintf("concurrent-process-%d", id))
			if err != nil {
				done <- fmt.Errorf("failed to find process %d: %v", id, err)
				return
			}

			done <- nil
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}