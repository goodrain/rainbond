package compose

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SupportLevel represents the level of support for a field
type SupportLevel string

const (
	// SupportLevelSupported indicates the field is fully supported
	SupportLevelSupported SupportLevel = "supported"
	// SupportLevelDegraded indicates the field is supported with limitations
	SupportLevelDegraded SupportLevel = "degraded"
	// SupportLevelUnsupported indicates the field is not supported
	SupportLevelUnsupported SupportLevel = "unsupported"
	// SupportLevelInfo indicates informational message
	SupportLevelInfo SupportLevel = "info"
)

// FieldIssue represents a single field support issue
type FieldIssue struct {
	Service    string       `json:"service"`
	Field      string       `json:"field"`
	Level      SupportLevel `json:"level"`
	Message    string       `json:"message"`
	Suggestion string       `json:"suggestion,omitempty"`
}

// FieldSupportReport tracks field support issues during parsing
type FieldSupportReport struct {
	Issues []FieldIssue `json:"issues"`
}

// NewFieldSupportReport creates a new field support report
func NewFieldSupportReport() *FieldSupportReport {
	return &FieldSupportReport{
		Issues: make([]FieldIssue, 0),
	}
}

// AddSupported records a supported field (for informational purposes)
func (r *FieldSupportReport) AddSupported(service, field, message string) {
	r.Issues = append(r.Issues, FieldIssue{
		Service: service,
		Field:   field,
		Level:   SupportLevelSupported,
		Message: message,
	})
}

// AddDegraded records a field that is supported with limitations
func (r *FieldSupportReport) AddDegraded(service, field, message, suggestion string) {
	r.Issues = append(r.Issues, FieldIssue{
		Service:    service,
		Field:      field,
		Level:      SupportLevelDegraded,
		Message:    message,
		Suggestion: suggestion,
	})
}

// AddUnsupported records a field that is not supported
func (r *FieldSupportReport) AddUnsupported(service, field, message, suggestion string) {
	r.Issues = append(r.Issues, FieldIssue{
		Service:    service,
		Field:      field,
		Level:      SupportLevelUnsupported,
		Message:    message,
		Suggestion: suggestion,
	})
}

// AddInfo records an informational message
func (r *FieldSupportReport) AddInfo(service, field, message, suggestion string) {
	r.Issues = append(r.Issues, FieldIssue{
		Service:    service,
		Field:      field,
		Level:      SupportLevelInfo,
		Message:    message,
		Suggestion: suggestion,
	})
}

// HasIssues returns true if there are any issues in the report
func (r *FieldSupportReport) HasIssues() bool {
	return len(r.Issues) > 0
}

// HasErrors returns true if there are any unsupported fields
func (r *FieldSupportReport) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Level == SupportLevelUnsupported {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any degraded fields
func (r *FieldSupportReport) HasWarnings() bool {
	for _, issue := range r.Issues {
		if issue.Level == SupportLevelDegraded {
			return true
		}
	}
	return false
}

// GetSummary generates a human-readable summary of the report
func (r *FieldSupportReport) GetSummary() string {
	if !r.HasIssues() {
		return "All fields are supported"
	}

	var lines []string

	// Group issues by level
	unsupported := []FieldIssue{}
	degraded := []FieldIssue{}
	info := []FieldIssue{}

	for _, issue := range r.Issues {
		switch issue.Level {
		case SupportLevelUnsupported:
			unsupported = append(unsupported, issue)
		case SupportLevelDegraded:
			degraded = append(degraded, issue)
		case SupportLevelInfo:
			info = append(info, issue)
		}
	}

	// Add unsupported fields
	if len(unsupported) > 0 {
		lines = append(lines, fmt.Sprintf("Unsupported fields (%d):", len(unsupported)))
		for _, issue := range unsupported {
			line := fmt.Sprintf("  - Service '%s', Field '%s': %s", issue.Service, issue.Field, issue.Message)
			if issue.Suggestion != "" {
				line += fmt.Sprintf(" (Suggestion: %s)", issue.Suggestion)
			}
			lines = append(lines, line)
		}
	}

	// Add degraded fields
	if len(degraded) > 0 {
		lines = append(lines, fmt.Sprintf("Fields with limitations (%d):", len(degraded)))
		for _, issue := range degraded {
			line := fmt.Sprintf("  - Service '%s', Field '%s': %s", issue.Service, issue.Field, issue.Message)
			if issue.Suggestion != "" {
				line += fmt.Sprintf(" (Suggestion: %s)", issue.Suggestion)
			}
			lines = append(lines, line)
		}
	}

	// Add info messages
	if len(info) > 0 {
		lines = append(lines, fmt.Sprintf("Information (%d):", len(info)))
		for _, issue := range info {
			line := fmt.Sprintf("  - Service '%s', Field '%s': %s", issue.Service, issue.Field, issue.Message)
			if issue.Suggestion != "" {
				line += fmt.Sprintf(" (%s)", issue.Suggestion)
			}
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// ToJSON exports the report as JSON
func (r *FieldSupportReport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetIssuesByLevel returns all issues of a specific level
func (r *FieldSupportReport) GetIssuesByLevel(level SupportLevel) []FieldIssue {
	result := []FieldIssue{}
	for _, issue := range r.Issues {
		if issue.Level == level {
			result = append(result, issue)
		}
	}
	return result
}

// GetIssuesByService returns all issues for a specific service
func (r *FieldSupportReport) GetIssuesByService(service string) []FieldIssue {
	result := []FieldIssue{}
	for _, issue := range r.Issues {
		if issue.Service == service {
			result = append(result, issue)
		}
	}
	return result
}

// SortByLevel sorts issues by level (unsupported first, then degraded, then info)
func (r *FieldSupportReport) SortByLevel() {
	levelOrder := map[SupportLevel]int{
		SupportLevelUnsupported: 0,
		SupportLevelDegraded:    1,
		SupportLevelInfo:        2,
		SupportLevelSupported:   3,
	}

	sort.Slice(r.Issues, func(i, j int) bool {
		return levelOrder[r.Issues[i].Level] < levelOrder[r.Issues[j].Level]
	})
}
