package md

import (
	"strings"

	"github.com/aidanlsb/perch/internal/termesc"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

// Render turns Markdown into ANSI for a terminal of the given width.
// On failure it returns the input, indented, so the TUI still shows the note.
func Render(markdown string, width int) string {
	markdown = termesc.Plain(markdown)
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(styles.DarkStyleConfig),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return markdown
	}
	out, err := r.Render(markdown)
	if err != nil {
		return markdown
	}
	return termesc.ForTUI(strings.TrimRight(out, "\n"))
}
