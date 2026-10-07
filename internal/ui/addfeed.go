package ui

import (
	"fmt"
	"net/url"
	"strings"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/su1uv/atom1c/internal/handlers"
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
		t.SetVirtualCursor(false)

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
	err         string
	saving      bool
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
	if m.saving {
		return m, nil, addFeedNoAction
	}
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
				name := strings.TrimSpace(m.inputs[0].Value())
				feedURL := strings.TrimSpace(m.inputs[1].Value())
				if validationError, invalidInput := validateFeedDraft(name, feedURL); validationError != "" {
					m.err = validationError
					m.focusIndex = invalidInput
					return m, m.focusInput(), addFeedNoAction
				}
				m.inputs[0].SetValue(name)
				m.inputs[1].SetValue(feedURL)
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
			m.err = ""
			return m, cmd, addFeedNoAction
		}
	}

	oldValues := make([]string, len(m.inputs))
	for i := range m.inputs {
		oldValues[i] = m.inputs[i].Value()
	}
	cmd := m.updateInputs(msg)
	for i := range m.inputs {
		if m.inputs[i].Value() != oldValues[i] {
			m.err = ""
			break
		}
	}
	return m, cmd, addFeedNoAction
}

func validateFeedDraft(name, feedURL string) (string, int) {
	if strings.TrimSpace(name) == "" {
		return "Feed name is required.", 0
	}
	parsed, err := url.Parse(strings.TrimSpace(feedURL))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
		return "Enter an absolute HTTP or HTTPS URL.", 1
	}
	return "", 0
}

func (m addFeedModel) submission() handlers.AddFeedParams {
	return handlers.AddFeedParams{
		Name: strings.TrimSpace(m.inputs[0].Value()),
		URL:  strings.TrimSpace(m.inputs[1].Value()),
	}
}

func (m *addFeedModel) reset() {
	for i := range m.inputs {
		m.inputs[i].SetValue("")
		m.inputs[i].Blur()
	}
	m.focusIndex = 0
	m.err = ""
	m.saving = false
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
				c.X += 1
				c.Y += 1
			}
		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs) && !m.saving {
		button = &focusedButton
	}
	if m.saving {
		fmt.Fprintf(&b, "\n\n%s\n\n", focusedStyle.Render("[ Saving… ]"))
	} else {
		fmt.Fprintf(&b, "\n\n%s\n\n", *button)
	}
	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.err))
		b.WriteString("\n\n")
	}

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
