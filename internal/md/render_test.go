package md

import "testing"

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
