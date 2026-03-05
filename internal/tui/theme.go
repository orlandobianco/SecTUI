package tui

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

var (
	ColorOK       = compat.AdaptiveColor{Light: lipgloss.Color("34"), Dark: lipgloss.Color("82")}
	ColorWarning  = compat.AdaptiveColor{Light: lipgloss.Color("214"), Dark: lipgloss.Color("214")}
	ColorCritical = compat.AdaptiveColor{Light: lipgloss.Color("196"), Dark: lipgloss.Color("196")}
	ColorInfo     = compat.AdaptiveColor{Light: lipgloss.Color("39"), Dark: lipgloss.Color("39")}
	ColorDimmed   = compat.AdaptiveColor{Light: lipgloss.Color("245"), Dark: lipgloss.Color("245")}
	ColorAccent   = compat.AdaptiveColor{Light: lipgloss.Color("135"), Dark: lipgloss.Color("135")}
	ColorText     = compat.AdaptiveColor{Light: lipgloss.Color("0"), Dark: lipgloss.Color("252")}
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
