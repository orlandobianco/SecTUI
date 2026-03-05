package tools

import "github.com/orlandobianco/SecTUI/internal/core"

type AppArmorTool struct{}

func (t *AppArmorTool) ID() string          { return "apparmor" }
func (t *AppArmorTool) Name() string        { return "AppArmor" }
func (t *AppArmorTool) Description() string { return core.T("tool.apparmor.description") }
func (t *AppArmorTool) Category() core.ToolCategory { return core.ToolCatAccessControl }

func (t *AppArmorTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	// AppArmor exposes its binary as aa-status or apparmor_status.
	if !binaryExists("aa-status") && !binaryExists("apparmor_status") {
		return core.ToolNotInstalled
	}
	if serviceActive("apparmor") {
		return core.ToolActive
	}
	return core.ToolInstalled
}

func (t *AppArmorTool) InstallCommand(p *core.PlatformInfo) string {
	if p.PackageManager == core.PkgApt {
		return "sudo apt install -y apparmor apparmor-utils"
	}
	return installCmd("apparmor", p.PackageManager)
}

func (t *AppArmorTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p) && isDebianBased(p)
}
