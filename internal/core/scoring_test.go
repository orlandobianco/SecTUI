package core

import "testing"

func TestCalculateScore_NoFindings(t *testing.T) {
	score := CalculateScore(nil)
	if score != 100 {
		t.Errorf("CalculateScore(nil) = %d, want 100", score)
	}
}

func TestCalculateScore_SingleFinding(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     int
	}{
		{"critical", SeverityCritical, 85},
		{"high", SeverityHigh, 90},
		{"medium", SeverityMedium, 95},
		{"low", SeverityLow, 98},
		{"info", SeverityInfo, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := []Finding{{Severity: tt.severity}}
			got := CalculateScore(findings)
			if got != tt.want {
				t.Errorf("CalculateScore(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestCalculateScore_MultipleFindings(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityCritical}, // -15
		{Severity: SeverityHigh},     // -10
		{Severity: SeverityMedium},   // -5
		{Severity: SeverityLow},      // -2
		{Severity: SeverityInfo},     // -0
	}
	// 100 - 15 - 10 - 5 - 2 = 68
	got := CalculateScore(findings)
	if got != 68 {
		t.Errorf("CalculateScore(mixed) = %d, want 68", got)
	}
}

func TestCalculateScore_FloorAtZero(t *testing.T) {
	// 7 critical findings = 7 * 15 = 105 penalty → should floor at 0
	findings := make([]Finding, 7)
	for i := range findings {
		findings[i].Severity = SeverityCritical
	}
	got := CalculateScore(findings)
	if got != 0 {
		t.Errorf("CalculateScore(7 critical) = %d, want 0", got)
	}
}

func TestCalculateModuleScore_FiltersModule(t *testing.T) {
	findings := []Finding{
		{Module: "ssh", Severity: SeverityCritical},    // -15
		{Module: "ssh", Severity: SeverityMedium},      // -5
		{Module: "kernel", Severity: SeverityCritical}, // ignored for ssh
	}
	got := CalculateModuleScore(findings, "ssh")
	if got != 80 {
		t.Errorf("CalculateModuleScore(ssh) = %d, want 80", got)
	}
}

func TestCalculateModuleScore_NoMatchingModule(t *testing.T) {
	findings := []Finding{
		{Module: "kernel", Severity: SeverityCritical},
	}
	got := CalculateModuleScore(findings, "ssh")
	if got != 100 {
		t.Errorf("CalculateModuleScore(no match) = %d, want 100", got)
	}
}
