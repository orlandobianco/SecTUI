package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

// isRoot returns true if the process is running with elevated privileges.
func isRoot() bool {
	return os.Geteuid() == 0
}

// fixState tracks the interactive fix confirmation flow.
type fixState int

const (
	fixIdle     fixState = iota
	fixNeedRoot          // not running as root
	fixConfirm           // waiting for y/n
	fixDone              // showing results
)

// fixResultEntry stores the outcome of a single fix attempt.
type fixResultEntry struct {
	Finding core.Finding
	Result  *core.ApplyResult
	Err     error
}

// fixCompleteMsg is sent when all fixes have been applied in the background.
type fixCompleteMsg struct {
	Results []fixResultEntry
}

type App struct {
	sidebar      Sidebar
	overview     Overview
	moduleView   ModuleView
	scannerView  ScannerView
	width        int
	height       int
	platform     *core.PlatformInfo
	config       *core.AppConfig
	report       *core.Report
	modules      []core.SecurityModule
	scanning     bool
	focusSidebar bool
	quitting     bool
	program      *tea.Program

	// Fix flow state
	fix         fixState
	fixFindings []core.Finding
	fixResults  []fixResultEntry
	fixModuleID string
}

type scanRequestMsg struct{}

func NewApp(platform *core.PlatformInfo, config *core.AppConfig) *App {
	sidebar := NewSidebar()
	overview := NewOverview(platform)

	return &App{
		sidebar:      sidebar,
		overview:     overview,
		scannerView:  NewScannerView(),
		platform:     platform,
		config:       config,
		focusSidebar: true,
	}
}

// SetModules configures the security modules used for scanning.
// Call this before passing the App to tea.NewProgram.
func (a *App) SetModules(mods []core.SecurityModule) {
	a.modules = mods
}

func (a *App) Init() tea.Cmd {
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		return a, nil

	case scanRequestMsg:
		return a.startScan()

	case ScanProgressMsg:
		a.scannerView, _ = a.scannerView.Update(msg)
		return a, nil

	case ScanFindingMsg:
		a.scannerView, _ = a.scannerView.Update(msg)
		return a, nil

	case ScanCompleteMsg:
		return a.handleScanComplete(msg)

	case ScanErrorMsg:
		a.scannerView, _ = a.scannerView.Update(msg)
		a.scanning = false
		return a, nil

	case ApplyFixRequestMsg:
		return a.handleFixRequest(msg)

	case fixCompleteMsg:
		return a.handleFixComplete(msg)

	case tea.KeyMsg:
		// Fix flow takes priority over everything else.
		if a.fix == fixNeedRoot {
			return a.handleFixNeedRootKeys(msg)
		}
		if a.fix == fixConfirm {
			return a.handleFixConfirmKeys(msg)
		}
		if a.fix == fixDone {
			return a.handleFixDoneKeys(msg)
		}

		// While scanning, only allow quit and cancel.
		if a.scanning {
			return a.handleScanningKeys(msg)
		}

		if cmd, handled := a.handleGlobalKeys(msg); handled {
			return a, cmd
		}

		if a.focusSidebar {
			return a.handleSidebarKeys(msg)
		}
		return a.handleContentKeys(msg)
	}

	return a, nil
}

func (a *App) View() string {
	if a.quitting {
		return ""
	}

	header := a.renderHeader()
	footer := a.renderFooter()
	body := a.renderBody()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (a *App) handleGlobalKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		a.quitting = true
		return tea.Quit, true
	case "tab":
		a.focusSidebar = !a.focusSidebar
		a.sidebar = a.sidebar.SetFocused(a.focusSidebar)
		return nil, true
	case "?":
		// TODO: toggle help overlay
		return nil, true
	case "s":
		return func() tea.Msg { return scanRequestMsg{} }, true
	}
	return nil, false
}

func (a *App) handleScanningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		a.quitting = true
		return a, tea.Quit
	case "esc":
		a.scanning = false
		return a, nil
	}
	return a, nil
}

func (a *App) handleSidebarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "l", "right":
		a.focusSidebar = false
		a.sidebar = a.sidebar.SetFocused(false)
		a.syncModuleView()
		return a, nil
	}

	var cmd tea.Cmd
	a.sidebar, cmd = a.sidebar.Update(msg)
	return a, cmd
}

func (a *App) handleContentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	selected := a.sidebar.Selected()

	// If we are viewing a module with findings, delegate keys to ModuleView.
	if selected.Section == "modules" && len(a.moduleView.findings) > 0 {
		switch msg.String() {
		case "h", "left", "esc":
			a.focusSidebar = true
			a.sidebar = a.sidebar.SetFocused(true)
			return a, nil
		}

		var cmd tea.Cmd
		a.moduleView, cmd = a.moduleView.Update(msg)
		return a, cmd
	}

	// Default: go back to sidebar.
	switch msg.String() {
	case "h", "left", "esc":
		a.focusSidebar = true
		a.sidebar = a.sidebar.SetFocused(true)
		return a, nil
	}
	return a, nil
}

// --- Fix flow ---

func (a *App) handleFixRequest(msg ApplyFixRequestMsg) (tea.Model, tea.Cmd) {
	if !isRoot() {
		a.fix = fixNeedRoot
		a.fixFindings = msg.Findings
		return a, nil
	}
	a.fix = fixConfirm
	a.fixFindings = msg.Findings
	a.fixModuleID = msg.ModuleID
	a.fixResults = nil
	return a, nil
}

func (a *App) handleFixNeedRootKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", " ", "n", "N":
		a.fix = fixIdle
		a.fixFindings = nil
		return a, nil
	}
	return a, nil
}

func (a *App) handleFixConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.fix = fixDone
		// Apply fixes in background and return results.
		findings := a.fixFindings
		platform := a.platform
		config := a.config
		mods := a.modules
		return a, func() tea.Msg {
			var results []fixResultEntry
			applyCtx := &core.ApplyContext{
				Platform: platform,
				Config:   config,
				DryRun:   false,
				Backup:   true,
			}
			for _, f := range findings {
				var mod core.SecurityModule
				for _, m := range mods {
					if m.ID() == f.Module {
						mod = m
						break
					}
				}
				if mod == nil {
					results = append(results, fixResultEntry{
						Finding: f,
						Err:     fmt.Errorf("module %q not found", f.Module),
					})
					continue
				}
				result, err := mod.ApplyFix(f.FixID, applyCtx)
				results = append(results, fixResultEntry{
					Finding: f,
					Result:  result,
					Err:     err,
				})
			}
			return fixCompleteMsg{Results: results}
		}
	case "n", "N", "esc":
		a.fix = fixIdle
		a.fixFindings = nil
		return a, nil
	}
	return a, nil
}

func (a *App) handleFixComplete(msg fixCompleteMsg) (tea.Model, tea.Cmd) {
	a.fix = fixDone
	a.fixResults = msg.Results
	return a, nil
}

func (a *App) handleFixDoneKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", " ":
		a.fix = fixIdle
		a.fixFindings = nil
		a.fixResults = nil
		// Re-scan to refresh findings and score.
		return a.startScan()
	}
	return a, nil
}

// --- Scan lifecycle ---

func (a *App) startScan() (tea.Model, tea.Cmd) {
	if a.scanning {
		return a, nil
	}
	if len(a.modules) == 0 {
		return a, nil
	}

	a.scanning = true
	a.scannerView = a.scannerView.StartScan()

	if a.program != nil {
		return a, RunScanWithProgressCmd(a.program, a.modules, a.platform, a.config)
	}
	return a, RunScanCmd(a.modules, a.platform, a.config)
}

func (a *App) handleScanComplete(msg ScanCompleteMsg) (tea.Model, tea.Cmd) {
	a.scanning = false
	a.report = msg.Report

	a.scannerView, _ = a.scannerView.Update(msg)
	a.overview = a.overview.SetReport(msg.Report)
	a.syncModuleView()

	return a, nil
}

// syncModuleView rebuilds the ModuleView for the currently selected sidebar module.
func (a *App) syncModuleView() {
	selected := a.sidebar.Selected()
	if selected.Section != "modules" || a.report == nil {
		return
	}

	moduleID := selected.ID
	var moduleFindings []core.Finding
	for _, f := range a.report.Findings {
		if f.Module == moduleID {
			moduleFindings = append(moduleFindings, f)
		}
	}

	w, h := a.contentDimensions()
	a.moduleView = NewModuleView(moduleID, moduleFindings).SetSize(w, h)
}

// --- Layout ---

func (a *App) updateLayout() {
	headerHeight := 1
	footerHeight := 1
	borderOverhead := 2

	bodyHeight := a.height - headerHeight - footerHeight - borderOverhead
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	a.sidebar = a.sidebar.SetSize(sidebarWidth, bodyHeight)

	contentWidth, _ := a.contentDimensions()
	a.overview = a.overview.SetSize(contentWidth, bodyHeight)
	a.scannerView = a.scannerView.SetSize(contentWidth, bodyHeight)
	a.moduleView = a.moduleView.SetSize(contentWidth, bodyHeight)
}

func (a *App) contentDimensions() (int, int) {
	contentWidth := a.width - sidebarWidth - 3
	if contentWidth < 10 {
		contentWidth = 10
	}

	bodyHeight := a.height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	return contentWidth, bodyHeight
}

// --- Rendering ---

func (a *App) renderHeader() string {
	score := "--"
	if a.report != nil {
		score = fmt.Sprintf("%d", a.report.Score)
	}

	left := StyleTitle.Render("SecTUI")

	statusIndicator := ""
	if a.scanning {
		statusIndicator = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render(" SCANNING ")
	}

	right := fmt.Sprintf("Score: %s/100  %s  %s",
		score,
		statusIndicator,
		StyleKeyhint.Render("[?]help"),
	)

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	content := left + fmt.Sprintf("%*s", gap, "") + right
	return StyleHeader.Width(a.width).Render(content)
}

func (a *App) renderFooter() string {
	if a.fix == fixNeedRoot {
		return StyleFooter.Width(a.width).Render(
			StyleKeyhint.Render("[Enter]") + " Back",
		)
	}
	if a.fix == fixConfirm {
		return StyleFooter.Width(a.width).Render(
			StyleKeyhint.Render("[y]") + " Confirm  " +
				StyleKeyhint.Render("[n]") + " Cancel",
		)
	}
	if a.fix == fixDone {
		return StyleFooter.Width(a.width).Render(
			StyleKeyhint.Render("[Enter]") + " Continue (re-scan)",
		)
	}

	if a.scanning {
		hints := []string{
			StyleKeyhint.Render("[Esc]") + " Cancel",
			StyleKeyhint.Render("[q]") + " Quit",
		}
		return StyleFooter.Width(a.width).Render(strings.Join(hints, "  "))
	}

	hints := []string{
		StyleKeyhint.Render("[Tab]") + " Focus",
		StyleKeyhint.Render("[s]") + " Scan",
		StyleKeyhint.Render("[?]") + " Help",
		StyleKeyhint.Render("[q]") + " Quit",
	}

	return StyleFooter.Width(a.width).Render(strings.Join(hints, "  "))
}

func (a *App) renderBody() string {
	sidebarView := a.sidebar.View()
	contentView := a.renderContent()

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)
}

func (a *App) renderContent() string {
	contentWidth, bodyHeight := a.contentDimensions()

	// Fix flow overlays the content area.
	if a.fix == fixNeedRoot {
		return a.renderFixNeedRoot(contentWidth, bodyHeight)
	}
	if a.fix == fixConfirm {
		return a.renderFixConfirm(contentWidth, bodyHeight)
	}
	if a.fix == fixDone {
		return a.renderFixResults(contentWidth, bodyHeight)
	}

	// If a scan is in progress, always show the scanner view.
	if a.scanning {
		s := a.scannerView.SetSize(contentWidth, bodyHeight)
		return s.View()
	}

	selected := a.sidebar.Selected()

	switch selected.Section {
	case "overview":
		o := a.overview.SetSize(contentWidth, bodyHeight)
		return o.View()
	case "modules":
		return a.renderModuleContent(selected, contentWidth, bodyHeight)
	case "tools":
		return a.renderToolPlaceholder(selected, contentWidth, bodyHeight)
	case "secstore":
		return a.renderSecStorePlaceholder(contentWidth, bodyHeight)
	default:
		return a.overview.View()
	}
}

func (a *App) renderFixNeedRoot(w, h int) string {
	title := StyleTitle.Render("Elevated Privileges Required")

	warnStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	cmdStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, warnStyle.Render("  SecTUI needs root privileges to modify system files."))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Scanning works without root, but applying fixes requires"))
	lines = append(lines, dimStyle.Render("  write access to system configuration files like:"))
	lines = append(lines, dimStyle.Render("  /etc/ssh/sshd_config, /etc/sysctl.d/, etc."))
	lines = append(lines, "")
	lines = append(lines, "  Restart SecTUI with elevated privileges:")
	lines = append(lines, "")
	lines = append(lines, "    "+cmdStyle.Render("sudo sectui"))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Or use the CLI for individual fixes:"))
	lines = append(lines, "")
	lines = append(lines, "    "+cmdStyle.Render("sudo sectui harden ssh"))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Press [Enter] to go back."))

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderFixConfirm(w, h int) string {
	title := StyleTitle.Render("Apply Fixes")

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, warnStyle.Render(fmt.Sprintf(
		"  You are about to apply %d fix(es):", len(a.fixFindings))))
	lines = append(lines, "")

	for i, f := range a.fixFindings {
		fixTitle := core.T(f.TitleKey)
		if fixTitle == f.TitleKey {
			fixTitle = f.FixID
		}
		lines = append(lines, fmt.Sprintf("  %d. %s  %s",
			i+1,
			accentStyle.Render(f.FixID),
			dimStyle.Render(fixTitle),
		))
	}

	lines = append(lines, "")
	lines = append(lines, warnStyle.Render("  This will modify system configuration files."))
	lines = append(lines, dimStyle.Render("  Backups will be created before each change."))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %s to apply, %s to cancel",
		lipgloss.NewStyle().Foreground(ColorOK).Bold(true).Render("[y]"),
		lipgloss.NewStyle().Foreground(ColorCritical).Bold(true).Render("[n]"),
	))

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderFixResults(w, h int) string {
	if len(a.fixResults) == 0 {
		title := StyleTitle.Render("Applying Fixes")
		spinner := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).
			Render("  Applying fixes...")
		content := title + "\n\n" + spinner
		return StyleContent.Width(w).Height(h).Render(content)
	}

	title := StyleTitle.Render("Fix Results")

	okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	var lines []string
	lines = append(lines, "")

	successes := 0
	for _, r := range a.fixResults {
		fixTitle := core.T(r.Finding.TitleKey)
		if fixTitle == r.Finding.TitleKey {
			fixTitle = r.Finding.FixID
		}

		if r.Err != nil {
			lines = append(lines, fmt.Sprintf("  %s %s: %v",
				failStyle.Render("FAIL"),
				fixTitle,
				r.Err,
			))
		} else if r.Result != nil && r.Result.Success {
			successes++
			msg := r.Result.Message
			if r.Result.BackupPath != "" {
				msg += dimStyle.Render(fmt.Sprintf(" (backup: %s)", r.Result.BackupPath))
			}
			lines = append(lines, fmt.Sprintf("  %s %s: %s",
				okStyle.Render(" OK "),
				fixTitle,
				msg,
			))
		} else {
			msg := "unknown error"
			if r.Result != nil {
				msg = r.Result.Message
			}
			lines = append(lines, fmt.Sprintf("  %s %s: %s",
				failStyle.Render("FAIL"),
				fixTitle,
				msg,
			))
		}
	}

	lines = append(lines, "")
	summaryStyle := lipgloss.NewStyle().Bold(true)
	if successes == len(a.fixResults) {
		lines = append(lines, summaryStyle.Foreground(ColorOK).Render(
			fmt.Sprintf("  All %d fix(es) applied successfully.", successes)))
	} else {
		lines = append(lines, summaryStyle.Foreground(ColorWarning).Render(
			fmt.Sprintf("  %d/%d fix(es) applied.", successes, len(a.fixResults))))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Press [Enter] to re-scan and see updated results."))

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderModuleContent(item SidebarItem, w, h int) string {
	if a.report != nil {
		if a.moduleView.moduleID != item.ID {
			a.syncModuleView()
		}
		mv := a.moduleView.SetSize(w, h)
		return mv.View()
	}

	return a.renderModulePlaceholder(item, w, h)
}

func (a *App) renderModulePlaceholder(item SidebarItem, w, h int) string {
	title := StyleTitle.Render(fmt.Sprintf("%s Module", item.Label))
	hint := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render("Run a scan to see findings for this module.")

	content := title + "\n\n" + hint
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderToolPlaceholder(item SidebarItem, w, h int) string {
	title := StyleTitle.Render(item.Label)
	hint := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render("Tool management will be available in a future update.")

	content := title + "\n\n" + hint
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderSecStorePlaceholder(w, h int) string {
	title := StyleTitle.Render("SecStore")
	hint := lipgloss.NewStyle().Foreground(ColorDimmed).
		Render("Browse and install security tools.\nComing in a future update.")

	content := title + "\n\n" + hint
	return StyleContent.Width(w).Height(h).Render(content)
}

// Run starts the TUI application.
func Run(platform *core.PlatformInfo, config *core.AppConfig) error {
	app := NewApp(platform, config)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunWithModules starts the TUI application with pre-configured security modules.
// This is the preferred entry point as it enables scanning.
func RunWithModules(platform *core.PlatformInfo, config *core.AppConfig, modules []core.SecurityModule) error {
	app := NewApp(platform, config)
	app.SetModules(modules)
	p := tea.NewProgram(app, tea.WithAltScreen())
	app.program = p
	_, err := p.Run()
	return err
}
