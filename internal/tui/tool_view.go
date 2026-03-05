package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

// toolViewState tracks what the tool view is currently showing.
type toolViewState int

const (
	tvIdle    toolViewState = iota
	tvRunning               // action is running
	tvResult                // showing action result
	tvConfirm               // confirming dangerous action
)

type toolActionRequestMsg struct {
	ManagerID string
	ActionID  string
}

type toolActionResultMsg struct {
	Result core.ActionResult
}

type ToolView struct {
	manager       core.ToolManager
	status        core.ServiceStatus
	actions       []core.QuickAction
	config        []core.ConfigEntry
	activity      []core.ActivityEntry
	result        *core.ActionResult
	pendingAction string // action awaiting confirm
	state         toolViewState
	width         int
	height        int
}

func NewToolView(manager core.ToolManager) ToolView {
	tv := ToolView{manager: manager}
	return tv.Refresh()
}

func (v ToolView) Refresh() ToolView {
	if v.manager == nil {
		return v
	}
	v.status = v.manager.GetServiceStatus()
	v.actions = v.manager.QuickActions()
	v.config = v.manager.ConfigSummary()
	v.activity = v.manager.RecentActivity(8)
	return v
}

func (v ToolView) SetSize(w, h int) ToolView {
	v.width = w
	v.height = h
	return v
}

func (v ToolView) Update(msg tea.Msg) (ToolView, tea.Cmd) {
	switch msg := msg.(type) {
	case toolActionResultMsg:
		v.result = &msg.Result
		v.state = tvResult
		return v, nil

	case tea.KeyMsg:
		if v.state == tvResult {
			switch msg.String() {
			case "enter", "esc", " ":
				v.result = nil
				v.state = tvIdle
				v = v.Refresh()
			}
			return v, nil
		}

		if v.state == tvConfirm {
			switch msg.String() {
			case "y", "Y":
				actionID := v.pendingAction
				manager := v.manager
				v.state = tvRunning
				v.pendingAction = ""
				return v, func() tea.Msg {
					res := manager.ExecuteAction(actionID)
					return toolActionResultMsg{Result: res}
				}
			case "n", "N", "esc":
				v.state = tvIdle
				v.pendingAction = ""
			}
			return v, nil
		}

		switch msg.String() {
		case "r":
			v = v.Refresh()
			return v, nil
		case "1", "2", "3", "4":
			idx := int(msg.String()[0]-'0') - 1
			if idx >= 0 && idx < len(v.actions) {
				action := v.actions[idx]
				if action.Dangerous {
					v.state = tvConfirm
					v.pendingAction = action.ID
					return v, nil
				}
				manager := v.manager
				v.state = tvRunning
				return v, func() tea.Msg {
					res := manager.ExecuteAction(action.ID)
					return toolActionResultMsg{Result: res}
				}
			}
		}
	}
	return v, nil
}

func (v ToolView) View() string {
	if v.manager == nil {
		return ""
	}

	if v.state == tvRunning {
		return v.renderRunning()
	}
	if v.state == tvResult && v.result != nil {
		return v.renderResult()
	}
	if v.state == tvConfirm {
		return v.renderConfirm()
	}

	return v.renderPanels()
}

func (v ToolView) ContextHints() []string {
	switch v.state {
	case tvConfirm:
		return []string{"[y] Confirm", "[n] Cancel"}
	case tvResult:
		return []string{"[Enter] Back"}
	default:
		hints := []string{"[1-4] Action", "[r] Refresh", "[h] Back"}
		return hints
	}
}

// --- 4-panel layout ---

func (v ToolView) renderPanels() string {
	cw := v.width - 2 // padding
	if cw < 20 {
		cw = 20
	}

	// Split into 2 columns.
	leftW := cw / 2
	rightW := cw - leftW

	// Title.
	title := StyleTitle.Render(v.manager.Name())

	// Calculate panel heights. Title takes 2 lines (title + blank).
	panelH := (v.height - 2) / 2
	if panelH < 3 {
		panelH = 3
	}

	// Build 4 panels as line arrays.
	statusLines := v.renderStatusPanel(leftW, panelH)
	actionsLines := v.renderActionsPanel(rightW, panelH)
	configLines := v.renderConfigPanel(leftW, panelH)
	activityLines := v.renderActivityPanel(rightW, panelH)

	// Join panels side by side, row by row.
	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')

	sepV := lipgloss.NewStyle().Foreground(ColorDimmed).Render("│")

	// Top row: Status | Actions
	topRows := maxInt(len(statusLines), len(actionsLines))
	for i := 0; i < topRows; i++ {
		left := safeGetLine(statusLines, i, leftW)
		right := safeGetLine(actionsLines, i, rightW)
		b.WriteString(left + sepV + right)
		b.WriteByte('\n')
	}

	// Horizontal separator.
	sepH := lipgloss.NewStyle().Foreground(ColorDimmed).Render(
		strings.Repeat("─", leftW) + "┼" + strings.Repeat("─", rightW))
	b.WriteString(sepH)
	b.WriteByte('\n')

	// Bottom row: Config | Activity
	botRows := maxInt(len(configLines), len(activityLines))
	for i := 0; i < botRows; i++ {
		left := safeGetLine(configLines, i, leftW)
		right := safeGetLine(activityLines, i, rightW)
		b.WriteString(left + sepV + right)
		b.WriteByte('\n')
	}

	return lipgloss.NewStyle().
		Width(v.width).Height(v.height).
		Padding(0, 1).
		Render(b.String())
}

func (v ToolView) renderStatusPanel(w, h int) []string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning)

	var lines []string
	lines = append(lines, headerStyle.Render("Status"))

	if v.status.Running {
		lines = append(lines, " Service: "+okStyle.Render("Active"))
	} else {
		lines = append(lines, " Service: "+warnStyle.Render("Inactive"))
	}

	if v.status.Enabled {
		lines = append(lines, " Enabled: "+okStyle.Render("Yes"))
	} else {
		lines = append(lines, " Enabled: "+warnStyle.Render("No"))
	}

	if v.status.PID > 0 {
		lines = append(lines, fmt.Sprintf(" PID: %s", dimStyle.Render(fmt.Sprintf("%d", v.status.PID))))
	}

	// Extra info (version, etc.)
	if ver, ok := v.status.Extra["version"]; ok {
		lines = append(lines, " Version: "+dimStyle.Render(ver))
	}
	if jails, ok := v.status.Extra["jails"]; ok {
		lines = append(lines, " Jails: "+dimStyle.Render(jails))
	}
	if enforce, ok := v.status.Extra["enforce"]; ok {
		lines = append(lines, " Enforce: "+dimStyle.Render(enforce))
	}
	if complain, ok := v.status.Extra["complain"]; ok {
		lines = append(lines, " Complain: "+dimStyle.Render(complain))
	}

	return padToHeight(lines, h)
}

func (v ToolView) renderActionsPanel(w, h int) []string {
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning)
	keyStyle := StyleKeyhint

	var lines []string
	lines = append(lines, headerStyle.Render("Quick Actions"))

	for _, a := range v.actions {
		label := a.Label
		if a.Dangerous {
			label += " " + warnStyle.Render("⚠")
		}
		line := fmt.Sprintf(" %s %s  %s",
			keyStyle.Render(fmt.Sprintf("[%c]", a.Key)),
			label,
			dimStyle.Render(a.Description),
		)
		// Truncate if too wide.
		if lipgloss.Width(line) > w {
			line = fmt.Sprintf(" %s %s",
				keyStyle.Render(fmt.Sprintf("[%c]", a.Key)),
				label,
			)
		}
		lines = append(lines, line)
	}

	return padToHeight(lines, h)
}

func (v ToolView) renderConfigPanel(w, h int) []string {
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var lines []string
	lines = append(lines, headerStyle.Render("Configuration"))

	for _, c := range v.config {
		line := fmt.Sprintf(" %s = %s", c.Key, dimStyle.Render(c.Value))
		lines = append(lines, line)
	}

	return padToHeight(lines, h)
}

func (v ToolView) renderActivityPanel(w, h int) []string {
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var lines []string
	lines = append(lines, headerStyle.Render("Recent Activity"))

	if len(v.activity) == 0 {
		lines = append(lines, " "+dimStyle.Render("No recent activity."))
	} else {
		for _, a := range v.activity {
			ts := a.Timestamp
			if len(ts) > 8 {
				// Show just time portion if possible.
				parts := strings.Fields(ts)
				if len(parts) >= 3 {
					ts = parts[2] // HH:MM:SS
				}
			}
			msg := a.Message
			maxMsg := w - len(ts) - 3
			if maxMsg > 0 && len(msg) > maxMsg {
				msg = msg[:maxMsg-1] + "…"
			}
			lines = append(lines, fmt.Sprintf(" %s %s", dimStyle.Render(ts), msg))
		}
	}

	return padToHeight(lines, h)
}

// --- Overlay screens ---

func (v ToolView) renderRunning() string {
	title := StyleTitle.Render(v.manager.Name())
	msg := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).
		Render("\n  Running action, please wait...")
	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(title + msg)
}

func (v ToolView) renderResult() string {
	title := StyleTitle.Render(v.manager.Name() + " — Action Result")
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var content string
	if v.result.Success {
		okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
		content = title + "\n\n" +
			okStyle.Render("  ✓ Success") + "\n\n" +
			wrapOutput(v.result.Message, v.width-6) + "\n\n" +
			dimStyle.Render("  Press Enter to go back.")
	} else {
		errStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
		content = title + "\n\n" +
			errStyle.Render("  ✗ Failed") + "\n\n" +
			wrapOutput(v.result.Message, v.width-6) + "\n\n" +
			dimStyle.Render("  Press Enter to go back.")
	}

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

func (v ToolView) renderConfirm() string {
	title := StyleTitle.Render(v.manager.Name() + " — Confirm Action")
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	okBtn := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	badBtn := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)

	// Find the action label.
	label := v.pendingAction
	for _, a := range v.actions {
		if a.ID == v.pendingAction {
			label = a.Label
			break
		}
	}

	content := title + "\n\n" +
		warnStyle.Render(fmt.Sprintf("  Execute: %s?", label)) + "\n\n" +
		dimStyle.Render("  This is a dangerous operation.") + "\n\n" +
		"  " + okBtn.Render("[y]") + " confirm  " + badBtn.Render("[n]") + " cancel"

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

// --- Helpers ---

// padToHeight ensures a slice has exactly h lines.
func padToHeight(lines []string, h int) []string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

// safeGetLine returns the i-th line padded to width, or an empty padded line.
func safeGetLine(lines []string, i int, w int) string {
	line := ""
	if i < len(lines) {
		line = lines[i]
	}
	// Pad to exact width using visible width.
	visible := lipgloss.Width(line)
	if visible < w {
		line += strings.Repeat(" ", w-visible)
	}
	return line
}

// wrapOutput indents multi-line command output.
func wrapOutput(text string, maxW int) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if maxW > 0 && len(line) > maxW {
			line = line[:maxW-1] + "…"
		}
		lines = append(lines, "  "+line)
	}
	return strings.Join(lines, "\n")
}
