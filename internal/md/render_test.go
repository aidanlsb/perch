package md

import (
	"strings"
	"testing"

	"github.com/charmbracelet/glamour/styles"
)

func TestRenderFallbackOrANSI(t *testing.T) {
	out := Render("# Hello", 40)
	if out == "" {
		t.Fatal("empty")
	}
}

func TestRenderNarrow(t *testing.T) {
	out := Render("hi", 5)
	if out == "" {
		t.Fatal("empty")
	}
}

func TestRenderStripsOSCFromInput(t *testing.T) {
	out := Render("hi\x1b]0;pwned\x07 there", 40)
	if strings.Contains(out, "\x1b]") || strings.Contains(out, "pwned") && strings.Contains(out, "\x07") {
		t.Fatalf("osc leaked: %q", out)
	}
	if strings.Contains(out, "\x1b]0;") {
		t.Fatalf("osc leaked: %q", out)
	}
}

func TestMarkdownStyleFollowsBackground(t *testing.T) {
	dark := markdownStyle(true)
	light := markdownStyle(false)
	if dark.Document.Color == nil || *dark.Document.Color != *styles.DarkStyleConfig.Document.Color {
		t.Fatalf("dark document color %v", dark.Document.Color)
	}
	if light.Document.Color == nil || *light.Document.Color != "234" {
		t.Fatalf("light document should use dark ink, got %v", light.Document.Color)
	}
	if *light.Document.Color == *dark.Document.Color {
		t.Fatal("light and dark document ink should differ")
	}
	if light.CodeBlock.Chroma == nil || light.CodeBlock.Chroma.Background.BackgroundColor == nil {
		t.Fatal("light chroma background")
	}
	if got := *light.CodeBlock.Chroma.Background.BackgroundColor; !strings.EqualFold(got, "#f4f4f5") {
		t.Fatalf("light chroma bg %s (dark-on-dark would wash out)", got)
	}
}

func TestRenderLightAndDarkDiffer(t *testing.T) {
	src := "# Hello\n\nSome **text** and `code`.\n"
	light := renderStyled(src, 40, false)
	dark := renderStyled(src, 40, true)
	if light == "" || dark == "" {
		t.Fatal("empty")
	}
	if light == dark {
		t.Fatal("light and dark markdown styles produced identical ANSI")
	}
}
