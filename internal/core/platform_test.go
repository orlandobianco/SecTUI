package core

import (
	"runtime"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	content := `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
VERSION_ID="22.04"
PRETTY_NAME="Ubuntu 22.04.3 LTS"
`
	fields := parseOSRelease(content)

	tests := []struct {
		key, want string
	}{
		{"NAME", "Ubuntu"},
		{"ID", "ubuntu"},
		{"VERSION_ID", "22.04"},
	}

	for _, tt := range tests {
		got := fields[tt.key]
		if got != tt.want {
			t.Errorf("parseOSRelease[%s] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestParseOSRelease_EmptyLines(t *testing.T) {
	content := `
ID=alpine

VERSION_ID="3.19"
`
	fields := parseOSRelease(content)
	if fields["ID"] != "alpine" {
		t.Errorf("ID = %q, want alpine", fields["ID"])
	}
	if fields["VERSION_ID"] != "3.19" {
		t.Errorf("VERSION_ID = %q, want 3.19", fields["VERSION_ID"])
	}
}

func TestNormalizeDistro(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ubuntu", "ubuntu"},
		{"pop", "ubuntu"},
		{"neon", "ubuntu"},
		{"elementary", "ubuntu"},
		{"linuxmint", "ubuntu"},
		{"zorin", "ubuntu"},
		{"manjaro", "arch"},
		{"endeavouros", "arch"},
		{"garuda", "arch"},
		{"rocky", "rhel"},
		{"alma", "rhel"},
		{"almalinux", "rhel"},
		{"centos", "rhel"},
		{"rhel", "rhel"},
		{"ol", "rhel"},
		{"fedora", "fedora"},   // not mapped → returns itself
		{"debian", "debian"},   // not mapped → returns itself
		{"alpine", "alpine"},   // not mapped → returns itself
		{"  Pop  ", "ubuntu"},  // trimmed + lowered
		{"MANJARO", "arch"},    // uppercased
		{"unknown", "unknown"}, // pass-through
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDistro(tt.input)
			if got != tt.want {
				t.Errorf("normalizeDistro(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectOS(t *testing.T) {
	os := detectOS()
	switch runtime.GOOS {
	case "darwin":
		if os != OSDarwin {
			t.Errorf("detectOS() = %v, want OSDarwin on darwin", os)
		}
	case "linux":
		if os != OSLinux {
			t.Errorf("detectOS() = %v, want OSLinux on linux", os)
		}
	}
}

func TestDetectArch(t *testing.T) {
	arch := detectArch()
	switch runtime.GOARCH {
	case "amd64":
		if arch != "x86_64" {
			t.Errorf("detectArch() = %q, want x86_64 on amd64", arch)
		}
	case "arm64":
		if arch != "aarch64" {
			t.Errorf("detectArch() = %q, want aarch64 on arm64", arch)
		}
	}
}

func TestDetectPlatform_NotNil(t *testing.T) {
	p := DetectPlatform()
	if p == nil {
		t.Fatal("DetectPlatform() returned nil")
	}
	if p.Arch == "" {
		t.Error("Arch is empty")
	}
	if p.OS.String() == "unknown" {
		t.Error("OS is unknown")
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityInfo, "info"},
		{SeverityLow, "low"},
		{SeverityMedium, "medium"},
		{SeverityHigh, "high"},
		{SeverityCritical, "critical"},
		{Severity(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestOSString(t *testing.T) {
	if OSLinux.String() != "linux" {
		t.Errorf("OSLinux.String() = %q", OSLinux.String())
	}
	if OSDarwin.String() != "darwin" {
		t.Errorf("OSDarwin.String() = %q", OSDarwin.String())
	}
}

func TestInitSystemString(t *testing.T) {
	tests := []struct {
		i    InitSystem
		want string
	}{
		{InitSystemd, "systemd"},
		{InitLaunchd, "launchd"},
		{InitOpenRC, "openrc"},
		{InitOther, "other"},
	}
	for _, tt := range tests {
		if got := tt.i.String(); got != tt.want {
			t.Errorf("InitSystem(%d).String() = %q, want %q", tt.i, got, tt.want)
		}
	}
}

func TestPackageManagerString(t *testing.T) {
	tests := []struct {
		p    PackageManager
		want string
	}{
		{PkgApt, "apt"},
		{PkgDnf, "dnf"},
		{PkgPacman, "pacman"},
		{PkgBrew, "brew"},
		{PkgApk, "apk"},
		{PkgNone, "none"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("PackageManager(%d).String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}
