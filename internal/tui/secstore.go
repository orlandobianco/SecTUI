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
	scrollTop  int // first visible pair index
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
				s.ensureVisible()
			}
		case "k", "up":
			if s.cursor > 0 {
				s.cursor--
				s.ensureVisible()
			}
		case "l", "right":
			// Move right within a row: if on left column, go to right.
			if s.cursor%2 == 0 && s.cursor+1 < len(s.filtered) {
				s.cursor++
				s.ensureVisible()
			}
		case "h", "left":
			// Move left within a row: if on right column, go to left.
			if s.cursor%2 == 1 {
				s.cursor--
				s.ensureVisible()
			}
		case "[", "shift+tab":
			s.category--
			if s.category < 0 {
				s.category = len(secStoreCategories) - 1
			}
			s.applyFilter()
		case "]", "1", "2", "3", "4", "5", "6", "7":
			k := msg.String()
			if k == "]" {
				s.category = (s.category + 1) % len(secStoreCategories)
			} else {
				idx := int(k[0]-'0') - 1
				if idx >= 0 && idx < len(secStoreCategories) {
					s.category = idx
				}
			}
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
		return s, func() tea.Msg { return refreshToolsMsg{} }
	}
	return s, nil
}

// --- Layout calculations ---

// cardRenderedHeight returns the total height a card occupies including borders.
// lipgloss border adds 2 lines (top+bottom). We use 4 lines of content inside.
const cardContentHeight = 4
const cardBorderH = 2
const cardTotalHeight = cardContentHeight + cardBorderH // 6
const cardGapY = 1                                      // vertical gap between rows

// visiblePairs returns how many card rows fit on screen.
func (s SecStoreView) visiblePairs() int {
	// Reserve: title(1) + blank(1) + catbar(1) + blank(1) + keybar(2) + padding ~2 = ~8
	usable := s.height - 8
	if usable < cardTotalHeight {
		return 1
	}
	rowH := cardTotalHeight + cardGapY
	pairs := usable / rowH
	if pairs < 1 {
		pairs = 1
	}
	return pairs
}

// totalPairs returns the total number of card rows.
func (s SecStoreView) totalPairs() int {
	n := len(s.filtered)
	return (n + 1) / 2
}

// ensureVisible scrolls so the cursor is on screen.
func (s *SecStoreView) ensureVisible() {
	pairIdx := s.cursor / 2
	vis := s.visiblePairs()

	if pairIdx < s.scrollTop {
		s.scrollTop = pairIdx
	}
	if pairIdx >= s.scrollTop+vis {
		s.scrollTop = pairIdx - vis + 1
	}
}

// colWidth computes the width available for each card column.
func (s SecStoreView) colWidth() int {
	// Content area has padding(2 each side) from StyleContent, so usable = width - 4.
	// We want: gap(2) between two columns.
	usable := s.width - 4
	w := (usable - 2) / 2
	if w < 20 {
		w = 20
	}
	// For very narrow terminals, switch to single column.
	if usable < 44 {
		w = usable
	}
	return w
}

// singleColumn returns true when terminal is too narrow for two columns.
func (s SecStoreView) singleColumn() bool {
	return (s.width - 4) < 44
}

// --- Rendering ---

func (s SecStoreView) renderTitle() string {
	return StyleTitle.Render("SecStore")
}

func (s SecStoreView) renderCategoryBar() string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	activeStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Underline(true)
	numStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var parts []string
	for i, cat := range secStoreCategories {
		num := numStyle.Render(fmt.Sprintf("%d:", i+1))
		if i == s.category {
			parts = append(parts, num+activeStyle.Render(cat.Label))
		} else {
			parts = append(parts, num+dimStyle.Render(cat.Label))
		}
	}

	return "  " + strings.Join(parts, "  ")
}

func (s SecStoreView) renderCards() string {
	if len(s.filtered) == 0 {
		return ""
	}

	cw := s.colWidth()
	single := s.singleColumn()
	vis := s.visiblePairs()
	total := s.totalPairs()

	var rows []string
	for p := s.scrollTop; p < s.scrollTop+vis && p < total; p++ {
		leftIdx := p * 2
		rightIdx := leftIdx + 1
		if single {
			rightIdx = -1
		}

		leftCard := s.renderCard(leftIdx, cw)
		if single || rightIdx >= len(s.filtered) {
			rows = append(rows, leftCard)
		} else {
			rightCard := s.renderCard(rightIdx, cw)
			row := lipgloss.JoinHorizontal(lipgloss.Top, leftCard, "  ", rightCard)
			rows = append(rows, row)
		}
	}

	body := strings.Join(rows, "\n")

	// Scroll indicator.
	if total > vis {
		indicator := lipgloss.NewStyle().Foreground(ColorDimmed).Render(
			fmt.Sprintf("  %d-%d of %d", s.scrollTop+1, minInt(s.scrollTop+vis, total), total),
		)
		if s.scrollTop > 0 {
			indicator += lipgloss.NewStyle().Foreground(ColorDimmed).Render(" ↑")
		}
		if s.scrollTop+vis < total {
			indicator += lipgloss.NewStyle().Foreground(ColorDimmed).Render(" ↓")
		}
		body += "\n" + indicator
	}

	return body
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
		Height(cardContentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(0, 1)

	// Name line: arrow indicator + tool name.
	nameStyle := lipgloss.NewStyle().Bold(true)
	if isSelected {
		nameStyle = nameStyle.Foreground(ColorAccent)
	} else {
		nameStyle = nameStyle.Foreground(ColorText)
	}

	arrow := "  "
	if isSelected {
		arrow = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("▸ ")
	}

	catStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	descStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	name := arrow + nameStyle.Render(card.Tool.Name())
	catLabel := catStyle.Render("  [" + categoryLabel(card.Tool.Category()) + "]")
	desc := wrapText(card.Tool.Description(), width-4, 2)
	descRendered := descStyle.Render("  " + strings.ReplaceAll(desc, "\n", "\n  "))

	content := name + "\n" + catLabel + "\n" + descRendered

	return cardStyle.Render(content)
}

func (s SecStoreView) renderKeybar() string {
	w := s.width - 6
	if w < 10 {
		w = 10
	}
	sep := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("─", w))

	hints := []string{
		StyleKeyhint.Render("[j/k]") + " Browse",
		StyleKeyhint.Render("[1-7]") + " Category",
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

	var lines []string
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  "+tool.Description()))
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorWarning).Render("  This will install the tool on your system."))
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
	s.scrollTop = 0
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

// wrapText wraps text to fit within width, returning at most maxLines lines.
func wrapText(text string, width, maxLines int) string {
	if width <= 0 {
		width = 20
	}
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
