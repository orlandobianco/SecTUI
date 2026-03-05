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

// --- Main View: 4-quadrant layout ---

func (o Overview) View() string {
	innerW := o.width - 2
	innerH := o.height - 1
	if innerW < 20 {
		innerW = 20
	}
	if innerH < 6 {
		innerH = 6
	}

	leftW := innerW / 2
	rightW := innerW - leftW
	topH := innerH / 2
	botH := innerH - topH

	topLeft := o.renderScorePanel(leftW, topH)
	topRight := o.renderDefenseGrid(rightW, topH)
	botLeft := o.renderToolStatus(leftW, botH)
	botRight := o.renderActivity(rightW, botH)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, topLeft, topRight)
	botRow := lipgloss.JoinHorizontal(lipgloss.Top, botLeft, botRight)
	return lipgloss.JoinVertical(lipgloss.Left, topRow, botRow)
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

// --- Top-Left: SECURITY SCORE ---

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

	// Score bar
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

	// Breathing: faint on odd frames
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

	// Platform info compact
	if o.platform != nil {
		osLabel := fmt.Sprintf("%s %s", o.platform.Distro, o.platform.Version)
		if o.platform.OS == core.OSDarwin {
			osLabel = fmt.Sprintf("macOS %s", o.platform.Version)
		}
		lines = append(lines, valStyle.Render(osLabel)+" "+dimStyle.Render("·")+" "+
			dimStyle.Render(o.platform.Arch))
		lines = append(lines, dimStyle.Render(o.platform.InitSystem.String())+" "+
			dimStyle.Render("·")+" "+dimStyle.Render(o.platform.PackageManager.String()))
	}
	lines = append(lines, "")

	// Threat summary
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
			// Two per line
			for i := 0; i < len(threatParts); i += 2 {
				line := threatParts[i]
				if i+1 < len(threatParts) {
					line += "  " + threatParts[i+1]
				}
				lines = append(lines, line)
			}
		}
		lines = append(lines, "")

		// Last scan
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

// --- Top-Right: DEFENSE GRID ---

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

// --- Bottom-Left: TOOL STATUS ---

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
			// Pulse if job running for this tool
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

// --- Bottom-Right: ACTIVITY ---

func (o Overview) renderActivity(w, h int) string {
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	var lines []string

	// Active jobs
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

	// Recent activity from tools with ToolManager
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
