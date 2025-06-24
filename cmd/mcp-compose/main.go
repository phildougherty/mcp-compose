package main

import (
	"fmt"
	"os"

	"mcpcompose/internal/cmd"
)

var version = "dev"

func main() {
	rootCmd := cmd.NewRootCommand(version)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
