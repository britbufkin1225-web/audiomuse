package service

import (
	"errors"
	"sort"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Phase 1E: the cross-layer lexical discovery projection.
//
// Every layer before this one answers "list the records of class X that match". This file
// answers "find anything in the corpus that mentions this", which is the question a reader has
// before they know which layer holds the answer.
//
// Why lexical substring matching first, and nothing cleverer? Because a deterministic baseline
// is testable and a semantic one is not, yet. Case-insensitive substring matching over named
// canonical fields produces the same bytes for the same corpus and the same query on every
// run, which means the contract can be pinned by tests before any ranking model, embedding or
// retrieval system is introduced on top of it. There is no stemming, no fuzzy matching, no
// scoring, no synonyms and no query rewriting here, and adding any of them is a later phase
// with its own contract rather than a quiet improvement to this one.
//
// The projection is built once, in New, from records the earlier phases already parsed. No
// request touches the filesystem, reparses a record, or writes to the index.

// ErrEmptySearchQuery reports a search request carrying no searchable term.
//
// Search requires a query rather than defaulting to "everything". Every layer already has its
// own list endpoint, so an empty search would be a second, slower way to serialise the whole
// corpus, and a caller who mistyped their parameter name would receive a full result set that
// looked like a successful search.
var ErrEmptySearchQuery = errors.New("search query must not be empty")

// ErrSearchQueryTooLong reports a direct service request beyond the same ceiling enforced by
// HTTP. Refusing it prevents a valid prefix plus an ignored suffix from silently becoming a
// different query.
var ErrSearchQueryTooLong = errors.New("search query exceeds maximum length")

// Multi-term bounds. They are deliberately narrow, and they are enforced in the service rather
// than only at the HTTP edge so a future internal caller inherits the same contract.
const (
	// MinSearchTerms is the floor for a composed query. One term is refused rather than run,
	// because a single-term all_terms search is literal search under a second name: the two
	// would be indistinguishable in behaviour and distinguishable in response shape, which is
	// exactly the kind of redundant spelling that makes an API contract ambiguous. A caller who
	// wants one term already has a mode for it.
	MinSearchTerms = 2
	// MaxSearchTerms is the ceiling. Each additional term is another full pass over the
	// projection, and a query composed of dozens of terms is not a question a reader asks; it
	// is a way to make one request cost more. The ceiling is a small fixed number rather than a
	// budget, so the cost of a request is knowable from the contract.
	MaxSearchTerms = 8
)

// ErrTooFewSearchTerms reports an all_terms query that normalised to fewer than MinSearchTerms
// distinct terms. It is refused rather than quietly demoted to literal search: the mode was
// named explicitly, so it should mean what it says.
var ErrTooFewSearchTerms = errors.New("all_terms search requires at least two distinct terms")

// ErrTooManySearchTerms reports an all_terms query past MaxSearchTerms distinct terms.
var ErrTooManySearchTerms = errors.New("all_terms search accepts at most eight distinct terms")

// Scope errors. A malformed class list is refused rather than repaired, for the reason every
// other malformed filter on this API is: a search whose scope silently differed from the one
// the caller wrote would return a result set — and now a facet breakdown — that does not mean
// what they think it means, and unlike a bad query string that is invisible in the response.
var (
	// ErrEmptySearchEntityType reports a class list carrying a blank member, which is what a
	// leading, trailing or doubled separator produces. It is refused rather than skipped: a
	// caller who wrote one comma too many has a bug, and answering it with a working search
	// hides the bug behind a correct-looking response.
	ErrEmptySearchEntityType = errors.New("entity type filter must not contain an empty value")

	// ErrDuplicateSearchEntityType reports a class named twice. A repeated class cannot change
	// which records match, so accepting it would be harmless and refusing it is still right:
	// the two spellings a caller might mean — a set and a multiset — differ, and a filter that
	// quietly collapses one into the other is a filter whose contract is guessed at.
	ErrDuplicateSearchEntityType = errors.New("entity type filter must not repeat a value")

	// ErrConflictingSearchScope reports the single-class and multi-class filters supplied
	// together. They are two spellings of one restriction, and there is no reading of both at
	// once that is not a guess: intersecting them can produce an empty set that looks like "the
	// corpus holds nothing of that kind", and preferring either one would silently discard a
	// filter the caller believes is applied. Refusing the combination also changes nothing for
	// an existing client, because a request naming both was already refused as an unknown
	// parameter before the class list existed.
	ErrConflictingSearchScope = errors.New("search accepts either a single type or an entity type list, not both")
)

// SearchQuery is a bounded, deterministic cross-layer discovery request.
//
// Q is required. Type is an optional filter naming exactly one searchable class and is
// rejected when outside domain.SearchEntityTypes, so a caller cannot filter by a class the
// contract does not define and read the empty result as "nothing of that kind matched".
//
// EntityTypes is the Phase 1H filter and names a set of classes. It is a separate field rather
// than a widening of Type because Type is a single exact value, as every other filter on every
// other AudioMuse endpoint is: re-reading it as a list would give one parameter two meanings and
// would silently turn a request that is refused today — type naming two classes — into one that
// succeeds, which is a change to an existing contract rather than an addition to it. The two are
// alternatives, not layers: supplying both is refused. See ErrConflictingSearchScope.
type SearchQuery struct {
	Q      string
	Type   string
	Limit  int
	Offset int

	// EntityTypes restricts results to a set of searchable classes, and is the multi-class
	// spelling Type deliberately is not. An empty slice means every searchable class, so the
	// zero SearchQuery is the unfiltered request it has always been. Each member is trimmed and
	// must name a class in domain.SearchEntityTypes exactly; a blank member, an unknown class,
	// experiment_run, or a class named twice is refused rather than dropped. It may not be
	// combined with Type. The list needs no length bound of its own: the set is closed at six
	// and repetition is refused, so a valid list cannot be longer than the model.
	EntityTypes []string

	// Mode selects how Q is composed into a query. An empty value is domain.SearchModeLiteral,
	// so the zero SearchQuery is the Phase 1E request it has always been and no existing caller
	// changes behaviour by recompiling. Any value outside domain.SearchQueryModes is refused.
	Mode string

	// IncludeContext asks the resolver to attach the bounded canonical context of every result
	// on the returned page. It is opt-in rather than the default so that the compact Phase 1E
	// response stays exactly what it was: a caller who only needs to know where a term appears
	// should not be made to receive, parse and discard the corpus's reference structure. It
	// changes what a result carries and never which results match or in what order; see
	// resolveContext.
	IncludeContext bool
}

// SearchResults is the discovery projection.
//
// Query echoes the normalised term that was actually searched — trimmed, lower-cased and
// bounded — rather than the caller's raw input, so a response describes the search that ran.
// Type echoes the class filter for the same reason: two cached responses to different requests
// must not be confusable.
type SearchResults struct {
	Query string `json:"query"`

	// QueryMode echoes the composition that ran, and is present only when it was not the
	// default. A literal search — whether the mode was omitted or spelled out — serialises no
	// query_mode key at all, so a Phase 1E/1F client sees the response it always saw and an
	// explicit literal request is not a third, subtly different shape. When it is present, the
	// meaning of Query changes with it: under all_terms, Query is the normalised term list
	// joined by single spaces rather than a phrase that was searched for contiguously, and the
	// mode is what tells the two apart.
	QueryMode string `json:"query_mode,omitempty"`

	Type string `json:"type,omitempty"`

	// EntityTypes echoes the class list that was applied, in the canonical class order rather
	// than the caller's, so — like Query — the response describes the search that ran. It is
	// present only when the caller supplied one: an unfiltered search serialises no key, and a
	// single-class Type search echoes Type as it always did rather than acquiring a second,
	// competing spelling of the same restriction. The slice is built per response.
	EntityTypes []string `json:"entity_types,omitempty"`

	// IncludeContext echoes the context control for the same reason Type echoes the class
	// filter. A query whose every hit happens to reference nothing would otherwise be
	// indistinguishable from one that never asked for context. It is omitted when false, so a
	// response to a plain Phase 1E request is unchanged byte for byte.
	IncludeContext bool `json:"include_context,omitempty"`

	Page Page `json:"page"`

	// Facets is the class composition of the complete filtered result set, counted before
	// paging. It is always present, including on a search that matched nothing, because a
	// zero-valued breakdown is an answer — "nothing of any kind matched" — and a caller should
	// not have to tell an empty facet object from an absent one. It is additive: no existing
	// key changed to make room for it. See domain.SearchFacets.
	Facets domain.SearchFacets `json:"facets"`

	Results []domain.SearchResult `json:"results"`
}

// searchField is one named, searchable, lower-cased projection of a canonical field.
//
// Name is the canonical field name from the record's own schema and is what a matched-field
// list reports. List-valued fields retain separate values, so a query cannot match by spanning
// two canonical entries.
type searchField struct {
	name   string
	values []string
}

// searchDocument is one canonical record as the discovery layer sees it.
//
// It holds display values and searchable values separately: title and summary are copied
// verbatim from the record for rendering, while fields carry the lower-cased text matching
// runs against. Nothing outside the record's own front matter or registry entry contributes,
// so a hit is always explainable by pointing at one field of one record.
type searchDocument struct {
	entityType domain.SearchEntityType
	id         string
	idLower    string
	title      string
	titleLower string
	// titleField is the canonical field name the display title was taken from. It differs by
	// class — term for a vocabulary entry, statement for a claim — which is exactly why the
	// matched-field list reports canonical names rather than a synthetic "title".
	titleField string
	summary    string
	fields     []searchField
}

// lexicalField builds one searchable field from one or more canonical values. Values remain
// separate so a query cannot manufacture a match by spanning a separator between list items.
func lexicalField(name string, values ...string) searchField {
	lowered := make([]string, len(values))
	for i, value := range values {
		lowered[i] = strings.ToLower(value)
	}
	return searchField{name: name, values: lowered}
}

// matchedFields returns the canonical names of every field the needle appears in, in the
// record's own field order. The slice is built per call, so a caller cannot reach the index
// through a returned result.
func (d searchDocument) matchedFields(needle string) []string {
	var out []string
	for _, field := range d.fields {
		if field.matches(needle) {
			out = append(out, field.name)
		}
	}
	return out
}

// unionMatchedFields returns every canonical field any term matched, in the record's own field
// order and without repetition.
//
// A composed hit still reports MatchedFields, and it reports the union rather than the fields
// of some chosen term: the question that field answers — "which of this record's fields did the
// query reach" — has the same meaning in both modes, and a client that reads it without knowing
// about term_matches gets a true answer rather than a partial one. The per-term breakdown lives
// in TermMatches, which is where the composition is actually explained. Like matchedFields, the
// slice is built per call, so nothing here is shared with the index.
func (d searchDocument) unionMatchedFields(termMatches []domain.SearchTermMatch) []string {
	var out []string
	for _, field := range d.fields {
		for _, match := range termMatches {
			if contains(match.MatchedFields, field.name) {
				out = append(out, field.name)
				break
			}
		}
	}
	return out
}

func (f searchField) matches(needle string) bool {
	for _, value := range f.values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func searchFieldsMatch(fields []searchField, needle string) bool {
	for _, field := range fields {
		if field.matches(needle) {
			return true
		}
	}
	return false
}

// searchIDField is the canonical field name every search document projects its ID under. The
// ranking policy asks questions about that field by name, so the name is written once here and
// read by both the document builder and the signal derivation rather than spelled independently
// in each; a drift between the two would silently stop the ID from being a high-priority field.
const searchIDField = "id"

// signals derives the ordered ranking signals one hit fired. It is the Phase 1I ranking policy,
// and it is the only place a relevance score can originate.
//
// phrase is the whole normalised query: the literal needle, or — for a composed search — the
// distinct terms joined by single spaces, which is exactly the query the response echoes. terms
// is nil for a literal search and the normalised term list for a composed one, and that is the
// only difference between the two modes' ranking: both ask the same questions of the same
// fields, and a composed query can additionally be asked how much of it one field carried.
//
// The signals are appended in domain.SearchMatchSignals order — descending weight — so the list
// needs no sort and cannot acquire one that depends on anything but the policy. Within each tier
// the branches are mutually exclusive, so no signal is emitted twice and no hit is described
// both as an exact name match and as a substring of the same name.
//
// Every question here is answered from this record's own fields. Nothing consults another
// record, the size of the corpus, the number of hits, the request, or anything outside the two
// arguments and the document, so a record's score is a function of the query and that record
// alone and cannot change because a different record happened to also match.
func (d searchDocument) signals(phrase string, terms []string, matched []string) []string {
	signals := make([]string, 0, len(domain.SearchMatchSignals))

	// Identity, strongest tier. A caller who typed a canonical ID typed an address, not a
	// description, and the record at that address is the answer.
	idExact := d.idLower == phrase
	if idExact {
		signals = append(signals, domain.SignalIDExact)
	}

	// The display field, second tier: exact, then prefix, then anywhere inside. The three are
	// mutually exclusive by construction, and each is a statement about where the whole query
	// sits in the name rather than about how much of the name it covers.
	//
	// All three test the phrase against the display value itself rather than against the matched
	// field list. For a literal search the two are the same question, because the phrase is the
	// needle the list was built from. For a composed search they are not: the list is the union
	// over the terms, so reading it here would report "the whole query is inside this name"
	// whenever any single term touched it — which is what the coverage tier below says, said
	// once, accurately.
	switch {
	case d.titleLower == phrase:
		signals = append(signals, domain.SignalTitleExact)
	case strings.HasPrefix(d.titleLower, phrase):
		signals = append(signals, domain.SignalTitlePrefix)
	case strings.Contains(d.titleLower, phrase):
		signals = append(signals, domain.SignalTitleSubstring)
	}

	if len(terms) > 0 {
		// The composed-only tiers. phrase_match asks whether the record carries the caller's
		// words together, in order, somewhere other than its display field — the display field
		// is already fully described by the tier above, so asking about it again here would add
		// weight for one fact stated twice. A composed query whose words appear side by side in
		// a definition is a stronger hit than one whose words are scattered over five fields,
		// and this is the signal that says so.
		if d.phraseMatchesOutsideTitle(phrase) {
			signals = append(signals, domain.SignalPhraseMatch)
		}
		// Term coverage of the display field: the second axis, and a different question from
		// the tier above. "Every one of your words is in this record's name" can be true of a
		// record that never carries them contiguously.
		switch covered := d.titleTermCoverage(terms); {
		case covered == len(terms):
			signals = append(signals, domain.SignalTitleAllTerms)
		case covered > 0:
			signals = append(signals, domain.SignalTitleTerms)
		}
	}

	// Text inside the canonical ID, below every statement about the name and above prose.
	// Suppressed when the ID is the whole query, which the strongest signal already said.
	if !idExact && contains(matched, searchIDField) {
		signals = append(signals, domain.SignalIDSubstring)
	}

	// The floor: the query reached a field that is neither identity nor name.
	for _, field := range matched {
		if field != searchIDField && field != d.titleField {
			signals = append(signals, domain.SignalFieldMatch)
			break
		}
	}
	return signals
}

// phraseMatchesOutsideTitle reports whether the complete normalised query occurs contiguously in
// some searchable field that is not the display field. The display field is excluded because the
// signal tier above already describes how the whole query sits in the name, and a score is only
// interpretable if each signal contributes evidence no other signal already contributed.
func (d searchDocument) phraseMatchesOutsideTitle(phrase string) bool {
	for _, field := range d.fields {
		if field.name == d.titleField {
			continue
		}
		if field.matches(phrase) {
			return true
		}
	}
	return false
}

// titleTermCoverage counts how many of a composed query's terms occur in the display field.
//
// It counts distinct terms, never occurrences: normaliseTerms has already collapsed repeats, so
// a caller cannot raise a record's score by typing one of its words twice, and a record cannot
// raise its own by repeating a word in its title. That is the line this phase keeps — coverage
// of the query is a property of the query, and frequency in the corpus is not something this
// layer measures.
func (d searchDocument) titleTermCoverage(terms []string) int {
	covered := 0
	for _, term := range terms {
		for _, field := range d.fields {
			if field.name == d.titleField && field.matches(term) {
				covered++
				break
			}
		}
	}
	return covered
}

// matchKindFor is the categorical precedence class of a hit, derived from the signals it fired
// rather than recomputed from the query.
//
// It used to be a second, independent classification of the same four cases, which meant the
// coarse class a client reads and the score it is ordered by were two answers to one question
// and could drift apart under any later edit. Deriving one from the other makes that impossible:
// a result's match_kind is now a summary of its own match_signals, so the two cannot disagree
// about whether the query was this record's name.
//
// The mapping reproduces the Phase 1E classes exactly. A prefix hit and an interior hit are both
// title_substring, because they were before and because the class is deliberately coarse; the
// distinction between them lives in the signals and in the score, which is where Phase 1I added
// it. See the match-kind block in domain.
func matchKindFor(signals []string) string {
	switch {
	case contains(signals, domain.SignalIDExact):
		return domain.MatchIDExact
	case contains(signals, domain.SignalTitleExact):
		return domain.MatchTitleExact
	case contains(signals, domain.SignalTitlePrefix), contains(signals, domain.SignalTitleSubstring):
		return domain.MatchTitleSubstring
	default:
		return domain.MatchFieldSubstring
	}
}

// lessSearchResult is the total order every search result set is sorted by, in both modes.
//
// Relevance first, descending. Then the canonical class order, and then the canonical ID
// ascending — the Phase 1E tie-break unchanged, now applied to score ties rather than to
// match-kind ties. The last key is unique within a class, so the order is total: no pair of
// distinct results compares equal, which is what makes repeated runs and separately built
// indexes produce the same bytes rather than merely the same set.
//
// It is one function used by both modes rather than a closure written twice, so literal and
// composed search cannot drift into disagreeing about how a tie is broken.
func lessSearchResult(a, b domain.SearchResult) bool {
	if a.RelevanceScore != b.RelevanceScore {
		return a.RelevanceScore > b.RelevanceScore
	}
	if a.EntityType != b.EntityType {
		return domain.SearchEntityRank(a.EntityType) < domain.SearchEntityRank(b.EntityType)
	}
	return a.ID < b.ID
}

// buildSearch constructs the discovery projection and, from the same field definitions, every
// per-layer lexical search corpus.
//
// It runs once, inside New, so the slice and the maps are never written to again and the
// immutability that makes Knowledge safe for concurrent readers still holds.
//
// The searchable field set of each class is defined here and only here. It is the Phase
// 1A-1D set unchanged, so Phase 1E adds a new way to reach existing search semantics rather
// than a second, subtly different implementation of them:
//
//	session     id, title
//	node        id, title, domain, status, definition, core_concepts
//	claim       id, statement
//	source      id, title, author
//	vocabulary  id, term, domain, definition, digital_relationship, best_use, technologies, tags
//	experiment  id, title, status, type, difficulty, purpose
//
// What is excluded is excluded for a reason, and each exclusion is the earlier phase's: source
// notes are prose about retrieval and external locators; the cross-reference lists on a node,
// an entry or a definition are another record's identity; an experiment's procedure and setup
// describe what a performer should do, so a hit there would return a definition that mentions a
// term in an instruction rather than one that is about it. Experiment runs have no document at
// all — see domain.SearchEntityTypes.
func (k *Knowledge) buildSearch() {
	docs := make([]searchDocument, 0,
		len(k.sessions)+len(k.nodes)+len(k.claims)+len(k.sources)+len(k.vocabulary)+len(k.experiments))

	// Documents are appended in the canonical class order and, within a class, in the canonical
	// record order the loader produced. Ordering is still applied explicitly at query time; this
	// only means the input to that sort is itself reproducible.
	for _, session := range k.sessions {
		docs = append(docs, searchDocument{
			entityType: domain.SearchSession,
			id:         session.ID,
			title:      session.Title,
			titleField: "title",
			fields: []searchField{
				lexicalField(searchIDField, session.ID),
				lexicalField("title", session.Title),
			},
		})
	}

	for _, node := range k.nodes {
		docs = append(docs, searchDocument{
			entityType: domain.SearchNode,
			id:         node.ID,
			title:      node.Title,
			titleField: "title",
			summary:    node.Definition,
			fields: []searchField{
				lexicalField(searchIDField, node.ID),
				lexicalField("title", node.Title),
				lexicalField("domain", node.Domain),
				lexicalField("status", node.Status),
				lexicalField("definition", node.Definition),
				lexicalField("core_concepts", node.CoreConcepts...),
			},
		})
	}

	// A claim's display field is its statement, which is also its only prose field. It is not
	// repeated as a summary: a claim is its statement, and duplicating it would suggest the
	// record carries a separate abstract of itself.
	for _, claim := range k.claims {
		docs = append(docs, searchDocument{
			entityType: domain.SearchClaim,
			id:         claim.ID,
			title:      claim.Statement,
			titleField: "statement",
			fields: []searchField{
				lexicalField(searchIDField, claim.ID),
				lexicalField("statement", claim.Statement),
			},
		})
	}

	// Author is optional on a registry entry. An absent author contributes no field at all
	// rather than an empty one, so a matched-field list can never name a field the record does
	// not have. A registry entry has no prose summary field, so a source hit is deliberately a
	// smaller result than a node hit.
	for _, source := range k.sources {
		fields := []searchField{
			lexicalField(searchIDField, source.ID),
			lexicalField("title", source.Title),
		}
		if source.Author != nil {
			fields = append(fields, lexicalField("author", *source.Author))
		}
		docs = append(docs, searchDocument{
			entityType: domain.SearchSource,
			id:         source.ID,
			title:      source.Title,
			titleField: "title",
			fields:     fields,
		})
	}

	for _, entry := range k.vocabulary {
		docs = append(docs, searchDocument{
			entityType: domain.SearchVocabulary,
			id:         entry.ID,
			title:      entry.Term,
			titleField: "term",
			summary:    entry.Definition,
			fields: []searchField{
				lexicalField(searchIDField, entry.ID),
				lexicalField("term", entry.Term),
				lexicalField("domain", entry.Domain),
				lexicalField("definition", entry.Definition),
				lexicalField("digital_relationship", entry.DigitalRelationship),
				lexicalField("best_use", entry.BestUse),
				lexicalField("technologies", entry.Technologies...),
				lexicalField("tags", entry.Tags...),
			},
		})
	}

	for _, experiment := range k.experiments {
		docs = append(docs, searchDocument{
			entityType: domain.SearchExperiment,
			id:         experiment.ID,
			title:      experiment.Title,
			titleField: "title",
			summary:    experiment.Purpose,
			fields: []searchField{
				lexicalField(searchIDField, experiment.ID),
				lexicalField("title", experiment.Title),
				lexicalField("status", experiment.Status),
				lexicalField("type", experiment.Type),
				lexicalField("difficulty", experiment.Difficulty),
				lexicalField("purpose", experiment.Purpose),
			},
		})
	}

	for i := range docs {
		docs[i].idLower = strings.ToLower(docs[i].id)
		docs[i].titleLower = strings.ToLower(docs[i].title)
	}
	k.searchDocs = docs

	// The per-layer field projections are derived from the same documents rather than assembled
	// a second time. ListSessions is the one exception: it tests its ID and title directly.
	for _, doc := range docs {
		switch doc.entityType {
		case domain.SearchNode:
			k.searchText[doc.id] = doc.fields
		case domain.SearchClaim:
			k.claimSearchText[doc.id] = doc.fields
		case domain.SearchSource:
			k.sourceSearchText[doc.id] = doc.fields
		case domain.SearchVocabulary:
			k.vocabularySearchText[doc.id] = doc.fields
		case domain.SearchExperiment:
			k.experimentSearchText[doc.id] = doc.fields
		}
	}
}

// searchScope is the resolved set of classes one search may return. An empty scope is every
// searchable class, which is what an unfiltered request has always meant.
//
// It is one type for both spellings of the filter — the single Type and the EntityTypes list —
// so the matching loops carry one admission test rather than two, and the two spellings cannot
// drift into disagreeing about what "restricted to this class" means.
type searchScope []domain.SearchEntityType

// has is exact membership. It is separate from admits because an empty scope admits everything
// and contains nothing, and the duplicate check needs the second question, not the first.
func (s searchScope) has(t domain.SearchEntityType) bool {
	for _, allowed := range s {
		if allowed == t {
			return true
		}
	}
	return false
}

// admits reports whether a class may appear in the result set.
func (s searchScope) admits(t domain.SearchEntityType) bool {
	return len(s) == 0 || s.has(t)
}

// names renders the scope for the response echo, in canonical class order. The slice is fresh
// on every call, so an echoed scope can never alias anything the index holds.
func (s searchScope) names() []string {
	out := make([]string, 0, len(s))
	for _, t := range s {
		out = append(out, string(t))
	}
	return out
}

// resolveSearchScope validates the two class filters and returns the classes a search may
// return, or nil for every class.
//
// It is a pure function of the request and runs before any matching, so a malformed scope costs
// one pass over at most a handful of short strings rather than a pass over the projection.
// Members are trimmed and then compared exactly, which is the comparison every canonical filter
// on this API already uses: "Node" and "NODE" are refused rather than folded, because a filter
// that guesses at a caller's spelling is a filter that can guess wrong and answer a different
// question. Nothing else is normalised — there is no alias table and no plural form, because
// the API has no alias convention to follow and inventing one here would make the class list
// the only place a canonical name is not written as the model spells it.
//
// The returned scope is in canonical class order rather than the caller's, so two spellings of
// one scope produce one response.
func resolveSearchScope(single string, requested []string) (searchScope, error) {
	if single != "" && len(requested) > 0 {
		return nil, ErrConflictingSearchScope
	}
	if single != "" {
		// Type is validated by the caller against the same closed set, so by here it names a
		// class. One class is a scope of one.
		return searchScope{domain.SearchEntityType(single)}, nil
	}
	if len(requested) == 0 {
		return nil, nil
	}
	scope := make(searchScope, 0, len(requested))
	for _, raw := range requested {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, ErrEmptySearchEntityType
		}
		if !domain.ValidSearchEntityType(value) {
			// experiment_run lands here with every other unknown class, which is the point:
			// discovery has no run class, and a scope that accepted the name would answer with
			// an empty set a caller could read as "no run mentions this".
			return nil, &InvalidFilterError{Param: "entity_types", Allowed: domain.SearchEntityTypeNames()}
		}
		class := domain.SearchEntityType(value)
		if scope.has(class) {
			return nil, ErrDuplicateSearchEntityType
		}
		scope = append(scope, class)
	}
	sort.SliceStable(scope, func(i, j int) bool {
		return domain.SearchEntityRank(scope[i]) < domain.SearchEntityRank(scope[j])
	})
	return scope, nil
}

// Search runs one bounded, deterministic cross-layer lexical query.
//
// The query is plain text and is treated as nothing else. It is never compiled as a regular
// expression, expanded as a glob, joined to a filesystem path, or handed to any interpreter;
// the only operation performed on it is a case-insensitive substring test against in-memory
// strings, so no query can reach the operator's disk or change how the search is evaluated.
//
// Ordering is explicit and total in both modes. Results are ordered by relevance score, then the
// canonical class order, then canonical ID. Nothing is left to Go map iteration or to the order
// the filesystem returned records in, so the same corpus and the same query always produce the
// same bytes.
//
// Composition is decided before matching and never after: the whole match set is built, ordered
// and only then paged, so a page boundary can never hide a record that satisfied the query.
//
// The order of operations is fixed and each step is separable: validate the request, match,
// restrict to the requested classes, order the complete set, count the facets, page, and only
// then resolve context. Scope restriction happens inside the match loop and changes nothing
// about matching itself — the same fields, the same substring test, the same match evidence,
// the same relative order — so a filtered result set is exactly the unfiltered one with other
// classes removed. Facets are counted from the ordered set before paging, which is what makes
// them stable across every page of one search.
func (k *Knowledge) Search(q SearchQuery) (SearchResults, error) {
	if q.Type != "" && !domain.ValidSearchEntityType(q.Type) {
		return SearchResults{}, &InvalidFilterError{Param: "type", Allowed: domain.SearchEntityTypeNames()}
	}

	// Scope is resolved before the query text is even looked at. It is the cheapest validation
	// on the request and the one most likely to be wrong in a hand-written query string, and
	// refusing it here means a malformed class list never reaches the projection.
	scope, err := resolveSearchScope(q.Type, q.EntityTypes)
	if err != nil {
		return SearchResults{}, err
	}

	// An unset mode is the Phase 1E default rather than an error, so the zero SearchQuery keeps
	// its meaning. Anything else must name a mode exactly.
	mode := domain.SearchQueryMode(q.Mode)
	if q.Mode == "" {
		mode = domain.SearchModeLiteral
	} else if !domain.ValidSearchQueryMode(q.Mode) {
		return SearchResults{}, &InvalidFilterError{Param: "query_mode", Allowed: domain.SearchQueryModeNames()}
	}

	// Apply the same strict ceiling as HTTP so direct callers cannot have a suffix silently
	// discarded and receive the results of a different query. The ceiling is on the whole query
	// in both modes: it bounds the text, and the term bounds below bound the composition.
	trimmed := strings.TrimSpace(q.Q)
	if len(trimmed) > MaxQueryChars {
		return SearchResults{}, ErrSearchQueryTooLong
	}
	needle := strings.ToLower(trimmed)
	if needle == "" {
		return SearchResults{}, ErrEmptySearchQuery
	}
	limit, offset := normalisePaging(q.Limit, q.Offset)

	var (
		echo     string
		results  []domain.SearchResult
		echoMode string
	)
	switch mode {
	case domain.SearchModeAllTerms:
		terms, err := normaliseTerms(needle)
		if err != nil {
			return SearchResults{}, err
		}
		// The echo is the normalised term list rather than the caller's spacing, so a response
		// describes the query that ran: repeated terms and runs of whitespace are already gone.
		echo = strings.Join(terms, " ")
		echoMode = string(domain.SearchModeAllTerms)
		results = k.searchAllTerms(scope, terms)
	default:
		echo = needle
		results = k.searchLiteral(scope, needle)
	}

	// Facets count the complete, ordered, scope-filtered match set, which is why they are taken
	// here and not after paging: they describe what the query found, and a breakdown that moved
	// with limit and offset would describe the page instead — the one thing the caller can
	// already see. The sum of the counts is therefore page.total, on every page of one search.
	facets := domain.NewSearchFacets(results)

	// The echo names the class list only when the caller supplied one. A single-class Type
	// search still echoes Type alone rather than acquiring a second spelling of its own filter.
	var echoTypes []string
	if len(q.EntityTypes) > 0 {
		echoTypes = scope.names()
	}

	// Context is resolved after ordering and paging, and only for the results actually being
	// returned. Resolving it before the page would do work the caller never sees and, more
	// importantly, would put a relationship count in reach of the ordering; keeping it here
	// makes it structurally impossible for context to influence which results matched or where
	// they sorted. It is equally impossible for context to satisfy a term: matching is finished
	// before the resolver is called, and it only ever adds a Context object to a hit.
	page, meta := paginate(results, limit, offset)
	if q.IncludeContext {
		k.resolveContext(page)
	}
	return SearchResults{
		Query:          echo,
		QueryMode:      echoMode,
		Type:           q.Type,
		EntityTypes:    echoTypes,
		IncludeContext: q.IncludeContext,
		Page:           meta,
		Facets:         facets,
		Results:        page,
	}, nil
}

// searchLiteral is the Phase 1E match: one contiguous needle, four categorical match classes.
//
// It is the only literal implementation. all_terms reuses the same field projection and the
// same substring test rather than carrying a second copy of them, so the two modes cannot drift
// into disagreeing about what "this record contains that text" means.
func (k *Knowledge) searchLiteral(scope searchScope, needle string) []domain.SearchResult {
	matched := make([]domain.SearchResult, 0, len(k.searchDocs))
	for _, doc := range k.searchDocs {
		if !scope.admits(doc.entityType) {
			continue
		}
		fields := doc.matchedFields(needle)
		if len(fields) == 0 {
			continue
		}
		// Signals are derived once per hit and are the source of both the score and the coarse
		// class, so the two cannot describe the same match differently. The separate rank the
		// sort used to carry is gone: the score already orders the four classes, because every
		// class-defining signal outweighs the sum of everything below it.
		signals := doc.signals(needle, nil, fields)
		matched = append(matched, domain.SearchResult{
			EntityType:     doc.entityType,
			ID:             doc.id,
			Title:          doc.title,
			Summary:        doc.summary,
			MatchKind:      matchKindFor(signals),
			RelevanceScore: domain.SearchRelevanceScore(signals),
			MatchSignals:   signals,
			MatchedFields:  fields,
		})
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return lessSearchResult(matched[i], matched[j])
	})
	return matched
}

// searchAllTerms is the Phase 1G composition: every term, somewhere in the same record.
//
// "The same record" is the whole of the new semantics and the whole of its limit. A term may be
// satisfied by any of that record's own searchable fields, so a node whose title carries one
// word and whose definition carries another is a hit; a term satisfied by a different record —
// including one this record references — is not a match, because the unit of retrieval is the
// canonical record and joining two of them would assert a relationship the corpus did not.
//
// Every accepted record remains the same categorical kind of hit, MatchAllTerms. Phase 1I also
// derives signals from where its terms and complete phrase matched, then orders the complete set
// by relevance score, canonical class and canonical ID.
func (k *Knowledge) searchAllTerms(scope searchScope, terms []string) []domain.SearchResult {
	matched := make([]domain.SearchResult, 0, len(k.searchDocs))
	for _, doc := range k.searchDocs {
		if !scope.admits(doc.entityType) {
			continue
		}
		termMatches := make([]domain.SearchTermMatch, 0, len(terms))
		for _, term := range terms {
			fields := doc.matchedFields(term)
			if len(fields) == 0 {
				// One missing term rejects the record. There is no partial hit and no
				// "matched 2 of 3": a caller asked for records containing all of these, and
				// a record containing some of them is not an answer to that question.
				termMatches = nil
				break
			}
			termMatches = append(termMatches, domain.SearchTermMatch{Term: term, MatchedFields: fields})
		}
		if len(termMatches) != len(terms) {
			continue
		}
		fields := doc.unionMatchedFields(termMatches)
		// A composed hit keeps its one categorical class — every all_terms hit satisfied every
		// term, so none of the four single-needle classes is true of it — and gains the score
		// that class could never express. phrase is the normalised term list joined by single
		// spaces, which is the query the response echoes, so a client can reproduce the phrase
		// signal from the response alone.
		signals := doc.signals(strings.Join(terms, " "), terms, fields)
		matched = append(matched, domain.SearchResult{
			EntityType:     doc.entityType,
			ID:             doc.id,
			Title:          doc.title,
			Summary:        doc.summary,
			MatchKind:      domain.MatchAllTerms,
			RelevanceScore: domain.SearchRelevanceScore(signals),
			MatchSignals:   signals,
			MatchedFields:  fields,
			TermMatches:    termMatches,
		})
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return lessSearchResult(matched[i], matched[j])
	})
	return matched
}

// normaliseTerms splits a normalised query into the distinct terms that will be required.
//
// Splitting is strings.Fields over the already trimmed and lower-cased query, and that is the
// whole of the tokenisation. There is no stemming, no accent folding, no Unicode normalisation
// pipeline and no punctuation stripping: a term keeps whatever punctuation the caller typed,
// so "room," and "room" are different terms and only the second finds a record that wrote the
// bare word. That is a documented limit of a deterministic composition rather than a defect to
// be papered over with heuristics, each of which would be an unsourced judgement about language.
//
// Duplicates collapse to their first occurrence, so a repeated term cannot change the result
// set, the ordering, the counts or the evidence — a query means the same thing however many
// times the caller typed one of its words.
func normaliseTerms(needle string) ([]string, error) {
	fields := strings.Fields(needle)
	terms := make([]string, 0, len(fields))
	for _, term := range fields {
		if !contains(terms, term) {
			terms = append(terms, term)
		}
	}
	// The bounds are applied to the distinct terms, because those are what actually execute:
	// a query of one word repeated is one term's worth of question, and refusing it is the
	// same refusal as refusing the word on its own.
	if len(terms) < MinSearchTerms {
		return nil, ErrTooFewSearchTerms
	}
	if len(terms) > MaxSearchTerms {
		return nil, ErrTooManySearchTerms
	}
	return terms, nil
}

// SearchCounts is the size of the discovery projection, broken out by searchable class.
//
// It is reported on diagnostics so an operator can confirm which layers the running process
// actually made discoverable. There is no experiment-run count: runs have no search document
// at all, and reporting a zero would read as "none were indexed this time".
type SearchCounts struct {
	Documents   int `json:"documents"`
	Sessions    int `json:"sessions"`
	Nodes       int `json:"nodes"`
	Claims      int `json:"claims"`
	Sources     int `json:"sources"`
	Vocabulary  int `json:"vocabulary"`
	Experiments int `json:"experiments"`
}

// SearchDiagnostics reports the size of the discovery projection.
func (k *Knowledge) SearchDiagnostics() SearchCounts {
	counts := SearchCounts{Documents: len(k.searchDocs)}
	for _, doc := range k.searchDocs {
		switch doc.entityType {
		case domain.SearchSession:
			counts.Sessions++
		case domain.SearchNode:
			counts.Nodes++
		case domain.SearchClaim:
			counts.Claims++
		case domain.SearchSource:
			counts.Sources++
		case domain.SearchVocabulary:
			counts.Vocabulary++
		case domain.SearchExperiment:
			counts.Experiments++
		}
	}
	return counts
}
