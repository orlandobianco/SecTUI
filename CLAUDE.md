# SecTUI - Project Conventions

## Language & Stack
- Go 1.24+ with Bubble Tea (TUI), Cobra (CLI), Lipgloss (styling)
- License: GPL-3.0

## Project Structure
```
cmd/sectui/main.go         # CLI entry point (Cobra subcommands)
internal/core/              # Types, interfaces, platform detection, config, scoring
internal/modules/           # Security scan modules (SSH, Firewall, Network, etc.)
internal/tools/             # External tool management (fail2ban, ClamAV, etc.)
internal/tui/               # Bubble Tea TUI (app, sidebar, views, theme)
locales/                    # i18n YAML files
docs/                       # Design docs (source of truth for architecture/UI)
```

## Conventions
- Module path: `github.com/orlandobianco/SecTUI`
- Build: `go build ./cmd/sectui`
- Test: `go test ./...`
- Vet: `go vet ./...`
- Version injection: `go build -ldflags "-X main.Version=X.Y.Z" ./cmd/sectui`

## Architecture
- **SecurityModule** interface: scan + harden (internal/core/interfaces.go)
- **SecurityTool** interface: detect + install external tools
- **ToolManager** interface: extends SecurityTool with management UI
- All findings use i18n keys (finding.xxx.title, finding.xxx.detail, finding.xxx.fix)
- Scoring: base 100, Critical -15, High -10, Medium -5, Low -2

## TUI
- Bubble Tea Elm Architecture: Model, Update, View
- Sidebar (22 chars fixed) + Content layout
- Sections: OVERVIEW, MODULES, TOOLS, SECSTORE
- Navigation: Tab focus toggle, j/k movement, Enter/l select, h/Esc back

## Code Style
- No docstring spam. Comments only where logic isn't obvious.
- Handle errors gracefully (skip checks if command fails, don't crash)
- No regex unless necessary - prefer strings package
- Config: TOML at ~/.config/sectui/config.toml (XDG)
- Dry-run by default for all hardening operations
- Backup before any config modifications

## Adding a Security Module
1. Create `internal/modules/<name>.go`
2. Implement `core.SecurityModule` interface
3. Register in `internal/modules/registry.go` AllModules()
4. Add i18n keys in locales/en.yml

## i18n
- All user-facing strings use i18n keys
- Finding keys: finding.<id>.title, finding.<id>.detail, finding.<id>.fix
- Locale priority: config > SECTUI_LOCALE env > LANG env > "en"
