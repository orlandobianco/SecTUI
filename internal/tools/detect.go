package tools

import (
	"os/exec"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

// binaryExists checks if a binary is available in PATH.
func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// serviceActive checks if a systemd service is running.
func serviceActive(service string) bool {
	out, err := exec.Command("systemctl", "is-active", service).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

// serviceActiveLaunchctl checks if a launchd service is loaded on macOS.
func serviceActiveLaunchctl(label string) bool {
	out, err := exec.Command("launchctl", "list").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), label)
}

// detectStatus determines tool status from binary existence and service state.
// For tools without a service (serviceCmd == ""), only binary presence is checked.
func detectStatus(binary, service string) core.ToolStatus {
	if !binaryExists(binary) {
		return core.ToolNotInstalled
	}
	if service == "" {
		return core.ToolInstalled
	}
	if serviceActive(service) {
		return core.ToolActive
	}
	return core.ToolInstalled
}

// installCmd returns the install command for a package given the system's package manager.
func installCmd(pkg string, pm core.PackageManager) string {
	switch pm {
	case core.PkgApt:
		return "sudo apt install -y " + pkg
	case core.PkgDnf:
		return "sudo dnf install -y " + pkg
	case core.PkgPacman:
		return "sudo pacman -S --noconfirm " + pkg
	case core.PkgBrew:
		return "brew install " + pkg
	case core.PkgApk:
		return "sudo apk add " + pkg
	default:
		return ""
	}
}

// isLinux returns true if the platform is Linux.
func isLinux(p *core.PlatformInfo) bool {
	return p.OS == core.OSLinux
}

// isDarwin returns true if the platform is macOS.
func isDarwin(p *core.PlatformInfo) bool {
	return p.OS == core.OSDarwin
}

// isDebianBased returns true for Debian, Ubuntu and derivatives.
func isDebianBased(p *core.PlatformInfo) bool {
	switch p.Distro {
	case "ubuntu", "debian", "pop", "mint", "elementary", "zorin", "neon", "kali":
		return true
	}
	return false
}

// isRHELBased returns true for RHEL, Fedora and derivatives.
func isRHELBased(p *core.PlatformInfo) bool {
	switch p.Distro {
	case "fedora", "rhel", "centos", "rocky", "alma", "oracle":
		return true
	}
	return false
}
