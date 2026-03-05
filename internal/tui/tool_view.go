package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/orlandobianco/SecTUI/internal/core"
)

// toolViewState tracks what the tool view is currently showing.
type toolViewState int

const (
	tvIdle          toolViewState = iota
	tvRunning                     // synchronous action is running (fast queries)
	tvResult                      // showing action result
	tvConfirm                     // confirming dangerous action
	tvJobView                     // viewing a background job (live output from file)
	tvHistory                     // viewing scan history table
	tvHistoryDetail               // viewing a single history record detail
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

	// History
	historyTable   table.Model
	historyRecords []ScanRecord
	historyDetail  *ScanRecord
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

	case tea.KeyPressMsg:
		if v.state == tvHistory {
			return v.handleHistoryKeys(msg)
		}
		if v.state == tvHistoryDetail {
			return v.handleHistoryDetailKeys(msg)
		}
		if v.state == tvJobView {
			return v.handleJobViewKeys(msg)
		}

		if v.state == tvResult {
			switch msg.String() {
			case "enter", "esc", "space":
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
				v.pendingAction = ""

				// Background action via external process.
				if backgroundActions[actionID] && v.jobs != nil {
					label := actionID
					for _, a := range v.actions {
						if a.ID == actionID {
							label = a.Label
							break
						}
					}
					_, err := v.jobs.LaunchJob(v.manager.ToolID(), actionID, label)
					if err != nil {
						v.result = &core.ActionResult{Success: false, Message: err.Error()}
						v.state = tvResult
						return v, nil
					}
					v.state = tvJobView
					v.scrollOffset = 0
					return v, nil
				}

				// Synchronous action.
				manager := v.manager
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
		case "5":
			if v.jobs != nil && v.manager != nil && v.hasBackgroundActions() {
				records := v.jobs.LoadHistory(v.manager.ToolID())
				if len(records) > 0 {
					v.jobs.MarkAllSeen(v.manager.ToolID())
					v.historyRecords = records
					v.historyTable = v.buildHistoryTable(records)
					v.state = tvHistory
				}
			}
			return v, nil
		case "1", "2", "3", "4":
			idx := int(msg.String()[0]-'0') - 1
			if idx >= 0 && idx < len(v.actions) {
				action := v.actions[idx]

				// If this tool already has a running background job, show monitor.
				if v.jobs != nil && v.jobs.HasRunning(v.manager.ToolID()) {
					v.state = tvJobView
					v.scrollOffset = 0
					return v, nil
				}

				// Archive any completed job before starting new one.
				if v.jobs != nil {
					if completed := v.jobs.CompletedJobFor(v.manager.ToolID()); completed != nil {
						_ = v.jobs.ArchiveJob(completed.ID)
					}
				}

				if action.Dangerous {
					v.state = tvConfirm
					v.pendingAction = action.ID
					return v, nil
				}

				// Background action dispatch via external process.
				if backgroundActions[action.ID] && v.jobs != nil {
					_, err := v.jobs.LaunchJob(v.manager.ToolID(), action.ID, action.Label)
					if err != nil {
						v.result = &core.ActionResult{Success: false, Message: err.Error()}
						v.state = tvResult
						return v, nil
					}
					v.state = tvJobView
					v.scrollOffset = 0
					return v, nil
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

func (v ToolView) handleJobViewKeys(msg tea.KeyPressMsg) (ToolView, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		v.state = tvIdle
		v.scrollOffset = 0
		v = v.Refresh()
		return v, nil
	case "enter":
		if v.jobs != nil {
			if completed := v.jobs.CompletedJobFor(v.manager.ToolID()); completed != nil {
				_ = v.jobs.ArchiveJob(completed.ID)
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

	switch v.state {
	case tvHistory:
		return v.renderHistory()
	case tvHistoryDetail:
		return v.renderHistoryDetail()
	case tvJobView:
		return v.renderJobView()
	case tvRunning:
		return v.renderRunning()
	case tvResult:
		if v.result != nil {
			return v.renderResult()
		}
	case tvConfirm:
		return v.renderConfirm()
	}

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
			return []string{"[j/k] Scroll", "[Enter] Archive", "[Esc] Back"}
		}
		return []string{"[j/k] Scroll", "[Esc] Back (job continues)"}
	case tvHistory:
		return []string{"[j/k] Navigate", "[Enter] Detail", "[d] Delete", "[Esc] Back"}
	case tvHistoryDetail:
		return []string{"[j/k] Scroll", "[Esc] Back to history"}
	default:
		return []string{"[1-5] Action", "[r] Refresh", "[h] Back"}
	}
}

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
	cw := v.width - 2
	if cw < 20 {
		cw = 20
	}

	leftW := cw / 2
	rightW := cw - leftW

	title := StyleTitle.Render(v.manager.Name())

	if v.jobs != nil {
		if job := v.jobs.RunningJobFor(v.manager.ToolID()); job != nil {
			frame := spinnerFrames[v.spinnerFrame%len(spinnerFrames)]
			warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
			title += "  " + warnStyle.Render(frame+" "+job.Label) +
				"  " + dimStyle.Render(FormatElapsed(job.Elapsed()))
		}
	}

	panelH := (v.height - 2) / 2
	if panelH < 3 {
		panelH = 3
	}

	statusLines := v.renderStatusPanel(leftW, panelH)
	actionsLines := v.renderActionsPanel(rightW, panelH)
	configLines := v.renderConfigPanel(leftW, panelH)
	activityLines := v.renderActivityPanel(rightW, panelH)

	var b strings.Builder
	b.WriteString(title)
	b.WriteByte('\n')

	sepV := lipgloss.NewStyle().Foreground(ColorDimmed).Render("│")

	topRows := maxInt(len(statusLines), len(actionsLines))
	for i := 0; i < topRows; i++ {
		left := safeGetLine(statusLines, i, leftW)
		right := safeGetLine(actionsLines, i, rightW)
		b.WriteString(left + sepV + right)
		b.WriteByte('\n')
	}

	sepH := lipgloss.NewStyle().Foreground(ColorDimmed).Render(
		strings.Repeat("─", leftW) + "┼" + strings.Repeat("─", rightW))
	b.WriteString(sepH)
	b.WriteByte('\n')

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
		if lipgloss.Width(line) > w {
			line = fmt.Sprintf(" %s %s",
				keyStyle.Render(fmt.Sprintf("[%c]", a.Key)),
				label,
			)
		}
		lines = append(lines, line)
	}

	// Append [5] Scan History if tool has background actions.
	if v.hasBackgroundActions() && v.jobs != nil {
		unseen := v.jobs.UnseenCount(v.manager.ToolID())
		records := v.jobs.LoadHistory(v.manager.ToolID())
		count := len(records)

		label := "Scan History"
		badge := ""
		if unseen > 0 {
			badge = fmt.Sprintf(" (%d new)", unseen)
		} else if count > 0 {
			badge = fmt.Sprintf(" (%d)", count)
		}

		line := fmt.Sprintf(" %s %s", keyStyle.Render("[5]"), label)
		if badge != "" {
			if unseen > 0 {
				line += warnStyle.Render(badge)
			} else {
				line += dimStyle.Render(badge)
			}
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
				parts := strings.Fields(ts)
				if len(parts) >= 3 {
					ts = parts[2]
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
		return v.renderPanels()
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	var lines []string

	// Title with spinner or completion status.
	if job.Done {
		if job.Success {
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

	if job.Done {
		// Show scan result table for completed jobs.
		lines = append(lines, v.renderScanResultTable(job)...)
	} else {
		// Show live output from log file.
		output := job.ReadNewOutput()
		fullOutput := job.FullOutput()

		if fullOutput == "" && output == "" {
			lines = append(lines, "  "+dimStyle.Render("Starting..."))
		} else {
			// Show last N lines of output (tail-like).
			allLines := strings.Split(strings.TrimRight(fullOutput, "\n"), "\n")
			maxVisible := v.height - 8
			if maxVisible < 5 {
				maxVisible = 5
			}
			start := 0
			if len(allLines) > maxVisible {
				start = len(allLines) - maxVisible
			}
			for _, line := range allLines[start:] {
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
		footer = dimStyle.Render("  [Esc] Back (job continues in background)")
	}

	content := strings.Join(visible, "\n") + "\n" + footer

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

// renderScanResultTable builds a summary table from the completed job's log output.
func (v ToolView) renderScanResultTable(job *Job) []string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	critStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	logContent := job.FullOutput()
	summary := parseScanLog(logContent, job)

	tableW := v.width - 8
	if tableW < 30 {
		tableW = 30
	}
	if tableW > 60 {
		tableW = 60
	}

	labelW := 14
	valueW := tableW - labelW - 3

	var lines []string

	// Summary table.
	border := lipgloss.RoundedBorder()
	boxStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(ColorDimmed).
		Width(tableW)

	var tableLines []string
	tableLines = append(tableLines, headerStyle.Render("Scan Summary"))
	tableLines = append(tableLines, lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("─", tableW-2)))

	row := func(key, val string) string {
		k := dimStyle.Width(labelW).Render(key)
		v := valStyle.Width(valueW).Render(val)
		return k + v
	}

	tableLines = append(tableLines, row("Target", summary.target))
	tableLines = append(tableLines, row("Files", fmt.Sprintf("%d", summary.totalFiles)))

	if summary.infected > 0 {
		tableLines = append(tableLines, dimStyle.Width(labelW).Render("Infected")+
			critStyle.Render(fmt.Sprintf("%d", summary.infected)))
	} else {
		tableLines = append(tableLines, row("Infected", "0"))
	}

	tableLines = append(tableLines, row("Clean", fmt.Sprintf("%d", summary.clean)))
	tableLines = append(tableLines, row("Duration", FormatElapsed(job.Elapsed())))

	lines = append(lines, boxStyle.Render(strings.Join(tableLines, "\n")))

	// Threats section.
	if len(summary.threats) > 0 {
		lines = append(lines, "")
		lines = append(lines, critStyle.Render("  Threats Found:"))
		for _, t := range summary.threats {
			lines = append(lines, "  "+critStyle.Render("✗")+" "+valStyle.Render(t.file))
			lines = append(lines, "    "+dimStyle.Render(t.virus))
		}
	} else if summary.totalFiles > 0 {
		lines = append(lines, "")
		lines = append(lines, okStyle.Render("  ✓ No threats found"))
	}

	// If it's not a scan result (e.g. hub update), show the raw message.
	if summary.totalFiles == 0 && job.Message != "" {
		lines = append(lines, "")
		lines = append(lines, v.formatActionOutput(&core.ActionResult{
			Success: job.Success,
			Message: job.Message,
		})...)
	}

	return lines
}

type threatEntry struct {
	file  string
	virus string
}

type scanSummary struct {
	target     string
	totalFiles int
	infected   int
	clean      int
	threats    []threatEntry
}

// parseScanLog parses clamscan-style output from the log file.
func parseScanLog(logContent string, job *Job) scanSummary {
	s := scanSummary{}

	// Determine target from actionID.
	switch job.ActionID {
	case "clam_scan_home":
		s.target = "/home"
	case "clam_scan_tmp":
		s.target = "/tmp"
	default:
		s.target = job.Label
	}

	if logContent == "" {
		return s
	}

	for _, line := range strings.Split(logContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasSuffix(trimmed, "FOUND") {
			s.infected++
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				file := strings.TrimSpace(parts[0])
				virus := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "FOUND"))
				s.threats = append(s.threats, threatEntry{file: file, virus: virus})
			}
		} else if strings.HasSuffix(trimmed, "OK") {
			s.clean++
		}
	}

	s.totalFiles = s.clean + s.infected
	return s
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

func padToHeight(lines []string, h int) []string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

func safeGetLine(lines []string, i int, w int) string {
	line := ""
	if i < len(lines) {
		line = lines[i]
	}
	visible := lipgloss.Width(line)
	if visible < w {
		line += strings.Repeat(" ", w-visible)
	}
	return line
}

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

// --- History ---

// hasBackgroundActions returns true if this tool has any background actions.
func (v ToolView) hasBackgroundActions() bool {
	for _, a := range v.actions {
		if backgroundActions[a.ID] {
			return true
		}
	}
	return false
}

func (v ToolView) buildHistoryTable(records []ScanRecord) table.Model {
	cols := []table.Column{
		{Title: "Date", Width: 14},
		{Title: "Action", Width: 16},
		{Title: "Duration", Width: 10},
		{Title: "Result", Width: 10},
		{Title: "Threats", Width: 8},
	}

	var rows []table.Row
	for _, r := range records {
		date := r.StartedAt.Format("Jan 02 15:04")
		dur := FormatElapsed(r.Duration)

		result := "✓ OK"
		if !r.Success {
			result = "✗ Failed"
		} else if r.Threats > 0 {
			result = "✗ Found"
		}

		threats := "-"
		if r.Files > 0 {
			threats = fmt.Sprintf("%d", r.Threats)
		}

		rows = append(rows, table.Row{date, r.Label, dur, result, threats})
	}

	h := v.height - 8
	if h < 3 {
		h = 3
	}
	if h > len(rows)+1 {
		h = len(rows) + 1
	}

	s := table.DefaultStyles()
	s.Header = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Padding(0, 1)
	s.Selected = lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(ColorAccent).
		Bold(true).
		Padding(0, 1)
	s.Cell = lipgloss.NewStyle().
		Foreground(ColorText).
		Padding(0, 1)

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(h),
		table.WithStyles(s),
	)
	return t
}

func (v ToolView) renderHistory() string {
	title := StyleTitle.Render(v.manager.Name() + " — Scan History")
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	if len(v.historyRecords) == 0 {
		content := title + "\n\n  " + dimStyle.Render("No scan history.")
		return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
			Render(content)
	}

	tableView := v.historyTable.View()

	footer := dimStyle.Render("  [j/k] Navigate  [Enter] Detail  [d] Delete  [Esc] Back")

	content := title + "\n\n" + tableView + "\n\n" + footer

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

func (v ToolView) renderHistoryDetail() string {
	if v.historyDetail == nil {
		return v.renderHistory()
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	// Build a virtual Job from the ScanRecord to reuse renderScanResultTable.
	hDir, _ := historyDir()
	virtualJob := &Job{
		ID:        v.historyDetail.ID,
		ToolID:    v.historyDetail.ToolID,
		ActionID:  v.historyDetail.ActionID,
		Label:     v.historyDetail.Label,
		StartedAt: v.historyDetail.StartedAt,
		Done:      true,
		Success:   v.historyDetail.Success,
		Message:   v.historyDetail.Message,
	}
	virtualJob.logPath = filepath.Join(hDir, v.historyDetail.ID+".log")

	var lines []string

	// Title with completion status.
	if v.historyDetail.Success {
		okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
		lines = append(lines, StyleTitle.Render(v.manager.Name()+" — "+v.historyDetail.Label)+"  "+
			okStyle.Render("✓ Complete")+"  "+
			dimStyle.Render(FormatElapsed(v.historyDetail.Duration)))
	} else {
		failStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
		lines = append(lines, StyleTitle.Render(v.manager.Name()+" — "+v.historyDetail.Label)+"  "+
			failStyle.Render("✗ Failed")+"  "+
			dimStyle.Render(FormatElapsed(v.historyDetail.Duration)))
	}

	lines = append(lines, dimStyle.Render("  "+v.historyDetail.StartedAt.Format("2006-01-02 15:04:05")))
	lines = append(lines, "")

	lines = append(lines, v.renderScanResultTable(virtualJob)...)
	lines = append(lines, "")

	// Scroll.
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
		footer = dimStyle.Render(fmt.Sprintf("  [j/k] Scroll  [g] Top  (%d%%)  [Esc] Back", pct))
	} else {
		footer = dimStyle.Render("  [Esc] Back to history")
	}

	content := strings.Join(visible, "\n") + "\n" + footer

	return lipgloss.NewStyle().Width(v.width).Height(v.height).Padding(0, 1).
		Render(content)
}

func (v ToolView) handleHistoryKeys(msg tea.KeyPressMsg) (ToolView, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		v.state = tvIdle
		v.historyRecords = nil
		return v, nil
	case "enter":
		cursor := v.historyTable.Cursor()
		if cursor >= 0 && cursor < len(v.historyRecords) {
			rec := v.historyRecords[cursor]
			v.historyDetail = &rec
			v.state = tvHistoryDetail
			v.scrollOffset = 0
		}
		return v, nil
	case "d":
		cursor := v.historyTable.Cursor()
		if cursor >= 0 && cursor < len(v.historyRecords) && v.jobs != nil {
			rec := v.historyRecords[cursor]
			v.jobs.DeleteHistoryRecord(rec.ID)
			// Reload history.
			records := v.jobs.LoadHistory(v.manager.ToolID())
			v.historyRecords = records
			if len(records) == 0 {
				v.state = tvIdle
				return v, nil
			}
			v.historyTable = v.buildHistoryTable(records)
		}
		return v, nil
	default:
		// Delegate to table for j/k/arrow navigation.
		var cmd tea.Cmd
		v.historyTable, cmd = v.historyTable.Update(msg)
		return v, cmd
	}
}

func (v ToolView) handleHistoryDetailKeys(msg tea.KeyPressMsg) (ToolView, tea.Cmd) {
	switch msg.String() {
	case "esc", "h":
		v.historyDetail = nil
		v.state = tvHistory
		v.scrollOffset = 0
		// Rebuild table in case something changed.
		if v.jobs != nil {
			records := v.jobs.LoadHistory(v.manager.ToolID())
			v.historyRecords = records
			if len(records) > 0 {
				v.historyTable = v.buildHistoryTable(records)
			} else {
				v.state = tvIdle
			}
		}
		return v, nil
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
