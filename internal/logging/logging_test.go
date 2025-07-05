package logging

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLogLevel(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARNING, "WARNING"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"WARNING", WARNING},
		{"warning", WARNING},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"FATAL", FATAL},
		{"fatal", FATAL},
		{"INVALID", INFO}, // Should default to INFO
		{"", INFO},        // Should default to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			logger := NewLogger(tt.input)
			if logger.level != tt.expected {
				t.Errorf("Expected level %s, got %s", tt.expected.String(), logger.level.String())
			}

			if logger.writer != os.Stdout {
				t.Error("Expected writer to be os.Stdout")
			}

			if logger.jsonFormat {
				t.Error("Expected jsonFormat to be false by default")
			}
		})
	}
}

func TestLoggerSetOutput(t *testing.T) {
	logger := NewLogger("INFO")
	buf := &bytes.Buffer{}

	logger.SetOutput(buf)

	if logger.writer != buf {
		t.Error("Expected writer to be set to buffer")
	}

	// Test that output goes to the buffer
	logger.Info("test message")
	
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected output to contain 'test message', got: %s", output)
	}
}

func TestLoggerSetJSONFormat(t *testing.T) {
	logger := NewLogger("INFO")

	// Test setting JSON format
	logger.SetJSONFormat(true)
	if !logger.jsonFormat {
		t.Error("Expected jsonFormat to be true")
	}

	// Test unsetting JSON format
	logger.SetJSONFormat(false)
	if logger.jsonFormat {
		t.Error("Expected jsonFormat to be false")
	}
}

func TestLoggerShouldLog(t *testing.T) {
	tests := []struct {
		loggerLevel LogLevel
		messageLevel LogLevel
		shouldLog   bool
	}{
		{DEBUG, DEBUG, true},
		{DEBUG, INFO, true},
		{DEBUG, WARNING, true},
		{DEBUG, ERROR, true},
		{DEBUG, FATAL, true},
		{INFO, DEBUG, false},
		{INFO, INFO, true},
		{INFO, WARNING, true},
		{INFO, ERROR, true},
		{INFO, FATAL, true},
		{WARNING, DEBUG, false},
		{WARNING, INFO, false},
		{WARNING, WARNING, true},
		{WARNING, ERROR, true},
		{WARNING, FATAL, true},
		{ERROR, DEBUG, false},
		{ERROR, INFO, false},
		{ERROR, WARNING, false},
		{ERROR, ERROR, true},
		{ERROR, FATAL, true},
		{FATAL, DEBUG, false},
		{FATAL, INFO, false},
		{FATAL, WARNING, false},
		{FATAL, ERROR, false},
		{FATAL, FATAL, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("logger_%s_message_%s", tt.loggerLevel.String(), tt.messageLevel.String()), func(t *testing.T) {
			logger := &Logger{level: tt.loggerLevel}
			result := logger.shouldLog(tt.messageLevel)
			if result != tt.shouldLog {
				t.Errorf("Expected shouldLog to be %v, got %v", tt.shouldLog, result)
			}
		})
	}
}

func TestLoggerTextFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("DEBUG")
	logger.SetOutput(buf)
	logger.SetJSONFormat(false)

	tests := []struct {
		method   func(string, ...interface{})
		level    string
		message  string
		args     []interface{}
	}{
		{logger.Debug, "DEBUG", "debug message", nil},
		{logger.Info, "INFO", "info message", nil},
		{logger.Warning, "WARNING", "warning message", nil},
		{logger.Error, "ERROR", "error message", nil},
		{logger.Debug, "DEBUG", "debug with args: %s %d", []interface{}{"test", 42}},
		{logger.Info, "INFO", "info with args: %s %d", []interface{}{"test", 42}},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf.Reset()
			
			if tt.args != nil {
				tt.method(tt.message, tt.args...)
			} else {
				tt.method(tt.message)
			}

			output := buf.String()
			
			// Check that output contains expected elements
			if !strings.Contains(output, tt.level) {
				t.Errorf("Expected output to contain level %s, got: %s", tt.level, output)
			}

			if tt.args != nil {
				expectedMessage := fmt.Sprintf(tt.message, tt.args...)
				if !strings.Contains(output, expectedMessage) {
					t.Errorf("Expected output to contain formatted message %s, got: %s", expectedMessage, output)
				}
			} else {
				if !strings.Contains(output, tt.message) {
					t.Errorf("Expected output to contain message %s, got: %s", tt.message, output)
				}
			}

			// Check timestamp format (should contain date/time)
			if !strings.Contains(output, "T") || !strings.Contains(output, ":") {
				t.Errorf("Expected output to contain timestamp, got: %s", output)
			}
		})
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("DEBUG")
	logger.SetOutput(buf)
	logger.SetJSONFormat(true)

	tests := []struct {
		method   func(string, ...interface{})
		level    string
		message  string
		args     []interface{}
	}{
		{logger.Debug, "DEBUG", "debug message", nil},
		{logger.Info, "INFO", "info message", nil},
		{logger.Warning, "WARNING", "warning message", nil},
		{logger.Error, "ERROR", "error message", nil},
		{logger.Debug, "DEBUG", "debug with args: %s %d", []interface{}{"test", 42}},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf.Reset()
			
			if tt.args != nil {
				tt.method(tt.message, tt.args...)
			} else {
				tt.method(tt.message)
			}

			output := buf.String()
			
			// Check that output is valid JSON structure
			if !strings.HasPrefix(output, "{") || !strings.HasSuffix(strings.TrimSpace(output), "}") {
				t.Errorf("Expected JSON output, got: %s", output)
			}

			// Check that output contains expected JSON fields
			expectedFields := []string{
				`"timestamp"`,
				`"level":"` + tt.level + `"`,
				`"message"`,
			}

			for _, field := range expectedFields {
				if !strings.Contains(output, field) {
					t.Errorf("Expected JSON output to contain %s, got: %s", field, output)
				}
			}

			// Check message content
			if tt.args != nil {
				expectedMessage := fmt.Sprintf(tt.message, tt.args...)
				if !strings.Contains(output, expectedMessage) {
					t.Errorf("Expected output to contain formatted message %s, got: %s", expectedMessage, output)
				}
			} else {
				if !strings.Contains(output, tt.message) {
					t.Errorf("Expected output to contain message %s, got: %s", tt.message, output)
				}
			}
		})
	}
}

func TestLoggerFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("WARNING")
	logger.SetOutput(buf)

	// These should not be logged
	logger.Debug("debug message")
	logger.Info("info message")

	// These should be logged
	logger.Warning("warning message")
	logger.Error("error message")

	output := buf.String()

	// Check that debug and info messages are not present
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should not be logged when level is WARNING")
	}

	if strings.Contains(output, "info message") {
		t.Error("Info message should not be logged when level is WARNING")
	}

	// Check that warning and error messages are present
	if !strings.Contains(output, "warning message") {
		t.Error("Warning message should be logged when level is WARNING")
	}

	if !strings.Contains(output, "error message") {
		t.Error("Error message should be logged when level is WARNING")
	}
}

func TestLoggerWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("INFO")
	logger.SetOutput(buf)

	fields := map[string]interface{}{
		"component": "test",
		"request_id": "123",
		"user_id": 456,
	}

	fieldLogger := logger.WithFields(fields)

	if fieldLogger.logger != logger {
		t.Error("Expected field logger to reference original logger")
	}

	if len(fieldLogger.fields) != len(fields) {
		t.Errorf("Expected %d fields, got %d", len(fields), len(fieldLogger.fields))
	}

	for k, v := range fields {
		if fieldLogger.fields[k] != v {
			t.Errorf("Expected field %s to be %v, got %v", k, v, fieldLogger.fields[k])
		}
	}
}

func TestFieldLoggerTextFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("DEBUG")
	logger.SetOutput(buf)
	logger.SetJSONFormat(false)

	fields := map[string]interface{}{
		"component": "test",
		"request_id": "123",
	}

	fieldLogger := logger.WithFields(fields)

	tests := []struct {
		method func(string, ...interface{})
		level  string
		message string
	}{
		{fieldLogger.Debug, "DEBUG", "debug with fields"},
		{fieldLogger.Info, "INFO", "info with fields"},
		{fieldLogger.Warning, "WARNING", "warning with fields"},
		{fieldLogger.Error, "ERROR", "error with fields"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf.Reset()
			tt.method(tt.message)

			output := buf.String()

			// Check basic log structure
			if !strings.Contains(output, tt.level) {
				t.Errorf("Expected output to contain level %s, got: %s", tt.level, output)
			}

			if !strings.Contains(output, tt.message) {
				t.Errorf("Expected output to contain message %s, got: %s", tt.message, output)
			}

			// Check that fields are included
			for k, v := range fields {
				expectedField := fmt.Sprintf("%s=%v", k, v)
				if !strings.Contains(output, expectedField) {
					t.Errorf("Expected output to contain field %s, got: %s", expectedField, output)
				}
			}
		})
	}
}

func TestFieldLoggerJSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("DEBUG")
	logger.SetOutput(buf)
	logger.SetJSONFormat(true)

	fields := map[string]interface{}{
		"component": "test",
		"request_id": "123",
	}

	fieldLogger := logger.WithFields(fields)

	tests := []struct {
		method func(string, ...interface{})
		level  string
		message string
	}{
		{fieldLogger.Debug, "DEBUG", "debug with fields"},
		{fieldLogger.Info, "INFO", "info with fields"},
		{fieldLogger.Warning, "WARNING", "warning with fields"},
		{fieldLogger.Error, "ERROR", "error with fields"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf.Reset()
			tt.method(tt.message)

			output := buf.String()

			// Check JSON structure
			if !strings.HasPrefix(output, "{") || !strings.HasSuffix(strings.TrimSpace(output), "}") {
				t.Errorf("Expected JSON output, got: %s", output)
			}

			// Check standard fields
			expectedFields := []string{
				`"timestamp"`,
				`"level":"` + tt.level + `"`,
				`"message"`,
			}

			for _, field := range expectedFields {
				if !strings.Contains(output, field) {
					t.Errorf("Expected JSON output to contain %s, got: %s", field, output)
				}
			}

			// Check custom fields
			for k, v := range fields {
				expectedField := fmt.Sprintf(`"%s":"%v"`, k, v)
				if !strings.Contains(output, expectedField) {
					t.Errorf("Expected JSON output to contain field %s, got: %s", expectedField, output)
				}
			}

			// Check message content
			if !strings.Contains(output, tt.message) {
				t.Errorf("Expected output to contain message %s, got: %s", tt.message, output)
			}
		})
	}
}

func TestFieldLoggerFiltering(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("WARNING")
	logger.SetOutput(buf)

	fields := map[string]interface{}{
		"component": "test",
	}

	fieldLogger := logger.WithFields(fields)

	// These should not be logged
	fieldLogger.Debug("debug message")
	fieldLogger.Info("info message")

	// These should be logged
	fieldLogger.Warning("warning message")
	fieldLogger.Error("error message")

	output := buf.String()

	// Check that debug and info messages are not present
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should not be logged when level is WARNING")
	}

	if strings.Contains(output, "info message") {
		t.Error("Info message should not be logged when level is WARNING")
	}

	// Check that warning and error messages are present
	if !strings.Contains(output, "warning message") {
		t.Error("Warning message should be logged when level is WARNING")
	}

	if !strings.Contains(output, "error message") {
		t.Error("Error message should be logged when level is WARNING")
	}
}

func TestLoggerFormatting(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("INFO")
	logger.SetOutput(buf)

	// Test message formatting with arguments
	logger.Info("User %s logged in with ID %d", "alice", 123)

	output := buf.String()
	expectedMessage := "User alice logged in with ID 123"

	if !strings.Contains(output, expectedMessage) {
		t.Errorf("Expected formatted message %s, got: %s", expectedMessage, output)
	}
}

func TestFieldLoggerFormatting(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("INFO")
	logger.SetOutput(buf)

	fields := map[string]interface{}{
		"session": "abc123",
	}

	fieldLogger := logger.WithFields(fields)

	// Test message formatting with arguments
	fieldLogger.Info("Processing request %d for user %s", 456, "bob")

	output := buf.String()
	expectedMessage := "Processing request 456 for user bob"

	if !strings.Contains(output, expectedMessage) {
		t.Errorf("Expected formatted message %s, got: %s", expectedMessage, output)
	}

	// Check that fields are still included
	if !strings.Contains(output, "session") {
		t.Error("Expected field 'session' to be included in output")
	}
}

// Note: We don't test the Fatal methods because they call os.Exit(),
// which would terminate the test process. In a real-world scenario,
// you might want to test these using a separate process or by mocking os.Exit.

func TestLoggerConcurrency(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewLogger("INFO")
	logger.SetOutput(buf)

	// Test concurrent logging
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			logger.Info("Message from goroutine %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	output := buf.String()

	// Check that we have 10 log messages
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 log lines, got %d", len(lines))
	}

	// Check that all messages are present
	for i := 0; i < 10; i++ {
		expectedMessage := fmt.Sprintf("Message from goroutine %d", i)
		if !strings.Contains(output, expectedMessage) {
			t.Errorf("Expected to find message for goroutine %d", i)
		}
	}
}