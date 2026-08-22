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

// SearchQuery is a bounded, deterministic cross-layer discovery request.
//
// Q is required. Type is an optional filter naming exactly one searchable class and is
// rejected when outside domain.SearchEntityTypes, so a caller cannot filter by a class the
// contract does not define and read the empty result as "nothing of that kind matched". A
// single type is accepted rather than a list, because every other filter on every other
// AudioMuse endpoint is a single exact value and a comma-separated set here would be a second
// query-string convention.
type SearchQuery struct {
	Q      string
	Type   string
	Limit  int
	Offset int
}

// SearchResults is the discovery projection.
//
// Query echoes the normalised term that was actually searched — trimmed, lower-cased and
// bounded — rather than the caller's raw input, so a response describes the search that ran.
// Type echoes the class filter for the same reason: two cached responses to different requests
// must not be confusable.
type SearchResults struct {
	Query   string                `json:"query"`
	Type    string                `json:"type,omitempty"`
	Page    Page                  `json:"page"`
	Results []domain.SearchResult `json:"results"`
}

// searchField is one named, searchable, lower-cased projection of a canonical field.
//
// Name is the canonical field name from the record's own schema and is what a matched-field
// list reports. List-valued fields are joined with a newline, which no single canonical value
// contains, so a query can never match by spanning two unrelated list entries.
type searchField struct {
	name  string
	value string
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

// lexicalField builds one searchable field from one or more canonical values.
func lexicalField(name string, values ...string) searchField {
	return searchField{name: name, value: strings.ToLower(strings.Join(values, "\n"))}
}

// corpus renders the document as the single joined haystack the per-layer list endpoints
// search. It is the reason those endpoints and this one cannot drift: both read the same field
// set from the same builder, so a field added to discovery is added to the layer list too, and
// neither can quietly acquire a field the other does not have.
func (d searchDocument) corpus() string {
	parts := make([]string, 0, len(d.fields))
	for _, field := range d.fields {
		parts = append(parts, field.value)
	}
	return strings.Join(parts, "\n")
}

// matchedFields returns the canonical names of every field the needle appears in, in the
// record's own field order. The slice is built per call, so a caller cannot reach the index
// through a returned result.
func (d searchDocument) matchedFields(needle string) []string {
	var out []string
	for _, field := range d.fields {
		if strings.Contains(field.value, needle) {
			out = append(out, field.name)
		}
	}
	return out
}

// classify assigns the categorical precedence class of a hit. See the match-kind block in
// domain: these are four mutually exclusive ways a query matched, not a score.
func (d searchDocument) classify(needle string, matched []string) (string, int) {
	switch {
	case d.idLower == needle:
		return domain.MatchIDExact, 0
	case d.titleLower == needle:
		return domain.MatchTitleExact, 1
	case contains(matched, d.titleField):
		return domain.MatchTitleSubstring, 2
	default:
		return domain.MatchFieldSubstring, 3
	}
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
				lexicalField("id", session.ID),
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
				lexicalField("id", node.ID),
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
				lexicalField("id", claim.ID),
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
			lexicalField("id", source.ID),
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
				lexicalField("id", entry.ID),
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
				lexicalField("id", experiment.ID),
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

	// The per-layer corpora are derived from the same documents rather than assembled a second
	// time. ListSessions is the one exception: its haystack is the session ID and title, which
	// it composes inline from the two fields it already holds.
	for _, doc := range docs {
		joined := doc.corpus()
		switch doc.entityType {
		case domain.SearchNode:
			k.searchText[doc.id] = joined
		case domain.SearchClaim:
			k.claimSearchText[doc.id] = joined
		case domain.SearchSource:
			k.sourceSearchText[doc.id] = joined
		case domain.SearchVocabulary:
			k.vocabularySearchText[doc.id] = joined
		case domain.SearchExperiment:
			k.experimentSearchText[doc.id] = joined
		}
	}
}

// Search runs one bounded, deterministic cross-layer lexical query.
//
// The query is plain text and is treated as nothing else. It is never compiled as a regular
// expression, expanded as a glob, joined to a filesystem path, or handed to any interpreter;
// the only operation performed on it is a case-insensitive substring test against in-memory
// strings, so no query can reach the operator's disk or change how the search is evaluated.
//
// Ordering is explicit and total: match-kind precedence, then the canonical class order, then
// canonical ID. Nothing is left to Go map iteration or to the order the filesystem returned
// records in, so the same corpus and the same query always produce the same bytes.
func (k *Knowledge) Search(q SearchQuery) (SearchResults, error) {
	if q.Type != "" && !domain.ValidSearchEntityType(q.Type) {
		return SearchResults{}, &InvalidFilterError{Param: "type", Allowed: domain.SearchEntityTypeNames()}
	}

	// boundedNeedle is the same normalisation every other AudioMuse list applies, so a term
	// that finds a record through /api/v1/nodes finds it through /api/v1/search too. The HTTP
	// layer refuses an over-long q outright; the truncation here is the service-level bound a
	// direct Go caller inherits.
	needle := boundedNeedle(q.Q)
	if needle == "" {
		return SearchResults{}, ErrEmptySearchQuery
	}
	limit, offset := normalisePaging(q.Limit, q.Offset)

	type ranked struct {
		rank   int
		result domain.SearchResult
	}
	matched := make([]ranked, 0, len(k.searchDocs))
	for _, doc := range k.searchDocs {
		if q.Type != "" && string(doc.entityType) != q.Type {
			continue
		}
		fields := doc.matchedFields(needle)
		if len(fields) == 0 {
			continue
		}
		kind, rank := doc.classify(needle, fields)
		matched = append(matched, ranked{rank: rank, result: domain.SearchResult{
			EntityType:    doc.entityType,
			ID:            doc.id,
			Title:         doc.title,
			Summary:       doc.summary,
			MatchKind:     kind,
			MatchedFields: fields,
		}})
	}

	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].rank != matched[j].rank {
			return matched[i].rank < matched[j].rank
		}
		ri, rj := matched[i].result, matched[j].result
		if ri.EntityType != rj.EntityType {
			return domain.SearchEntityRank(ri.EntityType) < domain.SearchEntityRank(rj.EntityType)
		}
		return ri.ID < rj.ID
	})

	results := make([]domain.SearchResult, 0, len(matched))
	for _, m := range matched {
		results = append(results, m.result)
	}

	page, meta := paginate(results, limit, offset)
	return SearchResults{Query: needle, Type: q.Type, Page: meta, Results: page}, nil
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
