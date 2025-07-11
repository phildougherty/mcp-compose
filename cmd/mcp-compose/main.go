package main

import (
	"fmt"
	"os"

	"github.com/phildougherty/mcp-compose/internal/cmd"
)

var version = "0.0.4"

func main() {
	rootCmd := cmd.NewRootCommand(version)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
