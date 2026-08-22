package service

import "github.com/britbufkin1225-web/audiomuse/backend/internal/domain"

// ProjectName is the canonical project identity from docs/project-scope.md.
const (
	ProjectName       = "AudioMuse"
	ProjectDescriptor = "A Resonant Atlas of Sound, Music & Signal"
	ModeReadOnly      = "read-only"
)

// Counts is the corpus size summary.
//
// ExperimentRuns is the total across every lifecycle state. It is deliberately not the count
// of runs that produced evidence: the per-state split lives on ExperimentRunTotals, because a
// single number here would let "three runs exist" be read as "three experiments were carried
// out and observed".
type Counts struct {
	Nodes             int `json:"nodes"`
	Sessions          int `json:"sessions"`
	Sources           int `json:"sources"`
	Claims            int `json:"claims"`
	Vocabulary        int `json:"vocabulary"`
	Experiments       int `json:"experiments"`
	ExperimentRuns    int `json:"experiment_runs"`
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

	// VocabularyDomains is the domain set present in the vocabulary layer. It is reported
	// beside Domains, which is the node layer's, rather than merged into it: the two layers
	// draw from the same canonical enum but need not use the same members of it.
	VocabularyDomains []string `json:"vocabulary_domains"`

	// ExperimentRuns is the corpus-wide run tally broken out by lifecycle state, so a client
	// can see at a glance how much of the experiment layer is planned rather than performed.
	ExperimentRuns domain.ExperimentRunCounts `json:"experiment_runs"`

	// Vocabulary is the bounded value set read from schemas/claim.schema.yaml and
	// schemas/source.schema.yaml. It is served here so a client can discover exactly which
	// filter values the evidence endpoints accept without hard-coding a copy of the
	// contract, and without discovering them by trial and error against 400 responses.
	Vocabulary domain.Vocabularies `json:"vocabulary"`
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
			Sources:           len(k.sources),
			Claims:            len(k.claims),
			Vocabulary:        len(k.vocabulary),
			Experiments:       len(k.experiments),
			ExperimentRuns:    len(k.runs),
			Edges:             k.graph.Metadata.EdgeCount,
			RelationshipTypes: len(k.relationshipTypes),
			Domains:           len(k.Domains()),
		},
		Domains:      k.Domains(),
		Statuses:     k.Statuses(),
		Validation:   k.report.Status(),
		WarningCount: len(k.report.Warnings()),
		CanonicalLayer: []string{
			"nodes", "sessions", "sources", "claims",
			"vocabulary", "experiments", "experiment-runs", "relationship-types",
		},
		VocabularyDomains: k.VocabularyDomains(),
		ExperimentRuns:    k.ExperimentRunTotals(),
		Vocabulary:        k.Vocabularies(),
	}
}

// Diagnostics is the sanitized validation view.
//
// Only warnings appear here: a fatal issue prevents startup, so a running process has none.
// Issue Path values are repository-relative and Ref values are canonical IDs, so nothing in
// this projection discloses the operator's filesystem layout.
type Diagnostics struct {
	Mode                         string                   `json:"mode"`
	Status                       string                   `json:"status"`
	ValidationScope              string                   `json:"validation_scope"`
	RepositorySemanticValidation string                   `json:"repository_semantic_validation"`
	Warnings                     []domain.ValidationIssue `json:"warnings"`
	Counts                       DiagnosticsCounts        `json:"counts"`
	Corpus                       DiagnosticsCorpus        `json:"corpus"`

	// Search is the size of the cross-layer discovery projection, reported so an operator can
	// confirm which layers the running process actually made discoverable. It is a count of
	// search documents, not of matches, and it holds no query, no history and no analytics: a
	// search request exists only for the life of that request.
	Search SearchCounts `json:"search"`
}

// DiagnosticsCounts summarises the report.
type DiagnosticsCounts struct {
	Fatal   int `json:"fatal"`
	Warning int `json:"warning"`
}

// DiagnosticsCorpus is the size of the loaded canonical corpus, reported so an operator can
// confirm which layers the running process actually projected.
//
// These are counts of records read from canonical files. They are not read from the generated
// Markdown indexes under vocabulary/, experiments/, experiment-runs/ or indexes/, which are
// rebuildable views rather than knowledge; a mismatch between the two is a signal worth having,
// which it would not be if the backend read the projection it was being compared against.
type DiagnosticsCorpus struct {
	Nodes          int `json:"nodes"`
	Sessions       int `json:"sessions"`
	Sources        int `json:"sources"`
	Claims         int `json:"claims"`
	Vocabulary     int `json:"vocabulary"`
	Experiments    int `json:"experiments"`
	ExperimentRuns int `json:"experiment_runs"`
}

// Diagnostics returns the sanitized validation warnings from the load that built the index.
func (k *Knowledge) Diagnostics() Diagnostics {
	warnings := k.report.Warnings()
	if warnings == nil {
		warnings = []domain.ValidationIssue{}
	}
	return Diagnostics{
		Mode:                         ModeReadOnly,
		Status:                       k.report.Status(),
		ValidationScope:              "runtime_projection",
		RepositorySemanticValidation: "external_precondition",
		Warnings:                     warnings,
		Counts: DiagnosticsCounts{
			Fatal:   len(k.report.Fatal()),
			Warning: len(warnings),
		},
		Corpus: DiagnosticsCorpus{
			Nodes:          len(k.nodes),
			Sessions:       len(k.sessions),
			Sources:        len(k.sources),
			Claims:         len(k.claims),
			Vocabulary:     len(k.vocabulary),
			Experiments:    len(k.experiments),
			ExperimentRuns: len(k.runs),
		},
		Search: k.SearchDiagnostics(),
	}
}
