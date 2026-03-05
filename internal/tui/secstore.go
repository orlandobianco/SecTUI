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
	All      bool
}{
	{Label: "All", All: true},
	{Label: "Firewall", Category: core.ToolCatFirewall},
	{Label: "IPS", Category: core.ToolCatIntrusionPrevention},
	{Label: "Malware", Category: core.ToolCatMalware},
	{Label: "VPN", Category: core.ToolCatVPN},
	{Label: "FIM", Category: core.ToolCatFileIntegrity},
	{Label: "Access", Category: core.ToolCatAccessControl},
}

type installToolRequestMsg struct {
	Tool core.SecurityTool
}

type installToolResultMsg struct {
	ToolID string
	Err    error
}

type refreshToolsMsg struct{}

type secStoreState int

const (
	secStoreIdle secStoreState = iota
	secStoreConfirm
	secStoreInstalling
	secStoreResult
)

type toolCard struct {
	Tool   core.SecurityTool
	Status core.ToolStatus
}

type SecStoreView struct {
	allCards   []toolCard
	filtered   []toolCard
	cursor     int
	category   int
	width      int
	height     int
	scrollTop  int
	state      secStoreState
	installErr error
	installID  string
}

func NewSecStoreView() SecStoreView {
	return SecStoreView{}
}

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

// Each tool row = 3 lines (name+cat, desc, blank separator).
const rowHeight = 3

func (s SecStoreView) usableHeight() int {
	// title(1) + catbar(1) + separator(1) + keybar(1) = 4 lines of chrome
	return s.height - 4
}

func (s SecStoreView) visibleRows() int {
	h := s.usableHeight()
	if h < rowHeight {
		return 1
	}
	return h / rowHeight
}

func (s *SecStoreView) ensureVisible() {
	vis := s.visibleRows()
	if s.cursor < s.scrollTop {
		s.scrollTop = s.cursor
	}
	if s.cursor >= s.scrollTop+vis {
		s.scrollTop = s.cursor - vis + 1
	}
}

func (s SecStoreView) Update(msg tea.Msg) (SecStoreView, tea.Cmd) {
	switch msg := msg.(type) {
	case installToolResultMsg:
		s.installErr = msg.Err
		s.installID = msg.ToolID
		s.state = secStoreResult
		return s, nil

	case tea.KeyMsg:
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
	if s.state == secStoreConfirm && s.cursor < len(s.filtered) {
		return s.renderConfirm()
	}
	if s.state == secStoreInstalling {
		return s.renderInstalling()
	}
	if s.state == secStoreResult {
		return s.renderResult()
	}
	if len(s.filtered) == 0 {
		return s.renderEmpty()
	}

	cw := s.contentWidth()
	var b strings.Builder

	// Title + category bar.
	b.WriteString(StyleTitle.Render("SecStore"))
	b.WriteByte('\n')
	b.WriteString(s.renderCategoryBar())
	b.WriteByte('\n')

	// Separator.
	sepStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	b.WriteString(sepStyle.Render(strings.Repeat("─", cw)))
	b.WriteByte('\n')

	// Tool list rows.
	vis := s.visibleRows()
	end := s.scrollTop + vis
	if end > len(s.filtered) {
		end = len(s.filtered)
	}

	for i := s.scrollTop; i < end; i++ {
		b.WriteString(s.renderRow(i, cw))
	}

	// Scroll indicator in place of keybar if needed.
	total := len(s.filtered)
	if total > vis {
		arrows := ""
		if s.scrollTop > 0 {
			arrows += "↑ "
		}
		if s.scrollTop+vis < total {
			arrows += "↓ "
		}
		info := fmt.Sprintf("%s%d/%d", arrows, s.cursor+1, total)
		b.WriteString(sepStyle.Render(padRight(info, cw)))
		b.WriteByte('\n')
	}

	// Keybar.
	hints := sepStyle.Render("[j/k]") + " nav  " +
		sepStyle.Render("[1-7]") + " filter  " +
		sepStyle.Render("[Enter]") + " install"
	b.WriteString(hints)

	return lipgloss.NewStyle().
		Width(s.width).Height(s.height).
		Padding(0, 1).
		Render(b.String())
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

// --- Row rendering ---

func (s SecStoreView) contentWidth() int {
	// Leave space for padding (1 each side from the outer style).
	w := s.width - 2
	if w < 20 {
		w = 20
	}
	return w
}

func (s SecStoreView) renderRow(idx int, cw int) string {
	card := s.filtered[idx]
	sel := idx == s.cursor

	// Line 1: marker + name + category badge.
	marker := "  "
	nameStyle := lipgloss.NewStyle().Foreground(ColorText)
	if sel {
		marker = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("▸ ")
		nameStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	}

	catBadge := lipgloss.NewStyle().Foreground(ColorDimmed).Render(
		" [" + categoryLabel(card.Tool.Category()) + "]",
	)

	line1 := marker + nameStyle.Render(card.Tool.Name()) + catBadge

	// Line 2: description (truncated to one line).
	descWidth := cw - 4 // indent
	desc := truncateLine(card.Tool.Description(), descWidth)
	descStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	line2 := "    " + descStyle.Render(desc)

	// Line 3: blank separator (or install hint if selected).
	line3 := ""
	if sel {
		line3 = "    " + lipgloss.NewStyle().Foreground(ColorInfo).Italic(true).Render("Enter to install")
	}

	return line1 + "\n" + line2 + "\n" + line3 + "\n"
}

func (s SecStoreView) renderCategoryBar() string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	activeStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Underline(true)

	var parts []string
	for i, cat := range secStoreCategories {
		num := dimStyle.Render(fmt.Sprintf("%d:", i+1))
		if i == s.category {
			parts = append(parts, num+activeStyle.Render(cat.Label))
		} else {
			parts = append(parts, num+dimStyle.Render(cat.Label))
		}
	}

	return strings.Join(parts, " ")
}

// --- Overlay screens ---

func (s SecStoreView) renderEmpty() string {
	title := StyleTitle.Render("SecStore")
	catBar := s.renderCategoryBar()
	msg := lipgloss.NewStyle().Foreground(ColorDimmed).Render(
		"\n  All available tools are already installed!\n  Check the TOOLS section in the sidebar.",
	)
	content := title + "\n" + catBar + "\n" + msg
	return lipgloss.NewStyle().Width(s.width).Height(s.height).Padding(0, 1).Render(content)
}

func (s SecStoreView) renderConfirm() string {
	card := s.filtered[s.cursor]
	tool := card.Tool

	title := StyleTitle.Render(fmt.Sprintf("Install %s?", tool.Name()))
	dim := lipgloss.NewStyle().Foreground(ColorDimmed)
	warn := lipgloss.NewStyle().Foreground(ColorWarning)
	ok := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	bad := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)

	content := title + "\n\n" +
		dim.Render("  "+tool.Description()) + "\n\n" +
		warn.Render("  This will install the tool on your system.") + "\n\n" +
		"  " + ok.Render("[y]") + " install  " + bad.Render("[n]") + " cancel"

	return lipgloss.NewStyle().Width(s.width).Height(s.height).Padding(0, 1).Render(content)
}

func (s SecStoreView) renderInstalling() string {
	title := StyleTitle.Render("Installing...")
	msg := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).
		Render("\n  Please wait...")
	return lipgloss.NewStyle().Width(s.width).Height(s.height).Padding(0, 1).Render(title + msg)
}

func (s SecStoreView) renderResult() string {
	dim := lipgloss.NewStyle().Foreground(ColorDimmed)
	var content string

	if s.installErr != nil {
		title := StyleTitle.Render("Installation Failed")
		errStyle := lipgloss.NewStyle().Foreground(ColorCritical)
		content = title + "\n\n" +
			errStyle.Render("  "+s.installID+": "+s.installErr.Error()) + "\n\n" +
			dim.Render("  Press Enter to go back.")
	} else {
		title := StyleTitle.Render("Installed!")
		ok := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
		content = title + "\n\n" +
			ok.Render("  "+s.installID+" installed successfully.") + "\n\n" +
			dim.Render("  Press Enter to continue.")
	}

	return lipgloss.NewStyle().Width(s.width).Height(s.height).Padding(0, 1).Render(content)
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

// truncateLine cuts text to fit in width, adding "…" if truncated.
func truncateLine(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(text) <= width {
		return text
	}
	if width <= 1 {
		return "…"
	}
	return text[:width-1] + "…"
}

// padRight pads a string to width with spaces.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
