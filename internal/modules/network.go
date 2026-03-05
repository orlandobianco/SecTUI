package modules

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const networkModuleID = "network"

type exposedService struct {
	port     int
	name     string
	severity core.Severity
	category string // "database" or "dev"
}

var knownDatabasePorts = []exposedService{
	{port: 5432, name: "PostgreSQL", severity: core.SeverityHigh, category: "database"},
	{port: 3306, name: "MySQL", severity: core.SeverityHigh, category: "database"},
	{port: 27017, name: "MongoDB", severity: core.SeverityHigh, category: "database"},
	{port: 6379, name: "Redis", severity: core.SeverityHigh, category: "database"},
}

var knownDevPorts = []exposedService{
	{port: 3000, name: "Dev server (3000)", severity: core.SeverityMedium, category: "dev"},
	{port: 8080, name: "Dev server (8080)", severity: core.SeverityMedium, category: "dev"},
	{port: 8443, name: "Dev server (8443)", severity: core.SeverityMedium, category: "dev"},
}

type listeningPort struct {
	port    int
	address string
	process string
}

type NetworkModule struct{}

func NewNetworkModule() *NetworkModule {
	return &NetworkModule{}
}

func (m *NetworkModule) ID() string            { return networkModuleID }
func (m *NetworkModule) NameKey() string        { return "module.network.name" }
func (m *NetworkModule) DescriptionKey() string { return "module.network.description" }
func (m *NetworkModule) Priority() int          { return 30 }
func (m *NetworkModule) IsApplicable(_ *core.PlatformInfo) bool { return true }

func (m *NetworkModule) Scan(ctx *core.ScanContext) []core.Finding {
	var ports []listeningPort
	var err error

	if ctx.Platform.OS == core.OSDarwin {
		ports, err = getListeningPortsDarwin()
	} else {
		ports, err = getListeningPortsLinux()
	}

	if err != nil {
		return nil
	}

	var findings []core.Finding
	findingNum := 1

	for _, lp := range ports {
		if !isWildcardAddress(lp.address) {
			continue
		}

		if svc := matchService(lp.port, knownDatabasePorts); svc != nil {
			findings = append(findings, core.Finding{
				ID:            fmt.Sprintf("net-%03d", findingNum),
				Module:        networkModuleID,
				Severity:      svc.severity,
				TitleKey:      "finding.net_database_exposed.title",
				DetailKey:     "finding.net_database_exposed.detail",
				CurrentValue:  fmt.Sprintf("%s on %s:%d (pid: %s)", svc.name, lp.address, lp.port, lp.process),
				ExpectedValue: fmt.Sprintf("%s bound to 127.0.0.1 only", svc.name),
			})
			findingNum++
		}

		if svc := matchService(lp.port, knownDevPorts); svc != nil {
			findings = append(findings, core.Finding{
				ID:            fmt.Sprintf("net-%03d", findingNum),
				Module:        networkModuleID,
				Severity:      svc.severity,
				TitleKey:      "finding.net_dev_port_exposed.title",
				DetailKey:     "finding.net_dev_port_exposed.detail",
				CurrentValue:  fmt.Sprintf("%s on %s:%d (pid: %s)", svc.name, lp.address, lp.port, lp.process),
				ExpectedValue: "Not exposed on 0.0.0.0",
			})
			findingNum++
		}
	}

	return findings
}

func (m *NetworkModule) AvailableFixes() []core.Fix {
	return nil
}

func (m *NetworkModule) PreviewFix(fixID string, _ *core.ScanContext) (string, error) {
	return "", fmt.Errorf("no fixes available for network module: %s", fixID)
}

func (m *NetworkModule) ApplyFix(fixID string, _ *core.ApplyContext) (*core.ApplyResult, error) {
	return nil, fmt.Errorf("no fixes available for network module: %s", fixID)
}

// --- port scanning ---

func getListeningPortsLinux() ([]listeningPort, error) {
	out, err := exec.Command("ss", "-tlnp").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseSSOutput(string(out)), nil
}

func getListeningPortsDarwin() ([]listeningPort, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseLsofOutput(string(out)), nil
}

// parseSSOutput parses `ss -tlnp` output.
// Example line:
//
//	LISTEN  0  128  0.0.0.0:5432  0.0.0.0:*  users:(("postgres",pid=1234,fd=5))
func parseSSOutput(output string) []listeningPort {
	var ports []listeningPort
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "LISTEN") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		local := fields[3]
		addr, port := splitAddressPort(local)
		if port == 0 {
			continue
		}

		process := ""
		if len(fields) >= 6 {
			process = extractProcessFromSS(fields[5])
		}

		ports = append(ports, listeningPort{
			port:    port,
			address: addr,
			process: process,
		})
	}

	return ports
}

// parseLsofOutput parses `lsof -iTCP -sTCP:LISTEN -P -n` output.
// Example line:
//
//	postgres  1234  user  5u  IPv4  0x1234  0t0  TCP  *:5432 (LISTEN)
func parseLsofOutput(output string) []listeningPort {
	var ports []listeningPort
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		name := fields[8]
		addr, port := splitLsofAddress(name)
		if port == 0 {
			continue
		}

		ports = append(ports, listeningPort{
			port:    port,
			address: addr,
			process: fields[0],
		})
	}

	return ports
}

// splitAddressPort splits "0.0.0.0:5432" or "[::]:5432" into address and port.
func splitAddressPort(s string) (string, int) {
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s, 0
	}

	addr := s[:idx]
	port, err := strconv.Atoi(s[idx+1:])
	if err != nil {
		return addr, 0
	}

	return addr, port
}

// splitLsofAddress handles lsof format like "*:5432" or "127.0.0.1:5432".
func splitLsofAddress(s string) (string, int) {
	// Remove "(LISTEN)" suffix if attached
	s = strings.TrimSuffix(s, "(LISTEN)")
	s = strings.TrimSpace(s)

	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s, 0
	}

	addr := s[:idx]
	port, err := strconv.Atoi(s[idx+1:])
	if err != nil {
		return addr, 0
	}

	// lsof uses "*" to mean 0.0.0.0
	if addr == "*" {
		addr = "0.0.0.0"
	}

	return addr, port
}

func extractProcessFromSS(field string) string {
	// field looks like: users:(("postgres",pid=1234,fd=5))
	start := strings.Index(field, "((\"")
	if start == -1 {
		return ""
	}
	end := strings.Index(field[start+3:], "\"")
	if end == -1 {
		return ""
	}
	return field[start+3 : start+3+end]
}

func isWildcardAddress(addr string) bool {
	return addr == "0.0.0.0" || addr == "*" || addr == "[::]" || addr == "::"
}

func matchService(port int, services []exposedService) *exposedService {
	for i := range services {
		if services[i].port == port {
			return &services[i]
		}
	}
	return nil
}
