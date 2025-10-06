package tui

import "github.com/charmbracelet/lipgloss"

var (
	purple               = lipgloss.Color("#8B5CF6")
	lightPurple          = lipgloss.Color("#A78BFA")
	darkPurple           = lipgloss.Color("#6D28D9")
	blue                 = lipgloss.Color("#3B82F6")
	lightBlue            = lipgloss.Color("#60A5FA")
	darkBlue             = lipgloss.Color("#1E40AF")
	gray                 = lipgloss.Color("#6B7280")
	lightGray            = lipgloss.Color("#9CA3AF")
	darkGray             = lipgloss.Color("#374151")
	white                = lipgloss.Color("#F9FAFB")
	black                = lipgloss.Color("#111827")
	red                  = lipgloss.Color("#EF4444")
	green                = lipgloss.Color("#10B981")
	yellow               = lipgloss.Color("#F59E0B")
	orange               = lipgloss.Color("#F97316")
	userBubbleColor      = lipgloss.Color("#2A4A7F")
	assistantBubbleColor = lipgloss.Color("#2F4F2F")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			Background(black).
			Padding(0, 1).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lightPurple).
			Italic(true).
			MarginBottom(1)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(purple).
			BorderBottom(true).
			Padding(0, 1)

	SubHeaderStyle = lipgloss.NewStyle().
			Foreground(lightBlue).
			Bold(true)

	UserMessageStyle = lipgloss.NewStyle().
				Background(userBubbleColor).
				Foreground(white).
				Padding(0, 1).
				MarginTop(1).
				MarginBottom(1).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(userBubbleColor)

	AssistantMessageStyle = lipgloss.NewStyle().
				Background(assistantBubbleColor).
				Foreground(white).
				Padding(0, 1).
				MarginTop(1).
				MarginBottom(1).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(assistantBubbleColor)

	SystemMessageStyle = lipgloss.NewStyle().
				Foreground(gray).
				Italic(true).
				MarginTop(1).
				MarginBottom(1)

	ToolCallHeaderStyle = lipgloss.NewStyle().
				Foreground(yellow).
				Bold(true)

	ToolCallBodyStyle = lipgloss.NewStyle().
				Foreground(gray).
				PaddingLeft(2)

	ToolCallStyle = lipgloss.NewStyle().
			Foreground(lightPurple).
			Background(black).
			Padding(1, 2).
			MarginBottom(1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Italic(true)

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(lightGray).
			Background(darkGray).
			Padding(1, 2).
			MarginBottom(1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(gray)

	ToolCallSuccessStyle = lipgloss.NewStyle().
				Foreground(green)

	ToolCallErrorStyle = lipgloss.NewStyle().
				Foreground(red)

	ToolCallRunningStyle = lipgloss.NewStyle().
				Foreground(yellow)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(green).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(yellow).
			Bold(true)

	InputBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(purple).
			Padding(0, 1)

	InputBoxFocusedStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lightBlue).
				Padding(0, 1)

	InputPromptStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true).
				MarginRight(1)

	BorderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(gray).
			Padding(1, 2)

	ThickBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(purple).
				Padding(1, 2)

	PanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(gray).
			Padding(1, 2).
			MarginBottom(1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(gray).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(gray).
			BorderTop(true).
			Padding(0, 1)

	StatusRunningStyle = lipgloss.NewStyle().
				Foreground(blue).
				Bold(true)

	StatusSuccessStyle = lipgloss.NewStyle().
				Foreground(green).
				Bold(true)

	StatusErrorStyle = lipgloss.NewStyle().
				Foreground(red).
				Bold(true)

	StatusIdleStyle = lipgloss.NewStyle().
			Foreground(gray)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(purple)

	KeyStyle = lipgloss.NewStyle().
			Foreground(lightPurple).
			Bold(true)

	ValueStyle = lipgloss.NewStyle().
			Foreground(lightGray)

	HelpStyle = lipgloss.NewStyle().
			Foreground(gray).
			Italic(true).
			MarginTop(1)

	DimmedStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(lightPurple).
			Bold(true)

	CodeStyle = lipgloss.NewStyle().
			Foreground(lightBlue).
			Background(black).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(darkBlue)

	CodeBlockStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2A2A2A")).
			Foreground(lipgloss.Color("#D4D4D4")).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(gray)

	ListItemStyle = lipgloss.NewStyle().
			Foreground(white).
			MarginLeft(2)

	ListItemSelectedStyle = lipgloss.NewStyle().
				Foreground(lightPurple).
				Bold(true).
				MarginLeft(2)

	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(darkPurple).
				Bold(true).
				Padding(0, 1).
				Align(lipgloss.Center)

	TableCellStyle = lipgloss.NewStyle().
			Foreground(lightGray).
			Padding(0, 1)

	TableCellSelectedStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(darkGray).
				Bold(true).
				Padding(0, 1)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(purple).
				Background(darkGray)

	ProgressBarFilledStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(purple)

	BadgeStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(purple).
			Padding(0, 1).
			Bold(true)

	BadgeSuccessStyle = lipgloss.NewStyle().
				Foreground(white).
				Background(green).
				Padding(0, 1).
				Bold(true)

	BadgeErrorStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(red).
			Padding(0, 1).
			Bold(true)

	BadgeWarningStyle = lipgloss.NewStyle().
				Foreground(black).
				Background(yellow).
				Padding(0, 1).
				Bold(true)

	BadgeInfoStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(blue).
			Padding(0, 1).
			Bold(true)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(gray).
			Faint(true)

	LinkStyle = lipgloss.NewStyle().
			Foreground(blue).
			Underline(true)

	TypingIndicatorStyle = lipgloss.NewStyle().
				Foreground(gray).
				Italic(true)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	ItalicStyle = lipgloss.NewStyle().
			Italic(true)

	ExpandIndicatorStyle = lipgloss.NewStyle().
				Foreground(purple).
				Bold(true)
)

func RenderTitle(text string) string {
	return TitleStyle.Render(text)
}

func RenderSubtitle(text string) string {
	return SubtitleStyle.Render(text)
}

func RenderHeader(text string) string {
	return HeaderStyle.Render(text)
}

func RenderUserMessage(text string, width int) string {
	return UserMessageStyle.Width(width).Render(text)
}

func RenderAssistantMessage(text string, width int) string {
	return AssistantMessageStyle.Width(width).Render(text)
}

func RenderSystemMessage(text string, width int) string {
	return SystemMessageStyle.Width(width).Render(text)
}

func RenderToolCall(name string, args string) string {
	return ToolCallStyle.Render("Tool: " + name + "\nArgs: " + args)
}

func RenderToolResult(result string) string {
	return ToolResultStyle.Render(result)
}

func RenderError(text string) string {
	return ErrorStyle.Render("Error: " + text)
}

func RenderSuccess(text string) string {
	return SuccessStyle.Render("Success: " + text)
}

func RenderWarning(text string) string {
	return WarningStyle.Render("Warning: " + text)
}

func RenderInputBox(text string, focused bool) string {
	if focused {
		return InputBoxFocusedStyle.Render(text)
	}

	return InputBoxStyle.Render(text)
}

func RenderInputPrompt(text string) string {
	return InputPromptStyle.Render(text)
}

func RenderPanel(text string, width int) string {
	return PanelStyle.Width(width).Render(text)
}

func RenderStatusRunning(text string) string {
	return StatusRunningStyle.Render(text)
}

func RenderStatusSuccess(text string) string {
	return StatusSuccessStyle.Render(text)
}

func RenderStatusError(text string) string {
	return StatusErrorStyle.Render(text)
}

func RenderStatusIdle(text string) string {
	return StatusIdleStyle.Render(text)
}

func RenderKey(text string) string {
	return KeyStyle.Render(text)
}

func RenderValue(text string) string {
	return ValueStyle.Render(text)
}

func RenderKeyValue(key, value string) string {
	return RenderKey(key) + ": " + RenderValue(value)
}

func RenderHelp(text string) string {
	return HelpStyle.Render(text)
}

func RenderDimmed(text string) string {
	return DimmedStyle.Render(text)
}

func RenderHighlight(text string) string {
	return HighlightStyle.Render(text)
}

func RenderCode(text string) string {
	return CodeStyle.Render(text)
}

func RenderCodeBlock(text string, width int) string {
	return CodeBlockStyle.Width(width).Render(text)
}

func RenderListItem(text string, selected bool) string {
	if selected {
		return ListItemSelectedStyle.Render("▸ " + text)
	}

	return ListItemStyle.Render("  " + text)
}

func RenderBadge(text string) string {
	return BadgeStyle.Render(text)
}

func RenderBadgeSuccess(text string) string {
	return BadgeSuccessStyle.Render(text)
}

func RenderBadgeError(text string) string {
	return BadgeErrorStyle.Render(text)
}

func RenderBadgeWarning(text string) string {
	return BadgeWarningStyle.Render(text)
}

func RenderBadgeInfo(text string) string {
	return BadgeInfoStyle.Render(text)
}

func RenderSeparator(width int) string {
	return SeparatorStyle.Render(lipgloss.NewStyle().Width(width).Render("─"))
}

func RenderTimestamp(text string) string {
	return TimestampStyle.Render(text)
}

func RenderLink(text string) string {
	return LinkStyle.Render(text)
}

func RenderProgressBar(filled, total, width int) string {
	if total == 0 {
		return ""
	}

	filledWidth := (filled * width) / total
	emptyWidth := width - filledWidth

	filledBar := ProgressBarFilledStyle.Render(lipgloss.NewStyle().Width(filledWidth).Render("█"))
	emptyBar := ProgressBarStyle.Render(lipgloss.NewStyle().Width(emptyWidth).Render("░"))

	return filledBar + emptyBar
}

func RenderBox(content string, width, height int) string {
	return BorderStyle.
		Width(width).
		Height(height).
		Render(content)
}

func RenderThickBox(content string, width, height int) string {
	return ThickBorderStyle.
		Width(width).
		Height(height).
		Render(content)
}

func RenderTable(headers []string, rows [][]string, selectedRow int) string {
	var table string

	headerRow := ""
	for _, header := range headers {
		headerRow += TableHeaderStyle.Render(header) + " "
	}
	table += headerRow + "\n"

	for i, row := range rows {
		rowStr := ""
		for _, cell := range row {
			if i == selectedRow {
				rowStr += TableCellSelectedStyle.Render(cell) + " "
			} else {
				rowStr += TableCellStyle.Render(cell) + " "
			}
		}
		table += rowStr + "\n"
	}

	return table
}

func JoinHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func JoinVertical(top, bottom string) string {
	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func AlignLeft(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).Render(text)
}

func AlignCenter(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}

func AlignRight(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(text)
}
