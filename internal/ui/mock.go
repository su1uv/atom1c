package ui

var feedsMock = []item{
	{name: "The Go Blog", url: "https://go.dev/blog/feed.atom"},
	{name: "Hacker News", url: "https://news.ycombinator.com/rss"},
	{name: "Lobsters", url: "https://lobste.rs/rss"},
	{name: "XKCD", url: "https://xkcd.com/atom.xml"},
	{name: "Dev.to", url: "https://dev.to/feed"},
}

var postsMock = []item{
	{name: "Understanding Generics in Go", url: "https://go.dev/blog/generics"},
	{name: "Ask HN: Best terminal tools?", url: "https://news.ycombinator.com/item?id=123456"},
	{name: "Writing a TUI with Bubble Tea", url: "https://lobste.rs/s/abc123"},
	{name: "XKCD: Compiling", url: "https://xkcd.com/303/"},
	{name: "10 Tips for Cleaner Go Code", url: "https://dev.to/t/clean-go"},
}
