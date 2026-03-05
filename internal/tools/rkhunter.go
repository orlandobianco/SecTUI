package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type RkhunterTool struct{}

func (t *RkhunterTool) ID() string          { return "rkhunter" }
func (t *RkhunterTool) Name() string        { return "rkhunter" }
func (t *RkhunterTool) Description() string { return core.T("tool.rkhunter.description") }
func (t *RkhunterTool) Category() core.ToolCategory { return core.ToolCatMalware }

func (t *RkhunterTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("rkhunter", "")
}

func (t *RkhunterTool) InstallCommand(p *core.PlatformInfo) string {
	if isDarwin(p) {
		return "brew install rkhunter"
	}
	return installCmd("rkhunter", p.PackageManager)
}

func (t *RkhunterTool) IsApplicable(_ *core.PlatformInfo) bool {
	return true
}
