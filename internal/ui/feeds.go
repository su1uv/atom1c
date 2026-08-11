package ui

import "github.com/su1uv/atom1c/internal/db"

type feed struct {
	title       string
	description string
}

func GenerateFeeds(dbFeed db.Feed) feed {
	return feed{
		title:       dbFeed.Name,
		description: dbFeed.Url,
	}
}

func (f feed) Title() string       { return f.title }
func (f feed) Description() string { return f.description }
func (f feed) FilterValue() string { return f.title }
