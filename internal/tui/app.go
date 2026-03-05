package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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
	secstoreView SecStoreView
	toolView     ToolView
	toolViewID   string // currently loaded ToolManager ID
	width        int
	height       int
	platform     *core.PlatformInfo
	config       *core.AppConfig
	report       *core.Report
	modules      []core.SecurityModule
	tools        []core.SecurityTool
	toolStatuses map[string]core.ToolStatus
	scanning     bool
	focusSidebar bool
	quitting     bool
	program      *tea.Program

	// Background job system
	jobs         *JobManager
	spinnerFrame int
	confirmQuit  bool

	// Fix flow state
	fix         fixState
	fixFindings []core.Finding
	fixResults  []fixResultEntry
	fixModuleID string

	// Help overlay
	showHelp bool
}

type scanRequestMsg struct{}

func NewApp(platform *core.PlatformInfo, config *core.AppConfig) *App {
	sidebar := NewSidebar()
	overview := NewOverview(platform)

	return &App{
		sidebar:      sidebar,
		overview:     overview,
		scannerView:  NewScannerView(),
		secstoreView: NewSecStoreView(),
		platform:     platform,
		config:       config,
		toolStatuses: make(map[string]core.ToolStatus),
		jobs:         NewJobManager(),
		focusSidebar: true,
	}
}

// SetModules configures the security modules used for scanning.
// Call this before passing the App to tea.NewProgram.
func (a *App) SetModules(mods []core.SecurityModule) {
	a.modules = mods
}

// SetTools configures the security tools and detects their status.
// Call this before passing the App to tea.NewProgram.
func (a *App) SetTools(allTools []core.SecurityTool) {
	a.tools = allTools
	a.refreshTools()
}

// refreshTools re-detects all tool statuses and updates sidebar + SecStore.
func (a *App) refreshTools() {
	a.toolStatuses = make(map[string]core.ToolStatus)
	for _, t := range a.tools {
		a.toolStatuses[t.ID()] = t.Detect(a.platform)
	}

	// Update sidebar TOOLS section with installed/active tools.
	var sidebarTools []SidebarItem
	for _, t := range a.tools {
		status := a.toolStatuses[t.ID()]
		if status == core.ToolActive || status == core.ToolInstalled {
			badge := "[OFF]"
			if status == core.ToolActive {
				badge = "[ON]"
			}
			spinning := a.jobs != nil && a.jobs.HasRunning(t.ID())
			sidebarTools = append(sidebarTools, SidebarItem{
				Label:   t.Name(),
				Section: "tools",
				ID:      t.ID(),
				Badge:   badge,
				Spinner: spinning,
			})
		}
	}
	a.sidebar = a.sidebar.SetTools(sidebarTools)

	// Update SecStore with not-installed tools.
	a.secstoreView = a.secstoreView.SetTools(a.tools, a.toolStatuses)
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

	case installToolRequestMsg:
		return a.handleInstallTool(msg)

	case installToolResultMsg:
		a.secstoreView, _ = a.secstoreView.Update(msg)
		return a, nil

	case refreshToolsMsg:
		a.refreshTools()
		return a, nil

	case toolActionResultMsg:
		a.toolView, _ = a.toolView.Update(msg)
		// Refresh tool statuses so sidebar badges ([ON]/[OFF]) update after start/stop.
		a.refreshTools()
		return a, nil

	case startJobMsg:
		a.refreshTools()
		return a, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
			return jobTickMsg{}
		})

	case jobCompletedMsg:
		a.jobs.Complete(msg.JobID, msg.Result)
		a.refreshTools()
		// Forward to toolView if it's showing this tool.
		a.toolView, _ = a.toolView.Update(msg)
		return a, nil

	case jobTickMsg:
		a.spinnerFrame = (a.spinnerFrame + 1) % len(spinnerFrames)
		a.sidebar = a.sidebar.SetSpinnerFrame(a.spinnerFrame)
		if a.jobs.HasAnyRunning() {
			return a, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
				return jobTickMsg{}
			})
		}
		// One last refresh to clear spinner badges.
		a.refreshTools()
		return a, nil

	case tea.KeyMsg:
		// Quit confirmation when jobs are running.
		if a.confirmQuit {
			switch msg.String() {
			case "y", "Y":
				a.quitting = true
				return a, tea.Quit
			case "n", "N", "esc":
				a.confirmQuit = false
			}
			return a, nil
		}

		// Help overlay takes priority over everything.
		if a.showHelp {
			switch msg.String() {
			case "?", "esc", "q", "enter":
				a.showHelp = false
			}
			return a, nil
		}

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

const (
	minWidth  = 60
	minHeight = 15
)

func (a *App) View() string {
	if a.quitting {
		return ""
	}

	// Before the first WindowSizeMsg we have 0×0 — render nothing.
	if a.width == 0 || a.height == 0 {
		return ""
	}

	// Terminal too small to render properly.
	if a.width < minWidth || a.height < minHeight {
		msg := fmt.Sprintf("Terminal too small (%d×%d).\nMinimum: %d×%d",
			a.width, a.height, minWidth, minHeight)
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, msg)
	}

	header := a.renderHeader()
	footer := a.renderFooter()
	body := a.renderBody()

	layout := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return lipgloss.Place(a.width, a.height, lipgloss.Left, lipgloss.Top, layout)
}

func (a *App) handleGlobalKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "q", "ctrl+c":
		if a.jobs != nil && a.jobs.HasAnyRunning() {
			a.confirmQuit = true
			return nil, true
		}
		a.quitting = true
		return tea.Quit, true
	case "tab":
		a.focusSidebar = !a.focusSidebar
		a.sidebar = a.sidebar.SetFocused(a.focusSidebar)
		return nil, true
	case "?":
		a.showHelp = !a.showHelp
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
		selected := a.sidebar.Selected()
		switch selected.Section {
		case "modules":
			a.syncModuleView()
		case "tools":
			if a.findToolManager(selected.ID) != nil {
				a.syncToolView(selected.ID)
			}
		}
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

	// SecStore has its own navigation.
	if selected.Section == "secstore" {
		switch msg.String() {
		case "h", "left", "esc":
			// Only go back if SecStore is in idle state (no overlay).
			if a.secstoreView.state == secStoreIdle {
				a.focusSidebar = true
				a.sidebar = a.sidebar.SetFocused(true)
				return a, nil
			}
		}

		var cmd tea.Cmd
		a.secstoreView, cmd = a.secstoreView.Update(msg)
		return a, cmd
	}

	// Tool management view.
	if selected.Section == "tools" {
		if a.findToolManager(selected.ID) != nil {
			a.syncToolView(selected.ID)
			switch msg.String() {
			case "h", "left", "esc":
				if a.toolView.state == tvIdle {
					a.focusSidebar = true
					a.sidebar = a.sidebar.SetFocused(true)
					return a, nil
				}
			}
			var cmd tea.Cmd
			a.toolView, cmd = a.toolView.Update(msg)
			// If the tool view just started a background job, kick off the ticker.
			if cmd != nil && a.jobs.HasAnyRunning() {
				a.refreshTools()
				return a, tea.Batch(cmd, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
					return jobTickMsg{}
				}))
			}
			return a, cmd
		}
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

// --- Tool install ---

func (a *App) handleInstallTool(msg installToolRequestMsg) (tea.Model, tea.Cmd) {
	tool := msg.Tool
	platform := a.platform
	return a, func() tea.Msg {
		cmd := tool.InstallCommand(platform)
		if cmd == "" {
			return installToolResultMsg{
				ToolID: tool.ID(),
				Err:    fmt.Errorf("no install command available for this platform"),
			}
		}
		// Execute the install command via shell.
		out, err := execShell(cmd)
		if err != nil {
			return installToolResultMsg{
				ToolID: tool.ID(),
				Err:    fmt.Errorf("%s: %s", err, out),
			}
		}
		return installToolResultMsg{ToolID: tool.ID(), Err: nil}
	}
}

// execShell runs a command string through sh -c and returns combined output.
func execShell(command string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
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

// findToolManager returns the ToolManager for a given tool ID, or nil.
func (a *App) findToolManager(id string) core.ToolManager {
	for _, t := range a.tools {
		if t.ID() == id {
			if tm, ok := t.(core.ToolManager); ok {
				return tm
			}
		}
	}
	return nil
}

// syncToolView initializes or refreshes the ToolView for the given tool ID.
func (a *App) syncToolView(id string) {
	if a.toolViewID == id {
		return // already loaded
	}
	tm := a.findToolManager(id)
	if tm == nil {
		return
	}
	w, h := a.contentDimensions()
	a.toolView = NewToolView(tm).SetJobs(a.jobs).SetSize(w, h)
	a.toolViewID = id
}

// --- Layout ---

func (a *App) updateLayout() {
	contentWidth, bodyHeight := a.contentDimensions()

	a.sidebar = a.sidebar.SetSize(sidebarWidth, bodyHeight)
	a.overview = a.overview.SetSize(contentWidth, bodyHeight)
	a.scannerView = a.scannerView.SetSize(contentWidth, bodyHeight)
	a.moduleView = a.moduleView.SetSize(contentWidth, bodyHeight)
	a.secstoreView = a.secstoreView.SetSize(contentWidth, bodyHeight)
	a.toolView = a.toolView.SetSize(contentWidth, bodyHeight)
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

	jobIndicator := ""
	if a.jobs != nil {
		if running := a.jobs.RunningJobs(); len(running) > 0 {
			frame := spinnerFrames[a.spinnerFrame%len(spinnerFrames)]
			jobIndicator = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).
				Render(fmt.Sprintf(" %s %d job(s) ", frame, len(running)))
		}
	}

	right := fmt.Sprintf("Score: %s/100  %s%s  %s",
		score,
		jobIndicator,
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
	if a.confirmQuit {
		return StyleFooter.Width(a.width).Render(
			StyleKeyhint.Render("[y]") + " Quit  " +
				StyleKeyhint.Render("[n]") + " Cancel",
		)
	}

	if a.showHelp {
		return StyleFooter.Width(a.width).Render(
			StyleKeyhint.Render("[?]") + " Close help  " +
				StyleKeyhint.Render("[Esc]") + " Close help",
		)
	}

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

	focusLabel := "→ Content"
	if a.focusSidebar {
		focusLabel = "← Sidebar"
	}

	// Global hints (always visible).
	global := []string{
		StyleKeyhint.Render("[Tab]") + " " + focusLabel,
		StyleKeyhint.Render("[s]") + " Scan",
		StyleKeyhint.Render("[?]") + " Help",
		StyleKeyhint.Render("[q]") + " Quit",
	}

	// Contextual hints from the active content view.
	ctx := a.contextHints()

	sep := lipgloss.NewStyle().Foreground(ColorDimmed)
	var parts []string
	parts = append(parts, strings.Join(global, "  "))
	if len(ctx) > 0 {
		var styled []string
		for _, h := range ctx {
			styled = append(styled, StyleKeyhint.Render(h))
		}
		parts = append(parts, sep.Render("│")+" "+strings.Join(styled, "  "))
	}

	return StyleFooter.Width(a.width).Render(strings.Join(parts, "  "))
}

func (a *App) contextHints() []string {
	if a.focusSidebar {
		return []string{"[j/k] Navigate", "[Enter] Select"}
	}
	selected := a.sidebar.Selected()
	switch selected.Section {
	case "modules":
		if len(a.moduleView.findings) > 0 {
			return a.moduleView.ContextHints()
		}
		return []string{"[h] Back"}
	case "secstore":
		return a.secstoreView.ContextHints()
	case "tools":
		if a.findToolManager(selected.ID) != nil {
			a.syncToolView(selected.ID)
			return a.toolView.ContextHints()
		}
		return []string{"[h] Back"}
	default:
		return nil
	}
}

func (a *App) renderBody() string {
	sidebarView := a.sidebar.View()
	contentView := a.renderContent()

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)
}

func (a *App) renderContent() string {
	contentWidth, bodyHeight := a.contentDimensions()

	// Quit confirm overlay.
	if a.confirmQuit {
		return a.renderQuitConfirm(contentWidth, bodyHeight)
	}

	// Help overlay takes priority.
	if a.showHelp {
		return a.renderHelp(contentWidth, bodyHeight)
	}

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
		if a.jobs != nil {
			allJobs := a.jobs.RunningJobs()
			o = o.SetActiveJobs(allJobs, a.spinnerFrame)
		}
		return o.View()
	case "modules":
		return a.renderModuleContent(selected, contentWidth, bodyHeight)
	case "tools":
		return a.renderToolContent(selected, contentWidth, bodyHeight)
	case "secstore":
		sv := a.secstoreView.SetSize(contentWidth, bodyHeight)
		return sv.View()
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

// renderToolContent shows the 4-panel ToolView for tools with ToolManager,
// or basic info for tools without one.
func (a *App) renderToolContent(item SidebarItem, w, h int) string {
	if a.findToolManager(item.ID) != nil {
		a.syncToolView(item.ID)
		tv := a.toolView.SetSize(w, h).SetSpinnerFrame(a.spinnerFrame)
		return tv.View()
	}

	// Fallback: basic info for tools without ToolManager.
	title := StyleTitle.Render(item.Label)

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	okStyle := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning)

	var lines []string
	lines = append(lines, "")

	status := a.toolStatuses[item.ID]
	switch status {
	case core.ToolActive:
		lines = append(lines, "  Status: "+okStyle.Render("Active"))
	case core.ToolInstalled:
		lines = append(lines, "  Status: "+warnStyle.Render("Installed (not running)"))
	default:
		lines = append(lines, "  Status: "+dimStyle.Render("Unknown"))
	}

	for _, t := range a.tools {
		if t.ID() == item.ID {
			lines = append(lines, "")
			lines = append(lines, "  "+dimStyle.Render(t.Description()))
			break
		}
	}

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderQuitConfirm(w, h int) string {
	title := StyleTitle.Render("Quit SecTUI?")

	warnStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	okBtn := lipgloss.NewStyle().Foreground(ColorOK).Bold(true)
	badBtn := lipgloss.NewStyle().Foreground(ColorCritical).Bold(true)

	running := a.jobs.RunningJobs()

	var lines []string
	lines = append(lines, "")
	lines = append(lines, warnStyle.Render(fmt.Sprintf(
		"  %d background job(s) still running:", len(running))))
	lines = append(lines, "")

	for _, job := range running {
		frame := spinnerFrames[a.spinnerFrame%len(spinnerFrames)]
		lines = append(lines, fmt.Sprintf("  %s %s  (%s)",
			warnStyle.Render(frame),
			job.Label,
			dimStyle.Render(FormatElapsed(job.Elapsed())),
		))
	}

	lines = append(lines, "")
	lines = append(lines, dimStyle.Render("  Quitting will not interrupt running jobs."))
	lines = append(lines, "")
	lines = append(lines, "  "+okBtn.Render("[y]")+" quit  "+badBtn.Render("[n]")+" cancel")

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(w).Height(h).Render(content)
}

func (a *App) renderHelp(w, h int) string {
	title := StyleTitle.Render("Keyboard Shortcuts")

	dimStyle := lipgloss.NewStyle().Foreground(ColorDimmed)
	keyStyle := StyleKeyhint
	sepStyle := lipgloss.NewStyle().Foreground(ColorDimmed)

	sep := sepStyle.Render(strings.Repeat("\u2500", maxInt(w-6, 10)))

	type entry struct {
		key  string
		desc string
	}

	sections := []struct {
		label   string
		entries []entry
	}{
		{
			label: "Navigation",
			entries: []entry{
				{"Tab", "Switch focus between sidebar and content"},
				{"j / k", "Move cursor down / up"},
				{"\u2191 / \u2193", "Move cursor down / up"},
				{"Enter / l", "Select item / enter module"},
				{"Esc / h", "Go back to sidebar"},
			},
		},
		{
			label: "Scanning",
			entries: []entry{
				{"s", "Start security scan"},
				{"Esc", "Cancel running scan"},
			},
		},
		{
			label: "Module View",
			entries: []entry{
				{"Space", "Toggle fix selection"},
				{"a", "Select / deselect all fixable findings"},
				{"Enter", "Apply selected fixes"},
			},
		},
		{
			label: "Tool View",
			entries: []entry{
				{"1-4", "Execute quick action"},
				{"r", "Refresh tool data"},
			},
		},
		{
			label: "General",
			entries: []entry{
				{"?", "Toggle this help"},
				{"q", "Quit SecTUI"},
			},
		},
	}

	var lines []string
	lines = append(lines, "")

	for i, sec := range sections {
		lines = append(lines, "  "+dimStyle.Bold(true).Render(sec.label))
		for _, e := range sec.entries {
			lines = append(lines, fmt.Sprintf("    %s  %s",
				keyStyle.Width(12).Render(e.key),
				dimStyle.Render(e.desc),
			))
		}
		if i < len(sections)-1 {
			lines = append(lines, "")
		}
	}

	lines = append(lines, "")
	lines = append(lines, sep)
	lines = append(lines, dimStyle.Render("  Press [?] or [Esc] to close"))

	content := title + "\n" + strings.Join(lines, "\n")
	return StyleContent.Width(w).Height(h).Render(content)
}

// Run starts the TUI application.
func Run(platform *core.PlatformInfo, config *core.AppConfig) error {
	app := NewApp(platform, config)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunWithModules starts the TUI application with pre-configured security modules and tools.
// This is the preferred entry point as it enables scanning and tool management.
func RunWithModules(platform *core.PlatformInfo, config *core.AppConfig, modules []core.SecurityModule, allTools []core.SecurityTool) error {
	app := NewApp(platform, config)
	app.SetModules(modules)
	if len(allTools) > 0 {
		app.SetTools(allTools)
	}
	p := tea.NewProgram(app, tea.WithAltScreen())
	app.program = p
	_, err := p.Run()
	return err
}
