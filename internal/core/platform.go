package core

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func DetectPlatform() *PlatformInfo {
	info := &PlatformInfo{
		OS:   detectOS(),
		Arch: detectArch(),
	}

	info.Distro, info.Version = detectDistro(info.OS)
	info.InitSystem = detectInitSystem(info.OS)
	info.PackageManager = detectPackageManager(info.OS, info.Distro)
	info.IsContainer = detectContainer()
	info.IsWSL = detectWSL()

	return info
}

func detectOS() OS {
	switch runtime.GOOS {
	case "linux":
		return OSLinux
	case "darwin":
		return OSDarwin
	default:
		return OSLinux
	}
}

func detectArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func detectDistro(osType OS) (distro, version string) {
	if osType == OSDarwin {
		return "macos", detectMacOSVersion()
	}

	return detectLinuxDistro()
}

func detectMacOSVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func detectLinuxDistro() (distro, version string) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown", "unknown"
	}

	fields := parseOSRelease(string(data))
	distro = normalizeDistro(fields["ID"])
	version = fields["VERSION_ID"]

	if distro == "" {
		distro = "unknown"
	}
	if version == "" {
		version = "unknown"
	}

	return distro, version
}

func parseOSRelease(content string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[key] = strings.Trim(val, `"`)
	}
	return fields
}

// normalizeDistro maps derivative distributions to their parent.
func normalizeDistro(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))

	derivatives := map[string]string{
		"pop":         "ubuntu",
		"neon":        "ubuntu",
		"elementary":  "ubuntu",
		"zorin":       "ubuntu",
		"linuxmint":   "ubuntu",
		"manjaro":     "arch",
		"endeavouros": "arch",
		"garuda":      "arch",
		"rocky":       "rhel",
		"alma":        "rhel",
		"almalinux":   "rhel",
		"centos":      "rhel",
		"rhel":        "rhel",
		"ol":          "rhel",
	}

	if parent, ok := derivatives[id]; ok {
		return parent
	}
	return id
}

func detectInitSystem(osType OS) InitSystem {
	if osType == OSDarwin {
		return InitLaunchd
	}

	if stat, err := os.Stat("/run/systemd/system"); err == nil && stat.IsDir() {
		return InitSystemd
	}

	if _, err := os.Stat("/sbin/openrc"); err == nil {
		return InitOpenRC
	}

	return InitOther
}

func detectPackageManager(osType OS, distro string) PackageManager {
	if osType == OSDarwin {
		if _, err := exec.LookPath("brew"); err == nil {
			return PkgBrew
		}
		return PkgNone
	}

	// Try distro-based detection first
	switch distro {
	case "ubuntu", "debian":
		return PkgApt
	case "rhel", "fedora":
		return PkgDnf
	case "arch":
		return PkgPacman
	case "alpine":
		return PkgApk
	}

	// Fallback: probe for known binaries
	managers := []struct {
		bin string
		pkg PackageManager
	}{
		{"apt-get", PkgApt},
		{"dnf", PkgDnf},
		{"pacman", PkgPacman},
		{"apk", PkgApk},
	}
	for _, m := range managers {
		if _, err := exec.LookPath(m.bin); err == nil {
			return m.pkg
		}
	}

	return PkgNone
}

func detectContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if env := os.Getenv("container"); env != "" {
		return true
	}

	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}

	content := strings.ToLower(string(data))
	return strings.Contains(content, "docker") ||
		strings.Contains(content, "lxc") ||
		strings.Contains(content, "kubepods")
}

func detectWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft")
}
