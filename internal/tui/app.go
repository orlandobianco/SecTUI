package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/orlandobianco/SecTUI/internal/core"
)

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
	program      *tea.Program // stored for async scan progress via p.Send()
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

	case tea.KeyMsg:
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
		// Cancel scan: exit scanning state but keep any partial data.
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
		// When entering a module, prepare the ModuleView with current findings.
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

	// Use the simple RunScanCmd (single Cmd return). For real-time progress,
	// RunScanWithProgressCmd can be used when a.program is set.
	if a.program != nil {
		return a, RunScanWithProgressCmd(a.program, a.modules, a.platform, a.config)
	}
	return a, RunScanCmd(a.modules, a.platform, a.config)
}

func (a *App) handleScanComplete(msg ScanCompleteMsg) (tea.Model, tea.Cmd) {
	a.scanning = false
	a.report = msg.Report

	// Update the scanner view so it knows the scan is done.
	a.scannerView, _ = a.scannerView.Update(msg)

	// Update the overview with the new report.
	a.overview = a.overview.SetReport(msg.Report)

	// Pre-sync the module view in case the user is already looking at a module.
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
	if a.scanning {
		hints := []string{
			StyleKeyhint.Render("[Esc]") + " Cancel",
			StyleKeyhint.Render("[q]") + " Quit",
		}
		content := ""
		for i, h := range hints {
			if i > 0 {
				content += "  "
			}
			content += h
		}
		return StyleFooter.Width(a.width).Render(content)
	}

	hints := []string{
		StyleKeyhint.Render("[Tab]") + " Focus",
		StyleKeyhint.Render("[s]") + " Scan",
		StyleKeyhint.Render("[?]") + " Help",
		StyleKeyhint.Render("[q]") + " Quit",
	}

	content := ""
	for i, h := range hints {
		if i > 0 {
			content += "  "
		}
		content += h
	}

	return StyleFooter.Width(a.width).Render(content)
}

func (a *App) renderBody() string {
	sidebarView := a.sidebar.View()
	contentView := a.renderContent()

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)
}

func (a *App) renderContent() string {
	contentWidth, bodyHeight := a.contentDimensions()

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

func (a *App) renderModuleContent(item SidebarItem, w, h int) string {
	// If we have a report, show the real ModuleView with findings.
	if a.report != nil {
		// Ensure the moduleView matches the selected module.
		if a.moduleView.moduleID != item.ID {
			a.syncModuleView()
		}
		mv := a.moduleView.SetSize(w, h)
		return mv.View()
	}

	// No report yet, show placeholder.
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
