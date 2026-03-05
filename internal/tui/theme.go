package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorOK       = lipgloss.AdaptiveColor{Light: "34", Dark: "82"}
	ColorWarning  = lipgloss.AdaptiveColor{Light: "214", Dark: "214"}
	ColorCritical = lipgloss.AdaptiveColor{Light: "196", Dark: "196"}
	ColorInfo     = lipgloss.AdaptiveColor{Light: "39", Dark: "39"}
	ColorDimmed   = lipgloss.AdaptiveColor{Light: "245", Dark: "245"}
	ColorAccent   = lipgloss.AdaptiveColor{Light: "135", Dark: "135"}
	ColorText     = lipgloss.AdaptiveColor{Light: "0", Dark: "252"}
)

var (
	StyleSidebar       lipgloss.Style
	StyleSidebarItem   lipgloss.Style
	StyleSidebarActive lipgloss.Style
	StyleSidebarHeader lipgloss.Style
	StyleHeader        lipgloss.Style
	StyleFooter        lipgloss.Style
	StyleContent       lipgloss.Style
	StyleTitle         lipgloss.Style
	StyleBadgeON       lipgloss.Style
	StyleBadgeOFF      lipgloss.Style
	StyleBadgeSpinner  lipgloss.Style
	StyleKeyhint       lipgloss.Style
)

func init() {
	StyleSidebar = lipgloss.NewStyle().
		Width(sidebarWidth).
		BorderStyle(lipgloss.NormalBorder()).
		BorderRight(true).
		BorderForeground(ColorDimmed).
		Padding(1, 1)

	StyleSidebarItem = lipgloss.NewStyle().
		Foreground(ColorText).
		PaddingLeft(2)

	StyleSidebarActive = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		PaddingLeft(0)

	StyleSidebarHeader = lipgloss.NewStyle().
		Foreground(ColorDimmed).
		Bold(true).
		PaddingLeft(1).
		PaddingTop(1)

	StyleHeader = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ColorDimmed)

	StyleFooter = lipgloss.NewStyle().
		Foreground(ColorDimmed).
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(ColorDimmed)

	StyleContent = lipgloss.NewStyle().
		Padding(1, 2)

	StyleTitle = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)

	StyleBadgeON = lipgloss.NewStyle().
		Foreground(ColorOK).
		Bold(true)

	StyleBadgeOFF = lipgloss.NewStyle().
		Foreground(ColorWarning)

	StyleBadgeSpinner = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	StyleKeyhint = lipgloss.NewStyle().
		Foreground(ColorInfo).
		Bold(true)
}
