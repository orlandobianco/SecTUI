package core

// SecurityModule represents a scan + harden module (ssh, firewall, network, etc.)
type SecurityModule interface {
	ID() string
	NameKey() string
	DescriptionKey() string
	Scan(ctx *ScanContext) []Finding
	AvailableFixes() []Fix
	ApplyFix(fixID string, ctx *ApplyContext) (*ApplyResult, error)
	PreviewFix(fixID string, ctx *ScanContext) (string, error)
	IsApplicable(platform *PlatformInfo) bool
	Priority() int // lower = scanned first
}

type Fix struct {
	ID          string
	FindingID   string
	TitleKey    string
	Description string
	Dangerous   bool
}

type ApplyResult struct {
	Success    bool
	Message    string
	BackupPath string
}

type ScanContext struct {
	Platform *PlatformInfo
	Config   *AppConfig
}

type ApplyContext struct {
	Platform *PlatformInfo
	Config   *AppConfig
	DryRun   bool
	Backup   bool
}

// SecurityTool represents an external security tool (ufw, fail2ban, clamav, etc.)
type SecurityTool interface {
	ID() string
	Name() string
	Description() string
	Category() ToolCategory
	Detect(platform *PlatformInfo) ToolStatus
	InstallCommand(platform *PlatformInfo) string
	IsApplicable(platform *PlatformInfo) bool
}

// ToolManager extends SecurityTool with full management capabilities and UI integration.
type ToolManager interface {
	SecurityTool
	ToolID() string
	GetServiceStatus() ServiceStatus
	QuickActions() []QuickAction
	ConfigSummary() []ConfigEntry
	RecentActivity(n int) []ActivityEntry
	ExecuteAction(actionID string) ActionResult
	RunScan() []Finding
}
