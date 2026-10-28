package ui

import "github.com/charmbracelet/lipgloss"

// Palette uses AdaptiveColor so the same styles stay readable on light and
// dark terminals. Light values are deep slate/ink (not washed-out gray) with
// a copper-gold accent; dark values keep a 256-color optic look.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "172"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#3f3f46", Dark: "245"}
	colDim    = lipgloss.AdaptiveColor{Light: "#52525b", Dark: "240"}
	colText   = lipgloss.AdaptiveColor{Light: "#1c1917", Dark: "252"}
	colSelFg  = lipgloss.AdaptiveColor{Light: "#1c1917", Dark: "255"}
	colSelBg  = lipgloss.AdaptiveColor{Light: "#f5e6c8", Dark: "237"}
	colErr    = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "167"}
	colOk     = lipgloss.AdaptiveColor{Light: "#166534", Dark: "108"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#a16207", Dark: "178"}
	colBorder = lipgloss.AdaptiveColor{Light: "#64748b", Dark: "238"}
	colLinkFg = lipgloss.AdaptiveColor{Light: "#fffbeb", Dark: "0"}
)

var (
	styleHeader = lipgloss.NewStyle().
			Foreground(colAccent).
			Bold(true)

	styleMuted = lipgloss.NewStyle().Foreground(colMuted)
	styleDim   = lipgloss.NewStyle().Foreground(colDim)
	styleErr   = lipgloss.NewStyle().Foreground(colErr)
	styleOk    = lipgloss.NewStyle().Foreground(colOk)
	styleWarn  = lipgloss.NewStyle().Foreground(colWarn)
	styleKey   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleThumb = lipgloss.NewStyle().Foreground(colAccent)

	styleSectionTitle = lipgloss.NewStyle().
				Foreground(colDim).
				Bold(true).
				PaddingLeft(1)

	styleNav    = lipgloss.NewStyle().PaddingLeft(1).Foreground(colText)
	styleNavSel = lipgloss.NewStyle().
			PaddingLeft(1).
			Foreground(colAccent).
			Background(colSelBg).
			Bold(true)

	styleRow     = lipgloss.NewStyle().Foreground(colText)
	styleRowSel  = lipgloss.NewStyle().Foreground(colSelFg).Background(colSelBg).Bold(true)
	styleRowIdle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	styleColHead = lipgloss.NewStyle().Foreground(colMuted).Bold(true)

	styleBoardHead = lipgloss.NewStyle().
			Foreground(colAccent).
			Bold(true)

	styleBoardHeadIdle = lipgloss.NewStyle().
				Foreground(colMuted).
				Bold(true)

	styleDateHead = lipgloss.NewStyle().
			Foreground(colAccent).
			Bold(true)

	styleFMKey = lipgloss.NewStyle().Foreground(colMuted)
	styleFMVal = lipgloss.NewStyle().Foreground(colText)

	styleLinkSel = lipgloss.NewStyle().Foreground(colLinkFg).Background(colAccent).Bold(true)
	styleLinkBad = lipgloss.NewStyle().Foreground(colWarn)
	styleStatus  = lipgloss.NewStyle().Foreground(colMuted)
)

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
