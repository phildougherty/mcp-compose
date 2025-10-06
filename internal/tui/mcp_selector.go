package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MCPServerInfo struct {
	Name      string
	Status    string
	Protocol  string
	ToolCount int
	Enabled   bool
}

type MCPSelectorModel struct {
	servers       []MCPServerInfo
	cursor        int
	width         int
	height        int
	confirmed     bool
	cancelled     bool
	allSelected   bool
}

func NewMCPSelectorModel(servers []MCPServerInfo) MCPSelectorModel {
	allSelected := true
	for _, s := range servers {
		if !s.Enabled {
			allSelected = false

			break
		}
	}

	return MCPSelectorModel{
		servers:     servers,
		cursor:      0,
		allSelected: allSelected,
	}
}

func (m MCPSelectorModel) Init() tea.Cmd {
	return nil
}

func (m MCPSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true

			return m, tea.Quit

		case "enter":
			m.confirmed = true

			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.servers)-1 {
				m.cursor++
			}

		case " ":
			if m.cursor < len(m.servers) {
				m.servers[m.cursor].Enabled = !m.servers[m.cursor].Enabled
				m.updateAllSelectedState()
			}

		case "a":
			m.toggleAll()

		case "d":
			m.deselectAll()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m *MCPSelectorModel) toggleAll() {
	newState := !m.allSelected
	for i := range m.servers {
		m.servers[i].Enabled = newState
	}
	m.allSelected = newState
}

func (m *MCPSelectorModel) deselectAll() {
	for i := range m.servers {
		m.servers[i].Enabled = false
	}
	m.allSelected = false
}

func (m *MCPSelectorModel) updateAllSelectedState() {
	allSelected := true
	for _, s := range m.servers {
		if !s.Enabled {
			allSelected = false

			break
		}
	}
	m.allSelected = allSelected
}

func (m MCPSelectorModel) View() string {
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	var s strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(purple).
		MarginBottom(1)

	s.WriteString(titleStyle.Render("Configure MCP Servers"))
	s.WriteString("\n\n")

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(white)

	s.WriteString(headerStyle.Render(fmt.Sprintf("%-4s %-25s %-12s %-12s %s\n",
		"", "Server", "Status", "Protocol", "Tools")))

	for i, server := range m.servers {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		checkbox := "[ ]"
		if server.Enabled {
			checkbox = "[✓]"
		}

		statusStyle := lipgloss.NewStyle()
		switch server.Status {
		case "Running", "running":
			statusStyle = statusStyle.Foreground(green)
		case "Stopped", "stopped":
			statusStyle = statusStyle.Foreground(red)
		default:
			statusStyle = statusStyle.Foreground(yellow)
		}

		serverNameStyle := lipgloss.NewStyle()
		if m.cursor == i {
			serverNameStyle = serverNameStyle.Bold(true).Foreground(lightPurple)
		}

		toolCountStr := fmt.Sprintf("%d tools", server.ToolCount)

		line := fmt.Sprintf("%s%s %-25s %-12s %-12s %s\n",
			cursor,
			checkbox,
			serverNameStyle.Render(server.Name),
			statusStyle.Render(server.Status),
			server.Protocol,
			toolCountStr,
		)

		s.WriteString(line)
	}

	s.WriteString("\n")

	helpStyle := lipgloss.NewStyle().
		Foreground(gray).
		MarginTop(1)

	help := []string{
		"↑/k, ↓/j: navigate",
		"space: toggle",
		"a: select all",
		"d: deselect all",
		"enter: confirm",
		"q/esc: cancel",
	}

	s.WriteString(helpStyle.Render(strings.Join(help, " • ")))

	selectedCount := 0
	for _, server := range m.servers {
		if server.Enabled {
			selectedCount++
		}
	}

	summaryStyle := lipgloss.NewStyle().
		Foreground(lightPurple).
		MarginTop(1)

	s.WriteString("\n")
	s.WriteString(summaryStyle.Render(fmt.Sprintf("Selected: %d / %d servers", selectedCount, len(m.servers))))

	return s.String()
}

func (m MCPSelectorModel) GetSelectedServers() []string {
	var selected []string
	for _, server := range m.servers {
		if server.Enabled {
			selected = append(selected, server.Name)
		}
	}

	return selected
}

func (m MCPSelectorModel) WasConfirmed() bool {
	return m.confirmed
}

func (m MCPSelectorModel) WasCancelled() bool {
	return m.cancelled
}
