package md

import (
	"strings"
	"testing"
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
