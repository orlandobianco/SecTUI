package modules

import "github.com/orlandobianco/SecTUI/internal/core"

// AllModules returns all registered security modules.
func AllModules() []core.SecurityModule {
	return []core.SecurityModule{
		NewSSHModule(),
		NewFirewallModule(),
		NewNetworkModule(),
	}
}

// ApplicableModules returns only modules applicable to the current platform.
func ApplicableModules(platform *core.PlatformInfo) []core.SecurityModule {
	var applicable []core.SecurityModule
	for _, m := range AllModules() {
		if m.IsApplicable(platform) {
			applicable = append(applicable, m)
		}
	}
	return applicable
}
