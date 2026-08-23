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

// SearchEntityTypeFacets is the entity-class composition of one complete search result set.
//
// It answers the question a result page cannot: "how much of what did this query actually
// find". A caller reading page one of a fifty-hit set can see that the corpus answered mostly
// with vocabulary entries and one claim, and can narrow with entity_types instead of paging
// through a set whose shape they cannot see.
//
// It is an explicit struct with one field per searchable class rather than a map, and the
// reason is the serialised order. Field order here is the canonical class order, so the object
// reads as the model's own layering rather than as an alphabetisation of it — a map would
// serialise claim, experiment, node in whatever order Go's key sort produced, which is stable
// but is not the model. It follows the same shape as the diagnostics counts the service
// already reports per class. The closed set is held to this struct by a test that walks
// SearchEntityTypes and requires every member to be counted, since a Go switch is not
// exhaustive on its own.
//
// Every field is always serialised, zero included. A class with no hits is a fact the caller
// asked for — "nothing of this kind matched" — and omitting it would make an absent class
// indistinguishable from a class this build does not search.
//
// These are counts of records, not scores. Nothing here ranks a class, orders the result set,
// or implies that a class with more hits is a better answer.
type SearchEntityTypeFacets struct {
	Session    int `json:"session"`
	Node       int `json:"node"`
	Claim      int `json:"claim"`
	Source     int `json:"source"`
	Vocabulary int `json:"vocabulary"`
	Experiment int `json:"experiment"`
}

// SearchFacets is the facet envelope of a search response.
//
// It is a named object with one member rather than a bare count map at the top level, so a
// later facet — were one ever contracted — is a new key inside it rather than a reshaping of
// the response. There is no experiment_run facet, for the reason there is no experiment_run
// search class: a zero would read as "no run mentioned this" rather than "runs are not
// searchable". See SearchEntityTypes.
type SearchFacets struct {
	EntityTypes SearchEntityTypeFacets `json:"entity_types"`
}

// NewSearchFacets counts one complete, already filtered and ordered result set.
//
// The caller passes the whole match set, never a page: the counts describe what the query
// found, and a facet that changed with limit and offset would describe the page instead and
// be useless for deciding how to narrow. The returned value is a copy of plain integers, so
// nothing here aliases the index or the result slice.
func NewSearchFacets(results []SearchResult) SearchFacets {
	var facets SearchFacets
	for _, result := range results {
		facets.EntityTypes.add(result.EntityType)
	}
	return facets
}

// add increments the counter for one class. An unknown class increments nothing, which keeps
// the sum invariant honest rather than silently attributing it to a neighbouring field; the
// closed set is enforced where documents are built, and a facet test walks SearchEntityTypes
// to require that every member of the set lands somewhere.
func (f *SearchEntityTypeFacets) add(t SearchEntityType) {
	switch t {
	case SearchSession:
		f.Session++
	case SearchNode:
		f.Node++
	case SearchClaim:
		f.Claim++
	case SearchSource:
		f.Source++
	case SearchVocabulary:
		f.Vocabulary++
	case SearchExperiment:
		f.Experiment++
	}
}

// Count reports one class's facet by name, so a caller — or a test walking the closed set —
// need not spell out a field per class.
func (f SearchEntityTypeFacets) Count(t SearchEntityType) int {
	switch t {
	case SearchSession:
		return f.Session
	case SearchNode:
		return f.Node
	case SearchClaim:
		return f.Claim
	case SearchSource:
		return f.Source
	case SearchVocabulary:
		return f.Vocabulary
	case SearchExperiment:
		return f.Experiment
	}
	return 0
}

// Total is the sum of every class count, which is the size of the complete filtered result
// set the facets were built from.
func (f SearchEntityTypeFacets) Total() int {
	return f.Session + f.Node + f.Claim + f.Source + f.Vocabulary + f.Experiment
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
// A result's own fields carry only its own record's data. Title and Summary are never borrowed
// from a related record, so a claim result cannot present its source's title as though claim
// and source were one record, and no provenance axis is flattened into another.
//
// Context is the one place another record may be named, and it is a separate, explicitly
// labelled sub-object for exactly that reason: every entity inside it arrives with the relation
// and the canonical field it was read from, so it reads as "this record references that one"
// rather than as part of the hit. It is absent unless the caller asked for it, so the default
// response is the Phase 1E shape unchanged. A unified search surface is still not a unified
// ontology. To read a related record itself rather than its identity, a client follows the ref
// to that record's own route, addressed by EntityType and ID.
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

	// TermMatches is the per-term evidence of a multi-term hit, present only for a query
	// composed under SearchModeAllTerms. It is absent from every literal result, so a Phase
	// 1E/1F response is unchanged, and it is a distinct field rather than a reshaping of
	// MatchedFields because the two answer different questions: MatchedFields is still the
	// union of this record's fields the query reached, while TermMatches says which term
	// reached which. Terms appear in normalised query order and fields in canonical field
	// order, so the evidence is as reproducible as the result it explains. See
	// SearchTermMatch.
	TermMatches []SearchTermMatch `json:"term_matches,omitempty"`

	// Context is the bounded canonical context of this hit, present only when the caller asked
	// for it. A pointer rather than a value so that an unrequested context is absent from the
	// response rather than serialised as an empty object that a client could mistake for "this
	// record has no context"; those are different facts and a requested-but-empty context is
	// served as an empty Related list. See context.go.
	Context *SearchResultContext `json:"context,omitempty"`
}

// Query modes: how the caller's text is composed into one lexical query.
//
// Phase 1E searched for one contiguous needle and nothing else, and that stays the default:
// a request that names no mode means exactly what it meant before, so whitespace in q is part
// of the phrase rather than a separator. Reinterpreting it silently would change the meaning
// of every query already in a client's code.
//
// The second mode exists because a reader who types two words usually means "a record about
// both", and the corpus may carry those words in different canonical fields of the same
// record — a node whose title says one and whose definition says the other. Making that
// reachable is a composition decision, not a retrieval one: SearchModeAllTerms still performs
// the same case-insensitive substring test, term by term, over the same field set. It does
// not stem, score, rank, expand or infer, and a record matching every term is a statement
// about text, not about meaning.
type SearchQueryMode string

const (
	// SearchModeLiteral is the Phase 1E contract: the whole normalised query is one needle.
	SearchModeLiteral SearchQueryMode = "literal"
	// SearchModeAllTerms requires every normalised term to occur somewhere in the same record.
	SearchModeAllTerms SearchQueryMode = "all_terms"
)

// SearchQueryModes is the closed set, in the order an error message lists them: the default
// first, so a caller reading the list sees what an omitted mode does.
var SearchQueryModes = []SearchQueryMode{SearchModeLiteral, SearchModeAllTerms}

// ValidSearchQueryMode reports whether a caller-supplied string names a query mode.
//
// The comparison is exact. "AND", "and", "all", "true" and "1" are refused rather than guessed
// at, for the same reason the tri-state boolean filters refuse "yes": a mode that silently
// resolved to the wrong composition would answer a different question than the one asked.
func ValidSearchQueryMode(value string) bool {
	for _, mode := range SearchQueryModes {
		if string(mode) == value {
			return true
		}
	}
	return false
}

// SearchQueryModeNames renders the closed set for an error message.
func SearchQueryModeNames() []string {
	out := make([]string, 0, len(SearchQueryModes))
	for _, mode := range SearchQueryModes {
		out = append(out, string(mode))
	}
	return out
}

// MatchAllTerms is the categorical match class of a multi-term hit.
//
// It is a fifth class rather than a reuse of the four single-needle ones, because none of them
// is true of a composed query: the record did not match "the query" in its title or in one
// field, it matched each term somewhere. Forcing it into MatchFieldSubstring would report a
// fact that did not happen, and forcing it into MatchTitleSubstring would require picking one
// term as the important one — a weighting judgement this layer does not make.
//
// Like the other four it is a label, not a score. Every all_terms hit carries this one class,
// so it orders nothing; multi-term ordering is the canonical class order and then canonical ID.
const MatchAllTerms = "all_terms"

// SearchTermMatch is the per-term evidence for one multi-term hit.
//
// A composed query is not one substring, so reporting it as one would misdescribe the hit.
// This says which canonical fields each term was found in, which is the whole of what the
// backend knows: no snippet, no offset, no highlighting, no rewritten prose, and no count. A
// term appearing five times in a field is recorded the same way as one appearing once, because
// frequency is the first step towards a relevance score and this layer has none.
//
// MatchedFields names fields of the hit's own record only. A term satisfied by a related
// record is not a match here: context is resolved after discovery and never feeds back into it.
type SearchTermMatch struct {
	Term          string   `json:"term"`
	MatchedFields []string `json:"matched_fields"`
}
