package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type FirewalldTool struct{}

func (t *FirewalldTool) ID() string                  { return "firewalld" }
func (t *FirewalldTool) Name() string                { return "firewalld" }
func (t *FirewalldTool) Description() string         { return core.T("tool.firewalld.description") }
func (t *FirewalldTool) Category() core.ToolCategory { return core.ToolCatFirewall }

func (t *FirewalldTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("firewall-cmd", "firewalld")
}

func (t *FirewalldTool) InstallCommand(p *core.PlatformInfo) string {
	return installCmd("firewalld", p.PackageManager)
}

func (t *FirewalldTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p) && isRHELBased(p)
}
