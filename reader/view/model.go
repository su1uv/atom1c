// Package view provides a standalone, responsive terminal viewport for article
// documents. It does not depend on the Atom1c application or its persistence.
package view

import (
	"strings"
	"unicode"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Model owns the focused reading column and its scroll position.
type Model struct {
	viewport      viewport.Model
	width, height int
}

// New creates a reader view sized for terminal dimensions. The document column
// is centered and capped at 80 cells while adapting to narrower screens.
func New(width, height int) Model {
	m := Model{viewport: viewport.New(viewport.WithWidth(ColumnWidth(width)), viewport.WithHeight(max(height-1, 0))), width: width, height: height}
	m.viewport.MouseWheelEnabled = false
	return m
}

// ColumnWidth returns the focused reading width for a terminal width.
func ColumnWidth(width int) int { return min(max(width-8, 1), 80) }

// SetSize updates terminal and viewport dimensions while preserving the current
// scroll offset, including a deliberate bottom position.
func (m *Model) SetSize(width, height int) {
	wasBottom := m.viewport.AtBottom()
	offset := m.viewport.YOffset()
	m.width, m.height = width, height
	m.viewport.SetWidth(ColumnWidth(width))
	m.viewport.SetHeight(max(height-1, 0))
	if wasBottom {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(offset)
	}
}

// SetContent replaces the already-rendered, terminal-ready Markdown document.
func (m *Model) SetContent(content string) { m.viewport.SetContent(content) }

// GetContent returns the currently rendered terminal document.
func (m Model) GetContent() string { return m.viewport.GetContent() }

// Update forwards viewport-owned key and mouse messages. Application-level
// controls such as reload or back navigation should be handled by the caller.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "home":
			m.viewport.GotoTop()
			return m, nil
		case "end":
			m.viewport.GotoBottom()
			return m, nil
		}
	}
	updated, cmd := m.viewport.Update(msg)
	m.viewport = updated
	return m, cmd
}

// View renders a centered document and a caller-supplied status line.
func (m Model) View(status string) tea.View {
	if m.width <= 0 || m.height <= 0 {
		return tea.NewView("")
	}
	parts := make([]string, 0, 2)
	if m.height > 1 {
		parts = append(parts, m.viewport.View())
	}
	parts = append(parts, ansi.Truncate(cleanStatus(status), m.width, ""))
	content := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, joinLines(parts))
	content = lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(content)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	joined := lines[0]
	for _, line := range lines[1:] {
		joined += "\n" + line
	}
	return joined
}

func cleanStatus(status string) string {
	if len(status) > 4096 {
		status = status[:4096]
	}
	status = ansi.Strip(status)
	status = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, status)
	return strings.Join(strings.Fields(status), " ")
}

func (m Model) Width() int             { return m.viewport.Width() }
func (m Model) Height() int            { return m.viewport.Height() }
func (m Model) TotalLineCount() int    { return m.viewport.TotalLineCount() }
func (m Model) YOffset() int           { return m.viewport.YOffset() }
func (m *Model) SetYOffset(offset int) { m.viewport.SetYOffset(offset) }
func (m Model) ScrollPercent() float64 { return m.viewport.ScrollPercent() }
func (m Model) AtTop() bool            { return m.viewport.AtTop() }
func (m Model) AtBottom() bool         { return m.viewport.AtBottom() }
func (m Model) PastBottom() bool       { return m.viewport.PastBottom() }
func (m *Model) ScrollDown(lines int)  { m.viewport.ScrollDown(lines) }
func (m *Model) ScrollUp(lines int)    { m.viewport.ScrollUp(lines) }
func (m *Model) PageDown()             { m.viewport.PageDown() }
func (m *Model) PageUp()               { m.viewport.PageUp() }
func (m *Model) GotoTop()              { m.viewport.GotoTop() }
func (m *Model) GotoBottom()           { m.viewport.GotoBottom() }
