package domain

// This file holds the AudioMuse context layer: the bounded canonical surroundings of one
// cross-layer search hit.
//
// Nothing here is a new canonical concept and nothing here is a graph concept. Every relation
// below is read from exactly one canonical field of exactly one canonical record, and every
// entity named is a record the backend already loads. The layer exists because a search result
// is deliberately a projection of one record (see discovery.go) and a reader who has just found
// a claim usually wants to know what stands behind it before deciding whether to open it.
//
// Context resolution is not traversal. It answers "what does this record directly reference,
// and what directly references it", once, for the hit itself. There is no depth, no frontier,
// no breadth-first expansion and no caller-supplied walk; a caller who wants the neighbourhood
// of a record continues to use the Phase 1C traversal routes. The two layers deliberately share
// their relationship names where they describe the same canonical connection, so one repository
// connection keeps one meaning wherever it is read.

// ContextRef addresses one canonical record for navigation.
//
// It is typed by SearchEntityType rather than by EntityType because context spans all six
// searchable classes while the traversal graph addresses only four. That difference is the
// point rather than an inconvenience: a ContextRef is a navigation reference, not a graph
// vertex, so naming a vocabulary entry or an experiment definition here does not enrol it in
// the graph, and traversal.go's rule that those classes are not graph entities is untouched.
//
// EntityType and ID together are the identity. Two classes may share an ID — a session and its
// registry entry do — so a ref is never resolved by ID alone. Label is deterministic record
// content, taken from the same display field the discovery layer already chose for that class,
// so a record's context label and its search title are the same string by construction. The ref
// carries no other record content, which is what keeps a context payload from growing
// recursively.
type ContextRef struct {
	EntityType SearchEntityType `json:"entity_type"`
	ID         string           `json:"id"`
	Label      string           `json:"label"`
}

// ContextRelation is one canonical relationship from a search hit to one other record.
//
// Origin names the canonical field the relation was read from, so "why is this here" is
// answerable from the relation itself rather than by re-deriving it. Derived separates a
// reference a record authored from the reverse read of one, following the Phase 1A rule that a
// derived view must never be mistakable for authored data: a node did not write down which
// experiments cite it, and a context item saying otherwise would misrepresent the corpus.
//
// Relation is a label, never a judgement. Nothing in this layer scores, ranks or weighs what it
// resolves: a contradicting source is presented exactly as a supporting one, with its own
// canonical relation name, and it is the reader who decides what that is worth.
type ContextRelation struct {
	Relation string     `json:"relation"`
	Entity   ContextRef `json:"entity"`
	Origin   string     `json:"origin"`
	Derived  bool       `json:"derived"`
}

// SearchResultContext is the bounded canonical context of one search hit.
//
// Related is a flat, relation-labelled list rather than a parent/children shape. AudioMuse has
// no ownership hierarchy to render: docs/knowledge-model.md describes node session_origin as a
// many-to-many contribution map rather than as ownership, and a record may be referenced from
// several layers at once. A "parent" field would therefore have to pick one of several equal
// relations and present it as containment, which would be an invented fact.
//
// Count is the size of the record's full canonical context and Returned is how much of it this
// response carries, so a truncated context reports what it left out instead of looking like a
// complete small one. Truncated is never false when anything was dropped.
type SearchResultContext struct {
	Related   []ContextRelation `json:"related"`
	Count     int               `json:"count"`
	Returned  int               `json:"returned"`
	Truncated bool              `json:"truncated"`
}

// Canonical fields that context relations outside the traversal graph are read from.
//
// The graph's own origins are reused unchanged for every relation the adjacency already
// carries; see the Origin block in traversal.go. The names below cover only the reference
// fields the graph deliberately does not model, so no canonical field acquires a second origin
// name depending on which layer read it.
const (
	OriginVocabularyNodeRefs       = "vocabulary.node_refs"
	OriginVocabularySessionRefs    = "vocabulary.session_refs"
	OriginVocabularyRelatedTerms   = "vocabulary.related_terms"
	OriginExperimentNodeRefs       = "experiment.node_refs"
	OriginExperimentVocabularyRefs = "experiment.vocabulary_refs"
	OriginExperimentSessionRefs    = "experiment.session_refs"
	OriginExperimentSourceRefs     = "experiment.source_refs"
	OriginExperimentRelated        = "experiment.related_experiments"
)

// Context relation names for the practice layer's reference fields.
//
// The traversal relation names are reused wherever the adjacency already describes a
// connection, so a claim's node appearance is appears_in in both layers. These four exist only
// for the reference fields the graph does not model, and each restates its field rather than
// interpreting it:
//
//	vocabulary.node_refs / session_refs      entry --references--> record
//	experiment.*_refs                        definition --references--> record
//	                                         record --referenced_by--> entry|definition
//	vocabulary.related_terms                 entry --related_term--> entry
//	experiment.related_experiments           definition --related_experiment--> definition
//
// references is deliberately one name rather than one per field. The canonical fields are
// literally reference lists, Origin already says which one was read, and inventing
// describes_node, applies_to_session and the rest would assert a relation the corpus does not
// state.
//
// The two symmetric lists emit no reverse relation. vocabulary/README.md states that related
// terms are human navigation only and imply neither equivalence nor a graph edge, and neither
// contract requires the pairing to be authored on both sides; emitting a reverse would turn "A
// listed B" into "B is related to A", which is a claim the corpus has not made. Where the
// pairing is authored on both records, both records already carry it.
const (
	ContextRelReferences        = "references"
	ContextRelReferencedBy      = "referenced_by"
	ContextRelRelatedTerm       = "related_term"
	ContextRelRelatedExperiment = "related_experiment"
)
