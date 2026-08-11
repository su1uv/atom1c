package ui

type item struct {
	title       string
	description string
}

func GenerateItem() item {
	return item{
		title:       "New item",
		description: "This is a new test item.",
	}
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.description }
func (i item) FilterValue() string { return i.title }
