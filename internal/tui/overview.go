package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

type Overview struct {
	platform     *core.PlatformInfo
	report       *core.Report
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

func (o Overview) View() string {
	var sections []string

	sections = append(sections, o.renderScoreGauge())

	if len(o.activeJobs) > 0 {
		sections = append(sections, o.renderActiveJobs())
	}

	sections = append(sections, o.renderPlatformInfo())

	if o.report != nil {
		sections = append(sections, o.renderFindingsSummary())
	} else {
		sections = append(sections, o.renderNoScanPlaceholder())
	}

	content := strings.Join(sections, "\n\n")
	return StyleContent.Width(o.width).Height(o.height).Render(content)
}

func (o Overview) renderActiveJobs() string {
	title := StyleTitle.Render("Active Jobs")

	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	frame := spinnerFrames[o.spinnerFrame%len(spinnerFrames)]

	var lines []string
	for _, job := range o.activeJobs {
		elapsed := FormatElapsed(job.Elapsed())
		if job.Done && job.Result != nil {
			status := "✓"
			statusStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
			if !job.Result.Success {
				status = "✗"
				statusStyle = lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
			}
			lines = append(lines, fmt.Sprintf("  %s %s  %s",
				statusStyle.Render(status),
				accentStyle.Render(job.Label),
				dimStyle.Render("completed"),
			))
		} else {
			lines = append(lines, fmt.Sprintf("  %s %s  %s",
				warnStyle.Render(frame),
				accentStyle.Render(job.Label),
				dimStyle.Render(elapsed),
			))
		}
	}

	return title + "\n" + strings.Join(lines, "\n")
}

func (o Overview) renderScoreGauge() string {
	score := 0
	if o.report != nil {
		score = o.report.Score
	}

	title := StyleTitle.Render("Security Score")

	barWidth := o.width - 20
	if barWidth < 20 {
		barWidth = 20
	}
	if barWidth > 50 {
		barWidth = 50
	}

	filled := barWidth * score / 100
	empty := barWidth - filled

	var barColor lipgloss.TerminalColor
	switch {
	case score >= 80:
		barColor = ColorOK
	case score >= 50:
		barColor = ColorWarning
	default:
		barColor = ColorCritical
	}

	filledStyle := lipgloss.NewStyle().Foreground(barColor)
	emptyStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	bar := filledStyle.Render(strings.Repeat("\u2588", filled)) +
		emptyStyle.Render(strings.Repeat("\u2591", empty))

	scoreText := lipgloss.NewStyle().Bold(true).Foreground(barColor).
		Render(fmt.Sprintf("%d/100", score))

	gauge := fmt.Sprintf("  %s %s", bar, scoreText)

	if o.report == nil {
		hint := lipgloss.NewStyle().Foreground(ColorDimmed).Render("  Press [s] to run your first scan")
		return title + "\n" + gauge + "\n" + hint
	}

	delta := ""
	if o.report.Score > 0 {
		delta = lipgloss.NewStyle().Foreground(ColorDimmed).
			Render(fmt.Sprintf("  Scanned %s ago", formatDuration(o.report.Duration)))
	}

	return title + "\n" + gauge + "\n" + delta
}

func (o Overview) renderPlatformInfo() string {
	title := StyleTitle.Render("System Info")

	if o.platform == nil {
		return title + "\n  " + lipgloss.NewStyle().Foreground(ColorDimmed).Render("Detecting platform...")
	}

	osLabel := fmt.Sprintf("%s %s", o.platform.Distro, o.platform.Version)
	if o.platform.OS == core.OSDarwin {
		osLabel = fmt.Sprintf("macOS %s", o.platform.Version)
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	valStyle := lipgloss.NewStyle().Foreground(ColorText)

	lines := []string{
		fmt.Sprintf("  %s  %s", dimStyle.Render("OS:"), valStyle.Render(osLabel)),
		fmt.Sprintf("  %s  %s", dimStyle.Render("Arch:"), valStyle.Render(o.platform.Arch)),
		fmt.Sprintf("  %s  %s", dimStyle.Render("Init:"), valStyle.Render(o.platform.InitSystem.String())),
		fmt.Sprintf("  %s  %s", dimStyle.Render("Pkg:"), valStyle.Render(o.platform.PackageManager.String())),
	}

	if o.platform.IsContainer {
		lines = append(lines, fmt.Sprintf("  %s  %s", dimStyle.Render("Container:"), valStyle.Render("yes")))
	}
	if o.platform.IsWSL {
		lines = append(lines, fmt.Sprintf("  %s  %s", dimStyle.Render("WSL:"), valStyle.Render("yes")))
	}

	return title + "\n" + strings.Join(lines, "\n")
}

func (o Overview) renderFindingsSummary() string {
	title := StyleTitle.Render("Findings Summary")

	counts := map[core.Severity]int{}
	for _, f := range o.report.Findings {
		counts[f.Severity]++
	}

	severities := []struct {
		sev   core.Severity
		label string
		color lipgloss.TerminalColor
	}{
		{core.SeverityCritical, "CRIT", ColorCritical},
		{core.SeverityHigh, "HIGH", ColorCritical},
		{core.SeverityMedium, "MED", ColorWarning},
		{core.SeverityLow, "LOW", ColorInfo},
		{core.SeverityInfo, "INFO", ColorDimmed},
	}

	maxCount := 0
	for _, s := range severities {
		if c := counts[s.sev]; c > maxCount {
			maxCount = c
		}
	}

	var lines []string
	for _, s := range severities {
		c := counts[s.sev]
		barLen := 0
		if maxCount > 0 {
			barLen = c * 20 / maxCount
		}

		labelStyle := lipgloss.NewStyle().Foreground(s.color).Width(5)
		barStyle := lipgloss.NewStyle().Foreground(s.color)

		line := fmt.Sprintf("  %s %s %d",
			labelStyle.Render(s.label),
			barStyle.Render(strings.Repeat("\u2588", barLen)),
			c,
		)
		lines = append(lines, line)
	}

	total := len(o.report.Findings)
	summary := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(fmt.Sprintf("  Total: %d findings across %d modules",
			total, len(o.report.ModulesScanned)))

	return title + "\n" + strings.Join(lines, "\n") + "\n\n" + summary
}

func (o Overview) renderNoScanPlaceholder() string {
	title := StyleTitle.Render("Findings")

	box := lipgloss.NewStyle().
		Foreground(ColorDimmed).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDimmed).
		Padding(1, 3).
		Align(lipgloss.Center).
		Width(o.width - 6)

	content := box.Render("No scan results yet.\nPress [s] to start a security scan.")
	return title + "\n\n" + content
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
