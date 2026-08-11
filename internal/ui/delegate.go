package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func NewItemDelegate(keys *DelegateKeyMap, styles *Styles) list.DefaultDelegate {
	d := list.NewDefaultDelegate()

	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		var title string

		if i, ok := m.SelectedItem().(feed); ok {
			title = i.Title()
		} else {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch {
			case key.Matches(msg, keys.Choose):
				return m.NewStatusMessage(styles.StatusMessage.Render("You choose " + title))

			case key.Matches(msg, keys.Remove):
				index := m.Index()
				m.RemoveItem(index)
				if len(m.Items()) == 0 {
					keys.Remove.SetEnabled(false)
				}
				return m.NewStatusMessage(styles.StatusMessage.Render("Deleted " + title))
			}
		}
		return nil
	}

	help := []key.Binding{keys.Choose, keys.Remove}
	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

type DelegateKeyMap struct {
	Choose key.Binding
	Remove key.Binding
}

func NewDelegateKeyMap() *DelegateKeyMap {
	return &DelegateKeyMap{
		Choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
		Remove: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete"),
		),
	}
}
