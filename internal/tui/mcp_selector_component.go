package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mcpSelectorComponent struct {
	availableServers []mcpServerInfo
	selectedServers  map[string]bool
	cursor           int
}

type mcpServerInfo struct {
	Name      string
	ToolCount int
	Status    string
}

func newMCPSelectorComponent(availableServers []map[string]interface{}, selectedServerNames []string) *mcpSelectorComponent {
	servers := make([]mcpServerInfo, 0, len(availableServers))
	for _, s := range availableServers {
		name, _ := s["name"].(string)
		toolCount, _ := s["tool_count"].(int)
		status, _ := s["status"].(string)

		servers = append(servers, mcpServerInfo{
			Name:      name,
			ToolCount: toolCount,
			Status:    status,
		})
	}

	selected := make(map[string]bool)
	for _, name := range selectedServerNames {
		selected[name] = true
	}

	return &mcpSelectorComponent{
		availableServers: servers,
		selectedServers:  selected,
		cursor:           0,
	}
}

func (m *mcpSelectorComponent) Update(msg tea.Msg) (*mcpSelectorComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.availableServers)-1 {
				m.cursor++
			}

		case " ":
			if m.cursor < len(m.availableServers) {
				serverName := m.availableServers[m.cursor].Name
				m.selectedServers[serverName] = !m.selectedServers[serverName]
			}
		}
	}

	return m, nil
}

func (m *mcpSelectorComponent) View() string {
	var s strings.Builder

	title := TitleStyle.Render("Configure MCP Servers")
	s.WriteString(title + "\n")

	help := DimmedStyle.Render("Use ↑/↓ or j/k to navigate, Space to toggle, Enter to confirm, Esc to cancel")
	s.WriteString(help + "\n\n")

	for i, server := range m.availableServers {
		cursor := " "
		cursorStyle := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "▶"
			cursorStyle = HighlightStyle
		}

		checkbox := "☐"
		if m.selectedServers[server.Name] {
			checkbox = SuccessStyle.Render("☑")
		}

		serverName := server.Name
		if i == m.cursor {
			serverName = cursorStyle.Render(serverName)
		}

		statusStyle := DimmedStyle
		switch server.Status {
		case "running":
			statusStyle = SuccessStyle
		case "stopped":
			statusStyle = ErrorStyle
		}

		s.WriteString(fmt.Sprintf("%s %s %s %s (%d tools)\n",
			cursorStyle.Render(cursor),
			checkbox,
			serverName,
			statusStyle.Render(server.Status),
			server.ToolCount,
		))
	}

	selectedCount := 0
	for _, selected := range m.selectedServers {
		if selected {
			selectedCount++
		}
	}

	s.WriteString("\n")
	s.WriteString(DimmedStyle.Render(fmt.Sprintf("Selected: %d/%d servers", selectedCount, len(m.availableServers))))

	return s.String()
}

func (m *mcpSelectorComponent) GetSelectedServers() []string {
	selected := make([]string, 0)
	for name, isSelected := range m.selectedServers {
		if isSelected {
			selected = append(selected, name)
		}
	}

	return selected
}
