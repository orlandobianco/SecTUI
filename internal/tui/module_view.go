package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

// ModuleView displays the findings for a single security module (SSH, Firewall, etc.)
// and allows the user to select findings for remediation.
type ModuleView struct {
	moduleID string
	findings []core.Finding
	cursor   int
	selected map[int]bool // indices toggled for fix
	width    int
	height   int
}

// NewModuleView creates a ModuleView for a given module with its findings.
func NewModuleView(moduleID string, findings []core.Finding) ModuleView {
	return ModuleView{
		moduleID: moduleID,
		findings: findings,
		cursor:   0,
		selected: make(map[int]bool),
	}
}

func (m ModuleView) Init() tea.Cmd {
	return nil
}

// ApplyFixRequestMsg is sent when the user presses Enter with selected fixes.
// The parent App handles this message to run the actual fix flow.
type ApplyFixRequestMsg struct {
	ModuleID string
	Findings []core.Finding
}

func (m ModuleView) Update(msg tea.Msg) (ModuleView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.findings)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ":
			m.toggleCurrent()
		case "a":
			m.toggleAll()
		case "enter":
			selected := m.SelectedFindings()
			if len(selected) > 0 {
				return m, func() tea.Msg {
					return ApplyFixRequestMsg{
						ModuleID: m.moduleID,
						Findings: selected,
					}
				}
			}
		}
	}
	return m, nil
}

func (m ModuleView) View() string {
	if len(m.findings) == 0 {
		return m.renderEmpty()
	}

	var sections []string

	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderFindings())
	sections = append(sections, m.renderDetail())

	content := strings.Join(sections, "\n")
	return StyleContent.Width(m.width).Height(m.height).Render(content)
}

// SetSize updates the dimensions available for rendering.
func (m ModuleView) SetSize(w, h int) ModuleView {
	m.width = w
	m.height = h
	return m
}

// SelectedFindings returns the findings the user has toggled for remediation.
func (m ModuleView) SelectedFindings() []core.Finding {
	var result []core.Finding
	for i, f := range m.findings {
		if m.selected[i] {
			result = append(result, f)
		}
	}
	return result
}

// --- private helpers ---

func (m *ModuleView) toggleCurrent() {
	if m.cursor < 0 || m.cursor >= len(m.findings) {
		return
	}
	// Only allow toggling findings that have an available fix.
	if m.findings[m.cursor].FixID == "" {
		return
	}
	m.selected[m.cursor] = !m.selected[m.cursor]
}

func (m *ModuleView) toggleAll() {
	allFixable := true
	for i, f := range m.findings {
		if f.FixID != "" && !m.selected[i] {
			allFixable = false
			break
		}
	}
	for i, f := range m.findings {
		if f.FixID != "" {
			m.selected[i] = !allFixable
		}
	}
}

func (m ModuleView) renderEmpty() string {
	title := StyleTitle.Render(fmt.Sprintf("%s Module", m.moduleName()))
	hint := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render("No findings. Run a scan to check this module.")
	content := title + "\n\n" + hint
	return StyleContent.Width(m.width).Height(m.height).Render(content)
}

func (m ModuleView) renderHeader() string {
	title := StyleTitle.Render(fmt.Sprintf("%s Configuration", m.moduleName()))

	counts := m.severityCounts()
	score := core.CalculateModuleScore(m.findings, m.moduleID)

	critStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
	highStyle := lipgloss.NewStyle().Foreground(ColorCritical)
	medStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	lowStyle := lipgloss.NewStyle().Foreground(ColorInfo)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var parts []string
	parts = append(parts, dimStyle.Render(fmt.Sprintf("%d findings", len(m.findings))))
	if c := counts[core.SeverityCritical]; c > 0 {
		parts = append(parts, critStyle.Render(fmt.Sprintf("%d CRIT", c)))
	}
	if c := counts[core.SeverityHigh]; c > 0 {
		parts = append(parts, highStyle.Render(fmt.Sprintf("%d HIGH", c)))
	}
	if c := counts[core.SeverityMedium]; c > 0 {
		parts = append(parts, medStyle.Render(fmt.Sprintf("%d MED", c)))
	}
	if c := counts[core.SeverityLow]; c > 0 {
		parts = append(parts, lowStyle.Render(fmt.Sprintf("%d LOW", c)))
	}

	scoreColor := ColorCritical
	if score >= 80 {
		scoreColor = ColorOK
	} else if score >= 50 {
		scoreColor = ColorWarning
	}
	scoreStyle := lipgloss.NewStyle().Foreground(scoreColor).Bold(true)
	parts = append(parts, scoreStyle.Render(fmt.Sprintf("Score: %d", score)))

	summary := "  " + strings.Join(parts, "   ")

	sep := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("\u2500", maxInt(m.width-6, 10)))

	return title + "\n" + summary + "\n" + sep
}

func (m ModuleView) renderFindings() string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	sectionLabel := dimStyle.Render("  Findings")

	// Determine how many findings lines we can show.
	// Reserve space: header ~3 lines, detail ~6 lines, section label ~2 lines.
	maxVisible := m.height - 12
	if maxVisible < 3 {
		maxVisible = 3
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.findings) {
		endIdx = len(m.findings)
	}

	var lines []string
	for i := startIdx; i < endIdx; i++ {
		f := m.findings[i]
		lines = append(lines, m.renderFindingLine(i, f))
	}

	if len(m.findings) > maxVisible {
		scrollHint := dimStyle.Render(
			fmt.Sprintf("  ... %d/%d findings (scroll with j/k)", m.cursor+1, len(m.findings)),
		)
		lines = append(lines, scrollHint)
	}

	return sectionLabel + "\n" + strings.Join(lines, "\n")
}

func (m ModuleView) renderFindingLine(idx int, f core.Finding) string {
	checkbox := "[ ]"
	if m.selected[idx] {
		checkbox = "[x]"
	}
	if f.FixID == "" {
		checkbox = " - "
	}

	sevLabel := m.severityLabel(f.Severity)

	// Resolve i18n key to actual translated title.
	title := core.T(f.TitleKey)
	if title == f.TitleKey {
		// Fallback if key not found: extract the check name (second-to-last segment).
		if parts := strings.Split(f.TitleKey, "."); len(parts) >= 3 {
			title = parts[len(parts)-2]
			title = strings.ReplaceAll(title, "_", " ")
			title = strings.Title(title) //nolint:staticcheck // acceptable for display
		}
	}

	isCurrent := idx == m.cursor

	lineContent := fmt.Sprintf("  %s %s  %s", checkbox, sevLabel, title)

	if isCurrent {
		return lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true).
			Render(lineContent)
	}
	return lipgloss.NewStyle().Foreground(ColorText).Render(lineContent)
}

func (m ModuleView) renderDetail() string {
	sep := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("\u2500", maxInt(m.width-6, 10)))

	if m.cursor < 0 || m.cursor >= len(m.findings) {
		return sep + "\n" + lipgloss.NewStyle().Foreground(ColorDimmed).Render("  No finding selected.")
	}

	f := m.findings[m.cursor]

	dimLabel := lipgloss.NewStyle().Foreground(ColorDimmed)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	var detail strings.Builder
	detail.WriteString(sep)
	detail.WriteString("\n")

	// WHY explanation - resolve i18n key.
	why := core.T(f.DetailKey)
	if why == f.DetailKey {
		// Fallback if key not found.
		if parts := strings.Split(f.DetailKey, "."); len(parts) >= 3 {
			why = parts[len(parts)-2]
			why = strings.ReplaceAll(why, "_", " ")
		}
	}
	detail.WriteString(fmt.Sprintf("  %s %s\n", dimLabel.Render("WHY:"), accentStyle.Render(why)))

	// Current / Expected values
	if f.CurrentValue != "" || f.ExpectedValue != "" {
		detail.WriteString(fmt.Sprintf("  %s %s    %s %s\n",
			dimLabel.Render("Current:"), valStyle.Render(f.CurrentValue),
			dimLabel.Render("Expected:"), valStyle.Render(f.ExpectedValue),
		))
	}

	// Fix availability
	if f.FixID != "" {
		detail.WriteString(fmt.Sprintf("  %s %s\n",
			dimLabel.Render("FIX:"),
			lipgloss.NewStyle().Foreground(ColorOK).Render("Auto-fix available ("+f.FixID+")"),
		))
	} else {
		detail.WriteString(fmt.Sprintf("  %s %s\n",
			dimLabel.Render("FIX:"),
			dimLabel.Render("Manual remediation required"),
		))
	}

	return detail.String()
}

// ContextHints returns key hints for the dynamic footer.
func (m ModuleView) ContextHints() []string {
	selectedCount := 0
	for _, v := range m.selected {
		if v {
			selectedCount++
		}
	}

	hints := []string{"[j/k] Browse", "[Space] Toggle fix"}

	if selectedCount > 0 {
		label := fmt.Sprintf("[Enter] Apply %d fix", selectedCount)
		if selectedCount > 1 {
			label += "es"
		}
		hints = append(hints, label)
	}

	hints = append(hints, "[h] Back")
	return hints
}

func (m ModuleView) severityCounts() map[core.Severity]int {
	counts := make(map[core.Severity]int)
	for _, f := range m.findings {
		counts[f.Severity]++
	}
	return counts
}

func (m ModuleView) severityLabel(s core.Severity) string {
	switch s {
	case core.SeverityCritical:
		return lipgloss.NewStyle().Foreground(ColorCritical).Bold(true).Width(5).Render("CRIT")
	case core.SeverityHigh:
		return lipgloss.NewStyle().Foreground(ColorCritical).Width(5).Render("HIGH")
	case core.SeverityMedium:
		return lipgloss.NewStyle().Foreground(ColorWarning).Width(5).Render("MED")
	case core.SeverityLow:
		return lipgloss.NewStyle().Foreground(ColorInfo).Width(5).Render("LOW")
	case core.SeverityInfo:
		return lipgloss.NewStyle().Foreground(ColorDimmed).Width(5).Render("INFO")
	default:
		return lipgloss.NewStyle().Foreground(ColorDimmed).Width(5).Render("?")
	}
}

func (m ModuleView) moduleName() string {
	switch m.moduleID {
	case "ssh":
		return "SSH"
	case "firewall":
		return "Firewall"
	case "network":
		return "Network"
	case "users":
		return "Users & Perms"
	case "updates":
		return "Updates"
	case "kernel":
		return "Kernel"
	default:
		return strings.Title(m.moduleID) //nolint:staticcheck
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
