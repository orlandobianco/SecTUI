package core

import (
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ThreatEvent struct {
	Timestamp time.Time
	Tool      string // "fail2ban", "crowdsec"
	Action    string // "BAN", "BLOCK", "ALERT"
	IP        string
	Geo       GeoIPResult
	Detail    string // jail name, scenario
}

type ThreatFeed struct {
	events      []ThreatEvent
	totalBans   int
	mu          sync.Mutex
	lastRefresh time.Time
}

func NewThreatFeed() *ThreatFeed {
	return &ThreatFeed{}
}

func (tf *ThreatFeed) Events() []ThreatEvent {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	out := make([]ThreatEvent, len(tf.events))
	copy(out, tf.events)
	return out
}

func (tf *ThreatFeed) TotalBlocked() int {
	tf.mu.Lock()
	defer tf.mu.Unlock()
	return tf.totalBans
}

// Refresh collects threat events from fail2ban and crowdsec.
// Should be called periodically (~5s). Skips if called too soon.
func (tf *ThreatFeed) Refresh() {
	tf.mu.Lock()
	if time.Since(tf.lastRefresh) < 4*time.Second {
		tf.mu.Unlock()
		return
	}
	tf.lastRefresh = time.Now()
	tf.mu.Unlock()

	var events []ThreatEvent
	var total int

	f2bEvents, f2bTotal := collectFail2ban()
	events = append(events, f2bEvents...)
	total += f2bTotal

	csEvents, csTotal := collectCrowdSec()
	events = append(events, csEvents...)
	total += csTotal

	// Sort by timestamp descending, keep most recent 20
	sortEventsByTime(events)
	if len(events) > 20 {
		events = events[:20]
	}

	// Async GeoIP resolve for IPs without cached geo
	for i := range events {
		if events[i].IP != "" {
			if geo, ok := LookupGeoIP(events[i].IP); ok {
				events[i].Geo = geo
			} else {
				go ResolveGeoIP(events[i].IP)
			}
		}
	}

	tf.mu.Lock()
	tf.events = events
	tf.totalBans = total
	tf.mu.Unlock()
}

var ipRegex = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

func collectFail2ban() ([]ThreatEvent, int) {
	// Check if fail2ban-client exists
	if _, err := exec.LookPath("fail2ban-client"); err != nil {
		return nil, 0
	}

	var events []ThreatEvent
	total := 0

	// Get banned IPs count
	out, err := exec.Command("fail2ban-client", "banned").Output()
	if err == nil {
		ips := ipRegex.FindAllString(string(out), -1)
		total = len(ips)
	}

	// Get recent bans from journal
	jOut, err := exec.Command("journalctl", "-u", "fail2ban", "-n", "30",
		"--no-pager", "--output=short").Output()
	if err != nil {
		return events, total
	}

	banRegex := regexp.MustCompile(`(?i)ban\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	for _, line := range strings.Split(string(jOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := banRegex.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		ip := matches[1]
		if !isValidIP(ip) {
			continue
		}

		// Extract jail name if present
		detail := "sshd"
		if strings.Contains(strings.ToLower(line), "recidive") {
			detail = "recidive"
		}

		ts := parseJournalTimestamp(line)
		events = append(events, ThreatEvent{
			Timestamp: ts,
			Tool:      "fail2ban",
			Action:    "BAN",
			IP:        ip,
			Detail:    detail,
		})
	}

	return events, total
}

func collectCrowdSec() ([]ThreatEvent, int) {
	if _, err := exec.LookPath("cscli"); err != nil {
		return nil, 0
	}

	var events []ThreatEvent
	total := 0

	// Get decisions
	out, err := exec.Command("cscli", "decisions", "list", "--no-color", "-o", "raw").Output()
	if err != nil {
		return events, total
	}

	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		total++

		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}

		ip := extractIP(fields[2]) // source field
		if ip == "" {
			continue
		}

		action := "BLOCK"
		scenario := ""
		if len(fields) > 4 {
			scenario = fields[4]
			// Shorten scenario
			if idx := strings.LastIndex(scenario, "/"); idx >= 0 {
				scenario = scenario[idx+1:]
			}
		}

		events = append(events, ThreatEvent{
			Timestamp: time.Now(), // raw output doesn't have clean timestamps
			Tool:      "crowdsec",
			Action:    action,
			IP:        ip,
			Detail:    scenario,
		})
	}

	return events, total
}

func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}

func extractIP(s string) string {
	s = strings.TrimSpace(s)
	// Remove CIDR notation
	if idx := strings.Index(s, "/"); idx > 0 {
		s = s[:idx]
	}
	if isValidIP(s) {
		return s
	}
	// Try regex
	matches := ipRegex.FindString(s)
	return matches
}

func parseJournalTimestamp(line string) time.Time {
	// systemd journal short format: "Mar 05 14:32:15 ..."
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return time.Now()
	}
	ts := strings.Join(parts[:3], " ")
	now := time.Now()
	layouts := []string{
		"Jan 02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
		}
	}
	return time.Now()
}

func sortEventsByTime(events []ThreatEvent) {
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && events[j].Timestamp.After(events[j-1].Timestamp); j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
}
