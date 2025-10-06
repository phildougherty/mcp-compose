package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func ExampleUsage() {
	servers := []MCPServerInfo{
		{
			Name:      "filesystem",
			Status:    "Running",
			Protocol:  "stdio",
			ToolCount: 5,
			Enabled:   true,
		},
		{
			Name:      "brave-search",
			Status:    "Running",
			Protocol:  "stdio",
			ToolCount: 3,
			Enabled:   false,
		},
		{
			Name:      "postgres",
			Status:    "Running",
			Protocol:  "stdio",
			ToolCount: 8,
			Enabled:   true,
		},
	}

	model := NewMCPSelectorModel(servers)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)

		return
	}

	if m, ok := finalModel.(MCPSelectorModel); ok {
		if m.WasConfirmed() {
			selectedServers := m.GetSelectedServers()
			fmt.Printf("Selected servers: %v\n", selectedServers)
		} else if m.WasCancelled() {
			fmt.Println("Selection cancelled")
		}
	}
}
