package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestOverlayLineKeepsBothSides(t *testing.T) {
	base := "\x1b[31mAAAAA\x1b[0m-----\x1b[32mBBBBB\x1b[0m"
	out := overlayLine(base, "XX", 5)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "AAAAA") {
		t.Fatalf("left lost: %q", plain)
	}
	if !strings.Contains(plain, "BBBBB") {
		t.Fatalf("right lost: %q", plain)
	}
	if !strings.Contains(plain, "XX") {
		t.Fatalf("modal lost: %q", plain)
	}
	i := strings.Index(plain, "XX")
	if i < 0 || !strings.Contains(plain[i+2:], "BBB") {
		t.Fatalf("right of modal missing: %q", plain)
	}
}

func TestOverlayLineDoesNotSliceANSIByByte(t *testing.T) {
	base := "\x1b[38;5;172mAAAA\x1b[0m    \x1b[38;5;108mBBBB\x1b[0m"
	if len(base) <= 12 {
		t.Fatalf("expected ANSI to inflate bytes, got %d %q", len(base), base)
	}
	out := overlayLine(base, "MM", 4)
	plain := ansi.Strip(out)
	if strings.Count(plain, "B") < 4 {
		t.Fatalf("right side dropped (old byte-slice bug): %q from bytes=%d", plain, len(base))
	}
	if strings.Count(plain, "A") < 4 {
		t.Fatalf("left side dropped: %q", plain)
	}
}

func TestOverlayOnDimsBackdrop(t *testing.T) {
	base := strings.Join([]string{
		styleHeader.Render("perch  home"),
		styleRow.Render("row-one-content"),
		styleRow.Render("row-two-content"),
		styleMuted.Render("footer keys"),
	}, "\n")
	modal := modalFrame("navigate", "▸ query█\nitem", 20)
	out := overlayOn(base, 40, 8, modal)
	if !strings.Contains(ansi.Strip(out), "navigate") {
		t.Fatalf("modal title missing:\n%s", out)
	}
	if !strings.Contains(out, "╭") {
		t.Fatalf("rounded modal missing:\n%s", out)
	}
}

func TestFrameBoxRoundedInlineHeader(t *testing.T) {
	out := frameBox("body", 28, 6, "▸ active-projects · board · 8", true, scrollState{})
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╮") {
		t.Fatalf("rounded corners missing: %q", plain)
	}
	first := strings.Split(plain, "\n")[0]
	if !strings.Contains(first, "▸ active-projects") {
		t.Fatalf("title not in top border: %q", first)
	}
	if strings.Contains(plain, "┌") || strings.Contains(plain, "┐") {
		t.Fatalf("square corners: %q", plain)
	}
}

func TestScrollThumbAppearsWhenOverflowing(t *testing.T) {
	body := strings.Repeat("line\n", 20)
	out := frameBox(body, 16, 8, "list · 20", false, scrollState{offset: 4, total: 20, view: 6})
	if !strings.Contains(out, thumbGlyph) {
		t.Fatalf("expected thumb:\n%s", ansi.Strip(out))
	}
}

func TestBreadcrumbTruncatesFromLeft(t *testing.T) {
	s := formatBreadcrumb([]string{"home", "dashboard", "project/perch"}, 18)
	if !strings.HasPrefix(ansi.Strip(s), "…") && !strings.Contains(s, "project/perch") {
		t.Fatalf("got %q", s)
	}
	if strings.Contains(s, "home › dashboard › project/perch") && lipgloss.Width(s) > 18 {
		t.Fatalf("not truncated: %q", s)
	}
	full := formatBreadcrumb([]string{"home", "zebra"}, 40)
	if full != "home › zebra" {
		t.Fatalf("got %q", full)
	}
}

func TestCellsFromSkipsDisplayWidth(t *testing.T) {
	s := "hello world"
	if got := cellsFrom(s, 6); got != "world" {
		t.Fatalf("got %q", got)
	}
	if got := cellsPrefix(s, 5); got != "hello" {
		t.Fatalf("prefix %q", got)
	}
}
