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
	tvRunning               // synchronous action is running (fast queries)
	tvResult                // showing action result
	tvConfirm               // confirming dangerous action
	tvJobView               // viewing a background job (live output)
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
	scrollOffset  int // vertical scroll for result/job view
	spinnerFrame  int
	jobs          *JobManager
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

func (v ToolView) SetJobs(jm *JobManager) ToolView {
	v.jobs = jm
	return v
}

func (v ToolView) SetSpinnerFrame(frame int) ToolView {
	v.spinnerFrame = frame
	return v
}

func (v ToolView) Update(msg tea.Msg) (ToolView, tea.Cmd) {
	switch msg := msg.(type) {
	case toolActionResultMsg:
		v.result = &msg.Result
		v.state = tvResult
		v.scrollOffset = 0
		return v, nil

	case jobCompletedMsg:
		// If viewing this tool's job, stay in job view (user will see completed state).
		return v, nil

	case tea.KeyMsg:
		// Job view: live output or completed result.
		if v.state == tvJobView {
			return v.handleJobViewKeys(msg)
		}

		if v.state == tvResult {
			switch msg.String() {
			case "enter", "esc", " ":
				v.result = nil
				v.state = tvIdle
				v.scrollOffset = 0
				v = v.Refresh()
			case "j", "down":
				v.scrollOffset++
			case "k", "up":
				if v.scrollOffset > 0 {
					v.scrollOffset--
				}
			case "g":
				v.scrollOffset = 0
			}
			return v, nil
		}

		if v.state == tvConfirm {
			switch msg.String() {
			case "y", "Y":
				actionID := v.pendingAction
				manager := v.manager
				v.pendingAction = ""

				// Check if this is a background action.
				if backgroundActions[actionID] && v.jobs != nil {
					label := actionID
					for _, a := range v.actions {
						if a.ID == actionID {
							label = a.Label
							break
						}
					}
					job := v.jobs.Start(manager.ToolID(), actionID, label)
					v.state = tvJobView
					v.scrollOffset = 0
					return v, func() tea.Msg {
						result := manager.ExecuteAction(actionID)
						return jobCompletedMsg{JobID: job.ID, Result: result}
					}
				}

				v.state = tvRunning
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

		// tvIdle
		switch msg.String() {
		case "r":
			v = v.Refresh()
			return v, nil
		case "1", "2", "3", "4":
			idx := int(msg.String()[0]-'0') - 1
			if idx >= 0 && idx < len(v.actions) {
				action := v.actions[idx]

				// If this tool already has a running background job, show it.
				if v.jobs != nil && v.jobs.HasRunning(v.manager.ToolID()) {
					v.state = tvJobView
					v.scrollOffset = 0
					return v, nil
				}

				// If there's a completed job for this tool, show it.
				if v.jobs != nil {
					if completed := v.jobs.CompletedJobFor(v.manager.ToolID()); completed != nil {
						v.state = tvJobView
						v.scrollOffset = 0
						return v, nil
					}
				}

				if action.Dangerous {
					v.state = tvConfirm
					v.pendingAction = action.ID
					return v, nil
				}

				// Background action dispatch.
				if backgroundActions[action.ID] && v.jobs != nil {
					job := v.jobs.Start(v.manager.ToolID(), action.ID, action.Label)
					v.state = tvJobView
					v.scrollOffset = 0
					manager := v.manager
					return v, func() tea.Msg {
						result := manager.ExecuteAction(action.ID)
						return jobCompletedMsg{JobID: job.ID, Result: result}
					}
				}

				// Synchronous action (fast).
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

func (v ToolView) handleJobViewKeys(msg tea.KeyMsg) (ToolView, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		// If the job is done, dismiss it and go back to idle.
		if v.jobs != nil {
			if completed := v.jobs.CompletedJobFor(v.manager.ToolID()); completed != nil {
				v.jobs.Dismiss(completed.ID)
			}
		}
		v.state = tvIdle
		v.scrollOffset = 0
		v = v.Refresh()
		return v, nil
	case "enter":
		// If job is done, dismiss and go back.
		if v.jobs != nil {
			if completed := v.jobs.CompletedJobFor(v.manager.ToolID()); completed != nil {
				v.jobs.Dismiss(completed.ID)
				v.state = tvIdle
				v.scrollOffset = 0
				v = v.Refresh()
				return v, nil
			}
		}
	case "j", "down":
		v.scrollOffset++
	case "k", "up":
		if v.scrollOffset > 0 {
			v.scrollOffset--
		}
	case "g":
		v.scrollOffset = 0
	}
	return v, nil
}

func (v ToolView) View() string {
	if v.manager == nil {
		return ""
	}

	if v.state == tvJobView {
		return v.renderJobView()
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

	// If this tool has a running or completed job, show indicator in the panels.
	return v.renderPanels()
}

func (v ToolView) ContextHints() []string {
	switch v.state {
	case tvConfirm:
		return []string{"[y] Confirm", "[n] Cancel"}
	case tvResult:
		return []string{"[j/k] Scroll", "[Enter] Back"}
	case tvJobView:
		job := v.currentJob()
		if job != nil && job.Done {
			return []string{"[j/k] Scroll", "[Enter] Dismiss", "[Esc] Back"}
		}
		return []string{"[j/k] Scroll", "[Esc] Back (job continues)"}
	default:
		hints := []string{"[1-4] Action", "[r] Refresh", "[h] Back"}
		return hints
	}
}

// currentJob returns the running or completed job for this tool.
func (v ToolView) currentJob() *Job {
	if v.jobs == nil || v.manager == nil {
		return nil
	}
	toolID := v.manager.ToolID()
	if job := v.jobs.RunningJobFor(toolID); job != nil {
		return job
	}
	return v.jobs.CompletedJobFor(toolID)
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

	// Show active job indicator next to title.
	if v.jobs != nil {
		if job := v.jobs.RunningJobFor(v.manager.ToolID()); job != nil {
			frame := spinnerFrames[v.spinnerFrame%len(spinnerFrames)]
			warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
			title += "  " + warnStyle.Render(frame+" "+job.Label) +
				"  " + dimStyle.Render(FormatElapsed(job.Elapsed()))
		}
	}

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

func (v ToolView) renderJobView() string {
	job := v.currentJob()
	if job == nil {
		// Fallback: no job found, go back to panels.
		return v.renderPanels()
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	var lines []string

	// Title with spinner or completion status.
	if job.Done {
		if job.Result != nil && job.Result.Success {
			okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
			lines = append(lines, StyleTitle.Render(v.manager.Name()+" — "+job.Label)+"  "+
				okStyle.Render("✓ Complete")+"  "+
				dimStyle.Render(FormatElapsed(job.Elapsed())))
		} else {
			failStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
			lines = append(lines, StyleTitle.Render(v.manager.Name()+" — "+job.Label)+"  "+
				failStyle.Render("✗ Failed")+"  "+
				dimStyle.Render(FormatElapsed(job.Elapsed())))
		}
	} else {
		frame := spinnerFrames[v.spinnerFrame%len(spinnerFrames)]
		lines = append(lines, StyleTitle.Render(v.manager.Name()+" — "+job.Label)+"  "+
			warnStyle.Render(frame)+"  "+
			dimStyle.Render(FormatElapsed(job.Elapsed())))
	}
	lines = append(lines, "")

	// Output content.
	if job.Done && job.Result != nil {
		// Show formatted result.
		lines = append(lines, v.formatActionOutput(job.Result)...)
	} else {
		// Show live output buffer.
		output := job.Output()
		if output == "" {
			lines = append(lines, "  "+dimStyle.Render("Waiting for output..."))
		} else {
			for _, line := range strings.Split(output, "\n") {
				if len(line) > v.width-6 {
					line = line[:v.width-7] + "…"
				}
				lines = append(lines, "  "+valStyle.Render(line))
			}
		}
	}
	lines = append(lines, "")

	// Scroll and render.
	totalLines := len(lines)
	viewH := v.height - 2
	if viewH < 5 {
		viewH = 5
	}

	maxScroll := totalLines - viewH + 2
	if maxScroll < 0 {
		maxScroll = 0
	}
	offset := v.scrollOffset
	if offset > maxScroll {
		offset = maxScroll
	}

	endIdx := offset + viewH - 1
	if endIdx > totalLines {
		endIdx = totalLines
	}
	visible := lines[offset:endIdx]

	var footer string
	if job.Done {
		if totalLines > viewH {
			pct := 0
			if maxScroll > 0 {
				pct = offset * 100 / maxScroll
			}
			footer = dimStyle.Render(fmt.Sprintf("  [j/k] Scroll  [g] Top  (%d%%)  [Enter] Dismiss", pct))
		} else {
			footer = dimStyle.Render("  [Enter] Dismiss  [Esc] Back")
		}
	} else {
		if totalLines > viewH {
			pct := 0
			if maxScroll > 0 {
				pct = offset * 100 / maxScroll
			}
			footer = dimStyle.Render(fmt.Sprintf("  [j/k] Scroll  [g] Top  (%d%%)  [Esc] Back (job continues)", pct))
		} else {
			footer = dimStyle.Render("  [Esc] Back (job continues in background)")
		}
	}

	content := strings.Join(visible, "\n") + "\n" + footer

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

// formatActionOutput formats an ActionResult message into styled lines.
func (v ToolView) formatActionOutput(result *core.ActionResult) []string {
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	keyLabelStyle := lipgloss.NewStyle().Foreground(ColorInfo).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	sepStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	maxW := v.width - 6
	if maxW < 20 {
		maxW = 20
	}

	var lines []string

	if result.Success {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorOK).Bold(true).Render("  ✓ Success"))
	} else {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorCritical).Bold(true).Render("  ✗ Failed"))
	}
	lines = append(lines, "")

	rawLines := strings.Split(result.Message, "\n")
	for _, rl := range rawLines {
		trimmed := strings.TrimSpace(rl)
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}

		if strings.HasSuffix(trimmed, ":") && len(trimmed) < 60 && !strings.Contains(trimmed, "=") {
			lines = append(lines, "  "+headerStyle.Render(trimmed))
			continue
		}

		stripped := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(trimmed, "-", ""), "+", ""), "=", "")
		if len(stripped) == 0 && len(trimmed) > 2 {
			sep := trimmed
			if len(sep) > maxW {
				sep = sep[:maxW]
			}
			lines = append(lines, "  "+sepStyle.Render(sep))
			continue
		}

		if colonIdx := strings.Index(trimmed, ":"); colonIdx > 0 && colonIdx < 40 {
			key := strings.TrimSpace(trimmed[:colonIdx])
			val := strings.TrimSpace(trimmed[colonIdx+1:])
			if !strings.Contains(key, "/") && len(key) < 35 {
				key = strings.TrimLeft(key, "|- ")
				line := fmt.Sprintf("  %s: %s", keyLabelStyle.Render(key), valStyle.Render(val))
				if lipgloss.Width(line) > maxW+2 {
					availW := maxW - lipgloss.Width(keyLabelStyle.Render(key)) - 4
					if availW > 3 && len(val) > availW {
						val = val[:availW-1] + "…"
					}
					line = fmt.Sprintf("  %s: %s", keyLabelStyle.Render(key), valStyle.Render(val))
				}
				lines = append(lines, line)
				continue
			}
		}

		if strings.Contains(trimmed, "|") && strings.Count(trimmed, "|") >= 2 {
			row := trimmed
			if len(row) > maxW {
				row = row[:maxW-1] + "…"
			}
			lines = append(lines, "  "+dimStyle.Render(row))
			continue
		}

		if (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")) ||
			(len(trimmed) > 2 && trimmed[0] >= '0' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')')) {
			item := trimmed
			if len(item) > maxW {
				item = item[:maxW-1] + "…"
			}
			lines = append(lines, "  "+dimStyle.Render("  ")+valStyle.Render(item))
			continue
		}

		indent := len(rl) - len(strings.TrimLeft(rl, " \t"))
		prefix := "  "
		if indent > 0 {
			prefix += strings.Repeat(" ", minInt(indent, 8))
		}
		text := strings.TrimSpace(rl)
		if len(text)+len(prefix) > maxW+2 {
			avail := maxW - len(prefix) + 2
			if avail > 3 && len(text) > avail {
				text = text[:avail-1] + "…"
			}
		}
		lines = append(lines, prefix+valStyle.Render(text))
	}

	return lines
}

func (v ToolView) renderResult() string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var lines []string

	lines = append(lines, StyleTitle.Render(v.manager.Name()+" — Action Result"))
	lines = append(lines, "")

	if v.result != nil {
		lines = append(lines, v.formatActionOutput(v.result)...)
	}

	lines = append(lines, "")

	totalLines := len(lines)
	viewH := v.height - 2
	if viewH < 5 {
		viewH = 5
	}

	maxScroll := totalLines - viewH + 2
	if maxScroll < 0 {
		maxScroll = 0
	}
	offset := v.scrollOffset
	if offset > maxScroll {
		offset = maxScroll
	}

	endIdx := offset + viewH - 1
	if endIdx > totalLines {
		endIdx = totalLines
	}
	visible := lines[offset:endIdx]

	var footer string
	if totalLines > viewH {
		pct := 0
		if maxScroll > 0 {
			pct = offset * 100 / maxScroll
		}
		footer = dimStyle.Render(fmt.Sprintf("  [j/k] Scroll  [g] Top  (%d%%)  [Enter] Back", pct))
	} else {
		footer = dimStyle.Render("  [Enter] Back")
	}

	content := strings.Join(visible, "\n") + "\n" + footer

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

func (v ToolView) renderConfirm() string {
	title := StyleTitle.Render(v.manager.Name() + " — Confirm Action")
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	critStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
	okBtn := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	badBtn := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)

	// Find the action label and description.
	label := v.pendingAction
	description := ""
	for _, a := range v.actions {
		if a.ID == v.pendingAction {
			label = a.Label
			description = a.Description
			break
		}
	}

	var content string
	content = title + "\n\n" +
		warnStyle.Render(fmt.Sprintf("  ⚠ Execute: %s?", label)) + "\n\n"

	if description != "" {
		content += "  " + critStyle.Render(description) + "\n\n"
	}

	content += dimStyle.Render("  This action requires explicit confirmation.") + "\n\n" +
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
