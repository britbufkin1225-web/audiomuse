package service_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// searchRefs renders a result set as type/id pairs.
//
// Type and ID together are the identity of a hit, not the ID alone: the fixture corpus
// registers session-01-fixture both as a session and as a registry entry, and a projection
// that reported bare IDs could not tell the two results apart.
func searchRefs(results service.SearchResults) []string {
	out := make([]string, 0, len(results.Results))
	for _, result := range results.Results {
		out = append(out, string(result.EntityType)+"/"+result.ID)
	}
	return out
}

func mustSearch(t testing.TB, k *service.Knowledge, q service.SearchQuery) service.SearchResults {
	t.Helper()
	results, err := k.Search(q)
	if err != nil {
		t.Fatalf("search %+v: %v", q, err)
	}
	return results
}

// universalNeedle is the one term every record in the fixture corpus contains, so a search for
// it reaches all six searchable classes. It is deliberately a single letter: the assertion is
// about class coverage, and a term chosen for its meaning would only cover the classes whose
// authors happened to use it.
const universalNeedle = "e"

// TestSearchSpansEveryCanonicalLayer is the point of the phase: one query reaches every
// searchable class without the caller naming which layer holds the answer.
func TestSearchSpansEveryCanonicalLayer(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})

	seen := map[domain.SearchEntityType]bool{}
	for _, result := range results.Results {
		seen[result.EntityType] = true
	}
	for _, want := range domain.SearchEntityTypes {
		if !seen[want] {
			t.Errorf("no %s result for a query every record matches: %v", want, searchRefs(results))
		}
	}
	if results.Page.Total != len(results.Results) {
		t.Errorf("total = %d, returned = %d", results.Page.Total, len(results.Results))
	}

	// A term with actual meaning reaches three separate layers at once, which is the case a
	// reader is really in: core knowledge, the evidence layer and the practice layer, found
	// without knowing which of them held the answer.
	crossLayer := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})
	for _, want := range []domain.SearchEntityType{
		domain.SearchSession, domain.SearchClaim, domain.SearchSource,
		domain.SearchVocabulary, domain.SearchExperiment,
	} {
		found := false
		for _, result := range crossLayer.Results {
			if result.EntityType == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no %s result for %q: %v", want, "fixture", searchRefs(crossLayer))
		}
	}
	if crossLayer.Query != "fixture" {
		t.Errorf("echoed query = %q, want the normalised needle", crossLayer.Query)
	}
}

// TestSearchIsCaseInsensitive covers the three spellings a reader actually types.
func TestSearchIsCaseInsensitive(t *testing.T) {
	k := evidenceIndex(t)
	want := searchRefs(mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200}))
	if len(want) == 0 {
		t.Fatal("baseline query matched nothing")
	}

	for _, spelling := range []string{"Fixture", "FIXTURE", "FiXtUrE", "  fixture  "} {
		got := searchRefs(mustSearch(t, k, service.SearchQuery{Q: spelling, Limit: 200}))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q returned %v, want the same results as %q", spelling, got, "fixture")
		}
	}
}

// TestSearchExactIDSortsFirst covers the top precedence class, and covers it in the one case
// where the class matters most: an ID two record classes share.
func TestSearchExactIDSortsFirst(t *testing.T) {
	k := evidenceIndex(t)

	results := mustSearch(t, k, service.SearchQuery{Q: "session-01-fixture"})
	want := []string{"session/session-01-fixture", "source/session-01-fixture"}
	if got := searchRefs(results); !reflect.DeepEqual(got, want) {
		t.Fatalf("results = %v, want %v (class order breaks the tie)", got, want)
	}
	for _, result := range results.Results {
		if result.MatchKind != domain.MatchIDExact {
			t.Errorf("%s/%s match_kind = %q, want %q", result.EntityType, result.ID,
				result.MatchKind, domain.MatchIDExact)
		}
	}

	// A node whose canonical ID is the whole query outranks the claims that merely mention it.
	results = mustSearch(t, k, service.SearchQuery{Q: "alpha"})
	if got := searchRefs(results)[0]; got != "node/alpha" {
		t.Errorf("first result = %q, want node/alpha", got)
	}
	if got := results.Results[0].MatchKind; got != domain.MatchIDExact {
		t.Errorf("match_kind = %q, want %q", got, domain.MatchIDExact)
	}
}

// TestSearchMatchKindPrecedence pins the four categorical classes and the order they impose.
func TestSearchMatchKindPrecedence(t *testing.T) {
	k := evidenceIndex(t)

	cases := []struct {
		name  string
		query string
		ref   string
		kind  string
	}{
		{"exact id", "fixture-term", "vocabulary/fixture-term", domain.MatchIDExact},
		{"exact display field", "Fixture Term", "vocabulary/fixture-term", domain.MatchTitleExact},
		{"display field substring", "listening", "experiment/fixture-listening-exercise", domain.MatchTitleSubstring},
		{"other field only", "resonant", "node/beta", domain.MatchFieldSubstring},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			results := mustSearch(t, k, service.SearchQuery{Q: tc.query, Limit: 200})
			for _, result := range results.Results {
				if string(result.EntityType)+"/"+result.ID != tc.ref {
					continue
				}
				if result.MatchKind != tc.kind {
					t.Fatalf("%s match_kind = %q, want %q", tc.ref, result.MatchKind, tc.kind)
				}
				return
			}
			t.Fatalf("%s did not appear in results for %q: %v", tc.ref, tc.query, searchRefs(results))
		})
	}

	// Precedence is an ordering, so a query producing two classes must emit them in that order.
	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})
	rank := map[string]int{
		domain.MatchIDExact: 0, domain.MatchTitleExact: 1,
		domain.MatchTitleSubstring: 2, domain.MatchFieldSubstring: 3,
	}
	previous := -1
	for _, result := range results.Results {
		current, ok := rank[result.MatchKind]
		if !ok {
			t.Fatalf("unknown match_kind %q", result.MatchKind)
		}
		if current < previous {
			t.Fatalf("match_kind %q followed a lower-precedence result: %v",
				result.MatchKind, searchRefs(results))
		}
		previous = current
	}
}

// TestSearchMatchedFieldsAreCanonicalAndComplete asserts the evidence a result carries for
// its own existence: every field named actually matched, and no matching field is omitted.
func TestSearchMatchedFieldsAreCanonicalAndComplete(t *testing.T) {
	k := evidenceIndex(t)

	cases := []struct {
		query string
		ref   string
		want  []string
	}{
		// One query matching several fields of one entity, in canonical field order.
		{"fixture", "vocabulary/fixture-term", []string{"id", "term", "digital_relationship", "technologies", "tags"}},
		{"fixture", "source/fixture-reference-work", []string{"id", "title", "author"}},
		// A source with no author contributes no author field, so none can be reported.
		{"fixture", "source/fixture-uncited-source", []string{"id"}},
		{"acoustics", "node/alpha", []string{"domain"}},
		{"outbound edges", "node/alpha", []string{"definition", "core_concepts"}},
		{"listening", "experiment/fixture-listening-exercise", []string{"id", "title", "type"}},
	}
	for _, tc := range cases {
		t.Run(tc.query+" "+tc.ref, func(t *testing.T) {
			results := mustSearch(t, k, service.SearchQuery{Q: tc.query, Limit: 200})
			for _, result := range results.Results {
				if string(result.EntityType)+"/"+result.ID != tc.ref {
					continue
				}
				if !reflect.DeepEqual(result.MatchedFields, tc.want) {
					t.Fatalf("matched_fields = %v, want %v", result.MatchedFields, tc.want)
				}
				return
			}
			t.Fatalf("%s did not appear in results for %q", tc.ref, tc.query)
		})
	}
}

// TestSearchDisplayFieldsComeFromTheRecord asserts a result carries its own record's values
// and never a fabricated or borrowed one.
func TestSearchDisplayFieldsComeFromTheRecord(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	byRef := map[string]domain.SearchResult{}
	for _, result := range results.Results {
		byRef[string(result.EntityType)+"/"+result.ID] = result
	}

	cases := []struct {
		ref     string
		title   string
		summary string
	}{
		{"vocabulary/fixture-term", "Fixture Term",
			"A synthetic terminology entry used by the backend tests."},
		{"experiment/fixture-listening-exercise", "Fixture Listening Exercise",
			"Exercise the experiment projection with a definition that has both a planned and a completed run."},
		{"session/session-01-fixture", "Fixture Session 1: Test Corpus", ""},
		{"source/fixture-reference-work", "A Fixture Reference Work", ""},
	}
	for _, tc := range cases {
		result, ok := byRef[tc.ref]
		if !ok {
			t.Errorf("%s missing from results", tc.ref)
			continue
		}
		if result.Title != tc.title {
			t.Errorf("%s title = %q, want %q", tc.ref, result.Title, tc.title)
		}
		if result.Summary != tc.summary {
			t.Errorf("%s summary = %q, want %q", tc.ref, result.Summary, tc.summary)
		}
	}

	// A claim's display field is its statement, and it is not repeated as a summary.
	claims := mustSearch(t, k, service.SearchQuery{Q: "alpha-carries-energy"})
	if len(claims.Results) != 1 {
		t.Fatalf("results = %v, want exactly the claim", searchRefs(claims))
	}
	if !strings.HasPrefix(claims.Results[0].Title, "The alpha fixture concept carries energy") {
		t.Errorf("claim title = %q, want the canonical statement", claims.Results[0].Title)
	}
	if claims.Results[0].Summary != "" {
		t.Errorf("claim summary = %q, want none", claims.Results[0].Summary)
	}
}

// TestSearchTypeFilter covers the class filter, including that it rejects a class outside the
// contract rather than answering with an empty set.
func TestSearchTypeFilter(t *testing.T) {
	k := evidenceIndex(t)

	for _, entityType := range domain.SearchEntityTypes {
		results := mustSearch(t, k, service.SearchQuery{
			Q: universalNeedle, Type: string(entityType), Limit: 200,
		})
		if len(results.Results) == 0 {
			t.Errorf("type=%s returned nothing for a query every record matches", entityType)
		}
		for _, result := range results.Results {
			if result.EntityType != entityType {
				t.Errorf("type=%s returned a %s result", entityType, result.EntityType)
			}
		}
		if results.Type != string(entityType) {
			t.Errorf("echoed type = %q, want %q", results.Type, entityType)
		}
	}

	// The filtered totals must add up to the unfiltered one: filtering selects, it never
	// changes what matched.
	unfiltered := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})
	sum := 0
	for _, entityType := range domain.SearchEntityTypes {
		sum += mustSearch(t, k, service.SearchQuery{
			Q: universalNeedle, Type: string(entityType), Limit: 200,
		}).Page.Total
	}
	if sum != unfiltered.Page.Total {
		t.Errorf("filtered totals sum to %d, unfiltered total is %d", sum, unfiltered.Page.Total)
	}
}

// TestSearchRejectsUnsupportedType covers the classes that are deliberately not searchable.
//
// experiment_run is the load-bearing case: it is a canonical record class the backend loads
// and can list, and it is still refused here rather than silently returning nothing, so a
// caller cannot read an empty result as "no run mentions this".
func TestSearchRejectsUnsupportedType(t *testing.T) {
	k := evidenceIndex(t)

	for _, unsupported := range []string{"experiment_run", "experiment-run", "run", "relationship_type", "Node", ""} {
		if unsupported == "" {
			continue
		}
		_, err := k.Search(service.SearchQuery{Q: "fixture", Type: unsupported})
		var invalid *service.InvalidFilterError
		if !errors.As(err, &invalid) {
			t.Fatalf("type=%q error = %v, want InvalidFilterError", unsupported, err)
		}
		if invalid.Param != "type" {
			t.Errorf("param = %q, want type", invalid.Param)
		}
		if !reflect.DeepEqual(invalid.Allowed, domain.SearchEntityTypeNames()) {
			t.Errorf("allowed = %v, want %v", invalid.Allowed, domain.SearchEntityTypeNames())
		}
	}
}

// TestSearchRequiresAQuery asserts search never degrades into "serialise the whole corpus".
func TestSearchRequiresAQuery(t *testing.T) {
	k := evidenceIndex(t)

	for _, blank := range []string{"", " ", "\t", "\n", "   \t  \n "} {
		results, err := k.Search(service.SearchQuery{Q: blank})
		if !errors.Is(err, service.ErrEmptySearchQuery) {
			t.Errorf("q=%q error = %v, want ErrEmptySearchQuery", blank, err)
		}
		if len(results.Results) != 0 {
			t.Errorf("q=%q returned %d results alongside an error", blank, len(results.Results))
		}
	}
}

// TestSearchEmptyResultIsAnAnswer asserts an unmatched query is a valid empty collection.
func TestSearchEmptyResultIsAnAnswer(t *testing.T) {
	k := evidenceIndex(t)

	results, err := k.Search(service.SearchQuery{Q: "no-canonical-record-contains-this-text"})
	if err != nil {
		t.Fatalf("unmatched query: %v", err)
	}
	if results.Results == nil {
		t.Error("results = nil, want an empty collection")
	}
	if len(results.Results) != 0 || results.Page.Total != 0 || results.Page.Count != 0 {
		t.Errorf("results = %+v, want empty", results)
	}
}

// TestSearchPaging covers the window and asserts the pages reassemble into the whole result.
func TestSearchPaging(t *testing.T) {
	k := evidenceIndex(t)
	full := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})
	if full.Page.Total < 6 {
		t.Fatalf("fixture corpus matched %d records, too few to page", full.Page.Total)
	}

	var walked []string
	for offset := 0; offset < full.Page.Total; offset += 3 {
		page := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 3, Offset: offset})
		if page.Page.Total != full.Page.Total {
			t.Errorf("offset %d reported total %d, want %d", offset, page.Page.Total, full.Page.Total)
		}
		if page.Page.Offset != offset || page.Page.Limit != 3 {
			t.Errorf("page meta = %+v, want limit 3 offset %d", page.Page, offset)
		}
		if page.Page.Count > 3 {
			t.Errorf("offset %d returned %d results, want at most 3", offset, page.Page.Count)
		}
		walked = append(walked, searchRefs(page)...)
	}
	if want := searchRefs(full); !reflect.DeepEqual(walked, want) {
		t.Errorf("walked pages = %v, want %v", walked, want)
	}

	// An offset past the end is an empty page, not an error and not a wrapped one.
	past := mustSearch(t, k, service.SearchQuery{Q: "fixture", Offset: full.Page.Total + 50})
	if len(past.Results) != 0 || past.Page.Total != full.Page.Total {
		t.Errorf("offset past end = %+v", past.Page)
	}
}

// TestSearchBounds asserts one request cannot be talked into serialising everything.
func TestSearchBounds(t *testing.T) {
	k := evidenceIndex(t)

	unbounded := mustSearch(t, k, service.SearchQuery{Q: "fixture"})
	if unbounded.Page.Limit != service.DefaultLimit {
		t.Errorf("default limit = %d, want %d", unbounded.Page.Limit, service.DefaultLimit)
	}

	for _, oversized := range []int{service.MaxLimit + 1, 10000, 1 << 30} {
		results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: oversized})
		if results.Page.Limit != service.MaxLimit {
			t.Errorf("limit %d was not clamped to %d: %+v", oversized, service.MaxLimit, results.Page)
		}
		if results.Page.Count > service.MaxLimit {
			t.Errorf("limit %d returned %d results", oversized, results.Page.Count)
		}
	}

	// A negative offset is read as the start of the result set, matching every other list.
	negative := mustSearch(t, k, service.SearchQuery{Q: "fixture", Offset: -5})
	if negative.Page.Offset != 0 {
		t.Errorf("negative offset = %d, want 0", negative.Page.Offset)
	}

	// A direct service caller gets the same strict ceiling as an HTTP caller. Silent truncation
	// could turn a valid prefix plus an ignored suffix into a different query.
	_, err := k.Search(service.SearchQuery{Q: strings.Repeat("a", service.MaxQueryChars+1)})
	if !errors.Is(err, service.ErrSearchQueryTooLong) {
		t.Errorf("over-long query error = %v, want ErrSearchQueryTooLong", err)
	}
}

// TestSearchDoesNotMatchAcrossCanonicalBoundaries guards both kinds of synthetic text the
// shared field projection could otherwise create: the separator between fields and the
// separator between values in a list-valued field. The cross-layer route and the older native
// lists must agree that matching occurs inside canonical values only.
func TestSearchDoesNotMatchAcrossCanonicalBoundaries(t *testing.T) {
	k := evidenceIndex(t)

	for _, query := range []string{
		"alpha\nalpha",     // node id -> title
		"go test\nfixture", // vocabulary technologies -> tags
	} {
		results, err := k.Search(service.SearchQuery{Q: query, Limit: 200})
		if err != nil {
			t.Fatalf("cross-layer q=%q: %v", query, err)
		}
		if results.Page.Total != 0 {
			t.Errorf("cross-layer q=%q matched synthetic boundary text: %v", query, searchRefs(results))
		}
	}

	if got := k.ListNodes(service.NodeQuery{Q: "alpha\nalpha"}).Page.Total; got != 0 {
		t.Errorf("node list matched %d records across an id/title boundary", got)
	}
	vocabulary, err := k.ListVocabulary(service.VocabularyQuery{Q: "go test\nfixture"})
	if err != nil {
		t.Fatalf("vocabulary list boundary query: %v", err)
	}
	if vocabulary.Page.Total != 0 {
		t.Errorf("vocabulary list matched %d records across list values/fields", vocabulary.Page.Total)
	}
}

// TestSearchIsDeterministic runs the same query twice against one index and once against a
// second index built from the same corpus.
//
// Both halves matter. The first would miss an ordering that depends on a map the index only
// builds once; the second is what catches ordering that depends on Go map iteration or on the
// order the filesystem happened to return records in.
func TestSearchIsDeterministic(t *testing.T) {
	first := evidenceIndex(t)
	second := evidenceIndex(t)

	for _, query := range []string{"fixture", "alpha", "acoustics", "e", "session-01-fixture"} {
		a := mustSearch(t, first, service.SearchQuery{Q: query, Limit: 200})
		b := mustSearch(t, first, service.SearchQuery{Q: query, Limit: 200})
		c := mustSearch(t, second, service.SearchQuery{Q: query, Limit: 200})

		if !reflect.DeepEqual(a, b) {
			t.Errorf("%q differed between two calls on one index", query)
		}
		if !reflect.DeepEqual(a, c) {
			t.Errorf("%q differed between two indexes over the same corpus:\n%v\n%v",
				query, searchRefs(a), searchRefs(c))
		}
	}
}

// TestSearchResultsAreDefensiveCopies asserts a caller cannot write through a result into the
// startup index. Knowledge is a package API as well as an HTTP backing store, and its
// immutability guarantee has to hold for every caller, not only for the handler that encodes.
func TestSearchResultsAreDefensiveCopies(t *testing.T) {
	k := evidenceIndex(t)
	before := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	for i := range before.Results {
		before.Results[i].ID = "mutated"
		before.Results[i].Title = "mutated"
		before.Results[i].Summary = "mutated"
		before.Results[i].MatchKind = "mutated"
		for j := range before.Results[i].MatchedFields {
			before.Results[i].MatchedFields[j] = "mutated"
		}
	}

	after := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})
	for _, result := range after.Results {
		if result.ID == "mutated" || result.Title == "mutated" || result.MatchKind == "mutated" {
			t.Fatalf("a returned result wrote through into the index: %+v", result)
		}
		for _, field := range result.MatchedFields {
			if field == "mutated" {
				t.Fatalf("a returned matched_fields slice wrote through into the index: %+v", result)
			}
		}
	}

	// The per-layer projections read the same documents, so they must be unharmed too.
	if got := listVocabulary(t, k, service.VocabularyQuery{Q: "fixture"}).Page.Total; got == 0 {
		t.Error("the vocabulary list stopped matching after a search result was mutated")
	}
}

// TestSearchTreatsQueriesAsPlainText covers the adversarial shapes a query can take.
//
// None of them may be interpreted: not as a regular expression, a glob, a path, a URL or a
// command. Each is either a literal substring of some canonical field or it matches nothing,
// and in every case the request completes normally.
func TestSearchTreatsQueriesAsPlainText(t *testing.T) {
	k := evidenceIndex(t)

	queries := []string{
		".*", "^alpha$", "a|b", "alpha.*energy", "[a-z]+", "(", "\\",
		"*", "?", "**/*.md", "../../etc/passwd", "..\\..\\windows",
		"/nodes/alpha", "nodes/acoustics/alpha.md", "C:\\Users",
		"http://example.com", "https://example.com/x?y=z",
		"'; DROP TABLE nodes; --", "1999", "0", "-1", "3.14",
		"café", "共鳴", "Ω", "🎵", "  ", "%20", "&&", "$(whoami)",
		strings.Repeat("alpha ", 20),
	}
	for _, query := range queries {
		results, err := k.Search(service.SearchQuery{Q: query, Limit: 200})
		if errors.Is(err, service.ErrEmptySearchQuery) {
			// A query that is only whitespace is legitimately refused; that is tested above.
			continue
		}
		if err != nil {
			t.Errorf("q=%q returned an error: %v", query, err)
			continue
		}
		if results.Results == nil {
			t.Errorf("q=%q returned nil rather than an empty collection", query)
		}
		// A regex or glob that was actually evaluated would match far more than a literal can.
		for _, result := range results.Results {
			if len(result.MatchedFields) == 0 {
				t.Errorf("q=%q matched %s/%s with no field", query, result.EntityType, result.ID)
			}
		}
	}

	// The one query that would match everything if the pattern were evaluated matches nothing.
	if got := mustSearch(t, k, service.SearchQuery{Q: ".*", Limit: 200}); got.Page.Total != 0 {
		t.Errorf(".* matched %d records, so it was not treated as literal text", got.Page.Total)
	}

	// A numeric query is text, and finds the record whose text contains it.
	if got := searchRefs(mustSearch(t, k, service.SearchQuery{Q: "1999", Limit: 200})); len(got) == 0 {
		t.Error("a numeric query matched nothing; the fixture corpus contains 1999")
	}
}

// TestSearchExcludesExperimentRuns is the guardrail this phase is most able to break.
//
// A run's observations, measurements and interpretation are evidence-bearing prose. A
// free-text hit inside them would read as "this run observed that", which is an assertion a
// discovery projection has no standing to make, so runs carry no search document at all.
func TestSearchExcludesExperimentRuns(t *testing.T) {
	k := evidenceIndex(t)

	runs, err := k.ListExperimentRuns(service.ExperimentRunQuery{Limit: 200})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs.Runs) == 0 {
		t.Fatal("fixture corpus has no runs, so this guardrail would pass vacuously")
	}

	runIDs := map[string]bool{}
	for _, run := range runs.Runs {
		runIDs[run.ID] = true
	}

	// No query, including a run's own canonical ID, may produce a run result.
	queries := []string{"fixture", "run", "planned", "completed", "observation", "exercise"}
	for id := range runIDs {
		queries = append(queries, id)
	}
	for _, query := range queries {
		for _, result := range mustSearch(t, k, service.SearchQuery{Q: query, Limit: 200}).Results {
			if runIDs[result.ID] {
				t.Errorf("q=%q returned experiment run %q as a %s result", query, result.ID, result.EntityType)
			}
			if string(result.EntityType) == "experiment_run" {
				t.Errorf("q=%q returned an experiment_run entity type", query)
			}
		}
	}

	// A run's observation text is not searchable. It is searched for here specifically so that
	// a future change which starts indexing observations fails loudly rather than quietly.
	run, err := k.ExperimentRunByID("fixture-listening-exercise-completed-a")
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	if len(run.Observations) == 0 {
		t.Fatal("the completed fixture run has no observations, so this assertion is vacuous")
	}
	needle := strings.ToLower(run.Observations[0].Statement)
	if len(needle) > service.MaxQueryChars {
		needle = needle[:service.MaxQueryChars]
	}
	if got := mustSearch(t, k, service.SearchQuery{Q: needle, Limit: 200}); got.Page.Total != 0 {
		t.Errorf("a run observation was discoverable by its own text: %v", searchRefs(got))
	}
}

// TestSearchDoesNotFlattenProvenance asserts a result carries its own record and nothing else.
//
// A unified discovery surface is not a unified ontology. A claim result must not present its
// source's title, an experiment result must not present its runs, and a vocabulary result must
// not present the node it cross-references — each of those is a different record, reached
// through its own route.
func TestSearchDoesNotFlattenProvenance(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	// Every display field a result carries is one of its own record's, so the surface of the DTO
	// is four display values, the matched-field evidence, and the Phase 1F context object.
	// Asserting the shape here is what makes a later field addition a deliberate contract
	// decision rather than a slip.
	//
	// Context is the one field that may name another record, and it was added deliberately
	// against this rule rather than in spite of it: it is a separate sub-object in which every
	// entity arrives with the relation and the canonical field it was read from, so nothing is
	// flattened into the hit's own fields. The rest of this test still holds with context
	// requested, which is what keeps the distinction real: TestContextPreservesResultIdentityAndOrder
	// asserts that stripping Context off an enriched result yields the plain result exactly, so
	// every display-field assertion below carries over to a context request unchanged.
	//
	// Phase 1G added the eighth field, TermMatches, and it is the safest kind of addition to
	// make against this rule: every value in it is a term the caller supplied and a canonical
	// field name of this record, so it introduces no record content at all, borrowed or
	// otherwise. TestAllTermsEvidenceNamesOnlyTheHitsOwnFields holds it to that.
	//
	// Phase 1I added the ninth and tenth, RelevanceScore and MatchSignals, and they are safer
	// still: neither can carry text of any kind. RelevanceScore is an integer derived from this
	// record's own fields, and MatchSignals holds names drawn from domain.SearchMatchSignals, a
	// closed vocabulary fixed at compile time and containing no record content, this record's or
	// anyone's. Nothing in either can be borrowed from a related record, because neither is read
	// from a record at all. TestRankingExplanationLeaksNothing holds them to that.
	if fields := reflect.TypeOf(domain.SearchResult{}).NumField(); fields != 10 {
		t.Errorf("SearchResult has %d fields; a new field must be justified against provenance "+
			"flattening before this expectation is updated", fields)
	}

	// A default search carries no context at all, so the Phase 1E result is byte-identical.
	for _, result := range results.Results {
		if result.Context != nil {
			t.Errorf("result %s/%s carries context that was never requested", result.EntityType, result.ID)
		}
		if result.TermMatches != nil {
			t.Errorf("literal result %s/%s carries multi-term evidence", result.EntityType, result.ID)
		}
	}

	for _, result := range results.Results {
		switch result.EntityType {
		case domain.SearchClaim:
			// A claim cites sources. None of their titles may appear on the hit.
			detail, err := k.ClaimByID(result.ID)
			if err != nil {
				t.Fatalf("claim lookup: %v", err)
			}
			for _, sourceID := range detail.SourceIDs {
				source, err := k.SourceByID(sourceID)
				if err != nil {
					continue
				}
				if strings.Contains(result.Title, source.Title) || strings.Contains(result.Summary, source.Title) {
					t.Errorf("claim %s carries the title of source %s", result.ID, sourceID)
				}
			}
			for _, field := range result.MatchedFields {
				if field != "id" && field != "statement" {
					t.Errorf("claim %s matched on %q, which is not a claim field it should search",
						result.ID, field)
				}
			}
		case domain.SearchExperiment:
			detail, err := k.ExperimentByID(result.ID)
			if err != nil {
				t.Fatalf("experiment lookup: %v", err)
			}
			for _, runID := range detail.RunIDs {
				if strings.Contains(result.Title, runID) || strings.Contains(result.Summary, runID) {
					t.Errorf("experiment %s carries run %s in its display fields", result.ID, runID)
				}
			}
		}
	}
}

// TestSearchAgreesWithEveryLayerList is the anti-drift assertion.
//
// Phase 1E defines the searchable field set of every class in one place and the per-layer
// lists read it from there. This asserts the consequence: a term that finds a record through
// its own layer's q parameter finds it through /search too, and vice versa. A future change
// that gave one of them a field the other lacks fails here.
func TestSearchAgreesWithEveryLayerList(t *testing.T) {
	k := evidenceIndex(t)

	for _, query := range []string{"fixture", "alpha", "acoustics", "synthetic", "1999", "listening"} {
		layer := map[string][]string{}
		for _, node := range k.ListNodes(service.NodeQuery{Q: query, Limit: 200}).Nodes {
			layer["node"] = append(layer["node"], node.ID)
		}
		for _, session := range k.ListSessions(service.SessionQuery{Q: query, Limit: 200}).Sessions {
			layer["session"] = append(layer["session"], session.ID)
		}
		claims, err := k.ListClaims(service.ClaimQuery{Q: query, Limit: 200})
		if err != nil {
			t.Fatalf("list claims: %v", err)
		}
		for _, claim := range claims.Claims {
			layer["claim"] = append(layer["claim"], claim.ID)
		}
		sources, err := k.ListSources(service.SourceQuery{Q: query, Limit: 200})
		if err != nil {
			t.Fatalf("list sources: %v", err)
		}
		for _, source := range sources.Sources {
			layer["source"] = append(layer["source"], source.ID)
		}
		for _, entry := range listVocabulary(t, k, service.VocabularyQuery{Q: query, Limit: 200}).Vocabulary {
			layer["vocabulary"] = append(layer["vocabulary"], entry.ID)
		}
		for _, experiment := range mustListExperiments(t, k, service.ExperimentQuery{Q: query, Limit: 200}).Experiments {
			layer["experiment"] = append(layer["experiment"], experiment.ID)
		}

		for _, entityType := range domain.SearchEntityTypes {
			var discovered []string
			for _, result := range mustSearch(t, k, service.SearchQuery{
				Q: query, Type: string(entityType), Limit: 200,
			}).Results {
				discovered = append(discovered, result.ID)
			}
			// Search orders by match precedence and the layer lists order by canonical ID, so
			// the sets are compared rather than the sequences.
			want := append([]string(nil), layer[string(entityType)]...)
			got := append([]string(nil), discovered...)
			sortStrings(want)
			sortStrings(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("q=%q type=%s: search found %v, the layer list found %v",
					query, entityType, got, want)
			}
		}
	}
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// TestSearchDiagnosticsReportTheProjection covers the one diagnostics addition.
func TestSearchDiagnosticsReportTheProjection(t *testing.T) {
	k := evidenceIndex(t)
	diagnostics := k.Diagnostics()
	counts := diagnostics.Search

	corpus := diagnostics.Corpus
	if counts.Nodes != corpus.Nodes || counts.Sessions != corpus.Sessions ||
		counts.Claims != corpus.Claims || counts.Sources != corpus.Sources ||
		counts.Vocabulary != corpus.Vocabulary || counts.Experiments != corpus.Experiments {
		t.Errorf("search counts %+v do not match the loaded corpus %+v", counts, corpus)
	}
	want := corpus.Nodes + corpus.Sessions + corpus.Claims + corpus.Sources +
		corpus.Vocabulary + corpus.Experiments
	if counts.Documents != want {
		t.Errorf("documents = %d, want %d", counts.Documents, want)
	}
	// The corpus has runs and the discovery projection has no document for any of them.
	if corpus.ExperimentRuns == 0 {
		t.Fatal("fixture corpus has no runs, so the exclusion below is vacuous")
	}
	if counts.Documents == want+corpus.ExperimentRuns {
		t.Error("experiment runs were counted into the discovery projection")
	}
}
