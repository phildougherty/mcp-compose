package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
)

type ViewMode int

const (
	ViewModeChat ViewMode = iota
	ViewModeMCPSelector
	ViewModeAgentSelector
)

type streamChunkMsg struct {
	chunk string
}

type streamCompleteMsg struct{}

type streamErrorMsg struct {
	err error
}

type toolCallsMsg struct {
	toolCalls []dashboard.ToolCall
}

type toolResultsMsg struct {
	toolResults []dashboard.ToolCall
}

type sessionSwitchMsg struct {
	sessionID string
}

type chatModel struct {
	chatService      *dashboard.ChatService
	messages         []dashboard.ChatMessage
	currentSession   *dashboard.ChatSession
	isStreaming      bool
	streamingContent strings.Builder
	input            *inputComponent
	mcpSelector      *mcpSelectorComponent
	agentSelector    *agentSelectorComponent
	viewMode         ViewMode
	err              error
	width            int
	height           int
	ctx              context.Context
	streamChan       <-chan string
	toolCallsActive  map[string]toolCallStatus
}

type toolCallStatus struct {
	Name      string
	Status    string
	StartTime string
}

func NewChatModel(chatService *dashboard.ChatService, session *dashboard.ChatSession) *chatModel {
	ctx := context.Background()

	availableServers, err := chatService.GetAvailableMCPServers()
	if err != nil {
		availableServers = []map[string]interface{}{}
	}

	return &chatModel{
		chatService:     chatService,
		currentSession:  session,
		messages:        session.Messages,
		input:           newInputComponent(),
		mcpSelector:     newMCPSelectorComponent(availableServers, session.MCPServers),
		agentSelector:   newAgentSelectorComponent(chatService),
		viewMode:        ViewModeChat,
		ctx:             ctx,
		toolCallsActive: make(map[string]toolCallStatus),
	}
}

func (m *chatModel) Init() tea.Cmd {
	return nil
}

func (m *chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case streamChunkMsg:
		return m.handleStreamChunk(msg)

	case streamCompleteMsg:
		return m.handleStreamComplete()

	case streamErrorMsg:
		return m.handleStreamError(msg)

	case toolCallsMsg:
		return m.handleToolCalls(msg)

	case toolResultsMsg:
		return m.handleToolResults(msg)

	case sessionSwitchMsg:
		return m.handleSessionSwitch(msg)

	default:
		return m.updateSubComponents(msg)
	}
}

func (m *chatModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeChat:
		return m.handleChatKeyPress(msg)
	case ViewModeMCPSelector:
		return m.handleMCPSelectorKeyPress(msg)
	case ViewModeAgentSelector:
		return m.handleAgentSelectorKeyPress(msg)
	default:
		return m, nil
	}
}

func (m *chatModel) handleChatKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "enter":
		if m.isStreaming {
			return m, nil
		}

		userInput := strings.TrimSpace(m.input.Value())
		if userInput == "" {
			return m, nil
		}

		if strings.HasPrefix(userInput, "/mcp") {
			m.viewMode = ViewModeMCPSelector
			m.input.Reset()

			return m, nil
		}

		if strings.HasPrefix(userInput, "/agents") {
			m.viewMode = ViewModeAgentSelector
			m.agentSelector = newAgentSelectorComponent(m.chatService)
			m.input.Reset()

			return m, m.agentSelector.Init()
		}

		m.input.Reset()

		return m, m.sendMessage(userInput)

	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		return m, cmd
	}
}

func (m *chatModel) handleMCPSelectorKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.viewMode = ViewModeChat

		return m, nil

	case "enter":
		selectedServers := m.mcpSelector.GetSelectedServers()
		err := m.chatService.SetSessionMCPServers(m.currentSession.ID, selectedServers)
		if err != nil {
			m.err = fmt.Errorf("failed to update MCP servers: %w", err)
		} else {
			m.currentSession.MCPServers = selectedServers
		}

		m.viewMode = ViewModeChat

		return m, nil

	default:
		var cmd tea.Cmd
		m.mcpSelector, cmd = m.mcpSelector.Update(msg)

		return m, cmd
	}
}

func (m *chatModel) handleAgentSelectorKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		m.viewMode = ViewModeChat

		return m, nil

	default:
		var cmd tea.Cmd
		m.agentSelector, cmd = m.agentSelector.Update(msg)

		return m, cmd
	}
}

func (m *chatModel) updateSubComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m.viewMode {
	case ViewModeChat:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	case ViewModeMCPSelector:
		var cmd tea.Cmd
		m.mcpSelector, cmd = m.mcpSelector.Update(msg)
		cmds = append(cmds, cmd)
	case ViewModeAgentSelector:
		var cmd tea.Cmd
		m.agentSelector, cmd = m.agentSelector.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *chatModel) sendMessage(userInput string) tea.Cmd {
	return func() tea.Msg {
		stream := true
		responseChan, err := m.chatService.SendMessage(m.currentSession.ID, userInput, stream)
		if err != nil {
			return streamErrorMsg{err: err}
		}

		m.streamChan = responseChan

		return m.listenToStream()()
	}
}

func (m *chatModel) listenToStream() tea.Cmd {
	return func() tea.Msg {
		if m.streamChan == nil {
			return streamCompleteMsg{}
		}

		chunk, ok := <-m.streamChan
		if !ok {
			return streamCompleteMsg{}
		}

		if strings.HasPrefix(chunk, "__TOOL_CALLS__") {
			toolCallsJSON := strings.TrimPrefix(chunk, "__TOOL_CALLS__")
			var toolCalls []dashboard.ToolCall
			if err := json.Unmarshal([]byte(toolCallsJSON), &toolCalls); err == nil {
				return toolCallsMsg{toolCalls: toolCalls}
			}

			return streamChunkMsg{chunk: chunk}
		}

		if strings.HasPrefix(chunk, "__TOOL_RESULTS__") {
			toolResultsJSON := strings.TrimPrefix(chunk, "__TOOL_RESULTS__")
			var toolResults []dashboard.ToolCall
			if err := json.Unmarshal([]byte(toolResultsJSON), &toolResults); err == nil {
				return toolResultsMsg{toolResults: toolResults}
			}

			return streamChunkMsg{chunk: chunk}
		}

		return streamChunkMsg{chunk: chunk}
	}
}

func (m *chatModel) handleStreamChunk(msg streamChunkMsg) (tea.Model, tea.Cmd) {
	m.isStreaming = true
	m.streamingContent.WriteString(msg.chunk)

	return m, m.listenToStream()
}

func (m *chatModel) handleStreamComplete() (tea.Model, tea.Cmd) {
	m.isStreaming = false

	if m.streamingContent.Len() > 0 {
		assistantMsg := dashboard.ChatMessage{
			Role:    "assistant",
			Content: m.streamingContent.String(),
		}
		m.messages = append(m.messages, assistantMsg)
		m.streamingContent.Reset()
	}

	session, err := m.chatService.GetSession(m.currentSession.ID)
	if err == nil {
		m.messages = session.Messages
		m.currentSession = session
	}

	return m, nil
}

func (m *chatModel) handleStreamError(msg streamErrorMsg) (tea.Model, tea.Cmd) {
	m.isStreaming = false
	m.err = msg.err
	m.streamingContent.Reset()

	return m, nil
}

func (m *chatModel) handleToolCalls(msg toolCallsMsg) (tea.Model, tea.Cmd) {
	if m.streamingContent.Len() > 0 {
		assistantMsg := dashboard.ChatMessage{
			Role:      "assistant",
			Content:   m.streamingContent.String(),
			ToolCalls: msg.toolCalls,
		}
		m.messages = append(m.messages, assistantMsg)
		m.streamingContent.Reset()
	} else {
		if len(m.messages) > 0 {
			lastIdx := len(m.messages) - 1
			m.messages[lastIdx].ToolCalls = append(m.messages[lastIdx].ToolCalls, msg.toolCalls...)
		}
	}

	for _, toolCall := range msg.toolCalls {
		m.toolCallsActive[toolCall.ID] = toolCallStatus{
			Name:   toolCall.Name,
			Status: "running",
		}
	}

	return m, m.listenToStream()
}

func (m *chatModel) handleToolResults(msg toolResultsMsg) (tea.Model, tea.Cmd) {
	if len(m.messages) > 0 {
		lastIdx := len(m.messages) - 1
		m.messages[lastIdx].ToolResults = append(m.messages[lastIdx].ToolResults, msg.toolResults...)
	}

	for _, toolResult := range msg.toolResults {
		if _, exists := m.toolCallsActive[toolResult.ID]; exists {
			status := "completed"
			if toolResult.Error != "" {
				status = "failed"
			}

			m.toolCallsActive[toolResult.ID] = toolCallStatus{
				Name:   toolResult.Name,
				Status: status,
			}
		}
	}

	return m, m.listenToStream()
}

func (m *chatModel) handleSessionSwitch(msg sessionSwitchMsg) (tea.Model, tea.Cmd) {
	session, err := m.chatService.GetSession(msg.sessionID)
	if err != nil {
		m.err = fmt.Errorf("failed to switch session: %w", err)

		return m, nil
	}

	m.currentSession = session
	m.messages = session.Messages
	m.toolCallsActive = make(map[string]toolCallStatus)
	m.streamingContent.Reset()
	m.isStreaming = false

	return m, nil
}

