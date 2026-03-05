package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type WireGuardTool struct{}

func (t *WireGuardTool) ID() string                  { return "wireguard" }
func (t *WireGuardTool) Name() string                { return "WireGuard" }
func (t *WireGuardTool) Description() string         { return core.T("tool.wireguard.description") }
func (t *WireGuardTool) Category() core.ToolCategory { return core.ToolCatVPN }

func (t *WireGuardTool) Detect(p *core.PlatformInfo) core.ToolStatus {
	if !binaryExists("wg") {
		return core.ToolNotInstalled
	}
	// WireGuard uses wg-quick@<iface> services; check if any is active.
	if isLinux(p) && serviceActive("wg-quick@wg0") {
		return core.ToolActive
	}
	return core.ToolInstalled
}

func (t *WireGuardTool) InstallCommand(p *core.PlatformInfo) string {
	switch p.PackageManager {
	case core.PkgApt:
		return "sudo apt install -y wireguard"
	case core.PkgDnf:
		return "sudo dnf install -y wireguard-tools"
	case core.PkgPacman:
		return "sudo pacman -S --noconfirm wireguard-tools"
	case core.PkgBrew:
		return "brew install wireguard-tools"
	default:
		return ""
	}
}

func (t *WireGuardTool) IsApplicable(_ *core.PlatformInfo) bool {
	return true
}
