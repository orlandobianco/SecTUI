package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type CrowdSecTool struct{}

func (t *CrowdSecTool) ID() string          { return "crowdsec" }
func (t *CrowdSecTool) Name() string        { return "CrowdSec" }
func (t *CrowdSecTool) Description() string { return core.T("tool.crowdsec.description") }
func (t *CrowdSecTool) Category() core.ToolCategory { return core.ToolCatIntrusionPrevention }

func (t *CrowdSecTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("cscli", "crowdsec")
}

func (t *CrowdSecTool) InstallCommand(_ *core.PlatformInfo) string {
	return "curl -s https://install.crowdsec.net | sudo bash"
}

func (t *CrowdSecTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p)
}
