package core

import "time"

// --- Severity ---

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// --- OS ---

type OS int

const (
	OSLinux OS = iota
	OSDarwin
)

func (o OS) String() string {
	switch o {
	case OSLinux:
		return "linux"
	case OSDarwin:
		return "darwin"
	default:
		return "unknown"
	}
}

// --- InitSystem ---

type InitSystem int

const (
	InitSystemd InitSystem = iota
	InitLaunchd
	InitOpenRC
	InitOther
)

func (i InitSystem) String() string {
	switch i {
	case InitSystemd:
		return "systemd"
	case InitLaunchd:
		return "launchd"
	case InitOpenRC:
		return "openrc"
	case InitOther:
		return "other"
	default:
		return "unknown"
	}
}

// --- PackageManager ---

type PackageManager int

const (
	PkgApt PackageManager = iota
	PkgDnf
	PkgPacman
	PkgBrew
	PkgApk
	PkgNone
)

func (p PackageManager) String() string {
	switch p {
	case PkgApt:
		return "apt"
	case PkgDnf:
		return "dnf"
	case PkgPacman:
		return "pacman"
	case PkgBrew:
		return "brew"
	case PkgApk:
		return "apk"
	case PkgNone:
		return "none"
	default:
		return "unknown"
	}
}

// --- ToolCategory ---

type ToolCategory int

const (
	ToolCatFirewall ToolCategory = iota
	ToolCatIntrusionPrevention
	ToolCatMalware
	ToolCatVPN
	ToolCatFileIntegrity
	ToolCatAccessControl
)

func (tc ToolCategory) String() string {
	switch tc {
	case ToolCatFirewall:
		return "firewall"
	case ToolCatIntrusionPrevention:
		return "intrusion_prevention"
	case ToolCatMalware:
		return "malware"
	case ToolCatVPN:
		return "vpn"
	case ToolCatFileIntegrity:
		return "file_integrity"
	case ToolCatAccessControl:
		return "access_control"
	default:
		return "unknown"
	}
}

// --- ToolStatus ---

type ToolStatus int

const (
	ToolNotInstalled ToolStatus = iota
	ToolInstalled
	ToolActive
	ToolNotApplicable
)

func (ts ToolStatus) String() string {
	switch ts {
	case ToolNotInstalled:
		return "not_installed"
	case ToolInstalled:
		return "installed"
	case ToolActive:
		return "active"
	case ToolNotApplicable:
		return "not_applicable"
	default:
		return "unknown"
	}
}

// --- Core Data Types ---

type Finding struct {
	ID            string
	Module        string // "ssh", "firewall", etc.
	Severity      Severity
	TitleKey      string // i18n key
	DetailKey     string // i18n key (explains WHY)
	FixID         string // empty if no auto-fix available
	CurrentValue  string
	ExpectedValue string
}

type Report struct {
	Timestamp      time.Time
	Platform       PlatformInfo
	Score          int // 0-100
	Findings       []Finding
	ModulesScanned []string
	Duration       time.Duration
}

type PlatformInfo struct {
	OS             OS
	Distro         string // "ubuntu", "debian", "fedora", etc.
	Version        string
	Arch           string // "x86_64", "aarch64"
	InitSystem     InitSystem
	PackageManager PackageManager
	IsContainer    bool
	IsWSL          bool
}

// --- Tool/Service Types ---

type ServiceStatus struct {
	Running bool
	Enabled bool
	PID     int
	Uptime  string
	Extra   map[string]string
}

type QuickAction struct {
	ID          string
	Label       string
	Key         rune
	Dangerous   bool
	Description string
}

type ConfigEntry struct {
	Key   string
	Value string
}

type ActivityEntry struct {
	Timestamp string
	Message   string
}

type ActionResult struct {
	Success bool
	Message string
}

// AppConfig is defined in config.go
