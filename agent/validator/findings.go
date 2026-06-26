package validator

import (
	"context"
	"fmt"

	"github.com/Y4NN777/7review/agent/review"
)

// Report summarizes deterministic checks over model findings.
type Report struct {
	Accepted []review.Finding
	Rejected []RejectedFinding
}

type RejectedFinding struct {
	Finding review.Finding
	Reason  string
}

// FindingValidator checks model output before report generation or HIL.
type FindingValidator interface {
	Validate(context.Context, *review.Context, []review.Finding) (Report, error)
}

// DefaultFindingValidator enforces severity, confidence, and file references.
type DefaultFindingValidator struct {
	MinConfidence float64
}

func (v DefaultFindingValidator) Validate(_ context.Context, rc *review.Context, findings []review.Finding) (Report, error) {
	minConfidence := v.MinConfidence
	if minConfidence == 0 {
		minConfidence = 0.45
	}

	changed := make(map[string]bool)
	for _, path := range rc.ChangedPaths() {
		changed[path] = true
	}
	for _, path := range rc.Request.ChangedPaths {
		changed[path] = true
	}

	var report Report
	seen := make(map[string]bool)
	for _, finding := range findings {
		if finding.ID != "" && seen[finding.ID] {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "duplicate finding id"})
			continue
		}
		if finding.ID != "" {
			seen[finding.ID] = true
		}
		if !validSeverity(finding.Severity) {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: fmt.Sprintf("invalid severity %q", finding.Severity)})
			continue
		}
		if finding.Confidence < minConfidence {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "confidence below threshold"})
			continue
		}
		if finding.Location.Path != "" && len(changed) > 0 && !changed[finding.Location.Path] {
			report.Rejected = append(report.Rejected, RejectedFinding{Finding: finding, Reason: "location is not in changed paths"})
			continue
		}
		report.Accepted = append(report.Accepted, finding)
	}
	return report, nil
}

func validSeverity(severity review.Severity) bool {
	switch severity {
	case review.SeverityInfo, review.SeverityLow, review.SeverityMedium, review.SeverityHigh, review.SeverityCritical:
		return true
	default:
		return false
	}
}
