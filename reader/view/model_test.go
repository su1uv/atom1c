package view

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestReaderViewOwnsFocusedColumnScrollingAndResize(t *testing.T) {
	m := New(100, 24)
	if m.Width() != 80 || m.Height() != 23 {
		t.Fatalf("initial viewport = %dx%d", m.Width(), m.Height())
	}
	m.SetContent(strings.Repeat("Reading line\n", 100))
	m.ScrollDown(3)
	if m.YOffset() != 3 {
		t.Fatalf("offset = %d", m.YOffset())
	}
	m.SetSize(40, 10)
	if m.Width() != 32 || m.Height() != 9 || m.YOffset() != 3 {
		t.Fatalf("resized viewport = %dx%d offset=%d", m.Width(), m.Height(), m.YOffset())
	}
	view := m.View("reader controls")
	if got := lipgloss.Width(view.Content); got > 40 {
		t.Fatalf("view width %d exceeds 40", got)
	}
	if got := lipgloss.Height(view.Content); got > 10 {
		t.Fatalf("view height %d exceeds 10", got)
	}
	m.GotoBottom()
	if !m.AtBottom() {
		t.Fatal("End position not tracked")
	}
	m.SetSize(60, 15)
	if !m.AtBottom() {
		t.Fatal("resize did not retain bottom position")
	}
}

func TestReaderViewScrollMessages(t *testing.T) {
	m := New(60, 12)
	m.SetContent(strings.Repeat("line\n", 50))
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if m.YOffset() != 1 {
		t.Fatal("j did not scroll")
	}
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	if m.YOffset() <= 1 {
		t.Fatal("PgDown did not page")
	}
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	if !m.AtTop() {
		t.Fatal("Home did not return to top")
	}
}

func TestReaderStatusCannotInjectTerminalControlSequences(t *testing.T) {
	m := New(80, 20)
	view := m.View("status\x1b]52;c;steal\a \x1b[2J ready")
	if strings.Contains(view.Content, "\x1b]52") || strings.Contains(view.Content, "\x1b[2J") || strings.Contains(view.Content, "steal") {
		t.Fatalf("status contains terminal control payload: %q", view.Content)
	}
}
