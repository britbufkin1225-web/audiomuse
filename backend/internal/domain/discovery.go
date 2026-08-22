package domain

// This file holds the AudioMuse discovery layer: the shape one canonical record takes when
// it is returned as a cross-layer search hit.
//
// Nothing here is a new canonical concept, and nothing here is a graph concept. A search
// result is a display projection of a record the backend already loads, carrying that
// record's own identity and its own descriptive text and nothing belonging to any other
// record. Discovery and graph semantics are separate contracts: appearing in a search result
// makes a record findable, not traversable, and no type in this file is an EntityRef, a
// GraphEntity or an edge endpoint. See traversal.go for the graph model.

// SearchEntityType is the class of a record that cross-layer search can return.
//
// The set is closed. It is deliberately not the same set as EntityTypes: traversal addresses
// the four record classes that stand in canonical graph relationships, while discovery covers
// every class that carries authored descriptive text worth searching. Vocabulary entries and
// experiment definitions are searchable here and are still not traversal entities.
//
// Experiment runs are absent on purpose. A run's prose is its observations, measurements and
// interpretation, and a free-text hit inside those would mean "this run observed that", which
// is an evidence assertion a discovery projection has no business making. Runs stay reachable
// through their own structured routes, where the caller states which definition and which
// lifecycle state they are asking about.
type SearchEntityType string

const (
	SearchSession    SearchEntityType = "session"
	SearchNode       SearchEntityType = "node"
	SearchClaim      SearchEntityType = "claim"
	SearchSource     SearchEntityType = "source"
	SearchVocabulary SearchEntityType = "vocabulary"
	SearchExperiment SearchEntityType = "experiment"
)

// SearchEntityTypes is the closed set, in the canonical order results are grouped by.
//
// The first four are the epistemic layering EntityTypes already uses — chronology, concept,
// statement, provenance — and the practice classes follow it. The order is fixed rather than
// sorted so that a result ordering and a validation error both read as the model rather than
// as an alphabetisation of it.
var SearchEntityTypes = []SearchEntityType{
	SearchSession, SearchNode, SearchClaim, SearchSource, SearchVocabulary, SearchExperiment,
}

// ValidSearchEntityType reports whether a caller-supplied string names a searchable class.
func ValidSearchEntityType(value string) bool {
	for _, t := range SearchEntityTypes {
		if string(t) == value {
			return true
		}
	}
	return false
}

// SearchEntityTypeNames renders the closed set for an error message.
func SearchEntityTypeNames() []string {
	out := make([]string, 0, len(SearchEntityTypes))
	for _, t := range SearchEntityTypes {
		out = append(out, string(t))
	}
	return out
}

// SearchEntityRank orders the searchable classes by the model's own layering rather than
// alphabetically. An unknown class sorts last and deterministically.
func SearchEntityRank(t SearchEntityType) int {
	for i, known := range SearchEntityTypes {
		if known == t {
			return i
		}
	}
	return len(SearchEntityTypes)
}

// Match kinds: the categorical precedence classes a result is ordered by.
//
// These are not a relevance score and are deliberately not rendered as a number. They are a
// fixed, hand-written precedence over four mutually exclusive ways a query can have matched,
// and calling them anything else would dress a priority list as information retrieval. There
// is no weighting, no field boosting and no tie-breaking by frequency; ties are broken by the
// entity class order and then by canonical ID, which is what makes the ordering reproducible.
//
// "Title" here means the record's display field, which is a different canonical field per
// class — a node, source or experiment title, a vocabulary term, a session title, a claim
// statement. MatchedFields always names the canonical field, never the word "title".
const (
	// MatchIDExact: the query is exactly the record's canonical ID.
	MatchIDExact = "id_exact"
	// MatchTitleExact: the query is exactly the record's display field.
	MatchTitleExact = "title_exact"
	// MatchTitleSubstring: the query appears inside the record's display field.
	MatchTitleSubstring = "title_substring"
	// MatchFieldSubstring: the query appears only in some other searchable field.
	MatchFieldSubstring = "field_substring"
)

// SearchResult is one cross-layer discovery hit.
//
// It is an explicit DTO rather than one of the canonical record types because the six
// searchable classes have genuinely different shapes, and flattening them into a shared
// record type — giving a claim a "title", giving a session a "definition" — would distort the
// canonical models to satisfy one response shape. Nothing here is invented: Title and Summary
// are copied verbatim from fields the record already carries, and a class with no natural
// summary field returns none rather than acquiring a fabricated one.
//
// A result carries only its own record's data. It never names a related record, so a claim
// result cannot present its source's title as though claim and source were one record, and no
// provenance axis is flattened into another. To follow a hit into its context, a client reads
// the record itself through its own route, addressed by EntityType and ID.
type SearchResult struct {
	EntityType SearchEntityType `json:"entity_type"`
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Summary    string           `json:"summary,omitempty"`

	// MatchKind is the categorical precedence class above, echoed so a client can see why a
	// result sorted where it did rather than having to trust the order.
	MatchKind string `json:"match_kind"`

	// MatchedFields names every canonical field of this record the query matched, in the
	// record's own field order. It is the evidence for the hit: a client can answer "why did
	// this appear" without a second request. Only fields that actually matched are listed, the
	// names are the canonical ones from the record's schema, no text is highlighted or
	// rewritten, and no semantic category is inferred.
	MatchedFields []string `json:"matched_fields"`
}
