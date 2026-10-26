package md

import (
	"strings"

	"github.com/aidanlsb/perch/internal/termesc"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// Render turns Markdown into ANSI for a terminal of the given width.
// Style follows the terminal background so light terminals are not washed out.
// On failure it returns the input, indented, so the TUI still shows the note.
func Render(markdown string, width int) string {
	return renderStyled(markdown, width, lipgloss.HasDarkBackground())
}

func renderStyled(markdown string, width int, darkBackground bool) string {
	markdown = termesc.Plain(markdown)
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyle(darkBackground)),
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

func markdownStyle(darkBackground bool) ansi.StyleConfig {
	if darkBackground {
		return styles.DarkStyleConfig
	}
	return lightStyle()
}

// lightStyle is Glamour's light theme with a few contrast fixes: the stock
// chroma block paints dark-on-dark, and the HR/image captions are too faint.
func lightStyle() ansi.StyleConfig {
	s := styles.LightStyleConfig
	hr := "242"
	s.HorizontalRule.Color = &hr
	img := "240"
	s.ImageText.Color = &img
	code := "236"
	s.CodeBlock.Color = &code
	if s.CodeBlock.Chroma != nil {
		chroma := *s.CodeBlock.Chroma
		bg := "#f4f4f5"
		chroma.Background.BackgroundColor = &bg
		comment := "#52525b"
		chroma.Comment.Color = &comment
		sub := "#52525b"
		chroma.GenericSubheading.Color = &sub
		s.CodeBlock.Chroma = &chroma
	}
	return s
}
