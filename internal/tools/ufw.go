package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type UFWTool struct{}

func (t *UFWTool) ID() string          { return "ufw" }
func (t *UFWTool) Name() string        { return "UFW" }
func (t *UFWTool) Description() string { return core.T("tool.ufw.description") }
func (t *UFWTool) Category() core.ToolCategory { return core.ToolCatFirewall }

func (t *UFWTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("ufw", "ufw")
}

func (t *UFWTool) InstallCommand(p *core.PlatformInfo) string {
	return installCmd("ufw", p.PackageManager)
}

func (t *UFWTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p) && isDebianBased(p)
}
