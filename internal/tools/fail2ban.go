package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type Fail2banTool struct{}

func (t *Fail2banTool) ID() string                  { return "fail2ban" }
func (t *Fail2banTool) Name() string                { return "fail2ban" }
func (t *Fail2banTool) Description() string         { return core.T("tool.fail2ban.description") }
func (t *Fail2banTool) Category() core.ToolCategory { return core.ToolCatIntrusionPrevention }

func (t *Fail2banTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("fail2ban-client", "fail2ban")
}

func (t *Fail2banTool) InstallCommand(p *core.PlatformInfo) string {
	return installCmd("fail2ban", p.PackageManager)
}

func (t *Fail2banTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p)
}
