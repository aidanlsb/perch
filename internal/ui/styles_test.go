package ui

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPaletteUsesAdaptiveColors(t *testing.T) {
	pairs := []struct {
		name string
		c    lipgloss.AdaptiveColor
	}{
		{"accent", colAccent},
		{"muted", colMuted},
		{"dim", colDim},
		{"text", colText},
		{"selFg", colSelFg},
		{"selBg", colSelBg},
		{"err", colErr},
		{"ok", colOk},
		{"warn", colWarn},
		{"border", colBorder},
		{"linkFg", colLinkFg},
	}
	for _, p := range pairs {
		if p.c.Light == "" || p.c.Dark == "" {
			t.Errorf("%s missing Light/Dark", p.name)
		}
		if p.c.Light == p.c.Dark {
			t.Errorf("%s Light and Dark are identical (%s)", p.name, p.c.Light)
		}
	}
}

func TestLightModeInkIsReadable(t *testing.T) {
	// Mid-dark slate/ink, not washed-out #888 / ANSI 245–252.
	for _, p := range []struct {
		name string
		hex  string
		max  float64
	}{
		{"muted", colMuted.Light, 0.28},
		{"dim", colDim.Light, 0.28},
		{"text", colText.Light, 0.18},
		{"selFg", colSelFg.Light, 0.12},
		{"footer/section", colDim.Light, 0.28},
	} {
		lum := hexLuminance(t, p.hex)
		if lum > p.max {
			t.Errorf("%s light %s luminance %.3f > %.2f (too faint on white)", p.name, p.hex, lum, p.max)
		}
	}
	selLum := hexLuminance(t, colSelBg.Light)
	if selLum < 0.7 {
		t.Errorf("selBg light %s luminance %.3f; need a light highlight", colSelBg.Light, selLum)
	}
	if hexLuminance(t, colSelFg.Light) >= selLum {
		t.Fatal("selected row foreground is not darker than its background")
	}
	acc := hexLuminance(t, colAccent.Light)
	if acc > 0.28 {
		t.Errorf("accent light %s luminance %.3f; too faint on white", colAccent.Light, acc)
	}
	if acc < 0.05 {
		t.Errorf("accent light %s luminance %.3f; too dark to read as copper", colAccent.Light, acc)
	}
	border := hexLuminance(t, colBorder.Light)
	if border > 0.45 {
		t.Errorf("border light %s luminance %.3f; too faint", colBorder.Light, border)
	}
}

func hexLuminance(t *testing.T, hex string) float64 {
	t.Helper()
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		t.Fatalf("want #rrggbb, got %q", hex)
	}
	r := hexByte(t, hex[0:2])
	g := hexByte(t, hex[2:4])
	b := hexByte(t, hex[4:6])
	return 0.2126*srgbLin(r) + 0.7152*srgbLin(g) + 0.0722*srgbLin(b)
}

func hexByte(t *testing.T, s string) uint8 {
	t.Helper()
	n, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		t.Fatal(err)
	}
	return uint8(n)
}

func srgbLin(c uint8) float64 {
	x := float64(c) / 255
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}
