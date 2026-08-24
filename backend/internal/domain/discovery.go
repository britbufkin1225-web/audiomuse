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

// Match kinds: the coarse categorical summary of how a literal result matched.
//
// These remain the four mutually exclusive Phase 1E classes and are not themselves a relevance
// score. Phase 1I derives them from MatchSignals so the coarse explanation cannot disagree with
// the score, then orders results by RelevanceScore, canonical entity class and canonical ID.
// There is still no field-frequency or occurrence-frequency tie-break.
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

	// RelevanceScore is the deterministic integer relevance of this hit: the exact sum of the
	// weights of every signal in MatchSignals, and nothing else. It is always present, on every
	// result of every mode, because a score that appeared only sometimes would make "this hit is
	// unranked" and "this hit scored nothing" the same response. It is an integer rather than a
	// normalised float: it is compared, never displayed as a percentage, and a float would make
	// two equal hits capable of differing in the last bit.
	//
	// It is comparable only within one response. Nothing calibrates it across queries, and a
	// larger score on a different query does not mean a better answer, because the signals a
	// query can even fire depend on the query. See the match-signal block.
	RelevanceScore int `json:"relevance_score"`

	// MatchSignals names every ranking signal this hit fired, in SearchMatchSignals order —
	// descending weight — so the first entry is always the strongest thing true of this hit. It
	// is the explanation of RelevanceScore and of the ordering together: a client can sum the
	// weights and recover the score, and can compare two results' lists and see which signal
	// separated them.
	//
	// Signals are facts about how the query reached this record's own canonical fields. No entry
	// is prose, none is generated, none is inferred, and none can name another record. Mutually
	// exclusive signals never appear together and no signal appears twice, so the list is the
	// minimal true description of the hit rather than every statement that could be made about
	// it. It is always non-empty: a record that matched nothing is not a result.
	MatchSignals []string `json:"match_signals"`

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
// so it orders nothing; Phase 1I orders multi-term hits by their relevance score before the
// canonical class and ID tie-breaks.
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

// Match signals: the deterministic ranking vocabulary.
//
// Phase 1E gave a result one categorical MatchKind, which answers "why did this match" but not
// "why is this above that" beyond four coarse tiers, and Phase 1G's composed mode answers
// neither — every all_terms hit carries one class, so the whole ordering was the class order and
// the ID. A signal is the missing middle: one named, individually observable property of how the
// normalised query reached this record, carrying a fixed weight.
//
// A signal is not a score component in the information-retrieval sense. Nothing here counts
// occurrences, measures a field's length, consults how often a record is referenced, or knows
// anything about any request but this one. Each signal is a yes-or-no fact about one query and
// one record's own canonical fields, and the whole of the relevance score is the sum of the
// weights of the signals that are true. That is the point of expressing it this way: a client
// holding match_signals can recompute relevance_score itself and check the backend's arithmetic,
// which a hand-tuned opaque score would not allow.
//
// "Title" here means the record's display field, which is a different canonical field per class
// exactly as it is for the match kinds; see the MatchKind block.
const (
	// SignalIDExact: the normalised query is exactly the record's canonical ID.
	SignalIDExact = "id_exact"
	// SignalTitleExact: the normalised query is exactly the record's display field.
	SignalTitleExact = "title_exact"
	// SignalTitlePrefix: the display field begins with the normalised query and continues past
	// it. A reader who types the start of a name means that name, and a record it opens is a
	// better answer than one that mentions the same text halfway through a sentence.
	SignalTitlePrefix = "title_prefix"
	// SignalTitleSubstring: the normalised query occurs inside the display field, neither at its
	// start nor as the whole of it.
	SignalTitleSubstring = "title_substring"
	// SignalPhraseMatch: the complete normalised query occurs contiguously in some field of the
	// record. It is emitted only for a composed query, because a literal hit is a contiguous
	// occurrence by definition and the signal would restate the mode instead of distinguishing
	// the hit. For a composed query it is the strongest thing that can be said short of the
	// display field itself: the record carries the caller's words together, in their order,
	// rather than scattered across five fields.
	SignalPhraseMatch = "phrase_match"
	// SignalTitleAllTerms: every term of a composed query occurs in the display field.
	SignalTitleAllTerms = "title_all_terms"
	// SignalTitleTerms: at least one term of a composed query, but not every term, occurs in the
	// display field. It is mutually exclusive with SignalTitleAllTerms rather than additive, so
	// complete coverage is never reported as partial coverage plus something.
	SignalTitleTerms = "title_terms"
	// SignalIDSubstring: the query — or, for a composed query, at least one of its terms —
	// occurs inside the canonical ID without being the whole of it. The ID is authored, stable
	// and short, so text inside it is a stronger indication than the same text inside prose,
	// and weaker than any statement about the display field.
	SignalIDSubstring = "id_substring"
	// SignalFieldMatch: the query reached at least one searchable field that is neither the
	// canonical ID nor the display field. It is the floor: every hit that is not explained by
	// identity or by its name carries this and nothing stronger.
	SignalFieldMatch = "field_match"
)

// SearchMatchSignals is the closed set, in descending weight order.
//
// The order is the ranking policy written down. A result's own signal list is emitted in this
// order, so two results that matched the same way describe themselves identically and a client
// diffing two responses sees an ordering change rather than a reshuffled explanation.
var SearchMatchSignals = []string{
	SignalIDExact,
	SignalTitleExact,
	SignalTitlePrefix,
	SignalTitleSubstring,
	SignalPhraseMatch,
	SignalTitleAllTerms,
	SignalTitleTerms,
	SignalIDSubstring,
	SignalFieldMatch,
}

// SearchSignalWeight is the fixed integer weight of one signal, or zero for an unknown name.
//
// The weights are powers of two chosen so that each is strictly greater than the sum of every
// weight below it. That is the whole of the ranking policy's soundness argument: it makes the
// integer sum behave exactly as a lexicographic comparison of the signal list would, so a
// record carrying a stronger signal outranks a record carrying every weaker signal at once, and
// no accumulation of weak evidence can ever overtake one strong piece. A weight table without
// that property would be a set of magic numbers whose ordering nobody could predict from
// reading it. SearchSignalWeightsAreLexicographic states the invariant, and a test enforces it.
//
// It is a switch rather than a map because a map would invite ranging over it, and a ranking
// policy must never be read in Go's map order. Weights are integers, and the score is their
// exact sum: there is no floating-point arithmetic anywhere in ranking, so no result's position
// depends on rounding.
func SearchSignalWeight(signal string) int {
	switch signal {
	case SignalIDExact:
		return 1024
	case SignalTitleExact:
		return 512
	case SignalTitlePrefix:
		return 256
	case SignalTitleSubstring:
		return 128
	case SignalPhraseMatch:
		return 64
	case SignalTitleAllTerms:
		return 32
	case SignalTitleTerms:
		return 16
	case SignalIDSubstring:
		return 8
	case SignalFieldMatch:
		return 4
	}
	return 0
}

// SearchRelevanceScore is the sum of the weights of the signals a hit carries.
//
// It is the only way a score is ever produced, so the score and the explanation cannot disagree:
// there is no path by which a result acquires relevance the signal list does not account for.
// An unknown signal contributes nothing rather than a default, so a score can never be inflated
// by a name this build does not define.
func SearchRelevanceScore(signals []string) int {
	total := 0
	for _, signal := range signals {
		total += SearchSignalWeight(signal)
	}
	return total
}

// SearchSignalWeightsAreLexicographic reports whether every signal's weight exceeds the sum of
// all weaker weights, which is the property that makes the summed score behave as a precedence
// order rather than as an accumulation. It exists so the invariant is checkable rather than
// merely asserted in a comment, and so a future weight change is caught by a test instead of by
// a reordered result set nobody expected.
func SearchSignalWeightsAreLexicographic() bool {
	remaining := 0
	for i := len(SearchMatchSignals) - 1; i >= 0; i-- {
		weight := SearchSignalWeight(SearchMatchSignals[i])
		if weight <= 0 || weight <= remaining {
			return false
		}
		remaining += weight
	}
	return true
}
