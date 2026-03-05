package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/orlandobianco/SecTUI/internal/core"
	"github.com/orlandobianco/SecTUI/internal/modules"
	"github.com/orlandobianco/SecTUI/internal/tools"
	"github.com/orlandobianco/SecTUI/internal/tui"
	"github.com/orlandobianco/SecTUI/locales"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags:
//
//	go build -ldflags "-X main.Version=1.0.0" ./cmd/sectui
var Version = "dev"

// ANSI color codes for terminal output.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiWhite  = "\033[37m"
	ansiBgRed  = "\033[41m"
	ansiBgYel  = "\033[43m"
)

// Global flag values bound by cobra's persistent flags.
var (
	flagNoColor    bool
	flagVerbose    bool
	flagConfigPath string
)

// Harden-specific flag values.
var (
	flagDryRun   bool
	flagNoBackup bool
)

// colorEnabled returns true when colored output should be used.
// It respects --no-color, the NO_COLOR env var, and non-interactive terminals.
func colorEnabled() bool {
	if flagNoColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// Heuristic: if TERM is "dumb" or empty, assume no color support.
	term := os.Getenv("TERM")
	if term == "" || term == "dumb" {
		return false
	}
	return true
}

// c wraps text in an ANSI escape sequence when colors are enabled.
func c(code, text string) string {
	if !colorEnabled() {
		return text
	}
	return code + text + ansiReset
}

// bold is a convenience wrapper for bold text.
func bold(text string) string {
	return c(ansiBold, text)
}

// --------------------------------------------------------------------------
// Scan logic
// --------------------------------------------------------------------------

// severityCounts tallies findings by severity level.
type severityCounts struct {
	crit int
	high int
	med  int
	low  int
	info int
}

func countSeverities(findings []core.Finding) severityCounts {
	var sc severityCounts
	for _, f := range findings {
		switch f.Severity {
		case core.SeverityCritical:
			sc.crit++
		case core.SeverityHigh:
			sc.high++
		case core.SeverityMedium:
			sc.med++
		case core.SeverityLow:
			sc.low++
		case core.SeverityInfo:
			sc.info++
		}
	}
	return sc
}

// severityLabel returns a short, padded, optionally colored severity tag.
func severityLabel(s core.Severity) string {
	switch s {
	case core.SeverityCritical:
		return c(ansiBgRed+ansiWhite+ansiBold, " CRIT ")
	case core.SeverityHigh:
		return c(ansiRed+ansiBold, " HIGH ")
	case core.SeverityMedium:
		return c(ansiYellow+ansiBold, " MED  ")
	case core.SeverityLow:
		return c(ansiCyan, " LOW  ")
	case core.SeverityInfo:
		return c(ansiWhite, " INFO ")
	default:
		return "  ???  "
	}
}

// runScan executes a scan across the supplied modules and returns a Report.
//
// scanType is one of "quick", "full", or a specific module ID (e.g. "ssh").
// When scanType names a specific module, only that module is scanned.
func runScan(platform *core.PlatformInfo, cfg *core.AppConfig, scanType string, modules []core.SecurityModule) *core.Report {
	start := time.Now()

	ctx := &core.ScanContext{
		Platform: platform,
		Config:   cfg,
	}

	// Filter to applicable modules and respect excluded list.
	excluded := make(map[string]bool, len(cfg.Scan.ExcludedModules))
	for _, id := range cfg.Scan.ExcludedModules {
		excluded[id] = true
	}

	var toScan []core.SecurityModule
	for _, m := range modules {
		if !m.IsApplicable(platform) {
			continue
		}
		if excluded[m.ID()] {
			if flagVerbose {
				fmt.Fprintf(os.Stderr, "  skipping excluded module: %s\n", m.ID())
			}
			continue
		}
		// If the user asked for a specific module, keep only that one.
		if scanType != "quick" && scanType != "full" && scanType != "" {
			if m.ID() != scanType {
				continue
			}
		}
		toScan = append(toScan, m)
	}

	// Sort by priority (lower first).
	sort.Slice(toScan, func(i, j int) bool {
		return toScan[i].Priority() < toScan[j].Priority()
	})

	var allFindings []core.Finding
	var scannedIDs []string

	for _, m := range toScan {
		if flagVerbose {
			fmt.Fprintf(os.Stderr, "  scanning module: %s\n", m.ID())
		}
		findings := m.Scan(ctx)
		allFindings = append(allFindings, findings...)
		scannedIDs = append(scannedIDs, m.ID())
	}

	score := core.CalculateScore(allFindings)

	return &core.Report{
		Timestamp:      start,
		Platform:       *platform,
		Score:          score,
		Findings:       allFindings,
		ModulesScanned: scannedIDs,
		Duration:       time.Since(start),
	}
}

// printReport writes a formatted table of findings and a score summary to stdout.
func printReport(report *core.Report) {
	if len(report.Findings) == 0 {
		fmt.Println(c(ansiGreen+ansiBold, "No findings. Your system looks good!"))
		fmt.Printf("\n%s %s\n", bold("Score:"), c(ansiGreen+ansiBold, fmt.Sprintf("%d/100", report.Score)))
		return
	}

	// Table header.
	header := fmt.Sprintf("  %-6s | %-40s | %-12s | %s", "SEV", "Finding", "Module", "Fix?")
	divider := "  " + strings.Repeat("-", 6) + "-+-" + strings.Repeat("-", 40) + "-+-" + strings.Repeat("-", 12) + "-+-" + strings.Repeat("-", 4)

	fmt.Println()
	fmt.Println(bold(header))
	fmt.Println(divider)

	for _, f := range report.Findings {
		hasFix := "no"
		if f.FixID != "" {
			hasFix = c(ansiGreen, "yes")
		}

		title := f.TitleKey
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		module := f.Module
		if len(module) > 12 {
			module = module[:9] + "..."
		}

		fmt.Printf("  %s | %-40s | %-12s | %s\n", severityLabel(f.Severity), title, module, hasFix)
	}

	fmt.Println(divider)

	// Score summary.
	sc := countSeverities(report.Findings)
	scoreColor := ansiGreen
	switch {
	case report.Score < 40:
		scoreColor = ansiRed + ansiBold
	case report.Score < 70:
		scoreColor = ansiYellow + ansiBold
	}

	fmt.Printf("\n  %s %s  %s %s CRIT, %s HIGH, %s MED, %s LOW\n",
		bold("Score:"),
		c(scoreColor, fmt.Sprintf("%d/100", report.Score)),
		bold("Findings:"),
		c(ansiRed, fmt.Sprintf("%d", sc.crit)),
		c(ansiRed, fmt.Sprintf("%d", sc.high)),
		c(ansiYellow, fmt.Sprintf("%d", sc.med)),
		c(ansiCyan, fmt.Sprintf("%d", sc.low)),
	)

	if flagVerbose {
		fmt.Printf("  Modules scanned: %s  Duration: %s\n",
			strings.Join(report.ModulesScanned, ", "),
			report.Duration.Round(time.Millisecond),
		)
	}
}

// --------------------------------------------------------------------------
// Module loading helper
// --------------------------------------------------------------------------

func loadModules() []core.SecurityModule {
	return modules.AllModules()
}

// --------------------------------------------------------------------------
// Config loading helper
// --------------------------------------------------------------------------

// loadConfig loads the configuration, either from the custom path specified
// by --config or from the default location.
func loadConfig() (*core.AppConfig, error) {
	if flagConfigPath != "" {
		return core.LoadConfigFrom(flagConfigPath)
	}
	return core.LoadConfig()
}

// --------------------------------------------------------------------------
// Exit code constants
// --------------------------------------------------------------------------

const (
	exitOK       = 0
	exitError    = 1
	exitCritical = 4 // Critical findings detected
)

// --------------------------------------------------------------------------
// Cobra commands
// --------------------------------------------------------------------------

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "sectui",
		Short: "SecTUI - Interactive security hardening for your server",
		Long: `SecTUI is an open-source TUI tool that helps you secure a Linux or macOS
server. It combines security scanning, an interactive hardening wizard,
and a real-time monitoring dashboard.

Running "sectui" with no subcommand launches the TUI dashboard.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			platform := core.DetectPlatform()
			cfg, err := loadConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
			}
			mods := modules.ApplicableModules(platform)
			allTools := tools.ApplicableTools(platform)
			return tui.RunWithModules(platform, cfg, mods, allTools)
		},
	}

	// Global persistent flags.
	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&flagConfigPath, "config", "", "Path to custom config file")

	// Register subcommands.
	root.AddCommand(newScanCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newHardenCmd())
	root.AddCommand(newReportCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newJobCmd())

	return root
}

// --- scan command ---

func newScanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [quick|full|<module>]",
		Short: "Scan your system for security issues",
		Long: `Run a security scan against your system.

Examples:
  sectui scan           Quick scan (default)
  sectui scan quick     Explicit quick scan
  sectui scan full      Full audit of all modules
  sectui scan ssh       Scan only the SSH module`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scanType := "quick"
			if len(args) > 0 {
				scanType = strings.ToLower(args[0])
			}

			return executeScan(scanType)
		},
	}

	return cmd
}

func executeScan(scanType string) error {
	if flagVerbose {
		fmt.Fprintf(os.Stderr, "Detecting platform...\n")
	}
	platform := core.DetectPlatform()

	if flagVerbose {
		fmt.Fprintf(os.Stderr, "Platform: %s %s %s (%s)\n",
			platform.OS, platform.Distro, platform.Version, platform.Arch)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}

	// Override color setting from config if --no-color was not explicitly set.
	if !flagNoColor && !cfg.General.Color {
		flagNoColor = true
	}

	allModules := loadModules()

	if len(allModules) == 0 {
		fmt.Println(c(ansiYellow, "No security modules registered yet."))
		fmt.Println("Module implementations are coming soon. Run with --verbose for details.")
		if flagVerbose {
			fmt.Fprintf(os.Stderr, "hint: implement modules in internal/modules/ and register them in loadModules()\n")
		}
		return nil
	}

	fmt.Printf("%s Running %s scan...\n\n", bold("SecTUI"), c(ansiCyan, scanType))

	report := runScan(platform, cfg, scanType, allModules)
	printReport(report)

	// Exit with code 4 if critical findings were found.
	sc := countSeverities(report.Findings)
	if sc.crit > 0 {
		os.Exit(exitCritical)
	}

	return nil
}

// --- status command ---

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [score]",
		Short: "Show system security status",
		Long: `Display a summary of your system's security posture.

Examples:
  sectui status         Full status summary
  sectui status score   Print only the numeric score (0-100)`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scoreOnly := false
			if len(args) > 0 && strings.ToLower(args[0]) == "score" {
				scoreOnly = true
			}

			return executeStatus(scoreOnly)
		},
	}

	return cmd
}

func executeStatus(scoreOnly bool) error {
	platform := core.DetectPlatform()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}

	if !flagNoColor && !cfg.General.Color {
		flagNoColor = true
	}

	allModules := loadModules()

	if len(allModules) == 0 && scoreOnly {
		// No modules yet: print a neutral score.
		fmt.Println("100")
		return nil
	}

	if len(allModules) == 0 {
		fmt.Println(c(ansiYellow, "No security modules registered yet."))
		fmt.Println("Module implementations are coming soon.")
		return nil
	}

	report := runScan(platform, cfg, "quick", allModules)

	if scoreOnly {
		fmt.Println(report.Score)
		return nil
	}

	// Full status output.
	fmt.Printf("\n%s\n", bold("SecTUI Security Status"))
	fmt.Printf("  Platform:  %s %s %s (%s)\n",
		platform.OS, platform.Distro, platform.Version, platform.Arch)
	fmt.Printf("  Init:      %s\n", platform.InitSystem)
	fmt.Printf("  Package:   %s\n", platform.PackageManager)

	if platform.IsContainer {
		fmt.Printf("  Container: %s\n", c(ansiYellow, "yes"))
	}
	if platform.IsWSL {
		fmt.Printf("  WSL:       %s\n", c(ansiYellow, "yes"))
	}

	printReport(report)

	sc := countSeverities(report.Findings)
	if sc.crit > 0 {
		os.Exit(exitCritical)
	}

	return nil
}

// --- harden command ---

func newHardenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "harden [check|ssh|firewall|kernel|all]",
		Short: "Harden your system interactively",
		Long: `Scan for security issues and apply fixes.

By default, harden runs in dry-run mode (--dry-run=true), showing what
changes would be made without actually modifying your system. Use
--dry-run=false to apply changes.

Examples:
  sectui harden                 Interactive: scan, show fixes, let user choose
  sectui harden check           Show score + fixable findings
  sectui harden ssh             Harden SSH specifically (dry-run by default)
  sectui harden firewall        Setup/harden firewall
  sectui harden kernel          Apply sysctl tweaks
  sectui harden all             Apply all available fixes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// Interactive mode: scan all, show fixable findings, let user choose.
				return executeHardenInteractive()
			}

			switch strings.ToLower(args[0]) {
			case "check":
				return executeHardenCheck()
			case "ssh", "firewall", "kernel":
				return executeHardenModule(strings.ToLower(args[0]))
			case "all":
				return executeHardenAll()
			default:
				return fmt.Errorf("unknown harden target: %q (expected check, ssh, firewall, kernel, or all)", args[0])
			}
		},
	}

	cmd.Flags().BoolVar(&flagDryRun, "dry-run", true, "Show what would change without applying (default: true)")
	cmd.Flags().BoolVar(&flagNoBackup, "no-backup", false, "Don't backup configs before changing")

	return cmd
}

// executeHardenInteractive scans all modules and lets the user choose which
// fixable findings to address one by one.
func executeHardenInteractive() error {
	platform := core.DetectPlatform()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}
	if !flagNoColor && !cfg.General.Color {
		flagNoColor = true
	}

	allModules := loadModules()
	if len(allModules) == 0 {
		fmt.Println(c(ansiYellow, "No security modules registered yet."))
		return nil
	}

	fmt.Printf("%s Scanning all modules...\n\n", bold("SecTUI Harden"))

	report := runScan(platform, cfg, "full", allModules)

	fixable := filterFixable(report.Findings)
	if len(fixable) == 0 {
		fmt.Println(c(ansiGreen+ansiBold, "No fixable findings. Your system looks good!"))
		fmt.Printf("\n%s %s\n", bold("Score:"), c(ansiGreen+ansiBold, fmt.Sprintf("%d/100", report.Score)))
		return nil
	}

	printFixableFindings(fixable)
	fmt.Printf("\n%s %d fixable finding(s) found.\n\n", bold("Summary:"), len(fixable))

	hardenFindings(fixable, platform, cfg, allModules)

	return nil
}

// executeHardenCheck runs a quick scan and shows fixable findings and score potential.
func executeHardenCheck() error {
	platform := core.DetectPlatform()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}
	if !flagNoColor && !cfg.General.Color {
		flagNoColor = true
	}

	allModules := loadModules()
	if len(allModules) == 0 {
		fmt.Println(c(ansiYellow, "No security modules registered yet."))
		return nil
	}

	fmt.Printf("%s Running check...\n\n", bold("SecTUI Harden"))

	report := runScan(platform, cfg, "quick", allModules)

	fixable := filterFixable(report.Findings)
	if len(fixable) == 0 {
		fmt.Println(c(ansiGreen+ansiBold, "No fixable findings. Your system looks good!"))
		fmt.Printf("\n%s %s\n", bold("Score:"), c(ansiGreen+ansiBold, fmt.Sprintf("%d/100", report.Score)))
		return nil
	}

	printFixableFindings(fixable)

	// Calculate potential score if all fixable findings were resolved.
	var nonFixable []core.Finding
	for _, f := range report.Findings {
		if f.FixID == "" {
			nonFixable = append(nonFixable, f)
		}
	}
	potentialScore := core.CalculateScore(nonFixable)

	currentScoreColor := ansiGreen
	switch {
	case report.Score < 40:
		currentScoreColor = ansiRed + ansiBold
	case report.Score < 70:
		currentScoreColor = ansiYellow + ansiBold
	}

	potentialScoreColor := ansiGreen
	switch {
	case potentialScore < 40:
		potentialScoreColor = ansiRed + ansiBold
	case potentialScore < 70:
		potentialScoreColor = ansiYellow + ansiBold
	}

	fmt.Printf("\n%s %s  ->  %s %s (if all %d fix(es) applied)\n",
		bold("Score:"),
		c(currentScoreColor, fmt.Sprintf("%d/100", report.Score)),
		bold("Potential:"),
		c(potentialScoreColor, fmt.Sprintf("%d/100", potentialScore)),
		len(fixable),
	)

	return nil
}

// executeHardenModule scans a specific module and applies its fixable findings.
func executeHardenModule(moduleID string) error {
	platform := core.DetectPlatform()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}
	if !flagNoColor && !cfg.General.Color {
		flagNoColor = true
	}

	allModules := loadModules()
	if len(allModules) == 0 {
		fmt.Println(c(ansiYellow, "No security modules registered yet."))
		return nil
	}

	fmt.Printf("%s Scanning module %s...\n\n", bold("SecTUI Harden"), c(ansiCyan, moduleID))

	beforeReport := runScan(platform, cfg, moduleID, allModules)

	fixable := filterFixable(beforeReport.Findings)
	if len(fixable) == 0 {
		fmt.Println(c(ansiGreen+ansiBold, fmt.Sprintf("No fixable findings for module %q.", moduleID)))
		fmt.Printf("\n%s %s\n", bold("Score:"), c(ansiGreen+ansiBold, fmt.Sprintf("%d/100", beforeReport.Score)))
		return nil
	}

	printFixableFindings(fixable)
	fmt.Println()

	hardenFindings(fixable, platform, cfg, allModules)

	// Show before/after score.
	afterReport := runScan(platform, cfg, moduleID, allModules)
	fmt.Printf("\n%s %s  ->  %s %s\n",
		bold("Before:"),
		scoreColorized(beforeReport.Score),
		bold("After:"),
		scoreColorized(afterReport.Score),
	)

	return nil
}

// executeHardenAll scans all modules and applies all fixable findings.
func executeHardenAll() error {
	platform := core.DetectPlatform()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}
	if !flagNoColor && !cfg.General.Color {
		flagNoColor = true
	}

	allModules := loadModules()
	if len(allModules) == 0 {
		fmt.Println(c(ansiYellow, "No security modules registered yet."))
		return nil
	}

	fmt.Printf("%s Scanning all modules...\n\n", bold("SecTUI Harden All"))

	beforeReport := runScan(platform, cfg, "full", allModules)

	fixable := filterFixable(beforeReport.Findings)
	if len(fixable) == 0 {
		fmt.Println(c(ansiGreen+ansiBold, "No fixable findings. Your system looks good!"))
		fmt.Printf("\n%s %s\n", bold("Score:"), c(ansiGreen+ansiBold, fmt.Sprintf("%d/100", beforeReport.Score)))
		return nil
	}

	printFixableFindings(fixable)
	fmt.Println()

	hardenFindings(fixable, platform, cfg, allModules)

	// Show before/after score.
	afterReport := runScan(platform, cfg, "full", allModules)
	fmt.Printf("\n%s %s  ->  %s %s\n",
		bold("Before:"),
		scoreColorized(beforeReport.Score),
		bold("After:"),
		scoreColorized(afterReport.Score),
	)

	return nil
}

// --------------------------------------------------------------------------
// Harden helpers
// --------------------------------------------------------------------------

// filterFixable returns only findings that have a FixID set.
func filterFixable(findings []core.Finding) []core.Finding {
	var fixable []core.Finding
	for _, f := range findings {
		if f.FixID != "" {
			fixable = append(fixable, f)
		}
	}
	return fixable
}

// printFixableFindings prints a table of fixable findings.
func printFixableFindings(findings []core.Finding) {
	header := fmt.Sprintf("  %-6s | %-40s | %-12s | %-20s", "SEV", "Finding", "Module", "Fix ID")
	divider := "  " + strings.Repeat("-", 6) + "-+-" + strings.Repeat("-", 40) + "-+-" + strings.Repeat("-", 12) + "-+-" + strings.Repeat("-", 20)

	fmt.Println(bold(header))
	fmt.Println(divider)

	for _, f := range findings {
		title := f.TitleKey
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		module := f.Module
		if len(module) > 12 {
			module = module[:9] + "..."
		}
		fixID := f.FixID
		if len(fixID) > 20 {
			fixID = fixID[:17] + "..."
		}

		fmt.Printf("  %s | %-40s | %-12s | %s\n", severityLabel(f.Severity), title, module, c(ansiCyan, fixID))
	}

	fmt.Println(divider)
}

// findModuleByID returns the SecurityModule matching the given ID, or nil.
func findModuleByID(moduleID string, allModules []core.SecurityModule) core.SecurityModule {
	for _, m := range allModules {
		if m.ID() == moduleID {
			return m
		}
	}
	return nil
}

// hardenFindings iterates over fixable findings and previews/applies each fix.
// It always asks the user for confirmation before applying each change.
// It respects the --dry-run and --no-backup flags.
func hardenFindings(findings []core.Finding, platform *core.PlatformInfo, cfg *core.AppConfig, allModules []core.SecurityModule) {
	// Check root privileges before attempting to apply fixes (skip check for dry-run).
	if !flagDryRun && os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "\n  %s Applying fixes requires root privileges.\n", c(ansiRed+ansiBold, "Error:"))
		fmt.Fprintf(os.Stderr, "  Run with: %s\n\n", c(ansiGreen+ansiBold, "sudo sectui harden --dry-run=false"))
		return
	}

	scanCtx := &core.ScanContext{
		Platform: platform,
		Config:   cfg,
	}

	applyCtx := &core.ApplyContext{
		Platform: platform,
		Config:   cfg,
		DryRun:   flagDryRun,
		Backup:   !flagNoBackup,
	}

	reader := bufio.NewReader(os.Stdin)

	for i, f := range findings {
		mod := findModuleByID(f.Module, allModules)
		if mod == nil {
			fmt.Fprintf(os.Stderr, "warning: module %q not found for fix %s, skipping\n", f.Module, f.FixID)
			continue
		}

		fmt.Printf("\n%s [%d/%d] %s (%s)\n",
			bold("Fix:"),
			i+1, len(findings),
			f.TitleKey,
			c(ansiCyan, f.FixID),
		)

		// Show preview.
		preview, err := mod.PreviewFix(f.FixID, scanCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s could not preview: %v\n", c(ansiYellow, "warning:"), err)
			continue
		}

		fmt.Println()
		for _, line := range strings.Split(preview, "\n") {
			if line != "" {
				fmt.Printf("  %s\n", line)
			}
		}

		if flagDryRun {
			fmt.Printf("\n  %s\n", c(ansiYellow, "[dry-run] No changes applied."))
			continue
		}

		// Always ask for explicit user confirmation before modifying the system.
		fmt.Printf("\n  Apply this fix? [y/N] ")
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "y" {
			fmt.Println("  Skipped.")
			continue
		}

		// Apply the fix.
		result, err := mod.ApplyFix(f.FixID, applyCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s %v\n", c(ansiRed+ansiBold, "Error:"), err)
			continue
		}

		if result.Success {
			fmt.Printf("  %s %s\n", c(ansiGreen+ansiBold, "OK:"), result.Message)
			if result.BackupPath != "" {
				fmt.Printf("  %s %s\n", bold("Backup:"), result.BackupPath)
			}
		} else {
			fmt.Printf("  %s %s\n", c(ansiRed+ansiBold, "Failed:"), result.Message)
		}
	}
}

// scoreColorized returns a colorized score string.
func scoreColorized(score int) string {
	color := ansiGreen + ansiBold
	switch {
	case score < 40:
		color = ansiRed + ansiBold
	case score < 70:
		color = ansiYellow + ansiBold
	}
	return c(color, fmt.Sprintf("%d/100", score))
}

// --- report command ---

var (
	flagReportFormat string
	flagReportOutput string
)

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a security report",
		Long: `Run a scan and export results in a structured format.

Examples:
  sectui report                        Markdown to stdout
  sectui report --format json          JSON to stdout
  sectui report --output report.md     Markdown to file
  sectui report -f json -o report.json JSON to file`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeReport()
		},
	}

	cmd.Flags().StringVarP(&flagReportFormat, "format", "f", "markdown", "Output format: markdown or json")
	cmd.Flags().StringVarP(&flagReportOutput, "output", "o", "", "Write to file instead of stdout")

	return cmd
}

func executeReport() error {
	platform := core.DetectPlatform()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config: %v (using defaults)\n", err)
	}

	allModules := loadModules()
	if len(allModules) == 0 {
		return fmt.Errorf("no security modules registered")
	}

	report := runScan(platform, cfg, "full", allModules)

	var output []byte
	switch strings.ToLower(flagReportFormat) {
	case "json":
		output, err = core.ReportToJSON(report)
		if err != nil {
			return fmt.Errorf("failed to generate JSON: %w", err)
		}
	case "markdown", "md":
		output = []byte(core.ReportToMarkdown(report))
	default:
		return fmt.Errorf("unknown format %q (expected: json, markdown)", flagReportFormat)
	}

	if flagReportOutput != "" {
		if err := os.WriteFile(flagReportOutput, output, 0o644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Report written to %s\n", flagReportOutput)
		return nil
	}

	fmt.Print(string(output))
	return nil
}

// --- config command ---

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config [path|show|set|reset|init]",
		Short: "Manage SecTUI configuration",
		Long: `View and modify the SecTUI configuration file.

Examples:
  sectui config path          Print the config file path
  sectui config show          Print current configuration
  sectui config init          Create default config if missing
  sectui config reset         Reset config to defaults
  sectui config set key val   Set a config value (dotted keys, e.g. general.locale)`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(args[0]) {
			case "path":
				p, err := core.ConfigPath()
				if err != nil {
					return err
				}
				fmt.Println(p)
				return nil

			case "show":
				return executeConfigShow()

			case "init":
				return executeConfigInit()

			case "reset":
				return executeConfigReset()

			case "set":
				if len(args) < 3 {
					return fmt.Errorf("usage: sectui config set <key> <value>")
				}
				return executeConfigSet(args[1], args[2])

			default:
				return fmt.Errorf("unknown config action: %q (expected: path, show, init, reset, set)", args[0])
			}
		},
	}

	return cmd
}

func executeConfigShow() error {
	path, err := core.ConfigPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "No config file found at %s\nRun 'sectui config init' to create one.\n", path)
			return nil
		}
		return err
	}
	fmt.Print(string(data))
	return nil
}

func executeConfigInit() error {
	path, err := core.ConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "Config already exists at %s\nUse 'sectui config reset' to overwrite.\n", path)
		return nil
	}

	cfg := core.DefaultConfig()
	if err := core.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}
	fmt.Printf("Config created at %s\n", path)
	return nil
}

func executeConfigReset() error {
	cfg := core.DefaultConfig()
	if err := core.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to reset config: %w", err)
	}
	path, _ := core.ConfigPath()
	fmt.Printf("Config reset to defaults at %s\n", path)
	return nil
}

func executeConfigSet(key, value string) error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = core.DefaultConfig()
	}

	switch strings.ToLower(key) {
	case "general.locale":
		cfg.General.Locale = value
	case "general.color":
		cfg.General.Color = value == "true" || value == "1"
	case "scan.default_type":
		cfg.Scan.DefaultType = value
	case "harden.dry_run_default":
		cfg.Harden.DryRunDefault = value == "true" || value == "1"
	case "harden.backup_default":
		cfg.Harden.BackupDefault = value == "true" || value == "1"
	default:
		return fmt.Errorf("unknown config key: %q", key)
	}

	if err := core.SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

// --- version command ---

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the SecTUI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sectui %s\n", Version)
		},
	}
}

// --------------------------------------------------------------------------
// main
// --------------------------------------------------------------------------

// resolveLocale determines the active locale using the priority:
// config file > SECTUI_LOCALE env > LANG env > "en".
func resolveLocale() string {
	// Try config file first.
	cfg, err := loadConfig()
	if err == nil && cfg.General.Locale != "" {
		return cfg.General.Locale
	}

	// Try SECTUI_LOCALE environment variable.
	if env := os.Getenv("SECTUI_LOCALE"); env != "" {
		// Normalise "en_US.UTF-8" -> "en"
		return strings.SplitN(strings.SplitN(env, ".", 2)[0], "_", 2)[0]
	}

	// Try LANG environment variable.
	if lang := os.Getenv("LANG"); lang != "" {
		return strings.SplitN(strings.SplitN(lang, ".", 2)[0], "_", 2)[0]
	}

	return "en"
}

func main() {
	// Initialise i18n before any command runs.
	locale := resolveLocale()
	if err := core.InitI18n(locales.FS, locale); err != nil {
		fmt.Fprintf(os.Stderr, "warning: i18n init failed: %v\n", err)
	}

	root := newRootCmd()

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitError)
	}
}
