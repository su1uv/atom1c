package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle           = blurredStyle
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	focusedButton = focusedStyle.Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

func initialAddFeedModel(style lipgloss.Style) addFeedModel {
	m := addFeedModel{
		inputs:      make([]textinput.Model, 2),
		modalKeyMap: newModalKeyMap(),
		style:       style,
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.CharLimit = 250

		s := t.Styles()
		s.Cursor.Color = lipgloss.Color("205")
		s.Focused.Prompt = focusedStyle
		s.Focused.Text = focusedStyle
		s.Blurred.Prompt = blurredStyle
		s.Blurred.Text = blurredStyle
		t.SetStyles(s)
		t.SetWidth(30)

		switch i {
		case 0:
			t.Placeholder = "FeedName"
		case 1:
			t.Placeholder = "FeedURL"
		}

		m.inputs[i] = t
	}

	return m
}

type addFeedModel struct {
	modalKeyMap *modalKeyMap
	focusIndex  int
	inputs      []textinput.Model
	cursorMode  cursor.Mode
	style       lipgloss.Style
}

type addFeedAction int

const (
	addFeedNoAction addFeedAction = iota
	addFeedClose
	addFeedSubmit
)

func (m addFeedModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *addFeedModel) open() tea.Cmd {
	m.focusIndex = 0
	cmd := m.inputs[0].Focus()
	for i := 1; i < len(m.inputs); i++ {
		m.inputs[i].Blur()
	}
	return cmd
}

func (m addFeedModel) Update(msg tea.Msg) (addFeedModel, tea.Cmd, addFeedAction) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.modalKeyMap.close):
			m.blurInputs()
			return m, nil, addFeedClose

		case key.Matches(msg, m.modalKeyMap.changeCursorMode):
			m.cursorMode++
			if m.cursorMode > cursor.CursorHide {
				m.cursorMode = cursor.CursorBlink
			}
			for i := range m.inputs {
				s := m.inputs[i].Styles()
				s.Cursor.Blink = m.cursorMode == cursor.CursorBlink
				m.inputs[i].SetStyles(s)
			}
			return m, nil, addFeedNoAction

		case key.Matches(msg, m.modalKeyMap.nextInput):
			s := msg.String()

			if s == "enter" && m.focusIndex == len(m.inputs) {
				// TODO: Create new feed
				m.blurInputs()
				return m, nil, addFeedSubmit
			}

			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmd := m.focusInput()
			return m, cmd, addFeedNoAction
		}
	}

	return m, m.updateInputs(msg), addFeedNoAction
}

func (m *addFeedModel) focusInput() tea.Cmd {
	var cmd tea.Cmd
	for i := range m.inputs {
		if i == m.focusIndex {
			cmd = m.inputs[i].Focus()
			continue
		}
		m.inputs[i].Blur()
	}
	return cmd
}

func (m *addFeedModel) blurInputs() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
}

func (m *addFeedModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m addFeedModel) View() tea.View {
	var b strings.Builder
	var c *tea.Cursor

	for i, in := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
		if m.cursorMode != cursor.CursorHide && in.Focused() {
			c = in.Cursor()
			if c != nil {
				c.Y += 1
			}
		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	fmt.Fprintf(&b, "\n\n%s\n\n", *button)

	b.WriteString(helpStyle.Render("cursor mode is "))
	b.WriteString(cursorModeHelpStyle.Render(m.cursorMode.String()))
	b.WriteString(helpStyle.Render(" (ctrl+r to change style)"))

	v := tea.NewView(m.style.Render(b.String()))
	v.Cursor = c
	return v
}

func overlayModal(bg, modal string, w, h int) string {
	mw, mh := lipgloss.Width(modal), lipgloss.Height(modal)
	x := max((w-mw)/2, 0)
	y := max((h-mh)/2, 0)

	baseLayer := lipgloss.NewLayer(bg).X(0).Y(0).Z(0)
	modalLayer := lipgloss.NewLayer(modal).X(x).Y(y).Z(1)

	return lipgloss.NewCompositor(baseLayer, modalLayer).Render()
}
