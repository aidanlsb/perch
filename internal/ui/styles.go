package ui

import "github.com/charmbracelet/lipgloss"

var (
	colAccent = lipgloss.Color("174")
	colMuted  = lipgloss.Color("245")
	colDim    = lipgloss.Color("240")
	colText   = lipgloss.Color("252")
	colSelBg  = lipgloss.Color("237")
	colErr    = lipgloss.Color("167")
	colOk     = lipgloss.Color("108")
	colWarn   = lipgloss.Color("180")
	colBorder = lipgloss.Color("238")
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
	styleRowSel = lipgloss.NewStyle().Foreground(colText).Background(colSelBg).Bold(true)

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

	styleLinkSel = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(colAccent).Bold(true)
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
