package benchmark

import (
	"fmt"
	"sort"
	"strings"
)

// VerificationStatus determines whether an implementation/case operation is
// eligible for timing.
type VerificationStatus string

const (
	StatusEligible    VerificationStatus = "eligible"
	StatusUnsupported VerificationStatus = "unsupported"
)

// VerificationRecord is the auditable result of one pre-timing correctness
// check.
type VerificationRecord struct {
	Implementation string             `json:"implementation"`
	CaseID         string             `json:"case_id"`
	Mode           Mode               `json:"mode"`
	Status         VerificationStatus `json:"status"`
	Class          string             `json:"class,omitempty"`
	Reason         string             `json:"reason,omitempty"`
}

// VerificationMatrix contains every requested operation for every pinned
// implementation. Unsupported operations remain visible but cannot be timed.
type VerificationMatrix struct {
	Records []VerificationRecord `json:"records"`
	index   map[string]VerificationRecord
}

// VerifyMatrix runs the semantic and concurrency preflight for the complete
// corpus.
func VerifyMatrix(corpus Corpus, runtimes []Runtime) VerificationMatrix {
	records := make([]VerificationRecord, 0, len(corpus.Cases)*len(runtimes)*4)
	for _, runtime := range runtimes {
		for _, sample := range corpus.Cases {
			for _, mode := range sample.Modes {
				record := VerificationRecord{Implementation: runtime.ID, CaseID: sample.ID, Mode: mode}
				if reason := runtime.UnsupportedReason(sample, mode); reason != "" {
					record.Status = StatusUnsupported
					record.Class = "api-or-safety-limit"
					record.Reason = reason
				} else if _, err := VerifyCase(runtime, sample, mode); err != nil {
					record.Status = StatusUnsupported
					record.Class = classifyVerificationFailure(err)
					record.Reason = err.Error()
				} else {
					record.Status = StatusEligible
				}
				records = append(records, record)
			}
		}
	}
	matrix := VerificationMatrix{Records: records}
	matrix.buildIndex()
	return matrix
}

func classifyVerificationFailure(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "compile failed"):
		return "compile-error"
	case strings.Contains(message, "structured error mismatch"):
		return "error-mismatch"
	case strings.Contains(message, "value mismatch"):
		return "value-mismatch"
	default:
		return "evaluation-error"
	}
}

func (m *VerificationMatrix) buildIndex() {
	m.index = make(map[string]VerificationRecord, len(m.Records))
	for _, record := range m.Records {
		m.index[verificationKey(record.Implementation, record.CaseID, record.Mode)] = record
	}
}

func verificationKey(implementation, caseID string, mode Mode) string {
	return implementation + "\x00" + caseID + "\x00" + string(mode)
}

// Record returns the exact preflight status for one matrix cell.
func (m *VerificationMatrix) Record(implementation, caseID string, mode Mode) (VerificationRecord, bool) {
	if m.index == nil {
		m.buildIndex()
	}
	record, ok := m.index[verificationKey(implementation, caseID, mode)]
	return record, ok
}

// Eligible reports whether a cell passed its pre-timing gate.
func (m *VerificationMatrix) Eligible(implementation, caseID string, mode Mode) bool {
	record, ok := m.Record(implementation, caseID, mode)
	return ok && record.Status == StatusEligible
}

// Unsupported returns stable, sorted unsupported records for reporting.
func (m *VerificationMatrix) Unsupported() []VerificationRecord {
	result := make([]VerificationRecord, 0)
	for _, record := range m.Records {
		if record.Status == StatusUnsupported {
			result = append(result, record)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Implementation != right.Implementation {
			return left.Implementation < right.Implementation
		}
		if left.CaseID != right.CaseID {
			return left.CaseID < right.CaseID
		}
		return left.Mode < right.Mode
	})
	return result
}

// ValidateWorkspace requires every workspace operation to pass before any
// benchmark collection can proceed.
func (m *VerificationMatrix) ValidateWorkspace(corpus Corpus) error {
	for _, sample := range corpus.Cases {
		for _, mode := range sample.Modes {
			record, ok := m.Record(WorkspaceImplementation, sample.ID, mode)
			if !ok {
				return fmt.Errorf("workspace verification is missing %s/%s", sample.ID, mode)
			}
			if record.Status != StatusEligible {
				return fmt.Errorf("workspace verification rejected %s/%s: %s", sample.ID, mode, record.Reason)
			}
		}
	}
	return nil
}

// ComparableCases returns cases that passed for every implementation in a
// mode. Unsupported implementations are never treated as losses or wins.
func (m *VerificationMatrix) ComparableCases(corpus Corpus, runtimes []Runtime, mode Mode) []Case {
	result := make([]Case, 0, len(corpus.Cases))
	for _, sample := range corpus.Cases {
		if !HasMode(sample, mode) {
			continue
		}
		comparable := true
		for _, runtime := range runtimes {
			if !m.Eligible(runtime.ID, sample.ID, mode) {
				comparable = false
				break
			}
		}
		if comparable {
			result = append(result, sample)
		}
	}
	return result
}

// MissingDimensions returns claim-required dimensions absent from the complete
// comparable subset.
func MissingDimensions(corpus Corpus, cases []Case, mode Mode) []string {
	covered := make(map[string]bool)
	for _, sample := range cases {
		for _, dimension := range sample.Dimensions {
			covered[dimension] = true
		}
	}
	missing := make([]string, 0)
	for _, required := range corpus.Coverage.RequiredDimensions[mode] {
		if !covered[required] {
			missing = append(missing, required)
		}
	}
	return missing
}
