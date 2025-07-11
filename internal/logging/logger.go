// internal/logging/logger.go
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DEBUG level for verbose debugging information
	DEBUG LogLevel = iota
	// INFO level for general information
	INFO
	// WARNING level for non-critical problems
	WARNING
	// ERROR level for error conditions
	ERROR
	// FATAL level for unrecoverable errors
	FATAL
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:

		return "DEBUG"
	case INFO:

		return "INFO"
	case WARNING:

		return "WARNING"
	case ERROR:

		return "ERROR"
	case FATAL:

		return "FATAL"
	default:

		return "UNKNOWN"
	}
}

// Logger provides structured logging functionality
type Logger struct {
	level      LogLevel
	writer     io.Writer
	jsonFormat bool
}

// NewLogger creates a new logger with the specified log level
func NewLogger(level string) *Logger {
	var logLevel LogLevel
	switch strings.ToUpper(level) {
	case "DEBUG":
		logLevel = DEBUG
	case "INFO":
		logLevel = INFO
	case "WARNING":
		logLevel = WARNING
	case "ERROR":
		logLevel = ERROR
	case "FATAL":
		logLevel = FATAL
	default:
		logLevel = INFO
	}

	return &Logger{
		level:      logLevel,
		writer:     os.Stdout,
		jsonFormat: false,
	}
}

// SetOutput sets the output writer for the logger
func (l *Logger) SetOutput(writer io.Writer) {
	l.writer = writer
}

// SetJSONFormat sets whether to use JSON format for logging
func (l *Logger) SetJSONFormat(useJSON bool) {
	l.jsonFormat = useJSON
}

// shouldLog determines if a message at the given level should be logged
func (l *Logger) shouldLog(level LogLevel) bool {

	return level >= l.level
}

// log logs a message at the given level with optional format arguments
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if !l.shouldLog(level) {

		return
	}

	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	timestamp := time.Now().Format(time.RFC3339)

	if l.jsonFormat {
		// Format as JSON
		jsonLog := fmt.Sprintf(`{"timestamp":"%s","level":"%s","message":%q}`,
			timestamp, level.String(), message)
		if _, err := fmt.Fprintln(l.writer, jsonLog); err != nil {
			// If we can't log, there's not much we can do. Print to stderr as fallback.
			fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
		}
	} else {
		// Format as text
		if _, err := fmt.Fprintf(l.writer, "[%s] %s: %s\n", timestamp, level.String(), message); err != nil {
			// If we can't log, there's not much we can do. Print to stderr as fallback.
			fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
		}
	}

	// If this is a fatal message, exit after logging
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log(WARNING, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits the program
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	// The program will exit in the log method
}

// WithFields creates a new logger with the specified fields
func (l *Logger) WithFields(fields map[string]interface{}) *FieldLogger {

	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// FieldLogger is a logger that includes additional fields in logs
type FieldLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug logs a debug message with fields
func (fl *FieldLogger) Debug(format string, args ...interface{}) {
	if !fl.logger.shouldLog(DEBUG) {

		return
	}
	fl.logWithFields(DEBUG, format, args...)
}

// Info logs an informational message with fields
func (fl *FieldLogger) Info(format string, args ...interface{}) {
	if !fl.logger.shouldLog(INFO) {

		return
	}
	fl.logWithFields(INFO, format, args...)
}

// Warning logs a warning message with fields
func (fl *FieldLogger) Warning(format string, args ...interface{}) {
	if !fl.logger.shouldLog(WARNING) {

		return
	}
	fl.logWithFields(WARNING, format, args...)
}

// Error logs an error message with fields
func (fl *FieldLogger) Error(format string, args ...interface{}) {
	if !fl.logger.shouldLog(ERROR) {

		return
	}
	fl.logWithFields(ERROR, format, args...)
}

// Fatal logs a fatal message with fields and exits the program
func (fl *FieldLogger) Fatal(format string, args ...interface{}) {
	if !fl.logger.shouldLog(FATAL) {

		return
	}
	fl.logWithFields(FATAL, format, args...)
	os.Exit(1)
}

// logWithFields logs a message with additional fields
func (fl *FieldLogger) logWithFields(level LogLevel, format string, args ...interface{}) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	timestamp := time.Now().Format(time.RFC3339)

	if fl.logger.jsonFormat {
		// Start with the base fields
		jsonParts := []string{
			fmt.Sprintf(`"timestamp":"%s"`, timestamp),
			fmt.Sprintf(`"level":"%s"`, level.String()),
			fmt.Sprintf(`"message":%q`, message),
		}

		// Add the additional fields
		for k, v := range fl.fields {
			jsonParts = append(jsonParts, fmt.Sprintf(`"%s":%q`, k, fmt.Sprintf("%v", v)))
		}

		// Combine into a JSON object
		jsonLog := "{" + strings.Join(jsonParts, ",") + "}"
		if _, err := fmt.Fprintln(fl.logger.writer, jsonLog); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write structured log: %v\n", err)
		}
	} else {
		// Format as text with fields
		fieldStr := ""
		for k, v := range fl.fields {
			fieldStr += fmt.Sprintf(" %s=%v", k, v)
		}
		if _, err := fmt.Fprintf(fl.logger.writer, "[%s] %s: %s%s\n", timestamp, level.String(), message, fieldStr); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write structured log: %v\n", err)
		}
	}

	// If this is a fatal message, exit after logging (handled by the caller)
}
