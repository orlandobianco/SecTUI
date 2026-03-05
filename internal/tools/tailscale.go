package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type TailscaleTool struct{}

func (t *TailscaleTool) ID() string          { return "tailscale" }
func (t *TailscaleTool) Name() string        { return "Tailscale" }
func (t *TailscaleTool) Description() string { return core.T("tool.tailscale.description") }
func (t *TailscaleTool) Category() core.ToolCategory { return core.ToolCatVPN }

func (t *TailscaleTool) Detect(p *core.PlatformInfo) core.ToolStatus {
	if !binaryExists("tailscale") {
		return core.ToolNotInstalled
	}
	if isLinux(p) && serviceActive("tailscaled") {
		return core.ToolActive
	}
	// On macOS, Tailscale runs as a GUI app; presence of the binary means installed.
	return core.ToolInstalled
}

func (t *TailscaleTool) InstallCommand(_ *core.PlatformInfo) string {
	return "curl -fsSL https://tailscale.com/install.sh | sh"
}

func (t *TailscaleTool) IsApplicable(_ *core.PlatformInfo) bool {
	return true
}
