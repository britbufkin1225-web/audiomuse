package service

import "github.com/britbufkin1225-web/audiomuse/backend/internal/domain"

// ProjectName is the canonical project identity from docs/project-scope.md.
const (
	ProjectName       = "AudioMuse"
	ProjectDescriptor = "A Resonant Atlas of Sound, Music & Signal"
	ModeReadOnly      = "read-only"
)

// Counts is the corpus size summary.
type Counts struct {
	Nodes             int `json:"nodes"`
	Sessions          int `json:"sessions"`
	Sources           int `json:"sources"`
	Edges             int `json:"edges"`
	RelationshipTypes int `json:"relationship_types"`
	Domains           int `json:"domains"`
}

// ProjectSummary is the corpus overview projection.
//
// Repository identifies the corpus by name and adapter kind only. The absolute filesystem
// path is deliberately not served: it identifies the operator's machine and account, adds
// nothing a client can act on, and would leak through any shared response or screenshot.
// The operator sees the full path once, in the startup log.
type ProjectSummary struct {
	Name           string   `json:"name"`
	Descriptor     string   `json:"descriptor"`
	Mode           string   `json:"mode"`
	Repository     RepoInfo `json:"repository"`
	Counts         Counts   `json:"counts"`
	Domains        []string `json:"domains"`
	Statuses       []string `json:"statuses"`
	Validation     string   `json:"validation"`
	WarningCount   int      `json:"warning_count"`
	CanonicalLayer []string `json:"canonical_layers_served"`
}

// RepoInfo names the corpus without revealing where it lives.
type RepoInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Project returns the corpus overview.
func (k *Knowledge) Project() ProjectSummary {
	return ProjectSummary{
		Name:       ProjectName,
		Descriptor: ProjectDescriptor,
		Mode:       ModeReadOnly,
		Repository: RepoInfo{Name: k.descriptor.Name, Kind: k.descriptor.Kind},
		Counts: Counts{
			Nodes:             len(k.nodes),
			Sessions:          len(k.sessions),
			Sources:           k.sourceCount,
			Edges:             k.graph.Metadata.EdgeCount,
			RelationshipTypes: len(k.relationshipTypes),
			Domains:           len(k.Domains()),
		},
		Domains:        k.Domains(),
		Statuses:       k.Statuses(),
		Validation:     k.report.Status(),
		WarningCount:   len(k.report.Warnings()),
		CanonicalLayer: []string{"nodes", "sessions", "sources", "relationship-types"},
	}
}

// Diagnostics is the sanitized validation view.
//
// Only warnings appear here: a fatal issue prevents startup, so a running process has none.
// Issue Path values are repository-relative and Ref values are canonical IDs, so nothing in
// this projection discloses the operator's filesystem layout.
type Diagnostics struct {
	Mode     string                   `json:"mode"`
	Status   string                   `json:"status"`
	Warnings []domain.ValidationIssue `json:"warnings"`
	Counts   DiagnosticsCounts        `json:"counts"`
}

// DiagnosticsCounts summarises the report.
type DiagnosticsCounts struct {
	Fatal   int `json:"fatal"`
	Warning int `json:"warning"`
}

// Diagnostics returns the sanitized validation warnings from the load that built the index.
func (k *Knowledge) Diagnostics() Diagnostics {
	warnings := k.report.Warnings()
	if warnings == nil {
		warnings = []domain.ValidationIssue{}
	}
	return Diagnostics{
		Mode:     ModeReadOnly,
		Status:   k.report.Status(),
		Warnings: warnings,
		Counts: DiagnosticsCounts{
			Fatal:   len(k.report.Fatal()),
			Warning: len(warnings),
		},
	}
}
