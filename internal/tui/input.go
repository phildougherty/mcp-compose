package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	maxInputHeight = 10
)

type CommandType int

const (
	CommandNone CommandType = iota
	CommandMCP
	CommandHelp
	CommandClear
	CommandSessions
)

type InputModel struct {
	content       []string
	cursorLine    int
	cursorCol     int
	width         int
	height        int
	scrollOffset  int
	focused       bool
	maxHeight     int
	commandType   CommandType
	borderStyle   lipgloss.Style
	textStyle     lipgloss.Style
	cursorStyle   lipgloss.Style
	placeholderStyle lipgloss.Style
	placeholder   string
}

func NewInputModel() InputModel {
	return InputModel{
		content:      []string{""},
		cursorLine:   0,
		cursorCol:    0,
		width:        80,
		height:       1,
		scrollOffset: 0,
		focused:      true,
		maxHeight:    maxInputHeight,
		commandType:  CommandNone,
		placeholder:  "Type your message... (Enter to send, Ctrl+J for newline)",
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),
		textStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
		cursorStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("86")),
		placeholderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

func (m *InputModel) Focus() {
	m.focused = true
}

func (m *InputModel) Blur() {
	m.focused = false
}

func (m *InputModel) SetWidth(width int) {
	m.width = width
}

func (m *InputModel) SetPlaceholder(placeholder string) {
	m.placeholder = placeholder
}

func (m *InputModel) Value() string {
	return strings.Join(m.content, "\n")
}

func (m *InputModel) CommandType() CommandType {
	return m.commandType
}

func (m *InputModel) Clear() {
	m.content = []string{""}
	m.cursorLine = 0
	m.cursorCol = 0
	m.scrollOffset = 0
	m.height = 1
	m.commandType = CommandNone
}

func (m *InputModel) detectCommand() {
	if len(m.content) == 0 || len(m.content[0]) == 0 {
		m.commandType = CommandNone

		return
	}

	firstLine := strings.TrimSpace(m.content[0])
	if !strings.HasPrefix(firstLine, "/") {
		m.commandType = CommandNone

		return
	}

	switch firstLine {
	case "/mcp":
		m.commandType = CommandMCP
	case "/help":
		m.commandType = CommandHelp
	case "/clear":
		m.commandType = CommandClear
	case "/sessions":
		m.commandType = CommandSessions
	default:
		m.commandType = CommandNone
	}
}

func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			m.detectCommand()

			return m, nil

		case "ctrl+j":
			m.insertNewline()

		case "backspace":
			m.deleteChar()

		case "delete":
			m.deleteCharForward()

		case "left":
			m.moveCursorLeft()

		case "right":
			m.moveCursorRight()

		case "up":
			m.moveCursorUp()

		case "down":
			m.moveCursorDown()

		case "home":
			m.cursorCol = 0

		case "end":
			m.cursorCol = len(m.content[m.cursorLine])

		case "ctrl+a":
			m.cursorCol = 0

		case "ctrl+e":
			m.cursorCol = len(m.content[m.cursorLine])

		case "ctrl+u":
			m.content[m.cursorLine] = m.content[m.cursorLine][m.cursorCol:]
			m.cursorCol = 0

		case "ctrl+k":
			m.content[m.cursorLine] = m.content[m.cursorLine][:m.cursorCol]

		default:
			if len(msg.Runes) > 0 {
				m.insertRune(msg.Runes[0])
			}
		}

		m.updateHeight()
		m.updateScroll()
	}

	return m, nil
}

func (m *InputModel) insertRune(r rune) {
	line := m.content[m.cursorLine]
	before := line[:m.cursorCol]
	after := line[m.cursorCol:]
	m.content[m.cursorLine] = before + string(r) + after
	m.cursorCol++
}

func (m *InputModel) insertNewline() {
	line := m.content[m.cursorLine]
	before := line[:m.cursorCol]
	after := line[m.cursorCol:]

	m.content[m.cursorLine] = before
	m.content = append(m.content[:m.cursorLine+1], append([]string{after}, m.content[m.cursorLine+1:]...)...)

	m.cursorLine++
	m.cursorCol = 0
}

func (m *InputModel) deleteChar() {
	if m.cursorCol > 0 {
		line := m.content[m.cursorLine]
		m.content[m.cursorLine] = line[:m.cursorCol-1] + line[m.cursorCol:]
		m.cursorCol--
	} else if m.cursorLine > 0 {
		prevLine := m.content[m.cursorLine-1]
		currentLine := m.content[m.cursorLine]
		m.content[m.cursorLine-1] = prevLine + currentLine
		m.content = append(m.content[:m.cursorLine], m.content[m.cursorLine+1:]...)
		m.cursorLine--
		m.cursorCol = len(prevLine)
	}
}

func (m *InputModel) deleteCharForward() {
	line := m.content[m.cursorLine]
	if m.cursorCol < len(line) {
		m.content[m.cursorLine] = line[:m.cursorCol] + line[m.cursorCol+1:]
	} else if m.cursorLine < len(m.content)-1 {
		nextLine := m.content[m.cursorLine+1]
		m.content[m.cursorLine] = line + nextLine
		m.content = append(m.content[:m.cursorLine+1], m.content[m.cursorLine+2:]...)
	}
}

func (m *InputModel) moveCursorLeft() {
	if m.cursorCol > 0 {
		m.cursorCol--
	} else if m.cursorLine > 0 {
		m.cursorLine--
		m.cursorCol = len(m.content[m.cursorLine])
	}
}

func (m *InputModel) moveCursorRight() {
	line := m.content[m.cursorLine]
	if m.cursorCol < len(line) {
		m.cursorCol++
	} else if m.cursorLine < len(m.content)-1 {
		m.cursorLine++
		m.cursorCol = 0
	}
}

func (m *InputModel) moveCursorUp() {
	if m.cursorLine > 0 {
		m.cursorLine--
		if m.cursorCol > len(m.content[m.cursorLine]) {
			m.cursorCol = len(m.content[m.cursorLine])
		}
	}
}

func (m *InputModel) moveCursorDown() {
	if m.cursorLine < len(m.content)-1 {
		m.cursorLine++
		if m.cursorCol > len(m.content[m.cursorLine]) {
			m.cursorCol = len(m.content[m.cursorLine])
		}
	}
}

func (m *InputModel) updateHeight() {
	m.height = len(m.content)
	if m.height > m.maxHeight {
		m.height = m.maxHeight
	}
}

func (m *InputModel) updateScroll() {
	if m.cursorLine < m.scrollOffset {
		m.scrollOffset = m.cursorLine
	} else if m.cursorLine >= m.scrollOffset+m.height {
		m.scrollOffset = m.cursorLine - m.height + 1
	}
}

func (m InputModel) View() string {
	if !m.focused {
		return m.borderStyle.Render("")
	}

	var lines []string

	if len(m.content) == 1 && len(m.content[0]) == 0 {
		lines = append(lines, m.placeholderStyle.Render(m.placeholder))
	} else {
		visibleEnd := m.scrollOffset + m.height
		if visibleEnd > len(m.content) {
			visibleEnd = len(m.content)
		}

		for i := m.scrollOffset; i < visibleEnd; i++ {
			line := m.content[i]

			if i == m.cursorLine && m.focused {
				before := line[:m.cursorCol]
				cursor := "█"
				after := ""
				if m.cursorCol < len(line) {
					after = line[m.cursorCol:]
				}

				renderedLine := m.textStyle.Render(before) +
					m.cursorStyle.Render(cursor) +
					m.textStyle.Render(after)
				lines = append(lines, renderedLine)
			} else {
				lines = append(lines, m.textStyle.Render(line))
			}
		}
	}

	content := strings.Join(lines, "\n")

	borderStyle := m.borderStyle
	if m.commandType != CommandNone {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("86"))
	}

	return borderStyle.
		Width(m.width - 4).
		Padding(0, 1).
		Render(content)
}
