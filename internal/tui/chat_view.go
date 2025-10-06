package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/dashboard"
)

func (m *chatModel) View() string {
	switch m.viewMode {
	case ViewModeChat:
		return m.renderChatView()
	case ViewModeMCPSelector:
		return m.mcpSelector.View()
	case ViewModeAgentSelector:
		return m.agentSelector.View()
	default:
		return ""
	}
}

func (m *chatModel) renderChatView() string {
	if m.err != nil {
		return RenderError(fmt.Sprintf("%v\n\nPress Esc to dismiss", m.err))
	}

	var s strings.Builder

	s.WriteString(m.renderHeader())
	s.WriteString("\n\n")
	s.WriteString(m.renderMessages())
	s.WriteString("\n")
	s.WriteString(m.renderInput())
	s.WriteString("\n")
	s.WriteString(m.renderFooter())

	return s.String()
}

func (m *chatModel) renderHeader() string {
	title := "New Chat"
	if m.currentSession != nil && m.currentSession.Title != "" {
		title = m.currentSession.Title
	}

	mcpCount := 0
	if m.currentSession != nil {
		mcpCount = len(m.currentSession.MCPServers)
	}

	headerText := fmt.Sprintf("  %s  ", title)

	serverInfo := ""
	if mcpCount > 0 {
		serverInfo = fmt.Sprintf("%d MCP Server%s", mcpCount, pluralize(mcpCount))
	} else {
		serverInfo = "No MCP Servers"
	}

	header := HeaderStyle.Width(m.width).Render(headerText)
	serverLine := SubHeaderStyle.Render(fmt.Sprintf("  %s  ", serverInfo))

	return header + "\n" + serverLine
}

func (m *chatModel) renderMessages() string {
	if len(m.messages) == 0 && !m.isStreaming {
		return m.renderEmptyState()
	}

	var msgs strings.Builder
	maxWidth := min(m.width-4, 76)

	for i, msg := range m.messages {
		rendered := m.renderMessage(msg, maxWidth)
		msgs.WriteString(rendered)

		if i < len(m.messages)-1 {
			nextMsg := m.messages[i+1]
			if msg.Role == "assistant" && nextMsg.Role == "assistant" {
				msgs.WriteString("\n\n")
			} else {
				msgs.WriteString("\n")
			}
		}
	}

	if m.isStreaming {
		if msgs.Len() > 0 {
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
				msgs.WriteString("\n\n")
			} else {
				msgs.WriteString("\n")
			}
		}
		msgs.WriteString(m.renderStreamingMessage(maxWidth))
	}

	return msgs.String()
}

func (m *chatModel) renderMessage(msg dashboard.ChatMessage, maxWidth int) string {
	switch msg.Role {
	case "user":
		return m.renderUserMessage(msg, maxWidth)
	case "assistant":
		return m.renderAssistantMessage(msg, maxWidth)
	case "system":
		return m.renderSystemMessage(msg, maxWidth)
	default:
		return ""
	}
}

func (m *chatModel) renderUserMessage(msg dashboard.ChatMessage, maxWidth int) string {
	content := m.renderMarkdown(msg.Content, maxWidth)
	wrapped := wrapText(content, maxWidth-6)

	return RenderUserMessage(fmt.Sprintf("You: %s", wrapped), maxWidth)
}

func (m *chatModel) renderAssistantMessage(msg dashboard.ChatMessage, maxWidth int) string {
	var parts []string

	if msg.Content != "" {
		content := m.renderMarkdown(msg.Content, maxWidth)
		wrapped := wrapText(content, maxWidth-6)
		parts = append(parts, RenderAssistantMessage(fmt.Sprintf("Assistant: %s", wrapped), maxWidth))
	}

	if len(msg.ToolCalls) > 0 {
		parts = append(parts, m.renderToolCalls(msg.ToolCalls))
	}

	if len(msg.ToolResults) > 0 {
		parts = append(parts, m.renderToolResults(msg.ToolResults))
	}

	return strings.Join(parts, "\n")
}

func (m *chatModel) renderStreamingMessage(maxWidth int) string {
	content := m.renderMarkdown(m.streamingContent.String(), maxWidth)
	wrapped := wrapText(content, maxWidth-6)
	indicator := TypingIndicatorStyle.Render(" ▊")

	return RenderAssistantMessage(fmt.Sprintf("Assistant: %s%s", wrapped, indicator), maxWidth)
}

func (m *chatModel) renderSystemMessage(msg dashboard.ChatMessage, maxWidth int) string {
	wrapped := wrapText(msg.Content, maxWidth-4)

	return RenderSystemMessage(fmt.Sprintf("System: %s", wrapped), maxWidth)
}

func (m *chatModel) renderToolCalls(toolCalls []dashboard.ToolCall) string {
	var parts []string

	for _, tc := range toolCalls {
		indicator := "▸"

		header := ToolCallHeaderStyle.Render(fmt.Sprintf("%s Tool: %s", indicator, tc.Name))
		argsStr := formatArgs(tc.Args)

		if argsStr != "" {
			body := ToolCallBodyStyle.Render("    " + argsStr)
			parts = append(parts, header+"\n"+body)
		} else {
			parts = append(parts, header)
		}
	}

	return strings.Join(parts, "\n")
}

func (m *chatModel) renderToolResults(toolResults []dashboard.ToolCall) string {
	var parts []string

	for _, tr := range toolResults {
		var status, icon, content string

		if tr.Error != "" {
			icon = "✗"
			status = ToolCallErrorStyle.Render(icon)
			content = ToolCallErrorStyle.Render(fmt.Sprintf("  Error: %s", truncate(tr.Error, 500)))
		} else if tr.Result != "" {
			icon = "✓"
			status = ToolCallSuccessStyle.Render(icon)
			content = ToolCallBodyStyle.Render("    " + truncate(tr.Result, 1000))
		} else {
			icon = "○"
			status = ToolCallRunningStyle.Render(icon)
			content = ToolCallRunningStyle.Render("  Running...")
		}

		duration := ""
		if tr.Duration > 0 {
			duration = fmt.Sprintf(" (%s)", tr.Duration.Round(time.Millisecond))
		}

		toolLine := fmt.Sprintf("%s %s%s", status, tr.Name, duration)

		if content != "" {
			parts = append(parts, toolLine+"\n"+content)
		} else {
			parts = append(parts, toolLine)
		}
	}

	return strings.Join(parts, "\n")
}

func (m *chatModel) renderMarkdown(content string, maxWidth int) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	inCodeBlock := false
	var codeLines []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				codeBlock := RenderCodeBlock(strings.Join(codeLines, "\n"), maxWidth-4)
				result.WriteString(codeBlock)
				result.WriteString("\n")
				codeLines = nil
				inCodeBlock = false
			} else {
				inCodeBlock = true
				lang := strings.TrimPrefix(line, "```")
				if lang != "" {
					codeLines = append(codeLines, fmt.Sprintf("// %s", lang))
				}
			}

			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)

			continue
		}

		line = m.renderInlineStyles(line)
		result.WriteString(line)
		result.WriteString("\n")
	}

	if inCodeBlock && len(codeLines) > 0 {
		codeBlock := RenderCodeBlock(strings.Join(codeLines, "\n"), maxWidth-4)
		result.WriteString(codeBlock)
	}

	return strings.TrimRight(result.String(), "\n")
}

func (m *chatModel) renderInlineStyles(line string) string {
	line = renderBold(line)
	line = renderItalic(line)

	return line
}

func renderBold(text string) string {
	for {
		start := strings.Index(text, "**")
		if start == -1 {
			break
		}

		end := strings.Index(text[start+2:], "**")
		if end == -1 {
			break
		}

		end += start + 2
		boldText := text[start+2 : end]
		replacement := BoldStyle.Render(boldText)
		text = text[:start] + replacement + text[end+2:]
	}

	return text
}

func renderItalic(text string) string {
	parts := strings.Split(text, "*")
	if len(parts) < 3 {
		return text
	}

	var result strings.Builder
	inItalic := false

	for i, part := range parts {
		if i == 0 {
			result.WriteString(part)

			continue
		}

		if strings.HasPrefix(parts[i-1], "*") || strings.HasSuffix(parts[i-1], "*") {
			result.WriteString("*")
			result.WriteString(part)

			continue
		}

		if !inItalic {
			inItalic = true
			result.WriteString(ItalicStyle.Render(part))
		} else {
			inItalic = false
			result.WriteString(part)
		}
	}

	return result.String()
}

func (m *chatModel) renderInput() string {
	displayText := m.input.Value()
	if displayText == "" {
		displayText = TypingIndicatorStyle.Render("Type your message... (/mcp, /agents)")
	}

	focused := !m.isStreaming

	return RenderInputBox(displayText, focused)
}

func (m *chatModel) renderFooter() string {
	shortcuts := []string{
		"ctrl+c/esc: quit",
		"enter: send",
		"/mcp: configure servers",
		"/agents: view sessions",
	}

	footer := strings.Join(shortcuts, " • ")

	return FooterStyle.Width(m.width).Render(footer)
}

func (m *chatModel) renderEmptyState() string {
	empty := `
Welcome to MCP-Compose Chat!

Start a conversation to interact with your MCP servers.
Type a message below and press Enter to begin.

Use /mcp to configure which MCP servers are available for this session.
Use /agents to view all chat sessions and their outputs.
`

	return SystemMessageStyle.Render(strings.TrimSpace(empty))
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine []string
	currentLength := 0

	for _, word := range words {
		wordLen := len(stripANSI(word))

		if currentLength+wordLen+len(currentLine) > width {
			if len(currentLine) > 0 {
				lines = append(lines, strings.Join(currentLine, " "))
				currentLine = nil
				currentLength = 0
			}

			if wordLen > width {
				lines = append(lines, word)

				continue
			}
		}

		currentLine = append(currentLine, word)
		currentLength += wordLen
	}

	if len(currentLine) > 0 {
		lines = append(lines, strings.Join(currentLine, " "))
	}

	return strings.Join(lines, "\n")
}

func stripANSI(str string) string {
	result := ""
	inEscape := false

	for _, char := range str {
		if char == '\x1b' {
			inEscape = true

			continue
		}

		if inEscape {
			if char == 'm' {
				inEscape = false
			}

			continue
		}

		result += string(char)
	}

	return result
}

func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}

	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("  %s: %v", k, v))
	}

	return strings.Join(parts, "\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}

func pluralize(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}
