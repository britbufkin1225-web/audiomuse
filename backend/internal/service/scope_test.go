package service_test

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// Phase 1H at the service: the entity-type scope filter and the facet counts.
//
// The suite is written as invariants against the unfiltered search rather than as expected
// result lists, and deliberately so. The claim Phase 1H makes is a relationship between two
// searches — a scoped result set is the unscoped one with other classes removed, and nothing
// else about it changed — so an assertion that named records would pin the fixture corpus
// instead of the contract, and would still pass if filtering quietly reordered or re-evidenced
// what it kept.

// scopeNeedle is a term the fixture corpus answers from several classes at once, which is the
// only situation in which a class filter means anything.
const scopeNeedle = "fixture"

// keepClasses is the reference implementation of the filter: the same results, in the same
// order, with the classes outside the scope dropped. Every scope assertion below compares
// against this rather than against a list of IDs.
func keepClasses(results service.SearchResults, want ...domain.SearchEntityType) []domain.SearchResult {
	keep := map[domain.SearchEntityType]bool{}
	for _, class := range want {
		keep[class] = true
	}
	out := make([]domain.SearchResult, 0, len(results.Results))
	for _, result := range results.Results {
		if keep[result.EntityType] {
			out = append(out, result)
		}
	}
	return out
}

// classNames renders a class list as the strings a request carries.
func classNames(classes ...domain.SearchEntityType) []string {
	out := make([]string, 0, len(classes))
	for _, class := range classes {
		out = append(out, string(class))
	}
	return out
}

// refsOf renders a bare result slice as type/id pairs, so an assertion comparing against
// keepClasses can report both sides in the same shape searchRefs reports a response in.
func refsOf(results []domain.SearchResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, string(result.EntityType)+"/"+result.ID)
	}
	return out
}

func containsClass(classes []domain.SearchEntityType, want domain.SearchEntityType) bool {
	for _, class := range classes {
		if class == want {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestSearchScopeFiltersEveryClass is the phase in one assertion, run once per searchable
// class: a scoped search returns exactly the hits of that class the unscoped search returned,
// in the same order and with the same evidence.
func TestSearchScopeFiltersEveryClass(t *testing.T) {
	k := evidenceIndex(t)
	all := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})

	for _, class := range domain.SearchEntityTypes {
		t.Run(string(class), func(t *testing.T) {
			scoped := mustSearch(t, k, service.SearchQuery{
				Q: universalNeedle, EntityTypes: classNames(class), Limit: 200,
			})
			want := keepClasses(all, class)
			if len(want) == 0 {
				t.Fatalf("the unscoped baseline matched no %s, so this class proves nothing", class)
			}
			if !reflect.DeepEqual(scoped.Results, want) {
				t.Errorf("scoped %s results = %v, want the %d unscoped hits of that class",
					class, searchRefs(scoped), len(want))
			}
			if scoped.Page.Total != len(want) {
				t.Errorf("page.total = %d, want %d", scoped.Page.Total, len(want))
			}
			// Every other class is absent from the results and reported as zero rather than
			// omitted, which is the difference between "none matched" and "not asked about".
			for _, other := range domain.SearchEntityTypes {
				got := scoped.Facets.EntityTypes.Count(other)
				if other == class {
					if got != len(want) {
						t.Errorf("facet %s = %d, want %d", other, got, len(want))
					}
					continue
				}
				if got != 0 {
					t.Errorf("facet %s = %d in a %s-scoped search, want 0", other, got, class)
				}
			}
			if got := scoped.Facets.EntityTypes.Total(); got != scoped.Page.Total {
				t.Errorf("facet sum = %d, page.total = %d", got, scoped.Page.Total)
			}
			// The echo describes the scope that ran.
			if !reflect.DeepEqual(scoped.EntityTypes, classNames(class)) {
				t.Errorf("echoed entity_types = %v, want %v", scoped.EntityTypes, classNames(class))
			}
		})
	}
}

// TestSearchScopeAcceptsSeveralClasses covers the case the single type filter cannot express:
// two layers at once, and neither of the others.
func TestSearchScopeAcceptsSeveralClasses(t *testing.T) {
	k := evidenceIndex(t)
	all := mustSearch(t, k, service.SearchQuery{Q: scopeNeedle, Limit: 200})

	for _, group := range [][]domain.SearchEntityType{
		{domain.SearchClaim, domain.SearchSource},
		{domain.SearchSession, domain.SearchExperiment},
		{domain.SearchVocabulary, domain.SearchClaim, domain.SearchSource},
	} {
		t.Run(strings.Join(classNames(group...), "+"), func(t *testing.T) {
			scoped := mustSearch(t, k, service.SearchQuery{
				Q: scopeNeedle, EntityTypes: classNames(group...), Limit: 200,
			})
			want := keepClasses(all, group...)
			if len(want) == 0 {
				t.Fatalf("the unscoped baseline matched none of %v", classNames(group...))
			}
			if !reflect.DeepEqual(scoped.Results, want) {
				t.Errorf("results = %v, want the unscoped subset of %v",
					searchRefs(scoped), classNames(group...))
			}
			for _, class := range domain.SearchEntityTypes {
				count := scoped.Facets.EntityTypes.Count(class)
				if !containsClass(group, class) && count != 0 {
					t.Errorf("facet %s = %d outside the scope, want 0", class, count)
				}
			}
			if got := scoped.Facets.EntityTypes.Total(); got != scoped.Page.Total {
				t.Errorf("facet sum = %d, page.total = %d", got, scoped.Page.Total)
			}
		})
	}
}

// TestSearchScopeOfEveryClassIsTheUnscopedSearch pins the boundary of the filter: naming all
// six classes must not be a seventh, subtly different search.
func TestSearchScopeOfEveryClassIsTheUnscopedSearch(t *testing.T) {
	k := evidenceIndex(t)
	all := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})
	scoped := mustSearch(t, k, service.SearchQuery{
		Q: universalNeedle, EntityTypes: classNames(domain.SearchEntityTypes...), Limit: 200,
	})

	if !reflect.DeepEqual(scoped.Results, all.Results) {
		t.Errorf("a scope naming every class changed the result set:\n%v\n%v",
			searchRefs(scoped), searchRefs(all))
	}
	if scoped.Facets != all.Facets {
		t.Errorf("facets differed: %+v vs %+v", scoped.Facets, all.Facets)
	}
	if scoped.Page != all.Page {
		t.Errorf("page differed: %+v vs %+v", scoped.Page, all.Page)
	}
}

// TestSearchScopeIsOrderedCanonically requires the scope to be a set rather than a sequence:
// the caller's ordering of the class list changes nothing, including the echo.
func TestSearchScopeIsOrderedCanonically(t *testing.T) {
	k := evidenceIndex(t)
	forward := mustSearch(t, k, service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"session", "claim", "experiment"}, Limit: 200,
	})
	reversed := mustSearch(t, k, service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"experiment", "claim", "session"}, Limit: 200,
	})

	if !reflect.DeepEqual(forward, reversed) {
		t.Errorf("class-list order changed the response:\n%+v\n%+v", forward, reversed)
	}
	if want := []string{"session", "claim", "experiment"}; !reflect.DeepEqual(forward.EntityTypes, want) {
		t.Errorf("echoed entity_types = %v, want the canonical order %v", forward.EntityTypes, want)
	}
	// Filtering removes classes; it does not group or reorder what it keeps. Phase 1H stated
	// that by requiring the surviving classes to ascend, which held only because the class order
	// was then the primary sort key. Phase 1I made relevance the primary key and the class order
	// the tie-break, so a high-scoring experiment may now precede a low-scoring session and the
	// ascent no longer describes a correct result set.
	//
	// The invariant Phase 1H actually asserted survives intact and is checked here directly
	// against keepClasses, the reference implementation of the filter: the scoped set is the
	// unscoped set with the excluded classes dropped, in the order the unscoped search produced.
	// That is strictly stronger than the ascent it replaces — it pins every hit's position
	// rather than only its class's — and it is the same comparison every other scope assertion
	// in this file already makes.
	unscoped := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})
	want := keepClasses(unscoped, domain.SearchSession, domain.SearchClaim, domain.SearchExperiment)
	if !reflect.DeepEqual(forward.Results, want) {
		t.Errorf("scoped results are not the unscoped results with other classes dropped:\n%v\n%v",
			searchRefs(forward), refsOf(want))
	}
}

// TestSearchScopeComposesWithQueryModes requires the filter to be orthogonal to composition:
// scoping an all_terms search removes classes and changes nothing else, including the per-term
// evidence a composed hit carries.
func TestSearchScopeComposesWithQueryModes(t *testing.T) {
	k := evidenceIndex(t)
	const composed = "fixture alpha"

	all := mustSearch(t, k, service.SearchQuery{
		Q: composed, Mode: string(domain.SearchModeAllTerms), Limit: 200,
	})
	if all.Page.Total == 0 {
		t.Fatal("the composed baseline matched nothing")
	}
	for _, class := range domain.SearchEntityTypes {
		want := keepClasses(all, class)
		scoped := mustSearch(t, k, service.SearchQuery{
			Q: composed, Mode: string(domain.SearchModeAllTerms),
			EntityTypes: classNames(class), Limit: 200,
		})
		if !reflect.DeepEqual(scoped.Results, want) {
			t.Errorf("composed %s results = %v, want the unscoped subset", class, searchRefs(scoped))
		}
		if scoped.QueryMode != string(domain.SearchModeAllTerms) {
			t.Errorf("query_mode echo = %q under a scoped composed search", scoped.QueryMode)
		}
		if got := scoped.Facets.EntityTypes.Total(); got != scoped.Page.Total {
			t.Errorf("facet sum = %d, page.total = %d", got, scoped.Page.Total)
		}
	}

	// The literal half of the same assertion: a scoped literal search is the unscoped literal
	// search minus other classes, with match kinds untouched.
	literal := mustSearch(t, k, service.SearchQuery{Q: scopeNeedle, Limit: 200})
	scopedLiteral := mustSearch(t, k, service.SearchQuery{
		Q: scopeNeedle, EntityTypes: classNames(domain.SearchClaim), Limit: 200,
	})
	if !reflect.DeepEqual(scopedLiteral.Results, keepClasses(literal, domain.SearchClaim)) {
		t.Errorf("scoped literal results = %v, want the unscoped subset", searchRefs(scopedLiteral))
	}
	if scopedLiteral.QueryMode != "" {
		t.Errorf("a scoped literal search echoed query_mode = %q", scopedLiteral.QueryMode)
	}
}

// TestSearchScopeComposesWithContext requires context resolution to stay a property of the
// returned page and nothing else: the same records, scoped, still resolve the same context.
func TestSearchScopeComposesWithContext(t *testing.T) {
	k := evidenceIndex(t)
	withContext := mustSearch(t, k, service.SearchQuery{
		Q: scopeNeedle, IncludeContext: true, Limit: 200,
	})
	scoped := mustSearch(t, k, service.SearchQuery{
		Q: scopeNeedle, EntityTypes: classNames(domain.SearchClaim), IncludeContext: true, Limit: 200,
	})

	want := keepClasses(withContext, domain.SearchClaim)
	if len(want) == 0 {
		t.Fatal("no claim hit to resolve context for")
	}
	if !reflect.DeepEqual(scoped.Results, want) {
		t.Error("scoped results with context differed from the unscoped subset")
	}
	if !scoped.IncludeContext {
		t.Error("include_context echo lost under a scoped search")
	}
	for _, result := range scoped.Results {
		if result.Context == nil {
			t.Errorf("%s/%s carries no context under include_context", result.EntityType, result.ID)
		}
	}
}

// TestSearchFacetsAreComputedBeforePaging is the facet contract's whole point: a facet that
// moved with the page would describe the page, which is the one thing the caller can already
// count for themselves.
func TestSearchFacetsAreComputedBeforePaging(t *testing.T) {
	k := evidenceIndex(t)
	full := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})
	if full.Page.Total < 3 {
		t.Fatalf("fixture corpus matched %d records; too few to page", full.Page.Total)
	}

	for _, paged := range []service.SearchQuery{
		{Q: universalNeedle, Limit: 1},
		{Q: universalNeedle, Limit: 1, Offset: 1},
		{Q: universalNeedle, Limit: 2, Offset: full.Page.Total - 1},
		// A page past the end of the result set still describes the whole set.
		{Q: universalNeedle, Limit: 5, Offset: full.Page.Total + 10},
	} {
		results := mustSearch(t, k, paged)
		if results.Facets != full.Facets {
			t.Errorf("limit=%d offset=%d facets = %+v, want %+v",
				paged.Limit, paged.Offset, results.Facets, full.Facets)
		}
		if results.Page.Total != full.Page.Total {
			t.Errorf("limit=%d offset=%d total = %d, want %d",
				paged.Limit, paged.Offset, results.Page.Total, full.Page.Total)
		}
		if got := results.Facets.EntityTypes.Total(); got != results.Page.Total {
			t.Errorf("facet sum = %d, page.total = %d", got, results.Page.Total)
		}
		if len(results.Results) > results.Facets.EntityTypes.Total() {
			t.Error("a page carried more results than the facets counted")
		}
	}

	// The same invariant under a scope, where the counts are also the filtered ones.
	scoped := mustSearch(t, k, service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"node", "claim"}, Limit: 200,
	})
	for offset := 0; offset <= scoped.Page.Total; offset++ {
		page := mustSearch(t, k, service.SearchQuery{
			Q: universalNeedle, EntityTypes: []string{"node", "claim"}, Limit: 1, Offset: offset,
		})
		if page.Facets != scoped.Facets {
			t.Fatalf("offset=%d scoped facets = %+v, want %+v", offset, page.Facets, scoped.Facets)
		}
	}
}

// TestSearchFacetsCountEveryClassOfTheResultSet cross-checks the counts against the results
// themselves, so a facet cannot drift from the set it claims to describe.
func TestSearchFacetsCountEveryClassOfTheResultSet(t *testing.T) {
	k := evidenceIndex(t)
	for _, query := range []string{universalNeedle, scopeNeedle, "alpha", "session-01-fixture"} {
		results := mustSearch(t, k, service.SearchQuery{Q: query, Limit: 200})
		counted := map[domain.SearchEntityType]int{}
		for _, result := range results.Results {
			counted[result.EntityType]++
		}
		for _, class := range domain.SearchEntityTypes {
			if got, want := results.Facets.EntityTypes.Count(class), counted[class]; got != want {
				t.Errorf("%q facet %s = %d, want %d", query, class, got, want)
			}
		}
		if got := results.Facets.EntityTypes.Total(); got != results.Page.Total {
			t.Errorf("%q facet sum = %d, page.total = %d", query, got, results.Page.Total)
		}
	}
}

// TestSearchFacetsCoverEveryDeclaredClass holds the facet struct to the closed set. A class
// added to domain.SearchEntityTypes without a facet field fails here rather than silently
// going uncounted and breaking the sum invariant in production.
func TestSearchFacetsCoverEveryDeclaredClass(t *testing.T) {
	for _, class := range domain.SearchEntityTypes {
		facets := domain.NewSearchFacets([]domain.SearchResult{{EntityType: class, ID: "x"}})
		if got := facets.EntityTypes.Count(class); got != 1 {
			t.Errorf("%s counted %d times, want 1", class, got)
		}
		if got := facets.EntityTypes.Total(); got != 1 {
			t.Errorf("%s produced a facet sum of %d, want 1: the class is uncounted", class, got)
		}
	}
}

// TestSearchFacetsOnAnEmptyResultSet: an answered search that found nothing still describes
// what it did not find, in full, so a caller never has to tell an empty facet object from an
// absent one.
func TestSearchFacetsOnAnEmptyResultSet(t *testing.T) {
	k := evidenceIndex(t)

	// Nothing in the corpus matches at all.
	empty := mustSearch(t, k, service.SearchQuery{Q: "no-canonical-record-contains-this-text"})
	// A valid scope the matched records happen not to belong to: the query matches a session
	// and a registry entry, and no node.
	scopedEmpty := mustSearch(t, k, service.SearchQuery{
		Q: "session-01-fixture", EntityTypes: classNames(domain.SearchNode),
	})

	for label, results := range map[string]service.SearchResults{
		"unmatched query": empty, "scoped away": scopedEmpty,
	} {
		if len(results.Results) != 0 || results.Page.Total != 0 {
			t.Fatalf("%s: expected an empty result set, got %v", label, searchRefs(results))
		}
		if results.Facets != (domain.SearchFacets{}) {
			t.Errorf("%s: facets = %+v, want every class at zero", label, results.Facets)
		}
		for _, class := range domain.SearchEntityTypes {
			if got := results.Facets.EntityTypes.Count(class); got != 0 {
				t.Errorf("%s: facet %s = %d, want 0", label, class, got)
			}
		}
	}

	// The scoped-away search still reports which scope produced the empty answer, and the same
	// query without the scope still matches, so the emptiness is the filter's and not the
	// query's.
	if want := classNames(domain.SearchNode); !reflect.DeepEqual(scopedEmpty.EntityTypes, want) {
		t.Errorf("echoed entity_types = %v, want %v", scopedEmpty.EntityTypes, want)
	}
	unscoped := mustSearch(t, k, service.SearchQuery{Q: "session-01-fixture"})
	if unscoped.Page.Total == 0 {
		t.Fatal("the unscoped control query matched nothing")
	}
}

// TestSearchScopeRejectsMalformedLists covers every shape a caller can get wrong. None is
// repaired, and none is silently ignored.
func TestSearchScopeRejectsMalformedLists(t *testing.T) {
	k := evidenceIndex(t)

	for _, tc := range []struct {
		name  string
		types []string
		want  error
	}{
		{"empty member", []string{"node", ""}, service.ErrEmptySearchEntityType},
		{"leading separator", []string{"", "node"}, service.ErrEmptySearchEntityType},
		{"consecutive separators", []string{"node", "", "claim"}, service.ErrEmptySearchEntityType},
		{"whitespace-only member", []string{"node", "   "}, service.ErrEmptySearchEntityType},
		{"only an empty member", []string{""}, service.ErrEmptySearchEntityType},
		{"duplicate", []string{"node", "node"}, service.ErrDuplicateSearchEntityType},
		{"duplicate after trimming", []string{" node ", "node"}, service.ErrDuplicateSearchEntityType},
		{"duplicate among valid values", []string{"claim", "node", "claim"}, service.ErrDuplicateSearchEntityType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := k.Search(service.SearchQuery{Q: scopeNeedle, EntityTypes: tc.types})
			if !errors.Is(err, tc.want) {
				t.Fatalf("entity_types=%v error = %v, want %v", tc.types, err, tc.want)
			}
		})
	}

	// An unknown class is the existing filter error, naming the parameter and the closed set.
	for _, unsupported := range []string{
		"experiment_run", "document", "nodes", "Node", "NODE", "vocab", "run", "graph", "*",
	} {
		t.Run("unsupported/"+unsupported, func(t *testing.T) {
			_, err := k.Search(service.SearchQuery{
				Q: scopeNeedle, EntityTypes: []string{"node", unsupported},
			})
			var invalid *service.InvalidFilterError
			if !errors.As(err, &invalid) {
				t.Fatalf("entity_types member %q error = %v, want InvalidFilterError", unsupported, err)
			}
			if invalid.Param != "entity_types" {
				t.Errorf("rejected parameter = %q, want entity_types", invalid.Param)
			}
			if !reflect.DeepEqual(invalid.Allowed, domain.SearchEntityTypeNames()) {
				t.Errorf("allowed = %v, want the closed searchable set", invalid.Allowed)
			}
			if strings.Contains(invalid.Error(), unsupported) {
				t.Errorf("the service error echoed the caller's value: %q", invalid.Error())
			}
		})
	}
}

// TestSearchScopeRejectsExperimentRuns is a guardrail rather than a feature test. Runs are not
// a searchable class, and a scope that accepted the name would answer with an empty set a
// caller could read as "no run mentions this".
func TestSearchScopeRejectsExperimentRuns(t *testing.T) {
	k := evidenceIndex(t)
	for _, types := range [][]string{
		{"experiment_run"},
		{"experiment", "experiment_run"},
		{"experiment_run", "node"},
	} {
		_, err := k.Search(service.SearchQuery{Q: scopeNeedle, EntityTypes: types})
		var invalid *service.InvalidFilterError
		if !errors.As(err, &invalid) || invalid.Param != "entity_types" {
			t.Fatalf("entity_types=%v error = %v, want an entity_types filter error", types, err)
		}
		if containsString(invalid.Allowed, "experiment_run") {
			t.Fatal("experiment_run is listed as a searchable class")
		}
	}
}

// TestSearchScopeRefusesTheTwoFiltersTogether: type and entity_types are two spellings of one
// restriction, and there is no reading of both at once that is not a guess.
func TestSearchScopeRefusesTheTwoFiltersTogether(t *testing.T) {
	k := evidenceIndex(t)
	for _, tc := range []service.SearchQuery{
		{Q: scopeNeedle, Type: "node", EntityTypes: []string{"node"}},
		{Q: scopeNeedle, Type: "node", EntityTypes: []string{"claim"}},
		{Q: scopeNeedle, Type: "node", EntityTypes: []string{""}},
	} {
		if _, err := k.Search(tc); !errors.Is(err, service.ErrConflictingSearchScope) {
			t.Errorf("type=%q entity_types=%v error = %v, want ErrConflictingSearchScope",
				tc.Type, tc.EntityTypes, err)
		}
	}

	// The single filter alone is unchanged, and produces the same records as the list spelling
	// of the same one class.
	single := mustSearch(t, k, service.SearchQuery{Q: scopeNeedle, Type: "claim", Limit: 200})
	list := mustSearch(t, k, service.SearchQuery{
		Q: scopeNeedle, EntityTypes: []string{"claim"}, Limit: 200,
	})
	if !reflect.DeepEqual(single.Results, list.Results) {
		t.Error("type=claim and entity_types=claim returned different records")
	}
	if single.Facets != list.Facets {
		t.Error("type=claim and entity_types=claim reported different facets")
	}
	// Each echoes only the filter it was given, so two different requests are never confusable.
	if single.Type != "claim" || len(single.EntityTypes) != 0 {
		t.Errorf("type search echoed type=%q entity_types=%v", single.Type, single.EntityTypes)
	}
	if list.Type != "" || !reflect.DeepEqual(list.EntityTypes, []string{"claim"}) {
		t.Errorf("list search echoed type=%q entity_types=%v", list.Type, list.EntityTypes)
	}
}

// TestSearchScopeIsValidatedBeforeTheQuery requires a malformed scope to be refused even when
// the rest of the request is valid, and to be refused whatever else is wrong with the request,
// so scope validation is not reached only through a particular shape of query.
func TestSearchScopeIsValidatedBeforeTheQuery(t *testing.T) {
	k := evidenceIndex(t)
	for _, tc := range []struct {
		name  string
		query service.SearchQuery
	}{
		{"valid query", service.SearchQuery{Q: scopeNeedle, EntityTypes: []string{"node", "node"}}},
		{"empty query", service.SearchQuery{Q: "", EntityTypes: []string{"node", "node"}}},
		{"over-long query", service.SearchQuery{
			Q: strings.Repeat("a", service.MaxQueryChars+1), EntityTypes: []string{"node", "node"},
		}},
		{"composed query", service.SearchQuery{
			Q: "fixture alpha", Mode: string(domain.SearchModeAllTerms),
			EntityTypes: []string{"node", "node"},
		}},
		{"paged request", service.SearchQuery{
			Q: scopeNeedle, EntityTypes: []string{"node", "node"}, Limit: 5, Offset: 5,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results, err := k.Search(tc.query)
			if !errors.Is(err, service.ErrDuplicateSearchEntityType) {
				t.Fatalf("error = %v, want ErrDuplicateSearchEntityType", err)
			}
			if len(results.Results) != 0 || results.Facets != (domain.SearchFacets{}) {
				t.Errorf("a refused request returned %+v", results)
			}
		})
	}
}

// TestSearchScopeDoesNotMutateTheIndex covers the defensive-copy contract on the two values
// Phase 1H added, and then re-runs the searches to require the index itself to be unchanged.
func TestSearchScopeDoesNotMutateTheIndex(t *testing.T) {
	k := evidenceIndex(t)
	query := service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"node", "claim"}, Limit: 200,
	}
	before := mustSearch(t, k, query)
	baseline := searchRefs(before)
	if len(baseline) == 0 {
		t.Fatal("the scoped baseline matched nothing")
	}

	// The caller's own request slice is not retained: mutating it after the call cannot reach
	// a later response.
	query.EntityTypes[0] = "experiment"

	// Nor is the echoed scope, the facets or the results shared with anything.
	before.EntityTypes[0] = "mutated"
	before.Facets.EntityTypes.Node = 9999
	before.Results[0].Title = "mutated"
	before.Results[0].MatchedFields = append(before.Results[0].MatchedFields, "mutated")

	after := mustSearch(t, k, service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"node", "claim"}, Limit: 200,
	})
	if got := searchRefs(after); !reflect.DeepEqual(got, baseline) {
		t.Errorf("results changed after a caller mutated a response: %v, want %v", got, baseline)
	}
	if !reflect.DeepEqual(after.EntityTypes, []string{"node", "claim"}) {
		t.Errorf("echoed entity_types = %v after mutation", after.EntityTypes)
	}
	if after.Facets.EntityTypes.Node == 9999 {
		t.Error("a mutated facet reached the next response")
	}
	for _, result := range after.Results {
		if result.Title == "mutated" || containsString(result.MatchedFields, "mutated") {
			t.Errorf("a mutated result reached the next response: %+v", result)
		}
	}

	// The unscoped projection is unchanged too, so scoping filtered one response rather than
	// removing documents from the index.
	unscoped := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})
	seen := map[domain.SearchEntityType]bool{}
	for _, result := range unscoped.Results {
		seen[result.EntityType] = true
	}
	for _, class := range domain.SearchEntityTypes {
		if !seen[class] {
			t.Errorf("class %s vanished from the unscoped search after a scoped one", class)
		}
	}
}

// TestSearchScopeIsSafeForConcurrentReaders: the index is immutable after New, and scoping
// added no per-request state to it. Concurrent scoped searches must agree with the serial ones.
func TestSearchScopeIsSafeForConcurrentReaders(t *testing.T) {
	k := evidenceIndex(t)
	queries := []service.SearchQuery{
		{Q: universalNeedle, Limit: 200},
		{Q: universalNeedle, EntityTypes: []string{"node"}, Limit: 200},
		{Q: universalNeedle, EntityTypes: []string{"claim", "source"}, Limit: 200},
		{Q: scopeNeedle, EntityTypes: []string{"vocabulary", "experiment"}, Limit: 200},
		{
			Q: "fixture alpha", Mode: string(domain.SearchModeAllTerms),
			EntityTypes: []string{"node", "claim"}, Limit: 200,
		},
	}
	want := make([]service.SearchResults, len(queries))
	for i, query := range queries {
		want[i] = mustSearch(t, k, query)
	}

	var wg sync.WaitGroup
	for round := 0; round < 8; round++ {
		for i, query := range queries {
			wg.Add(1)
			go func(i int, query service.SearchQuery) {
				defer wg.Done()
				got, err := k.Search(query)
				if err != nil {
					t.Errorf("concurrent search %+v: %v", query, err)
					return
				}
				if !reflect.DeepEqual(got.Results, want[i].Results) {
					t.Errorf("concurrent search %+v returned a different result set", query)
				}
				if got.Facets != want[i].Facets {
					t.Errorf("concurrent search %+v returned different facets", query)
				}
			}(i, query)
		}
	}
	wg.Wait()
}

// TestSearchWithoutScopeIsUnchanged is the regression half: a request that names no scope must
// behave exactly as it did before the filter existed, and the facets it now carries must
// describe that same unchanged set.
func TestSearchWithoutScopeIsUnchanged(t *testing.T) {
	k := evidenceIndex(t)
	for _, query := range []service.SearchQuery{
		{Q: scopeNeedle, Limit: 200},
		{Q: scopeNeedle, Type: "vocabulary", Limit: 200},
		{Q: universalNeedle, Limit: 5, Offset: 2},
		{Q: scopeNeedle, IncludeContext: true, Limit: 200},
		{Q: "fixture alpha", Mode: string(domain.SearchModeAllTerms), Limit: 200},
	} {
		results := mustSearch(t, k, query)
		if len(results.EntityTypes) != 0 {
			t.Errorf("%+v echoed entity_types = %v, want none", query, results.EntityTypes)
		}
		if got := results.Facets.EntityTypes.Total(); got != results.Page.Total {
			t.Errorf("%+v facet sum = %d, page.total = %d", query, got, results.Page.Total)
		}
	}

	// Determinism across two independently built indexes, with the scope applied.
	first := mustSearch(t, evidenceIndex(t), service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"claim", "node"}, Limit: 200,
	})
	second := mustSearch(t, evidenceIndex(t), service.SearchQuery{
		Q: universalNeedle, EntityTypes: []string{"claim", "node"}, Limit: 200,
	})
	if !reflect.DeepEqual(first, second) {
		t.Error("two indexes built from one corpus disagreed about a scoped search")
	}
}
