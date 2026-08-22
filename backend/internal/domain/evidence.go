package domain

// This file holds the AudioMuse evidence layer: claims, the references they make, and the
// derived reverse views the read API serves alongside them.
//
// Contracts: schemas/claim.schema.yaml and schemas/source.schema.yaml. The prose model is
// docs/claim-provenance-model.md, which states the load-bearing rule this file follows:
// claim type, confidence, dispute status and evidence are four independent axes and are
// never collapsed into a single score or a single boolean.

// ClaimEvidence is one registered source's standing to a claim.
//
// Contract: schemas/claim.schema.yaml (evidence[]), key set exactly {relation, source_id,
// note}. Relation is one of evidence_relations. Note is required by the contract and is
// carried verbatim: it is what separates a source that supports a claim from a source that
// merely mentions the topic.
type ClaimEvidence struct {
	Relation string `json:"relation"  yaml:"relation"`
	SourceID string `json:"source_id" yaml:"source_id"`
	Note     string `json:"note"      yaml:"note"`
}

// ClaimAttribution records who a statement is credited to, and the registered source that
// records the credit.
//
// Contract: schemas/claim.schema.yaml (attribution[]), key set exactly {actor, source_id}.
// It is stored separately from evidence because "who says so" is a different question from
// "what stands behind it".
type ClaimAttribution struct {
	Actor    string `json:"actor"     yaml:"actor"`
	SourceID string `json:"source_id" yaml:"source_id"`
}

// ClaimReference is a kind-qualified pointer at another canonical record.
//
// Contract: schemas/claim.schema.yaml, shared by derived_from[] and appears_in[], key set
// exactly {kind, ref}. The kind vocabularies differ between the two fields and are read
// from the schema rather than hard-coded here.
type ClaimReference struct {
	Kind string `json:"kind" yaml:"kind"`
	Ref  string `json:"ref"  yaml:"ref"`
}

// Claim is one canonical AudioMuse claim record.
//
// Contract: schemas/claim.schema.yaml version 1. Every field below is required by that
// contract, which also sets additional_properties: false, so the record's key set must
// equal this set exactly. Nothing is invented here and nothing canonical is omitted.
//
// The four independent axes are ClaimType (what kind of statement), Confidence (how
// strongly repository evidence supports it), Evidence (which sources support, contradict or
// qualify it) and DisputeStatus (whether registered sources conflict). ConfidenceBasis is
// carried because a confidence level without its stated reason is exactly the flattening
// docs/claim-provenance-model.md exists to prevent.
//
// Path is not canonical content: it is the repository-relative file the record was read
// from, retained so a projection can cite its origin.
type Claim struct {
	ID                string             `json:"id"                 yaml:"id"`
	Statement         string             `json:"statement"          yaml:"statement"`
	ClaimType         string             `json:"claim_type"         yaml:"claim_type"`
	Confidence        string             `json:"confidence"         yaml:"confidence"`
	ConfidenceBasis   string             `json:"confidence_basis"   yaml:"confidence_basis"`
	DisputeStatus     string             `json:"dispute_status"     yaml:"dispute_status"`
	TemporalPrecision string             `json:"temporal_precision" yaml:"temporal_precision"`
	Evidence          []ClaimEvidence    `json:"evidence"           yaml:"evidence"`
	Attribution       []ClaimAttribution `json:"attribution"        yaml:"attribution"`
	DerivedFrom       []ClaimReference   `json:"derived_from"       yaml:"derived_from"`
	AppearsIn         []ClaimReference   `json:"appears_in"         yaml:"appears_in"`
	OpenQuestions     []string           `json:"open_questions"     yaml:"open_questions"`

	Path string `json:"path" yaml:"-"`
}

// ClaimSummary is the list projection of a Claim.
//
// It carries identity, the full statement, and all four provenance axes, because a claim
// list whose entries did not say how well evidenced they are would invite exactly the
// misreading the claim layer exists to prevent. The per-entry notes, bases and reference
// objects stay in the detail projection.
type ClaimSummary struct {
	ID                string `json:"id"`
	Statement         string `json:"statement"`
	ClaimType         string `json:"claim_type"`
	Confidence        string `json:"confidence"`
	DisputeStatus     string `json:"dispute_status"`
	TemporalPrecision string `json:"temporal_precision"`
	EvidenceCount     int    `json:"evidence_count"`
	AttributionCount  int    `json:"attribution_count"`
	AppearsInCount    int    `json:"appears_in_count"`
}

// ClaimDetail is the single-record projection: the canonical claim plus flattened ID lists
// for the relationships a client would otherwise have to reassemble.
//
// SourceIDs is the distinct set of sources the claim cites in evidence or attribution.
// NodeIDs and SessionIDs are the appearance sites of those kinds. All three are derived
// views of fields already present on the embedded Claim, kept as separate sorted ID lists
// so a client can traverse without re-implementing the flattening, and never as a
// substitute for the canonical arrays.
//
// DerivedFrom is deliberately not flattened into NodeIDs: "the statement is made here" and
// "the statement was built from this" are different relations and merging them would assert
// something the record does not.
type ClaimDetail struct {
	Claim
	SourceIDs  []string `json:"source_ids"`
	NodeIDs    []string `json:"node_ids"`
	SessionIDs []string `json:"session_ids"`
}

// SourceClaimRef is the reverse read of one claim evidence entry, from the source's side.
//
// The relation is carried because "which claims does this source support" and "which claims
// does this source contradict" are different questions, and a bare claim ID list answers
// neither.
type SourceClaimRef struct {
	ClaimID  string `json:"claim_id"`
	Relation string `json:"relation"`
}

// SourceSummary is the list projection of a Source.
//
// It carries the registry identity fields plus the two evidence-grading fields the
// provenance contract adds, and the size of the source's evidential footprint. The optional
// registry fields stay pointers so an absent field remains distinguishable from an empty
// one, exactly as on Source.
type SourceSummary struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	Title         string  `json:"title"`
	Author        *string `json:"author,omitempty"`
	Year          *int    `json:"year,omitempty"`
	Relationship  string  `json:"relationship"`
	EvidenceClass *string `json:"evidence_class,omitempty"`
	Retrieval     *string `json:"retrieval,omitempty"`
	ClaimCount    int     `json:"claim_count"`
	NodeCount     int     `json:"node_count"`
}

// SourceDetail is the single-record projection: the canonical registry entry plus the
// derived reverse views of everything that cites it.
//
// The two node relations are kept apart on purpose. NodeIDs is topical — the nodes whose
// canonical sources: list names this source, which docs/claim-provenance-model.md describes
// as "this source is relevant to this concept". Claims is evidential — the claims that cite
// this source as materially supporting, contradicting or qualifying a specific statement.
// Merging them would erase the distinction the whole provenance layer was built to make.
//
// AttributedClaimIDs is separate again because attribution names who credits a statement
// rather than what stands behind it, and it carries no evidence relation.
//
// SessionIDs is claim-mediated: sessions in which a claim citing this source appears.
type SourceDetail struct {
	Source
	Claims             []SourceClaimRef `json:"claims"`
	AttributedClaimIDs []string         `json:"attributed_claim_ids"`
	NodeIDs            []string         `json:"node_ids"`
	SessionIDs         []string         `json:"session_ids"`
}

// ClaimVocabulary is the bounded set of values the claim contract declares.
//
// These lists are read from schemas/claim.schema.yaml at startup rather than compiled in,
// for the same reason tools/validate-claims.ps1 reads them: a vocabulary change is a schema
// change and must not be possible to make silently inside a validator. Every claim filter
// the API accepts is checked against these lists.
type ClaimVocabulary struct {
	ClaimTypes         []string `json:"claim_types"`
	ConfidenceLevels   []string `json:"confidence_levels"`
	DisputeStatuses    []string `json:"dispute_statuses"`
	TemporalPrecisions []string `json:"temporal_precisions"`
	EvidenceRelations  []string `json:"evidence_relations"`
	DerivedFromKinds   []string `json:"derived_from_kinds"`
	AppearsInKinds     []string `json:"appears_in_kinds"`
}

// SourceVocabulary is the bounded set of values the source contract declares.
//
// Read from schemas/source.schema.yaml for the same reason as ClaimVocabulary.
type SourceVocabulary struct {
	Types           []string `json:"types"`
	Relationships   []string `json:"relationships"`
	EvidenceClasses []string `json:"evidence_classes"`
	Retrievals      []string `json:"retrievals"`
}

// Vocabularies is the canonical contract vocabulary the backend read at startup.
type Vocabularies struct {
	Claim  ClaimVocabulary  `json:"claim"`
	Source SourceVocabulary `json:"source"`
}

// Claim reference kinds and evidence relations that the backend resolves by name.
//
// These constants are used only to decide which reference kind a resolver applies to. The
// permitted value sets themselves still come from the schema; naming one here does not
// widen or narrow the contract.
const (
	ClaimKindClaim         = "claim"
	ClaimKindNode          = "node"
	ClaimKindSession       = "session"
	ClaimKindVocabulary    = "vocabulary"
	ClaimKindDocument      = "document"
	ClaimKindExperimentRun = "experiment_run"
)
