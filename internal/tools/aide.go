package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type AIDETool struct{}

func (t *AIDETool) ID() string                  { return "aide" }
func (t *AIDETool) Name() string                { return "AIDE" }
func (t *AIDETool) Description() string         { return core.T("tool.aide.description") }
func (t *AIDETool) Category() core.ToolCategory { return core.ToolCatFileIntegrity }

func (t *AIDETool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("aide", "")
}

func (t *AIDETool) InstallCommand(p *core.PlatformInfo) string {
	return installCmd("aide", p.PackageManager)
}

func (t *AIDETool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p)
}
