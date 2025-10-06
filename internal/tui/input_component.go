package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type inputComponent struct {
	value  string
	cursor int
}

func newInputComponent() *inputComponent {
	return &inputComponent{
		value:  "",
		cursor: 0,
	}
}

func (i *inputComponent) Update(msg tea.Msg) (*inputComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "backspace":
			if i.cursor > 0 {
				i.value = i.value[:i.cursor-1] + i.value[i.cursor:]
				i.cursor--
			}

			return i, nil

		case "left":
			if i.cursor > 0 {
				i.cursor--
			}

			return i, nil

		case "right":
			if i.cursor < len(i.value) {
				i.cursor++
			}

			return i, nil

		case "home":
			i.cursor = 0

			return i, nil

		case "end":
			i.cursor = len(i.value)

			return i, nil

		default:
			if len(msg.String()) == 1 {
				i.value = i.value[:i.cursor] + msg.String() + i.value[i.cursor:]
				i.cursor++
			}

			return i, nil
		}
	}

	return i, nil
}

func (i *inputComponent) View() string {
	if i.cursor < len(i.value) {
		return "> " + i.value[:i.cursor] + "▊" + i.value[i.cursor:]
	}

	return "> " + i.value + "▊"
}

func (i *inputComponent) Value() string {
	return i.value
}

func (i *inputComponent) Reset() {
	i.value = ""
	i.cursor = 0
}
