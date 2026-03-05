package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

// SecStore categories for filtering.
var secStoreCategories = []struct {
	Label    string
	Category core.ToolCategory
	All      bool // true for the "All" pseudo-category
}{
	{Label: "All", All: true},
	{Label: "Firewall", Category: core.ToolCatFirewall},
	{Label: "IPS", Category: core.ToolCatIntrusionPrevention},
	{Label: "Malware", Category: core.ToolCatMalware},
	{Label: "VPN", Category: core.ToolCatVPN},
	{Label: "FIM", Category: core.ToolCatFileIntegrity},
	{Label: "Access", Category: core.ToolCatAccessControl},
}

const cardHeight = 6

// installToolRequestMsg is sent when the user confirms a tool install.
type installToolRequestMsg struct {
	Tool core.SecurityTool
}

// installToolResultMsg is sent when the install command finishes.
type installToolResultMsg struct {
	ToolID string
	Err    error
}

// refreshToolsMsg tells the App to re-detect all tools and refresh sidebar + SecStore.
type refreshToolsMsg struct{}

// secStoreState tracks the install flow overlay state.
type secStoreState int

const (
	secStoreIdle       secStoreState = iota
	secStoreConfirm                  // showing confirm dialog
	secStoreInstalling               // install running
	secStoreResult                   // showing result
)

type toolCard struct {
	Tool   core.SecurityTool
	Status core.ToolStatus
}

type SecStoreView struct {
	allCards   []toolCard // all not-installed tools
	filtered   []toolCard // after category filter
	cursor     int
	category   int // index into secStoreCategories
	width      int
	height     int
	state      secStoreState
	installErr error
	installID  string
}

func NewSecStoreView() SecStoreView {
	return SecStoreView{}
}

// SetTools populates the store with tools and their statuses.
// Only tools that are NotInstalled are shown.
func (s SecStoreView) SetTools(allTools []core.SecurityTool, statuses map[string]core.ToolStatus) SecStoreView {
	s.allCards = nil
	for _, t := range allTools {
		status := statuses[t.ID()]
		if status == core.ToolNotInstalled {
			s.allCards = append(s.allCards, toolCard{Tool: t, Status: status})
		}
	}
	s.applyFilter()
	return s
}

func (s SecStoreView) SetSize(w, h int) SecStoreView {
	s.width = w
	s.height = h
	return s
}

func (s SecStoreView) Update(msg tea.Msg) (SecStoreView, tea.Cmd) {
	switch msg := msg.(type) {
	case installToolResultMsg:
		s.installErr = msg.Err
		s.installID = msg.ToolID
		s.state = secStoreResult
		return s, nil

	case tea.KeyMsg:
		// Handle overlay states first.
		if s.state == secStoreConfirm {
			return s.handleConfirmKeys(msg)
		}
		if s.state == secStoreResult {
			return s.handleResultKeys(msg)
		}

		switch msg.String() {
		case "j", "down":
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
			}
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
			}
		case "tab":
			s.category = (s.category + 1) % len(secStoreCategories)
			s.applyFilter()
		case "enter", "i":
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				s.state = secStoreConfirm
			}
		}
	}
	return s, nil
}

func (s SecStoreView) View() string {
	// Overlay: install confirm.
	if s.state == secStoreConfirm && s.cursor < len(s.filtered) {
		return s.renderConfirm()
	}
	// Overlay: installing.
	if s.state == secStoreInstalling {
		return s.renderInstalling()
	}
	// Overlay: result.
	if s.state == secStoreResult {
		return s.renderResult()
	}

	// Empty state.
	if len(s.filtered) == 0 {
		return s.renderEmpty()
	}

	var sections []string
	sections = append(sections, s.renderTitle())
	sections = append(sections, s.renderCategoryBar())
	sections = append(sections, s.renderCards())
	sections = append(sections, s.renderKeybar())

	content := strings.Join(sections, "\n")
	return StyleContent.Width(s.width).Height(s.height).Render(content)
}

// --- Key handlers ---

func (s SecStoreView) handleConfirmKeys(msg tea.KeyMsg) (SecStoreView, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		tool := s.filtered[s.cursor].Tool
		s.state = secStoreInstalling
		return s, func() tea.Msg {
			return installToolRequestMsg{Tool: tool}
		}
	case "n", "N", "esc":
		s.state = secStoreIdle
	}
	return s, nil
}

func (s SecStoreView) handleResultKeys(msg tea.KeyMsg) (SecStoreView, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", " ":
		s.state = secStoreIdle
		s.installErr = nil
		s.installID = ""
		if s.cursor >= len(s.filtered) {
			s.cursor = maxInt(len(s.filtered)-1, 0)
		}
		// Ask App to refresh tool detection.
		return s, func() tea.Msg { return refreshToolsMsg{} }
	}
	return s, nil
}

// --- Rendering ---

func (s SecStoreView) renderTitle() string {
	return StyleTitle.Render("SecStore")
}

func (s SecStoreView) renderCategoryBar() string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	activeStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Underline(true)

	var parts []string
	for i, cat := range secStoreCategories {
		if i == s.category {
			parts = append(parts, activeStyle.Render(cat.Label))
		} else {
			parts = append(parts, dimStyle.Render(cat.Label))
		}
	}

	return "  " + strings.Join(parts, "  ")
}

func (s SecStoreView) renderCards() string {
	if len(s.filtered) == 0 {
		return ""
	}

	colWidth := (s.width - 8) / 2
	if colWidth < 20 {
		colWidth = 20
	}

	// Calculate visible cards based on available height.
	// Reserve: title 1 + catbar 1 + keybar 2 + padding ~4 = 8 lines.
	availableLines := s.height - 8
	if availableLines < cardHeight {
		availableLines = cardHeight
	}
	visiblePairs := availableLines / (cardHeight + 1)
	if visiblePairs < 1 {
		visiblePairs = 1
	}

	// Scroll so the selected card is always visible.
	pairIdx := s.cursor / 2
	startPair := 0
	if pairIdx >= visiblePairs {
		startPair = pairIdx - visiblePairs + 1
	}

	var rows []string
	for p := startPair; p < startPair+visiblePairs && p*2 < len(s.filtered); p++ {
		leftIdx := p * 2
		rightIdx := leftIdx + 1

		leftCard := s.renderCard(leftIdx, colWidth)
		rightCard := ""
		if rightIdx < len(s.filtered) {
			rightCard = s.renderCard(rightIdx, colWidth)
		}

		row := lipgloss.JoinHorizontal(lipgloss.Top, leftCard, "  ", rightCard)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func (s SecStoreView) renderCard(idx int, width int) string {
	card := s.filtered[idx]
	isSelected := idx == s.cursor

	borderColor := ColorDimmed
	if isSelected {
		borderColor = ColorAccent
	}

	cardStyle := lipgloss.NewStyle().
		Width(width).
		Height(cardHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	nameStyle := lipgloss.NewStyle().Bold(true)
	if isSelected {
		nameStyle = nameStyle.Foreground(ColorAccent)
	} else {
		nameStyle = nameStyle.Foreground(ColorText)
	}

	catStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	descStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	name := nameStyle.Render(card.Tool.Name())
	catLabel := catStyle.Render("[" + categoryLabel(card.Tool.Category()) + "]")
	desc := truncateLines(card.Tool.Description(), width-4, 2)
	descRendered := descStyle.Render(desc)

	var lines []string
	lines = append(lines, name)
	lines = append(lines, catLabel)
	lines = append(lines, descRendered)

	if isSelected {
		hintStyle := lipgloss.NewStyle().Foreground(ColorInfo).Italic(true)
		lines = append(lines, hintStyle.Render("Press [Enter] to install"))
	}

	content := strings.Join(lines, "\n")
	return cardStyle.Render(content)
}

func (s SecStoreView) renderKeybar() string {
	sep := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("\u2500", maxInt(s.width-6, 10)))

	hints := []string{
		StyleKeyhint.Render("[j/k]") + " Browse",
		StyleKeyhint.Render("[Tab]") + " Category",
		StyleKeyhint.Render("[Enter]") + " Install",
		StyleKeyhint.Render("[h]") + " Back",
	}

	return sep + "\n" + "  " + strings.Join(hints, "  ")
}

func (s SecStoreView) renderEmpty() string {
	title := StyleTitle.Render("SecStore")
	catBar := s.renderCategoryBar()

	emptyStyle := lipgloss.NewStyle().Foreground(ColorDimmed).Padding(2, 0)
	msg := emptyStyle.Render("  All available tools are already installed!\n\n  Check the TOOLS section in the sidebar to manage them.")

	content := title + "\n" + catBar + "\n\n" + msg
	return StyleContent.Width(s.width).Height(s.height).Render(content)
}

func (s SecStoreView) renderConfirm() string {
	card := s.filtered[s.cursor]
	tool := card.Tool

	title := StyleTitle.Render(fmt.Sprintf("Install %s?", tool.Name()))

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	cmdStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning)

	// We need the platform to show the install command, but we don't have it here.
	// Use Description instead and show the install hint.
	var lines []string
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  "+tool.Description()))
	lines = append(lines, "")
	lines = append(lines, warnStyle.Render("  This will install the tool on your system."))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %s to install, %s to cancel",
		cmdStyle.Render("[y]"),
		lipgloss.NewStyle().Foreground(ColorCritical).Bold(true).Render("[n]"),
	))

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(s.width).Height(s.height).Render(content)
}

func (s SecStoreView) renderInstalling() string {
	title := StyleTitle.Render("Installing...")
	spinner := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).
		Render("  Please wait while the tool is being installed...")

	content := title + "\n\n" + spinner
	return StyleContent.Width(s.width).Height(s.height).Render(content)
}

func (s SecStoreView) renderResult() string {
	if s.installErr != nil {
		title := StyleTitle.Render("Installation Failed")
		errStyle := lipgloss.NewStyle().Foreground(ColorCritical)
		dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

		var lines []string
		lines = append(lines, "")
		lines = append(lines, errStyle.Render(fmt.Sprintf("  Failed to install %s:", s.installID)))
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  "+s.installErr.Error()))
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("  Press [Enter] to go back."))

		content := title + "\n" + strings.Join(lines, "\n")
		return StyleContent.Width(s.width).Height(s.height).Render(content)
	}

	title := StyleTitle.Render("Installation Successful")
	okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, okStyle.Render(fmt.Sprintf("  %s installed successfully!", s.installID)))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  The tool is now available in the TOOLS section."))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Press [Enter] to continue."))

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(s.width).Height(s.height).Render(content)
}

// --- Helpers ---

func (s *SecStoreView) applyFilter() {
	cat := secStoreCategories[s.category]
	if cat.All {
		s.filtered = s.allCards
	} else {
		s.filtered = nil
		for _, c := range s.allCards {
			if c.Tool.Category() == cat.Category {
				s.filtered = append(s.filtered, c)
			}
		}
	}
	if s.cursor >= len(s.filtered) {
		s.cursor = maxInt(len(s.filtered)-1, 0)
	}
}

func categoryLabel(cat core.ToolCategory) string {
	switch cat {
	case core.ToolCatFirewall:
		return "Firewall"
	case core.ToolCatIntrusionPrevention:
		return "IPS"
	case core.ToolCatMalware:
		return "Malware"
	case core.ToolCatVPN:
		return "VPN"
	case core.ToolCatFileIntegrity:
		return "FIM"
	case core.ToolCatAccessControl:
		return "Access"
	default:
		return "Other"
	}
}

// truncateLines limits text to n lines, each capped at width characters.
func truncateLines(text string, width, maxLines int) string {
	words := strings.Fields(text)
	var lines []string
	var current string

	for _, w := range words {
		if current == "" {
			current = w
		} else if len(current)+1+len(w) <= width {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
			if len(lines) >= maxLines {
				break
			}
		}
	}
	if current != "" && len(lines) < maxLines {
		lines = append(lines, current)
	}

	return strings.Join(lines, "\n")
}
