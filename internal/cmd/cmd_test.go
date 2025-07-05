package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// testHelper provides utilities for testing CLI commands
type testHelper struct {
	originalStdout *os.File
	originalStderr *os.File
	buf            *bytes.Buffer
}

func newTestHelper() *testHelper {
	return &testHelper{
		originalStdout: os.Stdout,
		originalStderr: os.Stderr,
		buf:            &bytes.Buffer{},
	}
}

func (h *testHelper) captureOutput() {
	// For testing, we'll use a different approach - 
	// we'll redirect cobra command output directly
}

func (h *testHelper) restoreOutput() {
	os.Stdout = h.originalStdout
	os.Stderr = h.originalStderr
}

func (h *testHelper) getOutput() string {
	return h.buf.String()
}

func (h *testHelper) reset() {
	h.buf.Reset()
}

func TestNewRootCommand(t *testing.T) {
	rootCmd := NewRootCommand("test-version")

	if rootCmd.Use != "mcp-compose" {
		t.Errorf("Expected Use to be 'mcp-compose', got %q", rootCmd.Use)
	}

	if rootCmd.Short != "Manage MCP servers with compose" {
		t.Errorf("Expected correct short description, got %q", rootCmd.Short)
	}

	if rootCmd.Version != "test-version" {
		t.Errorf("Expected Version to be 'test-version', got %q", rootCmd.Version)
	}

	// Test persistent flags
	fileFlag := rootCmd.PersistentFlags().Lookup("file")
	if fileFlag == nil {
		t.Error("Expected 'file' persistent flag to exist")
	} else if fileFlag.DefValue != "mcp-compose.yaml" {
		t.Errorf("Expected 'file' flag default to be 'mcp-compose.yaml', got %q", fileFlag.DefValue)
	}

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("Expected 'verbose' persistent flag to exist")
	} else if verboseFlag.DefValue != "false" {
		t.Errorf("Expected 'verbose' flag default to be 'false', got %q", verboseFlag.DefValue)
	}
}

func TestRootCommandSubcommands(t *testing.T) {
	rootCmd := NewRootCommand("test-version")

	expectedCommands := []string{
		"up",
		"down",
		"start",
		"stop",
		"restart",
		"ls",
		"logs",
		"validate",
		"completion",
		"create-config",
		"proxy",
		"reload",
		"dashboard",
		"task-scheduler",
		"memory",
	}

	for _, expectedCmd := range expectedCommands {
		if cmd, _, err := rootCmd.Find([]string{expectedCmd}); err != nil || cmd == rootCmd {
			t.Errorf("Expected subcommand %q to exist", expectedCmd)
		}
	}
}

func TestNewUpCommand(t *testing.T) {
	cmd := NewUpCommand()

	if cmd.Use != "up [SERVER...]" {
		t.Errorf("Expected Use to be 'up [SERVER...]', got %q", cmd.Use)
	}

	if cmd.Short != "Create and start MCP servers" {
		t.Errorf("Expected correct short description, got %q", cmd.Short)
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}
}

func TestNewDownCommand(t *testing.T) {
	cmd := NewDownCommand()

	if cmd.Use != "down [SERVER|proxy|dashboard|task-scheduler|memory]..." {
		t.Errorf("Expected specific Use pattern, got %q", cmd.Use)
	}

	if cmd.Short != "Stop and remove MCP servers, proxy, dashboard, task-scheduler, or memory server" {
		t.Errorf("Expected correct short description, got %q", cmd.Short)
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}

	// Test that Long description contains examples
	if !strings.Contains(cmd.Long, "Examples:") {
		t.Error("Expected Long description to contain examples")
	}
}

func TestNewValidateCommand(t *testing.T) {
	cmd := NewValidateCommand()

	if cmd.Use != "validate" {
		t.Errorf("Expected Use to be 'validate', got %q", cmd.Use)
	}

	if cmd.Short != "Validate the compose file" {
		t.Errorf("Expected correct short description, got %q", cmd.Short)
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set")
	}
}

func TestCommandInheritFlags(t *testing.T) {
	rootCmd := NewRootCommand("test-version")
	
	// Test that subcommands inherit persistent flags
	subcommands := []string{"up", "down", "validate", "ls"}
	
	for _, subcmdName := range subcommands {
		subcmd, _, err := rootCmd.Find([]string{subcmdName})
		if err != nil {
			t.Fatalf("Failed to find subcommand %q: %v", subcmdName, err)
		}

		// Check inherited flags
		fileFlag := subcmd.Flags().Lookup("file")
		if fileFlag == nil {
			t.Errorf("Subcommand %q should inherit 'file' flag", subcmdName)
		}

		verboseFlag := subcmd.Flags().Lookup("verbose")
		if verboseFlag == nil {
			t.Errorf("Subcommand %q should inherit 'verbose' flag", subcmdName)
		}
	}
}

func TestCommandExecution(t *testing.T) {
	// Create temporary config file for testing
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name        string
		cmdFactory  func() *cobra.Command
		args        []string
		expectError bool
	}{
		{
			name:        "validate with valid config",
			cmdFactory:  NewValidateCommand,
			args:        []string{"--file", configFile},
			expectError: false,
		},
		{
			name:        "validate with nonexistent config",
			cmdFactory:  NewValidateCommand,
			args:        []string{"--file", "/nonexistent/config.yaml"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFactory()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestRootCommandHelp(t *testing.T) {
	helper := newTestHelper()
	rootCmd := NewRootCommand("test-version")
	rootCmd.SetOut(helper.buf)
	rootCmd.SetErr(helper.buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Help command should not return error: %v", err)
	}

	output := helper.getOutput()
	
	// Check that help output contains expected content
	expectedContent := []string{
		"mcp-compose",
		"Manage MCP servers with compose",
		"Available Commands:",
		"Flags:",
		"--file",
		"--verbose",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("Help output should contain %q, got:\n%s", expected, output)
		}
	}
}

func TestRootCommandVersion(t *testing.T) {
	helper := newTestHelper()

	rootCmd := NewRootCommand("1.2.3")
	rootCmd.SetOut(helper.buf)
	rootCmd.SetErr(helper.buf)
	rootCmd.SetArgs([]string{"--version"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Version command should not return error: %v", err)
	}

	output := helper.getOutput()
	if !strings.Contains(output, "1.2.3") {
		t.Errorf("Version output should contain version number, got: %s", output)
	}
}

func TestSubcommandHelp(t *testing.T) {
	helper := newTestHelper()
	
	subcommands := []struct {
		name       string
		cmdFactory func() *cobra.Command
	}{
		{"up", NewUpCommand},
		{"down", NewDownCommand},
		{"validate", NewValidateCommand},
	}

	for _, subcmd := range subcommands {
		t.Run(subcmd.name+"_help", func(t *testing.T) {
			helper.reset()

			cmd := subcmd.cmdFactory()
			cmd.SetOut(helper.buf)
			cmd.SetErr(helper.buf)
			cmd.SetArgs([]string{"--help"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("Help for %q should not return error: %v", subcmd.name, err)
			}

			output := helper.getOutput()
			if !strings.Contains(output, subcmd.name) {
				t.Errorf("Help output for %q should contain command name, got:\n%s", subcmd.name, output)
			}
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	// Create temporary invalid config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-config.yaml")
	invalidContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello
    # missing closing quote
`
	if err := os.WriteFile(configFile, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid config file: %v", err)
	}

	cmd := NewValidateCommand()
	cmd.SetArgs([]string{"--file", configFile})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error for invalid config file")
	}
}

func TestFlagShortcuts(t *testing.T) {
	helper := newTestHelper()
	rootCmd := NewRootCommand("test-version")
	rootCmd.SetOut(helper.buf)
	rootCmd.SetErr(helper.buf)

	// Test short flag for file
	rootCmd.SetArgs([]string{"-c", "test.yaml", "--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command with short flags should not error: %v", err)
	}

	// Test short flag for verbose
	rootCmd.SetArgs([]string{"-v", "--help"})
	helper.reset()
	
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command with verbose flag should not error: %v", err)
	}
}

func TestCommandArguments(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *cobra.Command
		args     []string
		expected bool // whether args should be valid
	}{
		{
			name:     "up with no args",
			cmd:      NewUpCommand(),
			args:     []string{},
			expected: true,
		},
		{
			name:     "up with server names",
			cmd:      NewUpCommand(),
			args:     []string{"server1", "server2"},
			expected: true,
		},
		{
			name:     "down with no args",
			cmd:      NewDownCommand(),
			args:     []string{},
			expected: true,
		},
		{
			name:     "down with special services",
			cmd:      NewDownCommand(),
			args:     []string{"proxy", "dashboard"},
			expected: true,
		},
		{
			name:     "validate with no args",
			cmd:      NewValidateCommand(),
			args:     []string{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that command accepts the arguments without validation errors
			// Note: actual execution might fail due to missing services, but argument parsing should work
			tt.cmd.SetArgs(tt.args)
			_, err := tt.cmd.ExecuteC()
			
			// We mainly care that argument parsing works, not the actual execution
			if tt.expected && err != nil && strings.Contains(err.Error(), "unknown command") {
				t.Errorf("Command should accept arguments %v, but got parse error: %v", tt.args, err)
			}
		})
	}
}