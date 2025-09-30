package cmd

import (
	"fmt"
	"time"

	"github.com/phildougherty/mcp-compose/internal/auth"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/logging"
	"github.com/spf13/cobra"
)

func NewRotateSecretCommand() *cobra.Command {
	var gracePeriod string
	var outputToConsole bool
	var forceRotation bool

	cmd := &cobra.Command{
		Use:   "rotate-secret",
		Short: "Rotate the API key for the proxy",
		Long:  `Rotate the API key for the proxy with graceful downtime-free transition.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, _ := cmd.Flags().GetString("file")

			return runRotateSecret(configFile, gracePeriod, outputToConsole, forceRotation)
		},
	}

	cmd.Flags().StringVarP(&gracePeriod, "grace-period", "g", "24h", "Grace period for old key to remain valid")
	cmd.Flags().BoolVarP(&outputToConsole, "output", "o", false, "Output new key to console instead of updating config file")
	cmd.Flags().BoolVarP(&forceRotation, "force", "f", false, "Force rotation even if recent rotation detected")

	return cmd
}

func runRotateSecret(configPath, gracePeriod string, outputToConsole, forceRotation bool) error {
	logger := logging.NewLogger("debug")

	if configPath == "" {
		configPath = "mcp-compose.yaml"
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.ProxyAuth.Enabled {
		return fmt.Errorf("proxy authentication is not enabled in config")
	}

	graceDuration, err := time.ParseDuration(gracePeriod)
	if err != nil {
		return fmt.Errorf("invalid grace period: %w", err)
	}

	sm := auth.NewSecretManager(logger)
	defer sm.Close()

	if cfg.ProxyAuth.APIKey != "" {
		if err := sm.AddKey(cfg.ProxyAuth.APIKey, auth.APIKeyTypePrimary); err != nil {
			return fmt.Errorf("failed to add current key: %w", err)
		}
	}

	newKey, err := sm.RotateKey(graceDuration)
	if err != nil {
		return fmt.Errorf("failed to rotate key: %w", err)
	}

	if outputToConsole {
		fmt.Println("\n=== New API Key ===")
		fmt.Println(newKey)
		fmt.Println("\nNOTE: Config file was not updated.")
		return nil
	}

	cfg.ProxyAuth.APIKey = newKey

	if err := config.SaveConfig(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Info("API key rotated successfully")

	fmt.Println("\n=== API Key Rotation Successful ===")
	fmt.Printf("New Key (first 8 chars): %s...\n", newKey[:8])
	fmt.Printf("Grace Period: %s\n", gracePeriod)
	fmt.Println("\nRestart mcp-compose for changes to take effect")

	return nil
}
