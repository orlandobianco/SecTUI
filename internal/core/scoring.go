package core

func severityPenalty(s Severity) int {
	switch s {
	case SeverityCritical:
		return 15
	case SeverityHigh:
		return 10
	case SeverityMedium:
		return 5
	case SeverityLow:
		return 2
	default:
		return 0
	}
}

// CalculateScore computes an overall hardening score (0-100) from all findings.
func CalculateScore(findings []Finding) int {
	score := 100
	for _, f := range findings {
		score -= severityPenalty(f.Severity)
	}
	if score < 0 {
		return 0
	}
	return score
}

// CalculateModuleScore computes a score considering only findings from a specific module.
func CalculateModuleScore(findings []Finding, moduleID string) int {
	score := 100
	for _, f := range findings {
		if f.Module == moduleID {
			score -= severityPenalty(f.Severity)
		}
	}
	if score < 0 {
		return 0
	}
	return score
}
