package ui

import "github.com/charmbracelet/lipgloss"

// Palette uses AdaptiveColor so the same styles stay readable on light and
// dark terminals. Light values are mid-dark slate/ink (not washed-out gray);
// dark values keep the previous 256-color look.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#b45353", Dark: "174"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#4b5563", Dark: "245"}
	colDim    = lipgloss.AdaptiveColor{Light: "#52525b", Dark: "240"}
	colText   = lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "252"}
	colSelFg  = lipgloss.AdaptiveColor{Light: "#111827", Dark: "255"}
	colSelBg  = lipgloss.AdaptiveColor{Light: "#e5e7eb", Dark: "237"}
	colErr    = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "167"}
	colOk     = lipgloss.AdaptiveColor{Light: "#166534", Dark: "108"}
	colWarn   = lipgloss.AdaptiveColor{Light: "#a16207", Dark: "180"}
	colBorder = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "238"}
	colLinkFg = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "0"}
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

	styleRow    = lipgloss.NewStyle().Foreground(colText)
	styleRowSel = lipgloss.NewStyle().Foreground(colSelFg).Background(colSelBg).Bold(true)

	styleColHead = lipgloss.NewStyle().Foreground(colMuted).Bold(true).Underline(true)

	styleBoardHead = lipgloss.NewStyle().
			Foreground(colAccent).
			Bold(true)

	styleDateHead = lipgloss.NewStyle().
			Foreground(colAccent).
			Bold(true).
			MarginTop(1)

	styleFMKey = lipgloss.NewStyle().Foreground(colMuted)
	styleFMVal = lipgloss.NewStyle().Foreground(colText)

	styleLinkSel = lipgloss.NewStyle().Foreground(colLinkFg).Background(colAccent).Bold(true)
	styleLinkBad = lipgloss.NewStyle().Foreground(colWarn)
	styleFooter  = lipgloss.NewStyle().Foreground(colDim)
	styleStatus  = lipgloss.NewStyle().Foreground(colMuted)
	styleHelpBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(1, 2)
	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1)
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
