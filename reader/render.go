package reader

import (
	"fmt"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

// RenderMarkdown styles a Markdown document for a terminal width. The output
// uses the requested light/dark palette and never exceeds width terminal cells.
func RenderMarkdown(markdownText string, width int, dark bool) (string, error) {
	width = max(width, 1)
	if width > 500 {
		width = 500
	}
	markdownText = cleanMarkdown(markdownText)
	if len(markdownText) > MaxMarkdownBytes {
		return "", fmt.Errorf("Markdown exceeds %d-byte limit", MaxMarkdownBytes)
	}
	style := styles.LightStyleConfig
	if dark {
		style = styles.DarkStyleConfig
	}
	zero := uint(0)
	style.Document.Margin = &zero
	wrap := max(width-2, 1)
	renderer, err := glamour.NewTermRenderer(glamour.WithStyles(style), glamour.WithWordWrap(wrap))
	if err != nil {
		return "", fmt.Errorf("create Markdown renderer: %w", err)
	}
	rendered, err := renderer.Render(markdownText)
	if err != nil {
		return "", fmt.Errorf("render Markdown: %w", err)
	}
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	for i, line := range lines {
		visible := strings.TrimRight(ansi.Strip(line), " ")
		line = ansi.Truncate(line, ansi.StringWidth(visible), "")
		if ansi.StringWidth(line) > width {
			lines[i] = ansi.Truncate(line, width, "")
		} else {
			lines[i] = line
		}
	}
	return strings.Join(lines, "\n"), nil
}
