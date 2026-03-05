package tools

import "github.com/orlandobianco/SecTUI/internal/core"

// AllTools returns every known security tool.
func AllTools() []core.SecurityTool {
	return []core.SecurityTool{
		&UFWTool{},
		&FirewalldTool{},
		&Fail2banTool{},
		&CrowdSecTool{},
		&ClamAVTool{},
		&RkhunterTool{},
		&WireGuardTool{},
		&TailscaleTool{},
		&AIDETool{},
		&AppArmorTool{},
	}
}

// ApplicableTools returns tools that are relevant for the current platform.
func ApplicableTools(platform *core.PlatformInfo) []core.SecurityTool {
	var applicable []core.SecurityTool
	for _, t := range AllTools() {
		if t.IsApplicable(platform) {
			applicable = append(applicable, t)
		}
	}
	return applicable
}

// DetectAll returns the status of each applicable tool on this platform.
func DetectAll(platform *core.PlatformInfo) map[string]core.ToolStatus {
	statuses := make(map[string]core.ToolStatus)
	for _, t := range ApplicableTools(platform) {
		statuses[t.ID()] = t.Detect(platform)
	}
	return statuses
}
