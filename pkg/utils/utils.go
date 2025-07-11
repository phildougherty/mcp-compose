// pkg/utils/utils.go
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mcpcompose/internal/constants"
)

// FindComposeFile tries to find a compose file in the current directory
func FindComposeFile(fileName string) (string, error) {
	// If file name is absolute, return it directly
	if filepath.IsAbs(fileName) {

		return fileName, nil
	}

	// Try current directory
	if _, err := os.Stat(fileName); err == nil {

		return fileName, nil
	}

	// Try in .mcp directory
	mcpDir := ".mcp"
	mcpFile := filepath.Join(mcpDir, fileName)
	if _, err := os.Stat(mcpFile); err == nil {

		return mcpFile, nil
	}

	return "", fmt.Errorf("compose file '%s' not found", fileName)
}

// FormatDuration formats a duration in a human-readable format
func FormatDuration(d time.Duration) string {
	if d.Hours() > constants.HoursInDay {
		days := int(d.Hours() / constants.HoursInDay)

		return fmt.Sprintf("%d days", days)
	}

	if d.Hours() >= 1 {

		return fmt.Sprintf("%.1f hours", d.Hours())
	}

	if d.Minutes() >= 1 {

		return fmt.Sprintf("%.1f minutes", d.Minutes())
	}

	if d.Seconds() >= 1 {

		return fmt.Sprintf("%.1f seconds", d.Seconds())
	}

	return "less than a second"
}

// FormatSize formats a size in a human-readable format
func FormatSize(size int64) string {
	const (
		B   = 1
		KiB = 1024 * B
		MiB = 1024 * KiB
		GiB = 1024 * MiB
		TiB = 1024 * GiB
		PiB = 1024 * TiB
	)

	switch {
	case size >= PiB:

		return fmt.Sprintf("%.2f PiB", float64(size)/float64(PiB))
	case size >= TiB:

		return fmt.Sprintf("%.2f TiB", float64(size)/float64(TiB))
	case size >= GiB:

		return fmt.Sprintf("%.2f GiB", float64(size)/float64(GiB))
	case size >= MiB:

		return fmt.Sprintf("%.2f MiB", float64(size)/float64(MiB))
	case size >= KiB:

		return fmt.Sprintf("%.2f KiB", float64(size)/float64(KiB))
	default:

		return fmt.Sprintf("%d B", size)
	}
}

// ParseEnvFile parses an environment file and returns a map of environment variables
func ParseEnvFile(filePath string) (map[string]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {

		return nil, err
	}

	envVars := make(map[string]string)

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split by = and set the environment variable
		parts := strings.SplitN(line, "=", constants.StringSplitParts)
		if len(parts) == constants.StringSplitParts {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			value = strings.Trim(value, "\"'")

			envVars[key] = value
		}
	}

	return envVars, nil
}
