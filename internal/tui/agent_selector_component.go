package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
)

type agentSelectorComponent struct {
	chatService      *dashboard.ChatService
	sessions         []dashboard.ChatSession
	cursor           int
	err              error
	loading          bool
	selectedSession  *dashboard.ChatSession
	viewingMessages  bool
	sessionSwitched  bool
}

type agentListLoadedMsg struct {
	sessions []dashboard.ChatSession
	err      error
}

func newAgentSelectorComponent(chatService *dashboard.ChatService) *agentSelectorComponent {
	return &agentSelectorComponent{
		chatService:     chatService,
		sessions:        []dashboard.ChatSession{},
		cursor:          0,
		loading:         true,
		viewingMessages: false,
	}
}

func (m *agentSelectorComponent) Init() tea.Cmd {
	return m.loadSessions()
}

func (m *agentSelectorComponent) loadSessions() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.chatService.ListSessions("default")
		if err != nil {
			return agentListLoadedMsg{sessions: nil, err: err}
		}

		return agentListLoadedMsg{sessions: sessions, err: nil}
	}
}

func (m *agentSelectorComponent) Update(msg tea.Msg) (*agentSelectorComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case agentListLoadedMsg:
		m.loading = false
		m.sessions = msg.sessions
		m.err = msg.err

		return m, nil

	case tea.KeyMsg:
		if m.viewingMessages {
			switch msg.String() {
			case "esc", "q":
				m.viewingMessages = false
				m.selectedSession = nil

				return m, nil
			}

			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
			}

		case "enter":
			if m.cursor < len(m.sessions) {
				m.selectedSession = &m.sessions[m.cursor]
				m.viewingMessages = true
			}
		}
	}

	return m, nil
}

func (m *agentSelectorComponent) View() string {
	var s strings.Builder

	title := TitleStyle.Render("Agent Sessions")
	s.WriteString(title + "\n")

	if m.loading {
		s.WriteString(DimmedStyle.Render("Loading sessions...") + "\n")

		return s.String()
	}

	if m.err != nil {
		s.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n")

		return s.String()
	}

	if m.viewingMessages && m.selectedSession != nil {
		return m.renderSessionMessages()
	}

	help := DimmedStyle.Render("Use ↑/↓ or j/k to navigate, Enter to view session, Esc to cancel")
	s.WriteString(help + "\n\n")

	if len(m.sessions) == 0 {
		s.WriteString(DimmedStyle.Render("No sessions found.") + "\n")

		return s.String()
	}

	for i, session := range m.sessions {
		cursor := " "
		cursorStyle := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "▶"
			cursorStyle = HighlightStyle
		}

		title := session.Title
		if title == "" {
			title = "Untitled Session"
		}

		if i == m.cursor {
			title = cursorStyle.Render(title)
		}

		lastUsed := session.LastUsed.Format("Jan 02, 15:04")
		messageCount := len(session.Messages)

		providerModel := fmt.Sprintf("%s/%s", session.Provider, session.Model)

		s.WriteString(fmt.Sprintf("%s %s\n",
			cursorStyle.Render(cursor),
			title,
		))
		s.WriteString(fmt.Sprintf("  %s | %d messages | Last used: %s\n",
			DimmedStyle.Render(providerModel),
			messageCount,
			DimmedStyle.Render(lastUsed),
		))
	}

	s.WriteString("\n")
	s.WriteString(DimmedStyle.Render(fmt.Sprintf("Total sessions: %d", len(m.sessions))))

	return s.String()
}

func (m *agentSelectorComponent) renderSessionMessages() string {
	var s strings.Builder

	title := TitleStyle.Render(fmt.Sprintf("Session: %s", m.selectedSession.Title))
	s.WriteString(title + "\n")

	help := DimmedStyle.Render("Press Esc or q to go back")
	s.WriteString(help + "\n\n")

	info := fmt.Sprintf("Provider: %s | Model: %s | Created: %s",
		m.selectedSession.Provider,
		m.selectedSession.Model,
		m.selectedSession.CreatedAt.Format("Jan 02, 2006 15:04"),
	)
	s.WriteString(DimmedStyle.Render(info) + "\n\n")

	if len(m.selectedSession.Messages) == 0 {
		s.WriteString(DimmedStyle.Render("No messages in this session.") + "\n")

		return s.String()
	}

	for _, msg := range m.selectedSession.Messages {
		roleStyle := DimmedStyle
		roleLabel := msg.Role

		switch msg.Role {
		case "user":
			roleStyle = SuccessStyle
			roleLabel = "User"
		case "assistant":
			roleStyle = HighlightStyle
			roleLabel = "Assistant"
		case "system":
			roleStyle = DimmedStyle
			roleLabel = "System"
		}

		timestamp := msg.CreatedAt.Format("15:04:05")

		s.WriteString(fmt.Sprintf("%s [%s]:\n",
			roleStyle.Render(roleLabel),
			DimmedStyle.Render(timestamp),
		))

		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}

		s.WriteString(fmt.Sprintf("%s\n\n", content))

		if len(msg.ToolCalls) > 0 {
			s.WriteString(DimmedStyle.Render(fmt.Sprintf("  Tool Calls: %d\n", len(msg.ToolCalls))))
			for _, tc := range msg.ToolCalls {
				status := "✓"
				if tc.Error != "" {
					status = "✗"
				}

				s.WriteString(DimmedStyle.Render(fmt.Sprintf("    %s %s (%v)\n", status, tc.Name, tc.Duration)))
			}
			s.WriteString("\n")
		}
	}

	return s.String()
}

func (m *agentSelectorComponent) GetSelectedSession() *dashboard.ChatSession {
	return m.selectedSession
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}

	return fmt.Sprintf("%dh", int(d.Hours()))
}
