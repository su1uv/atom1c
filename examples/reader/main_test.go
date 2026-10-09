package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/reader"
	readerview "github.com/su1uv/atom1c/reader/view"
)

func TestStandaloneViewTreatsWebsiteMetadataAsLiteral(t *testing.T) {
	m := model{viewport: readerview.New(80, 24), dark: true, document: reader.Article{
		Title: `Title](javascript:alert(1))`, SiteName: `**not formatting**`, Author: `[file](file:///etc/passwd)`, Markdown: "Plain body",
	}}
	if err := m.renderDocument(); err != nil {
		t.Fatal(err)
	}
	got := m.viewport.GetContent()
	if strings.Contains(got, "\x1b]8;") {
		t.Fatalf("metadata created a terminal hyperlink: %q", got)
	}
	plain := ansi.Strip(got)
	for _, want := range []string{"Title](javascript:alert(1))", "**not formatting**", "[file](file:///etc/passwd)", "Plain body"} {
		if !strings.Contains(plain, want) {
			t.Errorf("literal metadata missing %q: %q", want, plain)
		}
	}
}
