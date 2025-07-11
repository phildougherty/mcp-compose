// internal/cmd/reload.go
package cmd

import (
	"fmt"
	"net/http"

	"github.com/phildougherty/mcp-compose/internal/constants"

	"github.com/spf13/cobra"
)

func NewReloadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reload",
		Short: "Reload MCP proxy configuration to discover new servers",
		Long: `Reload the MCP proxy configuration to discover newly started servers.
This will refresh the proxy's server list without restarting the proxy.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			port, _ := cmd.Flags().GetInt("port")
			apiKey, _ := cmd.Flags().GetString("api-key")

			return reloadProxy(port, apiKey)
		},
	}

	cmd.Flags().IntP("port", "p", constants.DefaultProxyPort, "Proxy server port")
	cmd.Flags().String("api-key", "", "API key for proxy authentication")

	return cmd
}

func reloadProxy(port int, apiKey string) error {
	url := fmt.Sprintf("http://localhost:%d/api/reload", port)

	// Create HTTP request
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {

		return fmt.Errorf("failed to create reload request: %w", err)
	}

	// Add API key if provided
	if apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {

		return fmt.Errorf("failed to send reload request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {

		return fmt.Errorf("reload failed with status: %d", resp.StatusCode)
	}

	fmt.Println("✅ Proxy configuration reloaded successfully")

	return nil
}
