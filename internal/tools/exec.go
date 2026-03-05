package tools

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

// runCmd executes a command and returns trimmed stdout.
func runCmd(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// runCmdSudo executes a command via sudo.
func runCmdSudo(name string, args ...string) (string, error) {
	full := append([]string{name}, args...)
	return runCmd("sudo", full...)
}

// RunCmdSudoToFile executes a sudo command with stdout/stderr directed to a file.
// Returns the process exit code and any error.
func RunCmdSudoToFile(logFile *os.File, name string, args ...string) (int, error) {
	full := append([]string{name}, args...)
	cmd := exec.Command("sudo", full...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err := cmd.Run()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return exitCode, err
}

// BuildSudoCmd creates an exec.Cmd for sudo execution without starting it.
func BuildSudoCmd(name string, args ...string) *exec.Cmd {
	full := append([]string{name}, args...)
	return exec.Command("sudo", full...)
}

// journalLines reads the last n lines from a systemd journal unit.
func journalLines(unit string, n int) []core.ActivityEntry {
	out, err := runCmd("journalctl", "-u", unit, "-n", strconv.Itoa(n), "--no-pager", "--output=short")
	if err != nil {
		return nil
	}
	return parseJournalOutput(out)
}

// journalLinesGrep reads journal entries filtered by a grep pattern (e.g. kernel messages).
func journalLinesGrep(grepArg string, n int, extraArgs ...string) []core.ActivityEntry {
	args := append([]string{"--grep=" + grepArg, "-n", strconv.Itoa(n), "--no-pager", "--output=short"}, extraArgs...)
	out, err := runCmd("journalctl", args...)
	if err != nil {
		return nil
	}
	return parseJournalOutput(out)
}

func parseJournalOutput(out string) []core.ActivityEntry {
	var entries []core.ActivityEntry
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		// journalctl short format: "Mon DD HH:MM:SS hostname unit[pid]: message"
		// We extract timestamp (first 15 chars) and message (after first ]: or : )
		ts, msg := splitJournalLine(line)
		entries = append(entries, core.ActivityEntry{Timestamp: ts, Message: msg})
	}
	return entries
}

func splitJournalLine(line string) (string, string) {
	// Try to extract "Mon DD HH:MM:SS" (15 chars).
	if len(line) < 15 {
		return "", line
	}
	ts := line[:15]
	rest := line[15:]

	// Find the message after the first ": " past the hostname/unit.
	if idx := strings.Index(rest, ": "); idx != -1 {
		return strings.TrimSpace(ts), strings.TrimSpace(rest[idx+2:])
	}
	return strings.TrimSpace(ts), strings.TrimSpace(rest)
}

// parseKeyValue parses lines with "key <sep> value" format.
func parseKeyValue(text, sep string) []core.ConfigEntry {
	var entries []core.ConfigEntry
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, sep, 2)
		if len(parts) == 2 {
			entries = append(entries, core.ConfigEntry{
				Key:   strings.TrimSpace(parts[0]),
				Value: strings.TrimSpace(parts[1]),
			})
		}
	}
	return entries
}

// serviceStatus queries systemctl for detailed service info.
func serviceStatusInfo(service string) core.ServiceStatus {
	props := []string{"ActiveState", "SubState", "MainPID", "ActiveEnterTimestamp", "UnitFileState"}
	out, err := runCmd("systemctl", "show", service, "--property="+strings.Join(props, ","), "--no-pager")
	if err != nil {
		return core.ServiceStatus{}
	}

	vals := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 {
			vals[parts[0]] = parts[1]
		}
	}

	pid, _ := strconv.Atoi(vals["MainPID"])
	running := vals["ActiveState"] == "active"
	enabled := vals["UnitFileState"] == "enabled"
	uptime := ""
	if ts := vals["ActiveEnterTimestamp"]; ts != "" && running {
		uptime = ts
	}

	return core.ServiceStatus{
		Running: running,
		Enabled: enabled,
		PID:     pid,
		Uptime:  uptime,
		Extra:   vals,
	}
}

// countFiles counts non-hidden files in a directory path.
func countFiles(dir string) int {
	out, err := runCmd("ls", "-1", dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// readFile reads a file via cat and returns its content.
func readFile(path string) (string, error) {
	return runCmd("cat", path)
}

// actionOK is a shorthand for a successful ActionResult.
func actionOK(msg string) core.ActionResult {
	return core.ActionResult{Success: true, Message: msg}
}

// actionErr is a shorthand for a failed ActionResult.
func actionErr(format string, args ...interface{}) core.ActionResult {
	return core.ActionResult{Success: false, Message: fmt.Sprintf(format, args...)}
}
