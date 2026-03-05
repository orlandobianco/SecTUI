package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/orlandobianco/SecTUI/internal/core"
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

// loadModules returns all registered security modules.
//
// TODO: Once individual module implementations (NewSSHModule, NewFirewallModule,
// NewNetworkModule, etc.) are created, import the modules package and call:
//
//	modules.ApplicableModules(platform)
//
// For now this returns an empty slice so the CLI compiles and runs.
func loadModules() []core.SecurityModule {
	// TODO: Replace with modules.AllModules() once module implementations exist.
	// import "github.com/orlandobianco/SecTUI/internal/modules"
	// return modules.AllModules()
	return []core.SecurityModule{}
}

// --------------------------------------------------------------------------
// Config loading helper
// --------------------------------------------------------------------------

// loadConfig loads the configuration, either from the custom path specified
// by --config or from the default location.
func loadConfig() (*core.AppConfig, error) {
	if flagConfigPath != "" {
		// TODO: Implement LoadConfigFrom(path) in core package for custom paths.
		// For now, fall through to the default loader and log a warning.
		if flagVerbose {
			fmt.Fprintf(os.Stderr, "warning: --config flag not yet fully supported, using default config path\n")
		}
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
			// Default action: launch TUI dashboard.
			fmt.Println(bold("Starting SecTUI dashboard..."))
			// TODO: Wire actual TUI dashboard (Bubbletea/Ratatui) here.
			return nil
		},
	}

	// Global persistent flags.
	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "Disable colored output")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Enable verbose output")
	root.PersistentFlags().StringVar(&flagConfigPath, "config", "", "Path to custom config file")

	// Register subcommands.
	root.AddCommand(newScanCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newVersionCmd())

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

func main() {
	root := newRootCmd()

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitError)
	}
}
