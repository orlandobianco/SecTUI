package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

// --- Messages ---

// ScanMsg triggers the start of a new security scan.
type ScanMsg struct{}

// ScanProgressMsg is sent as each module begins scanning.
type ScanProgressMsg struct {
	Module  string
	Percent int
}

// ScanFindingMsg is sent when a module produces findings during a scan.
type ScanFindingMsg struct {
	Findings []core.Finding
}

// ScanCompleteMsg is sent when the full scan finishes.
type ScanCompleteMsg struct {
	Report *core.Report
}

// ScanErrorMsg is sent if the scan encounters a fatal error.
type ScanErrorMsg struct {
	Err error
}

// --- ScannerView ---

// ScannerView renders the scan progress UI with a progress bar,
// current module indicator, and a live findings feed.
type ScannerView struct {
	scanning   bool
	percent    int
	currentMod string
	findings   []core.Finding
	report     *core.Report // set when done
	err        error
	width      int
	height     int
}

// NewScannerView creates a fresh ScannerView in idle state.
func NewScannerView() ScannerView {
	return ScannerView{}
}

func (s ScannerView) Init() tea.Cmd {
	return nil
}

func (s ScannerView) Update(msg tea.Msg) (ScannerView, tea.Cmd) {
	switch msg := msg.(type) {
	case ScanProgressMsg:
		s.currentMod = msg.Module
		s.percent = msg.Percent
		return s, nil

	case ScanFindingMsg:
		s.findings = append(s.findings, msg.Findings...)
		return s, nil

	case ScanCompleteMsg:
		s.scanning = false
		s.percent = 100
		s.report = msg.Report
		return s, nil

	case ScanErrorMsg:
		s.scanning = false
		s.err = msg.Err
		return s, nil
	}

	return s, nil
}

func (s ScannerView) View() string {
	var sections []string

	sections = append(sections, s.renderTitle())
	sections = append(sections, s.renderProgress())
	sections = append(sections, s.renderFindings())
	sections = append(sections, s.renderFooter())

	content := strings.Join(sections, "\n")
	return StyleContent.Width(s.width).Height(s.height).Render(content)
}

// SetSize updates the rendering dimensions.
func (s ScannerView) SetSize(w, h int) ScannerView {
	s.width = w
	s.height = h
	return s
}

// StartScan resets state and marks the scanner as active.
func (s ScannerView) StartScan() ScannerView {
	s.scanning = true
	s.percent = 0
	s.currentMod = ""
	s.findings = nil
	s.report = nil
	s.err = nil
	return s
}

// IsComplete returns true if the scan finished and a report is available.
func (s ScannerView) IsComplete() bool {
	return s.report != nil
}

// Report returns the scan report (nil if scan is still in progress).
func (s ScannerView) Report() *core.Report {
	return s.report
}

// RunScanCmd returns a tea.Cmd that runs all modules sequentially in a goroutine,
// sending progress and finding messages through the Bubble Tea message loop.
func RunScanCmd(modules []core.SecurityModule, platform *core.PlatformInfo, config *core.AppConfig) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctx := &core.ScanContext{
			Platform: platform,
			Config:   config,
		}

		// Sort modules by priority so important modules scan first.
		sorted := make([]core.SecurityModule, len(modules))
		copy(sorted, modules)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Priority() < sorted[j].Priority()
		})

		var allFindings []core.Finding
		var scannedModules []string
		total := len(sorted)

		for i, mod := range sorted {
			// We cannot send intermediate tea.Msg from inside a single Cmd.
			// The Bubble Tea model will receive only the final returned message.
			// So we accumulate everything and return ScanCompleteMsg at the end.
			_ = i // progress would be sent via Program.Send in a real async impl

			findings := mod.Scan(ctx)
			allFindings = append(allFindings, findings...)
			scannedModules = append(scannedModules, mod.ID())

			// Simulate brief work to show progress in future async implementation.
			_ = total
		}

		score := core.CalculateScore(allFindings)

		report := &core.Report{
			Timestamp:      time.Now(),
			Platform:       *platform,
			Score:          score,
			Findings:       allFindings,
			ModulesScanned: scannedModules,
			Duration:       time.Since(start),
		}

		return ScanCompleteMsg{Report: report}
	}
}

// RunScanWithProgressCmd returns a tea.Cmd that uses p.Send() for intermediate
// progress updates. This is the preferred approach for real-time feedback.
func RunScanWithProgressCmd(
	p *tea.Program,
	modules []core.SecurityModule,
	platform *core.PlatformInfo,
	config *core.AppConfig,
) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		ctx := &core.ScanContext{
			Platform: platform,
			Config:   config,
		}

		sorted := make([]core.SecurityModule, len(modules))
		copy(sorted, modules)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Priority() < sorted[j].Priority()
		})

		var allFindings []core.Finding
		var scannedModules []string
		total := len(sorted)

		for i, mod := range sorted {
			percent := 0
			if total > 0 {
				percent = (i * 100) / total
			}
			p.Send(ScanProgressMsg{
				Module:  mod.ID(),
				Percent: percent,
			})

			findings := mod.Scan(ctx)
			allFindings = append(allFindings, findings...)
			scannedModules = append(scannedModules, mod.ID())

			if len(findings) > 0 {
				p.Send(ScanFindingMsg{Findings: findings})
			}
		}

		score := core.CalculateScore(allFindings)

		return ScanCompleteMsg{
			Report: &core.Report{
				Timestamp:      time.Now(),
				Platform:       *platform,
				Score:          score,
				Findings:       allFindings,
				ModulesScanned: scannedModules,
				Duration:       time.Since(start),
			},
		}
	}
}

// --- private rendering helpers ---

func (s ScannerView) renderTitle() string {
	if s.err != nil {
		return StyleTitle.Render("Security Scan") + "  " +
			lipgloss.NewStyle().Foreground(ColorCritical).Render("ERROR")
	}
	if s.report != nil {
		return StyleTitle.Render("Security Scan") + "  " +
			lipgloss.NewStyle().Foreground(ColorOK).Render("COMPLETE")
	}
	return StyleTitle.Render("Security Scan")
}

func (s ScannerView) renderProgress() string {
	barWidth := s.width - 24
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}

	filled := barWidth * s.percent / 100
	empty := barWidth - filled

	filledStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	emptyStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	bar := filledStyle.Render(strings.Repeat("\u2588", filled)) +
		emptyStyle.Render(strings.Repeat("\u2591", empty))

	percentLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).
		Render(fmt.Sprintf("%d%%", s.percent))

	progressLine := fmt.Sprintf("  Progress: %s %s", bar, percentLabel)

	currentLine := ""
	if s.currentMod != "" {
		currentLine = fmt.Sprintf("  Current:  Scanning %s...", s.currentMod)
	} else if s.report != nil {
		currentLine = fmt.Sprintf("  Completed in %s", formatDuration(s.report.Duration))
	}
	if s.err != nil {
		currentLine = "  " + lipgloss.NewStyle().Foreground(ColorCritical).Render(s.err.Error())
	}

	sep := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("\u2500", maxInt(s.width-6, 10)))

	return progressLine + "\n" + currentLine + "\n" + sep
}

func (s ScannerView) renderFindings() string {
	if len(s.findings) == 0 {
		hint := lipgloss.NewStyle().Foreground(ColorDimmed).Render("  Waiting for findings...")
		return hint
	}

	// Show the most recent findings, limited by available height.
	maxLines := s.height - 12
	if maxLines < 3 {
		maxLines = 3
	}

	startIdx := 0
	if len(s.findings) > maxLines {
		startIdx = len(s.findings) - maxLines
	}

	var lines []string
	for _, f := range s.findings[startIdx:] {
		sevLabel := scanSeverityLabel(f.Severity)

		title := f.TitleKey
		if parts := strings.Split(f.TitleKey, "."); len(parts) >= 2 {
			title = parts[len(parts)-1]
			title = strings.ReplaceAll(title, "_", " ")
		}

		modLabel := lipgloss.NewStyle().Foreground(ColorDimmed).Width(10).Render(f.Module)
		line := fmt.Sprintf("  %s  %s  %s", sevLabel, title, modLabel)
		lines = append(lines, line)
	}

	if len(s.findings) > maxLines {
		scrollHint := lipgloss.NewStyle().Foreground(ColorDimmed).
			Render(fmt.Sprintf("  ... %d total findings", len(s.findings)))
		lines = append(lines, scrollHint)
	}

	return strings.Join(lines, "\n")
}

func (s ScannerView) renderFooter() string {
	sep := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render(strings.Repeat("\u2500", maxInt(s.width-6, 10)))

	score := "--"
	if s.report != nil {
		score = fmt.Sprintf("%d", s.report.Score)
	} else if len(s.findings) > 0 {
		score = fmt.Sprintf("%d", core.CalculateScore(s.findings))
	}

	scoreLabel := fmt.Sprintf("  Score: %s/100", score)

	var hints []string
	if s.scanning {
		hints = append(hints, StyleKeyhint.Render("[Esc]")+" Cancel")
	} else if s.report != nil {
		hints = append(hints, StyleKeyhint.Render("[Enter]")+" View results")
		hints = append(hints, StyleKeyhint.Render("[Esc]")+" Back")
	}

	return sep + "\n" + scoreLabel + "\n" + "  " + strings.Join(hints, "  ")
}

func scanSeverityLabel(s core.Severity) string {
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
