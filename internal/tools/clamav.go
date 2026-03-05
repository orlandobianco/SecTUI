package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type ClamAVTool struct{}

func (t *ClamAVTool) ID() string                  { return "clamav" }
func (t *ClamAVTool) Name() string                { return "ClamAV" }
func (t *ClamAVTool) Description() string         { return core.T("tool.clamav.description") }
func (t *ClamAVTool) Category() core.ToolCategory { return core.ToolCatMalware }

func (t *ClamAVTool) Detect(p *core.PlatformInfo) core.ToolStatus {
	if !binaryExists("clamscan") {
		return core.ToolNotInstalled
	}
	// On macOS there is no clamav-daemon service.
	if isDarwin(p) {
		return core.ToolInstalled
	}
	if serviceActive("clamav-daemon") || serviceActive("clamd") {
		return core.ToolActive
	}
	return core.ToolInstalled
}

func (t *ClamAVTool) InstallCommand(p *core.PlatformInfo) string {
	switch p.PackageManager {
	case core.PkgApt:
		return "sudo apt install -y clamav clamav-daemon"
	case core.PkgDnf:
		return "sudo dnf install -y clamav clamav-update clamd"
	case core.PkgPacman:
		return "sudo pacman -S --noconfirm clamav"
	case core.PkgBrew:
		return "brew install clamav"
	default:
		return ""
	}
}

func (t *ClamAVTool) IsApplicable(_ *core.PlatformInfo) bool {
	return true // works on both Linux and macOS
}
