package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Severity separates the two different things repository validation can find.
//
// SeverityFatal means the projection would be wrong or ambiguous, so the process must not
// serve it. SeverityWarning means the projection is correct but the corpus has a gap worth
// a human's attention. The backend never repairs a record in either case.
type Severity string

const (
	SeverityFatal   Severity = "fatal"
	SeverityWarning Severity = "warning"
)

// Validation issue codes. These are stable strings so a diagnostics consumer can match on
// them without parsing prose.
const (
	CodeMalformedRecord     = "malformed_record"
	CodeMissingField        = "missing_required_field"
	CodeUnknownField        = "unknown_field"
	CodeInvalidID           = "invalid_id"
	CodeDuplicateID         = "duplicate_id"
	CodeUnresolvedTarget    = "unresolved_relationship_target"
	CodeInvalidRelationType = "invalid_relationship_type"
	CodeSelfLink            = "self_link"
	CodeDuplicateEdge       = "duplicate_edge"
	CodeUnresolvedSession   = "unresolved_session_reference"
	CodeUnresolvedSource    = "unresolved_source_reference"
	CodeUnsafePath          = "unsafe_path"
	CodeMissingLocator      = "missing_locator_target"
	CodeMissingSessionDir   = "missing_session_directory"
	CodeUncitedSession      = "uncited_session"
	CodeUncitedSource       = "uncited_source"
)

// Evidence-layer validation issue codes, added in Phase 1B.
//
// They are separate constants rather than reuses of the node codes above so a diagnostics
// consumer can tell an evidence-integrity failure from a graph-integrity failure without
// parsing prose. Where a Phase 1A code already means exactly the right thing — a malformed
// record, a duplicate ID, a missing or unknown field, an unsafe path — it is reused rather
// than duplicated under an evidence-specific name.
const (
	CodeInvalidVocabulary    = "invalid_vocabulary_value"
	CodeUnresolvedClaim      = "unresolved_claim_reference"
	CodeClaimDerivationCycle = "claim_derivation_cycle"
	CodeDuplicateEvidence    = "duplicate_evidence_reference"
	CodeGeneratedAppearance  = "generated_appearance_site"
	CodeMissingAppearance    = "missing_appearance_site"
	CodeUnresolvedDocument   = "unresolved_document_reference"
	CodeUnresolvedContract   = "unresolved_contract_reference"
)

// ValidationIssue is one finding. Ref is the canonical ID the finding is about; Path is the
// repository-relative file it was read from. Neither carries an absolute filesystem path,
// so an issue is safe to serve over the diagnostics endpoint as-is.
type ValidationIssue struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Ref      string   `json:"ref,omitempty"`
	Path     string   `json:"path,omitempty"`
}

func (i ValidationIssue) String() string {
	parts := []string{string(i.Severity), i.Code}
	if i.Ref != "" {
		parts = append(parts, i.Ref)
	}
	if i.Path != "" {
		parts = append(parts, i.Path)
	}
	return fmt.Sprintf("%s: %s", strings.Join(parts, " "), i.Message)
}

// ValidationReport is the full set of findings from one corpus load.
type ValidationReport struct {
	Issues []ValidationIssue `json:"issues"`
}

// Add appends an issue to the report.
func (r *ValidationReport) Add(issue ValidationIssue) {
	r.Issues = append(r.Issues, issue)
}

// Fatal returns the fatal issues in report order.
func (r *ValidationReport) Fatal() []ValidationIssue { return r.bySeverity(SeverityFatal) }

// Warnings returns the non-fatal issues in report order.
func (r *ValidationReport) Warnings() []ValidationIssue { return r.bySeverity(SeverityWarning) }

func (r *ValidationReport) bySeverity(s Severity) []ValidationIssue {
	out := make([]ValidationIssue, 0, len(r.Issues))
	for _, issue := range r.Issues {
		if issue.Severity == s {
			out = append(out, issue)
		}
	}
	return out
}

// HasFatal reports whether the corpus cannot be safely projected.
func (r *ValidationReport) HasFatal() bool { return len(r.Fatal()) > 0 }

// Status summarises the report as PASS, WARN or FAIL for startup logging.
func (r *ValidationReport) Status() string {
	switch {
	case r.HasFatal():
		return "FAIL"
	case len(r.Issues) > 0:
		return "WARN"
	default:
		return "PASS"
	}
}

// Sort orders issues deterministically so two loads of an unchanged corpus produce an
// identical report regardless of traversal or map iteration order.
func (r *ValidationReport) Sort() {
	sort.SliceStable(r.Issues, func(a, b int) bool {
		x, y := r.Issues[a], r.Issues[b]
		if x.Severity != y.Severity {
			return x.Severity == SeverityFatal
		}
		if x.Code != y.Code {
			return x.Code < y.Code
		}
		if x.Ref != y.Ref {
			return x.Ref < y.Ref
		}
		if x.Path != y.Path {
			return x.Path < y.Path
		}
		return x.Message < y.Message
	})
}

// FatalError renders the fatal issues as a single error suitable for aborting startup.
func (r *ValidationReport) FatalError() error {
	fatal := r.Fatal()
	if len(fatal) == 0 {
		return nil
	}
	lines := make([]string, 0, len(fatal)+1)
	lines = append(lines, fmt.Sprintf("canonical repository failed validation with %d fatal issue(s):", len(fatal)))
	for _, issue := range fatal {
		lines = append(lines, "  - "+issue.String())
	}
	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}
