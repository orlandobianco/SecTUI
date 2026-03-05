package tools

import (
	"testing"

	"github.com/orlandobianco/SecTUI/internal/core"
)

func TestAllTools_Returns10(t *testing.T) {
	tools := AllTools()
	if len(tools) != 10 {
		t.Errorf("AllTools() returned %d tools, want 10", len(tools))
	}
}

func TestAllTools_UniqueIDs(t *testing.T) {
	tools := AllTools()
	seen := make(map[string]bool)
	for _, tool := range tools {
		id := tool.ID()
		if seen[id] {
			t.Errorf("duplicate tool ID: %s", id)
		}
		seen[id] = true
	}
}

func TestAllTools_NonEmptyFields(t *testing.T) {
	tools := AllTools()
	for _, tool := range tools {
		if tool.ID() == "" {
			t.Error("tool has empty ID")
		}
		if tool.Name() == "" {
			t.Errorf("tool %s has empty Name", tool.ID())
		}
	}
}

func TestApplicableTools_UbuntuLinux(t *testing.T) {
	platform := &core.PlatformInfo{
		OS:             core.OSLinux,
		Distro:         "ubuntu",
		PackageManager: core.PkgApt,
	}

	tools := ApplicableTools(platform)

	// Ubuntu should see: ufw, fail2ban, crowdsec, clamav, rkhunter, wireguard, tailscale, aide, apparmor
	// Should NOT see: firewalld (RHEL-only)
	ids := toolIDs(tools)

	wantPresent := []string{"ufw", "fail2ban", "crowdsec", "clamav", "rkhunter", "wireguard", "tailscale", "aide", "apparmor"}
	wantAbsent := []string{"firewalld"}

	for _, id := range wantPresent {
		if !ids[id] {
			t.Errorf("ApplicableTools(ubuntu) missing %s", id)
		}
	}
	for _, id := range wantAbsent {
		if ids[id] {
			t.Errorf("ApplicableTools(ubuntu) should not include %s", id)
		}
	}
}

func TestApplicableTools_FedoraLinux(t *testing.T) {
	platform := &core.PlatformInfo{
		OS:             core.OSLinux,
		Distro:         "fedora",
		PackageManager: core.PkgDnf,
	}

	tools := ApplicableTools(platform)
	ids := toolIDs(tools)

	// Fedora should see: firewalld, fail2ban, crowdsec, clamav, rkhunter, wireguard, tailscale, aide
	// Should NOT see: ufw, apparmor (Debian-only)
	wantPresent := []string{"firewalld", "fail2ban", "crowdsec", "clamav", "rkhunter", "wireguard", "tailscale", "aide"}
	wantAbsent := []string{"ufw", "apparmor"}

	for _, id := range wantPresent {
		if !ids[id] {
			t.Errorf("ApplicableTools(fedora) missing %s", id)
		}
	}
	for _, id := range wantAbsent {
		if ids[id] {
			t.Errorf("ApplicableTools(fedora) should not include %s", id)
		}
	}
}

func TestApplicableTools_MacOS(t *testing.T) {
	platform := &core.PlatformInfo{
		OS:             core.OSDarwin,
		PackageManager: core.PkgBrew,
	}

	tools := ApplicableTools(platform)
	ids := toolIDs(tools)

	// macOS should see cross-platform tools only: clamav, rkhunter, wireguard, tailscale
	wantPresent := []string{"clamav", "rkhunter", "wireguard", "tailscale"}
	wantAbsent := []string{"ufw", "firewalld", "fail2ban", "crowdsec", "aide", "apparmor"}

	for _, id := range wantPresent {
		if !ids[id] {
			t.Errorf("ApplicableTools(darwin) missing %s", id)
		}
	}
	for _, id := range wantAbsent {
		if ids[id] {
			t.Errorf("ApplicableTools(darwin) should not include %s", id)
		}
	}
}

func TestDetectAll_ReturnsStatusMap(t *testing.T) {
	platform := &core.PlatformInfo{
		OS:             core.OSLinux,
		Distro:         "ubuntu",
		PackageManager: core.PkgApt,
	}

	statuses := DetectAll(platform)
	if statuses == nil {
		t.Fatal("DetectAll returned nil")
	}

	// Should have an entry for every applicable tool.
	applicable := ApplicableTools(platform)
	for _, tool := range applicable {
		if _, ok := statuses[tool.ID()]; !ok {
			t.Errorf("DetectAll missing status for %s", tool.ID())
		}
	}

	// Each status should be a valid ToolStatus value.
	for id, status := range statuses {
		if status < core.ToolNotInstalled || status > core.ToolNotApplicable {
			t.Errorf("DetectAll[%s] = %d, not a valid ToolStatus", id, status)
		}
	}
}

func TestToolCategories(t *testing.T) {
	tools := AllTools()
	categoryMap := map[string]core.ToolCategory{
		"ufw":       core.ToolCatFirewall,
		"firewalld": core.ToolCatFirewall,
		"fail2ban":  core.ToolCatIntrusionPrevention,
		"crowdsec":  core.ToolCatIntrusionPrevention,
		"clamav":    core.ToolCatMalware,
		"rkhunter":  core.ToolCatMalware,
		"wireguard": core.ToolCatVPN,
		"tailscale": core.ToolCatVPN,
		"aide":      core.ToolCatFileIntegrity,
		"apparmor":  core.ToolCatAccessControl,
	}

	for _, tool := range tools {
		want, ok := categoryMap[tool.ID()]
		if !ok {
			t.Errorf("no expected category for tool %s", tool.ID())
			continue
		}
		if tool.Category() != want {
			t.Errorf("tool %s category = %v, want %v", tool.ID(), tool.Category(), want)
		}
	}
}

func TestInstallCmd(t *testing.T) {
	tests := []struct {
		pkg  string
		pm   core.PackageManager
		want string
	}{
		{"fail2ban", core.PkgApt, "sudo apt install -y fail2ban"},
		{"fail2ban", core.PkgDnf, "sudo dnf install -y fail2ban"},
		{"fail2ban", core.PkgPacman, "sudo pacman -S --noconfirm fail2ban"},
		{"fail2ban", core.PkgBrew, "brew install fail2ban"},
		{"fail2ban", core.PkgApk, "sudo apk add fail2ban"},
		{"fail2ban", core.PkgNone, ""},
	}

	for _, tt := range tests {
		t.Run(tt.pm.String(), func(t *testing.T) {
			got := installCmd(tt.pkg, tt.pm)
			if got != tt.want {
				t.Errorf("installCmd(%q, %s) = %q, want %q", tt.pkg, tt.pm, got, tt.want)
			}
		})
	}
}

func TestIsDebianBased(t *testing.T) {
	yes := []string{"ubuntu", "debian", "pop", "mint", "elementary", "zorin", "neon", "kali"}
	no := []string{"fedora", "rhel", "centos", "arch", "alpine", ""}

	for _, d := range yes {
		p := &core.PlatformInfo{Distro: d}
		if !isDebianBased(p) {
			t.Errorf("isDebianBased(%q) = false, want true", d)
		}
	}
	for _, d := range no {
		p := &core.PlatformInfo{Distro: d}
		if isDebianBased(p) {
			t.Errorf("isDebianBased(%q) = true, want false", d)
		}
	}
}

func TestIsRHELBased(t *testing.T) {
	yes := []string{"fedora", "rhel", "centos", "rocky", "alma", "oracle"}
	no := []string{"ubuntu", "debian", "arch", "alpine", ""}

	for _, d := range yes {
		p := &core.PlatformInfo{Distro: d}
		if !isRHELBased(p) {
			t.Errorf("isRHELBased(%q) = false, want true", d)
		}
	}
	for _, d := range no {
		p := &core.PlatformInfo{Distro: d}
		if isRHELBased(p) {
			t.Errorf("isRHELBased(%q) = true, want false", d)
		}
	}
}

func TestToolInstallCommands_NonEmpty(t *testing.T) {
	platforms := []struct {
		name string
		info *core.PlatformInfo
	}{
		{"ubuntu", &core.PlatformInfo{OS: core.OSLinux, Distro: "ubuntu", PackageManager: core.PkgApt}},
		{"fedora", &core.PlatformInfo{OS: core.OSLinux, Distro: "fedora", PackageManager: core.PkgDnf}},
		{"macos", &core.PlatformInfo{OS: core.OSDarwin, PackageManager: core.PkgBrew}},
	}

	for _, pl := range platforms {
		t.Run(pl.name, func(t *testing.T) {
			tools := ApplicableTools(pl.info)
			for _, tool := range tools {
				cmd := tool.InstallCommand(pl.info)
				if cmd == "" {
					t.Errorf("tool %s has empty InstallCommand on %s", tool.ID(), pl.name)
				}
			}
		})
	}
}

// toolIDs converts a slice of SecurityTool to a set of IDs.
func toolIDs(tools []core.SecurityTool) map[string]bool {
	ids := make(map[string]bool)
	for _, t := range tools {
		ids[t.ID()] = true
	}
	return ids
}
