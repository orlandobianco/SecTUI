package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/orlandobianco/SecTUI/internal/core"
)

type Overview struct {
	platform     *core.PlatformInfo
	report       *core.Report
	modules      []core.SecurityModule
	tools        []core.SecurityTool
	toolStatuses map[string]core.ToolStatus
	activeJobs   []*Job
	spinnerFrame int
	width        int
	height       int
	sysStats     *core.SystemStats
	threatFeed   *core.ThreatFeed
}

func NewOverview(platform *core.PlatformInfo) Overview {
	return Overview{platform: platform}
}

func (o Overview) SetReport(r *core.Report) Overview {
	o.report = r
	return o
}

func (o Overview) SetSize(w, h int) Overview {
	o.width = w
	o.height = h
	return o
}

func (o Overview) SetActiveJobs(jobs []*Job, frame int) Overview {
	o.activeJobs = jobs
	o.spinnerFrame = frame
	return o
}

func (o Overview) SetModules(mods []core.SecurityModule) Overview {
	o.modules = mods
	return o
}

func (o Overview) SetTools(tools []core.SecurityTool, statuses map[string]core.ToolStatus) Overview {
	o.tools = tools
	o.toolStatuses = statuses
	return o
}

func (o Overview) SetSystemStats(s *core.SystemStats) Overview {
	o.sysStats = s
	return o
}

func (o Overview) SetThreatFeed(tf *core.ThreatFeed) Overview {
	o.threatFeed = tf
	return o
}

// --- Main View: 3×2 grid layout ---

func (o Overview) View() string {
	innerW := o.width - 2
	innerH := o.height - 1
	if innerW < 20 {
		innerW = 20
	}
	if innerH < 12 {
		innerH = 12
	}

	leftW := innerW / 2
	rightW := innerW - leftW

	row1H := innerH * 40 / 100
	row2H := innerH * 35 / 100
	row3H := innerH - row1H - row2H

	if row1H < 4 {
		row1H = 4
	}
	if row2H < 4 {
		row2H = 4
	}
	if row3H < 3 {
		row3H = 3
	}

	topLeft := o.renderSystemPanel(leftW, row1H)
	topRight := o.renderScorePanel(rightW, row1H)
	midLeft := o.renderDefenseGrid(leftW, row2H)
	midRight := o.renderThreatFeed(rightW, row2H)
	botLeft := o.renderToolStatus(leftW, row3H)
	botRight := o.renderActivity(rightW, row3H)

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, topLeft, topRight)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, midLeft, midRight)
	row3 := lipgloss.JoinHorizontal(lipgloss.Top, botLeft, botRight)

	return lipgloss.JoinVertical(lipgloss.Left, row1, row2, row3)
}

// --- Panel helper ---

func panelBox(title, content string, w, h int) string {
	titleStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDimmed).
		Width(w).Height(h).
		Padding(0, 1)
	inner := titleStyle.Render(title) + "\n" + content
	return box.Render(inner)
}

// --- Row 1 Left: SYSTEM ---

func (o Overview) renderSystemPanel(w, h int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	if o.sysStats == nil {
		frame := spinnerFrames[o.spinnerFrame%len(spinnerFrames)]
		spinner := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render(frame)
		return panelBox("SYSTEM", spinner+" "+dimStyle.Render("Collecting system stats..."), w, h)
	}

	s := o.sysStats

	// ASCII server art with dynamic LED colors
	var cpuColor color.Color
	switch {
	case s.CPUPercent >= 80:
		cpuColor = ColorCritical
	case s.CPUPercent >= 50:
		cpuColor = ColorWarning
	default:
		cpuColor = ColorOK
	}

	ledStyle := lipgloss.NewStyle().Foreground(cpuColor)
	dimLed := lipgloss.NewStyle().Foreground(ColorDimmed)
	boxStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	// Animate LEDs
	led1 := ledStyle.Render("●")
	led2 := ledStyle.Render("●")
	if o.spinnerFrame%4 < 2 {
		led2 = dimLed.Render("○")
	}

	server := []string{
		boxStyle.Render("┌─────────┐"),
		boxStyle.Render("│") + dimLed.Render(" ░░░░░░░ ") + boxStyle.Render("│"),
		boxStyle.Render("│") + dimLed.Render(" ░░") + ledStyle.Render("▓▓") + dimLed.Render("░░░ ") + boxStyle.Render("│"),
		boxStyle.Render("│") + dimLed.Render(" ░░░░░░░ ") + boxStyle.Render("│"),
		boxStyle.Render("│") + " " + led1 + " " + led2 + "      " + boxStyle.Render("│"),
		boxStyle.Render("│") + ledStyle.Render(" ▓▓▓▓▓▓▓ ") + boxStyle.Render("│"),
		boxStyle.Render("└─────────┘"),
	}

	// Metrics
	labelStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Width(5)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	cpuSpark := sparkline(s.CPUHistory, 10)
	ramSpark := sparkline(s.RAMHistory, 10)

	cpuLine := labelStyle.Render("CPU") + " " + colorSparkline(cpuSpark, s.CPUPercent) + " " +
		colorPercent(s.CPUPercent)
	ramLine := labelStyle.Render("RAM") + " " + colorSparkline(ramSpark, s.MemPercent) + " " +
		valStyle.Render(formatBytes(s.MemUsed)) + dimStyle.Render("/") + dimStyle.Render(formatBytes(s.MemTotal))
	diskLine := labelStyle.Render("DISK") + " " + miniBar(s.DiskPercent, 10) + " " +
		colorPercent(s.DiskPercent)
	loadLine := labelStyle.Render("LOAD") + " " + valStyle.Render(fmt.Sprintf("%.1f %.1f %.1f",
		s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2]))
	upLine := labelStyle.Render("UP") + " " + valStyle.Render(formatUptime(s.Uptime))
	netLine := labelStyle.Render("NET") + " " +
		lipgloss.NewStyle().Foreground(ColorOK).Render("↑"+formatBytesRate(s.NetBytesSent)) + " " +
		lipgloss.NewStyle().Foreground(ColorInfo).Render("↓"+formatBytesRate(s.NetBytesRecv))

	metrics := []string{cpuLine, ramLine, diskLine, loadLine, upLine, netLine}

	// Compose: server art left, metrics right
	serverW := 13 // width of ASCII server
	var lines []string
	for i := 0; i < max(len(server), len(metrics)); i++ {
		left := ""
		if i < len(server) {
			left = server[i]
		}
		right := ""
		if i < len(metrics) {
			right = metrics[i]
		}
		// Pad server column
		leftPad := lipgloss.NewStyle().Width(serverW).Render(left)
		lines = append(lines, leftPad+" "+right)
	}

	return panelBox("SYSTEM", strings.Join(lines, "\n"), w, h)
}

// --- Row 1 Right: SECURITY SCORE ---

func (o Overview) renderScorePanel(w, h int) string {
	if o.report == nil && len(o.activeJobs) == 0 {
		return o.renderWelcome(w, h)
	}

	score := 0
	if o.report != nil {
		score = o.report.Score
	}

	var barColor color.Color
	switch {
	case score >= 80:
		barColor = ColorOK
	case score >= 50:
		barColor = ColorWarning
	default:
		barColor = ColorCritical
	}

	barW := w - 14
	if barW < 10 {
		barW = 10
	}
	if barW > 40 {
		barW = 40
	}
	filled := barW * score / 100
	empty := barW - filled

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	if o.spinnerFrame%2 == 1 && o.report != nil {
		filledStyle = filledStyle.Faint(true)
	}

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	scoreText := lipgloss.NewStyle().Bold(true).Foreground(barColor).
		Render(fmt.Sprintf("%d/100", score))

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	var lines []string
	lines = append(lines, fmt.Sprintf("%s %s", bar, scoreText))
	lines = append(lines, "")

	if o.platform != nil {
		osLabel := fmt.Sprintf("%s %s", o.platform.Distro, o.platform.Version)
		if o.platform.OS == core.OSDarwin {
			osLabel = fmt.Sprintf("macOS %s", o.platform.Version)
		}
		lines = append(lines, valStyle.Render(osLabel)+" "+dimStyle.Render("·")+" "+
			dimStyle.Render(o.platform.Arch))
	}
	lines = append(lines, "")

	if o.report != nil {
		counts := map[core.Severity]int{}
		for _, f := range o.report.Findings {
			counts[f.Severity]++
		}

		type sevEntry struct {
			label string
			clr   color.Color
			count int
		}
		sevs := []sevEntry{
			{"CRIT", ColorCritical, counts[core.SeverityCritical]},
			{"HIGH", ColorCritical, counts[core.SeverityHigh]},
			{"MED", ColorWarning, counts[core.SeverityMedium]},
			{"LOW", ColorInfo, counts[core.SeverityLow]},
		}

		var threatParts []string
		for _, s := range sevs {
			if s.count > 0 {
				style := lipgloss.NewStyle().Foreground(s.clr).Bold(true)
				barLen := s.count
				if barLen > 8 {
					barLen = 8
				}
				threatParts = append(threatParts,
					style.Render(s.label)+" "+
						lipgloss.NewStyle().Foreground(s.clr).Render(strings.Repeat("█", barLen))+
						" "+dimStyle.Render(fmt.Sprintf("%d", s.count)))
			}
		}
		if len(threatParts) > 0 {
			for i := 0; i < len(threatParts); i += 2 {
				line := threatParts[i]
				if i+1 < len(threatParts) {
					line += "  " + threatParts[i+1]
				}
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")

		lines = append(lines, dimStyle.Render(fmt.Sprintf("Last scan: %s (%s)",
			formatTimeAgo(o.report.Timestamp),
			formatDuration(o.report.Duration))))
	} else {
		lines = append(lines, dimStyle.Render("Press [s] to start your first scan"))
	}

	return panelBox("SECURITY SCORE", strings.Join(lines, "\n"), w, h)
}

func (o Overview) renderWelcome(w, h int) string {
	ascii := ` ___         _____ _   _ ___
/ __| ___ __|_   _| | | |_ _|
\__ \/ -_) _| | | | |_| || |
|___/\___\__| |_|  \___/|___|`

	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	hint := "Press [s] to start your first scan"
	if o.spinnerFrame%2 == 0 {
		hint = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render(hint)
	} else {
		hint = dimStyle.Render(hint)
	}

	content := accentStyle.Render(ascii) + "\n\n" + hint

	return panelBox("SECURITY SCORE", content, w, h)
}

// --- Row 2 Left: DEFENSE GRID ---

func (o Overview) renderDefenseGrid(w, h int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	if o.report == nil || len(o.modules) == 0 {
		return panelBox("DEFENSE GRID", dimStyle.Render("Run scan to see module scores"), w, h)
	}

	barMaxW := w - 20
	if barMaxW < 5 {
		barMaxW = 5
	}
	if barMaxW > 20 {
		barMaxW = 20
	}

	var lines []string
	for _, mod := range o.modules {
		score := core.CalculateModuleScore(o.report.Findings, mod.ID())

		var barColor color.Color
		switch {
		case score >= 80:
			barColor = ColorOK
		case score >= 50:
			barColor = ColorWarning
		default:
			barColor = ColorCritical
		}

		barFilled := barMaxW * score / 100
		barEmpty := barMaxW - barFilled

		nameStyle := lipgloss.NewStyle().Foreground(ColorText).Width(10)
		barStyle := lipgloss.NewStyle().Foreground(barColor)
		emptyBarStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
		scoreStyle := lipgloss.NewStyle().Foreground(barColor).Bold(true)

		name := moduleName(mod.ID())
		line := nameStyle.Render(name) + " " +
			barStyle.Render(strings.Repeat("█", barFilled)) +
			emptyBarStyle.Render(strings.Repeat("░", barEmpty)) +
			" " + scoreStyle.Render(fmt.Sprintf("%d", score))
		lines = append(lines, line)
	}

	return panelBox("DEFENSE GRID", strings.Join(lines, "\n"), w, h)
}

// --- Row 2 Right: THREAT FEED ---

func (o Overview) renderThreatFeed(w, h int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	critStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)

	if o.threatFeed == nil {
		return panelBox("THREAT FEED", dimStyle.Render("Initializing..."), w, h)
	}

	events := o.threatFeed.Events()
	totalBlocked := o.threatFeed.TotalBlocked()

	if len(events) == 0 && totalBlocked == 0 {
		// Check if any threat tools are installed
		hasThreatTool := false
		for _, t := range o.tools {
			id := t.ID()
			if id == "fail2ban" || id == "crowdsec" {
				if o.toolStatuses != nil && o.toolStatuses[id] >= core.ToolInstalled {
					hasThreatTool = true
					break
				}
			}
		}
		if !hasThreatTool {
			hint := dimStyle.Render("Install fail2ban or CrowdSec") + "\n" +
				dimStyle.Render("for threat monitoring")
			return panelBox("THREAT FEED", lipgloss.NewStyle().Foreground(ColorOK).Render("◆")+" "+hint, w, h)
		}
		return panelBox("THREAT FEED",
			lipgloss.NewStyle().Foreground(ColorOK).Bold(true).Render("✓")+" "+
				dimStyle.Render("No threats detected"), w, h)
	}

	maxEvents := h - 3 // leave room for header + footer
	if maxEvents < 1 {
		maxEvents = 1
	}
	if len(events) > maxEvents {
		events = events[:maxEvents]
	}

	var lines []string
	for _, ev := range events {
		ts := ev.Timestamp.Format("15:04")

		dot := critStyle.Render("●")
		toolLabel := lipgloss.NewStyle().Foreground(ColorWarning).Render(ev.Tool)
		actionLabel := critStyle.Render(ev.Action)

		line1 := fmt.Sprintf("%s %s %s %s",
			dot, dimStyle.Render(ts), toolLabel, actionLabel)

		// Second line: IP + geo + detail
		ipStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
		geoStr := ""
		if ev.Geo.Country != "" {
			geoStr = " " + lipgloss.NewStyle().Foreground(ColorWarning).Render("("+ev.Geo.Country+")")
		}
		detailStr := ""
		if ev.Detail != "" {
			detailStr = " " + dimStyle.Render(ev.Detail)
		}
		line2 := "  " + ipStyle.Render(ev.IP) + geoStr + detailStr

		lines = append(lines, line1)
		lines = append(lines, line2)
	}

	// Footer: shield count
	if totalBlocked > 0 {
		lines = append(lines, "")
		shieldIcon := lipgloss.NewStyle().Foreground(ColorOK).Bold(true).Render("◆")
		lines = append(lines, shieldIcon+" "+
			lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(fmt.Sprintf("%d", totalBlocked))+
			" "+dimStyle.Render("IPs blocked"))
	}

	return panelBox("THREAT FEED", strings.Join(lines, "\n"), w, h)
}

// --- Row 3 Left: TOOL STATUS ---

func (o Overview) renderToolStatus(w, h int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	if len(o.tools) == 0 {
		return panelBox("TOOL STATUS", dimStyle.Render("No tools configured"), w, h)
	}

	activeCount := 0
	totalCount := 0

	var lines []string
	for _, t := range o.tools {
		status := core.ToolNotInstalled
		if o.toolStatuses != nil {
			status = o.toolStatuses[t.ID()]
		}
		if status == core.ToolNotApplicable {
			continue
		}
		totalCount++

		var dot string
		var nameStyle lipgloss.Style
		var statusLabel string

		switch status {
		case core.ToolActive:
			activeCount++
			if o.hasRunningJob(t.ID()) && o.spinnerFrame%2 == 1 {
				dot = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("●")
			} else {
				dot = lipgloss.NewStyle().Foreground(ColorOK).Bold(true).Render("●")
			}
			nameStyle = lipgloss.NewStyle().Foreground(ColorText)
			statusLabel = lipgloss.NewStyle().Foreground(ColorOK).Render("active")
		case core.ToolInstalled:
			dot = lipgloss.NewStyle().Foreground(ColorWarning).Render("○")
			nameStyle = lipgloss.NewStyle().Foreground(ColorText)
			statusLabel = dimStyle.Render("installed")
		default:
			dot = dimStyle.Render("◌")
			nameStyle = dimStyle
			statusLabel = dimStyle.Render("not installed")
		}

		nameW := w - 20
		if nameW < 8 {
			nameW = 8
		}
		name := nameStyle.Width(nameW).Render(t.Name())
		lines = append(lines, fmt.Sprintf("%s %s %s", dot, name, statusLabel))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(fmt.Sprintf("%d/%d tools active", activeCount, totalCount)))

	return panelBox("TOOL STATUS", strings.Join(lines, "\n"), w, h)
}

// --- Row 3 Right: ACTIVITY ---

func (o Overview) renderActivity(w, h int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	var lines []string

	if len(o.activeJobs) > 0 {
		frame := spinnerFrames[o.spinnerFrame%len(spinnerFrames)]
		for _, job := range o.activeJobs {
			elapsed := FormatElapsed(job.Elapsed())
			if job.Done {
				status := "✓"
				statusStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
				if !job.Success {
					status = "✗"
					statusStyle = lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
				}
				lines = append(lines, fmt.Sprintf("%s %s  %s",
					statusStyle.Render(status),
					accentStyle.Render(job.Label),
					dimStyle.Render("done"),
				))
			} else {
				lines = append(lines, fmt.Sprintf("%s %s  %s",
					warnStyle.Render(frame),
					accentStyle.Render(job.Label),
					dimStyle.Render(elapsed),
				))
			}
		}
		lines = append(lines, "")
	}

	recentLines := o.gatherRecentActivity(h - len(lines) - 3)
	if len(recentLines) > 0 {
		lines = append(lines, dimStyle.Render("Recent:"))
		lines = append(lines, recentLines...)
	}

	if len(lines) == 0 {
		lines = append(lines, dimStyle.Render("Start a scan or manage tools"))
	}

	return panelBox("ACTIVITY", strings.Join(lines, "\n"), w, h)
}

// --- Helpers ---

func (o Overview) hasRunningJob(toolID string) bool {
	for _, j := range o.activeJobs {
		if j.ToolID == toolID && !j.Done {
			return true
		}
	}
	return false
}

func (o Overview) gatherRecentActivity(maxLines int) []string {
	if maxLines <= 0 {
		maxLines = 3
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	var entries []string
	for _, t := range o.tools {
		tm, ok := t.(core.ToolManager)
		if !ok {
			continue
		}
		recent := tm.RecentActivity(3)
		for _, a := range recent {
			ts := a.Timestamp
			if len(ts) > 8 {
				parts := strings.Fields(ts)
				if len(parts) >= 3 {
					ts = parts[2]
				}
			}
			msg := a.Message
			if len(msg) > 30 {
				msg = msg[:29] + "…"
			}
			entries = append(entries, fmt.Sprintf("%s %s", dimStyle.Render(ts), valStyle.Render(msg)))
		}
	}

	if len(entries) > maxLines {
		entries = entries[:maxLines]
	}
	return entries
}

func moduleName(id string) string {
	switch id {
	case "ssh":
		return "SSH"
	case "firewall":
		return "Firewall"
	case "network":
		return "Network"
	case "users":
		return "Users"
	case "updates":
		return "Updates"
	case "kernel":
		return "Kernel"
	case "filesystem":
		return "Filesys"
	default:
		return strings.Title(id) //nolint:staticcheck
	}
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	sec := int(d.Seconds())
	if sec < 60 {
		return fmt.Sprintf("%ds ago", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm ago", sec/60)
	}
	return fmt.Sprintf("%dh ago", sec/3600)
}

func formatDuration(d time.Duration) string {
	sec := int(d.Seconds())
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm %ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%dh %dm", sec/3600, (sec%3600)/60)
}

// --- Sparkline & formatting helpers ---

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(data []float64, width int) string {
	if len(data) == 0 {
		return strings.Repeat(string(sparkChars[0]), width)
	}
	// Take last `width` samples
	start := 0
	if len(data) > width {
		start = len(data) - width
	}
	subset := data[start:]

	var b strings.Builder
	for _, v := range subset {
		idx := int(v / 100.0 * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		b.WriteRune(sparkChars[idx])
	}
	// Pad if not enough data
	for b.Len() < width {
		b.WriteRune(sparkChars[0])
	}
	return b.String()
}

func colorSparkline(spark string, percent float64) string {
	var clr color.Color
	switch {
	case percent >= 80:
		clr = ColorCritical
	case percent >= 50:
		clr = ColorWarning
	default:
		clr = ColorOK
	}
	return lipgloss.NewStyle().Foreground(clr).Render(spark)
}

func colorPercent(percent float64) string {
	var clr color.Color
	switch {
	case percent >= 80:
		clr = ColorCritical
	case percent >= 50:
		clr = ColorWarning
	default:
		clr = ColorOK
	}
	return lipgloss.NewStyle().Foreground(clr).Bold(true).Render(fmt.Sprintf("%.0f%%", percent))
}

func miniBar(percent float64, width int) string {
	filled := int(percent / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	var clr color.Color
	switch {
	case percent >= 80:
		clr = ColorCritical
	case percent >= 50:
		clr = ColorWarning
	default:
		clr = ColorOK
	}
	return lipgloss.NewStyle().Foreground(clr).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(ColorDimmed).Render(strings.Repeat("░", empty))
}

func formatBytes(b uint64) string {
	const (
		gb = 1024 * 1024 * 1024
		mb = 1024 * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1fG", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.0fM", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%dK", b/1024)
	}
}

func formatBytesRate(b uint64) string {
	const (
		mb = 1024 * 1024
		kb = 1024
	)
	switch {
	case b >= mb:
		return fmt.Sprintf("%.1fM", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.0fK", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
