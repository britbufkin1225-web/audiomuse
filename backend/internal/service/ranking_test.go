package service_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// Phase 1I: deterministic relevance ranking and match explainability.
//
// Every assertion in this file is about one of two questions a search response must answer on
// its own — why did this result match, and why is it above that one — and about the property
// that makes both answers worth anything: that the same corpus and the same request produce the
// same ordering and the same explanation every time.

// signalsOf renders one result's explanation compactly for a failure message.
func signalsOf(result domain.SearchResult) string {
	return string(result.EntityType) + "/" + result.ID + " score=" +
		itoa(result.RelevanceScore) + " " + strings.Join(result.MatchSignals, "+")
}

// itoa avoids pulling strconv in for one call site in a failure message.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// resultFor returns one hit by class and ID, failing if the query did not reach it.
func resultFor(t testing.TB, results service.SearchResults, ref string) domain.SearchResult {
	t.Helper()
	for _, result := range results.Results {
		if string(result.EntityType)+"/"+result.ID == ref {
			return result
		}
	}
	t.Fatalf("%s did not appear in %v", ref, searchRefs(results))
	return domain.SearchResult{}
}

// rankOf returns the zero-based position of one hit in a result set.
func rankOf(t testing.TB, results service.SearchResults, ref string) int {
	t.Helper()
	for i, result := range results.Results {
		if string(result.EntityType)+"/"+result.ID == ref {
			return i
		}
	}
	t.Fatalf("%s did not appear in %v", ref, searchRefs(results))
	return -1
}

// TestRankingExactIDOutranksEveryWeakerMatch is the strongest signal in one assertion. A caller
// who typed a canonical ID typed an address, and the record at that address is the answer no
// matter how much text any other record carries.
func TestRankingExactIDOutranksEveryWeakerMatch(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "alpha", Limit: 200})

	first := results.Results[0]
	if got := string(first.EntityType) + "/" + first.ID; got != "node/alpha" {
		t.Fatalf("first result = %q, want node/alpha: %v", got, searchRefs(results))
	}
	if !containsString(first.MatchSignals, domain.SignalIDExact) {
		t.Errorf("node/alpha signals = %v, want %s", first.MatchSignals, domain.SignalIDExact)
	}
	// Not merely first: strictly above every other hit, so the position is the score's doing
	// rather than a tie the class order happened to settle.
	for _, other := range results.Results[1:] {
		if other.RelevanceScore >= first.RelevanceScore {
			t.Errorf("%s scored %d against the exact-ID hit's %d",
				signalsOf(other), other.RelevanceScore, first.RelevanceScore)
		}
		if containsString(other.MatchSignals, domain.SignalIDExact) {
			t.Errorf("%s claims an exact ID match it does not have", signalsOf(other))
		}
	}
}

// TestRankingExactNameOutranksDescriptiveText asserts the second tier: a record the query names
// beats a record that merely mentions the query in its prose.
func TestRankingExactNameOutranksDescriptiveText(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "Fixture Term", Limit: 200})

	named := resultFor(t, results, "vocabulary/fixture-term")
	if !containsString(named.MatchSignals, domain.SignalTitleExact) {
		t.Fatalf("vocabulary/fixture-term signals = %v, want %s",
			named.MatchSignals, domain.SignalTitleExact)
	}
	if rank := rankOf(t, results, "vocabulary/fixture-term"); rank != 0 {
		t.Errorf("the exact-name hit ranked %d, want 0: %v", rank, searchRefs(results))
	}
	for _, other := range results.Results {
		if other.ID == named.ID && other.EntityType == named.EntityType {
			continue
		}
		if other.RelevanceScore >= named.RelevanceScore {
			t.Errorf("%s matched only descriptive text yet scored %d against the exact name's %d",
				signalsOf(other), other.RelevanceScore, named.RelevanceScore)
		}
	}
}

// TestRankingNamePrefixOutranksInteriorNameMatch is the signal Phase 1I adds inside an existing
// match kind. Both records below are title_substring hits and were previously ordered only by
// class and ID; a reader typing the start of a name means that name, and the record whose name
// begins with it is now the better answer.
func TestRankingNamePrefixOutranksInteriorNameMatch(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	// "Fixture Listening Exercise" begins with the query. "A Fixture Reference Work" carries it
	// in the middle. Both are title_substring, so the coarse class cannot separate them.
	prefix := resultFor(t, results, "experiment/fixture-listening-exercise")
	interior := resultFor(t, results, "source/fixture-reference-work")

	for _, tc := range []struct {
		result domain.SearchResult
		want   string
	}{
		{prefix, domain.SignalTitlePrefix},
		{interior, domain.SignalTitleSubstring},
	} {
		if !containsString(tc.result.MatchSignals, tc.want) {
			t.Errorf("%s is missing %s", signalsOf(tc.result), tc.want)
		}
		if tc.result.MatchKind != domain.MatchTitleSubstring {
			t.Errorf("%s match_kind = %q, want %q; the coarse class must not have changed",
				signalsOf(tc.result), tc.result.MatchKind, domain.MatchTitleSubstring)
		}
	}
	if prefix.RelevanceScore <= interior.RelevanceScore {
		t.Errorf("prefix hit %s did not outscore interior hit %s",
			signalsOf(prefix), signalsOf(interior))
	}
	if rankOf(t, results, "experiment/fixture-listening-exercise") >=
		rankOf(t, results, "source/fixture-reference-work") {
		t.Errorf("the prefix hit did not sort above the interior hit: %v", searchRefs(results))
	}
}

// TestRankingIdentityFieldOutranksProseOnly asserts the low tier: text inside a canonical ID is
// a stronger indication than the same text inside a description, and both are far below a name.
func TestRankingIdentityFieldOutranksProseOnly(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	// This source's ID carries the query and nothing else does; it is the weakest hit the query
	// produces, and it is still a hit rather than a discarded record.
	weakest := resultFor(t, results, "source/fixture-uncited-source")
	if want := []string{domain.SignalIDSubstring}; !reflect.DeepEqual(weakest.MatchSignals, want) {
		t.Errorf("signals = %v, want %v", weakest.MatchSignals, want)
	}
	if last := results.Results[len(results.Results)-1]; last.ID != weakest.ID {
		t.Errorf("the ID-only hit is not last: %v", searchRefs(results))
	}
	if weakest.MatchKind != domain.MatchFieldSubstring {
		t.Errorf("match_kind = %q, want %q", weakest.MatchKind, domain.MatchFieldSubstring)
	}
}

// TestRankingPhraseOutranksScatteredTerms is the composed-mode signal. Two records carry the
// caller's words side by side; a third carries both words in the same field with other words
// between them. Contiguity is a stronger statement about the text, and nothing else separates
// the three.
func TestRankingPhraseOutranksScatteredTerms(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, allTerms("synthetic entry"))

	contiguous := []string{"vocabulary/fixture-companion", "vocabulary/fixture-orphan-term"}
	for _, ref := range contiguous {
		result := resultFor(t, results, ref)
		if !containsString(result.MatchSignals, domain.SignalPhraseMatch) {
			t.Errorf("%s is missing %s", signalsOf(result), domain.SignalPhraseMatch)
		}
	}
	scattered := resultFor(t, results, "vocabulary/fixture-term")
	if containsString(scattered.MatchSignals, domain.SignalPhraseMatch) {
		t.Fatalf("%s claims a contiguous phrase it does not carry", signalsOf(scattered))
	}
	for _, ref := range contiguous {
		if resultFor(t, results, ref).RelevanceScore <= scattered.RelevanceScore {
			t.Errorf("%s did not outscore the scattered hit %s",
				signalsOf(resultFor(t, results, ref)), signalsOf(scattered))
		}
		if rankOf(t, results, ref) >= rankOf(t, results, "vocabulary/fixture-term") {
			t.Errorf("the phrase hits did not sort above the scattered hit: %v", searchRefs(results))
		}
	}
}

// TestRankingCompleteTermCoverageOutranksPartial is the token-coverage tier: a record whose name
// carries every one of the caller's words beats one whose name carries some of them, and both
// beat nothing. It is a different question from contiguity, and this is where they separate.
func TestRankingCompleteTermCoverageOutranksPartial(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, allTerms("fixture term"))

	exact := resultFor(t, results, "vocabulary/fixture-term")           // name is the query
	complete := resultFor(t, results, "vocabulary/fixture-orphan-term") // name carries both
	partial := resultFor(t, results, "vocabulary/fixture-companion")    // name carries one
	for _, tc := range []struct {
		result domain.SearchResult
		want   string
	}{
		{exact, domain.SignalTitleExact},
		{complete, domain.SignalTitleAllTerms},
		{partial, domain.SignalTitleTerms},
	} {
		if !containsString(tc.result.MatchSignals, tc.want) {
			t.Errorf("%s is missing %s", signalsOf(tc.result), tc.want)
		}
	}
	// The two coverage signals are mutually exclusive: complete coverage is never also reported
	// as partial coverage.
	if containsString(complete.MatchSignals, domain.SignalTitleTerms) {
		t.Errorf("%s reports complete and partial coverage at once", signalsOf(complete))
	}
	if exact.RelevanceScore <= complete.RelevanceScore ||
		complete.RelevanceScore <= partial.RelevanceScore {
		t.Errorf("coverage did not order the hits: %s, %s, %s",
			signalsOf(exact), signalsOf(complete), signalsOf(partial))
	}
	want := []string{
		"vocabulary/fixture-term", "vocabulary/fixture-orphan-term",
		"vocabulary/fixture-companion", "experiment/fixture-visualization-exercise",
	}
	if got := searchRefs(results); !reflect.DeepEqual(got, want) {
		t.Errorf("ordering = %v, want %v", got, want)
	}
}

// TestRankingTiesBreakByClassThenID pins the documented tie-break. Equal relevance is settled by
// the canonical class order and then the canonical ID, never by the order the loader happened to
// produce records in.
func TestRankingTiesBreakByClassThenID(t *testing.T) {
	k := evidenceIndex(t)
	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	var ties int
	for i := 1; i < len(results.Results); i++ {
		previous, current := results.Results[i-1], results.Results[i]
		if previous.RelevanceScore != current.RelevanceScore {
			if previous.RelevanceScore < current.RelevanceScore {
				t.Fatalf("scores ascended at %d: %v", i, searchRefs(results))
			}
			continue
		}
		ties++
		previousRank := domain.SearchEntityRank(previous.EntityType)
		currentRank := domain.SearchEntityRank(current.EntityType)
		if previousRank > currentRank {
			t.Errorf("a %s tie left canonical class order at %d: %v",
				previous.MatchKind, i, searchRefs(results))
		}
		if previousRank == currentRank && previous.ID >= current.ID {
			t.Errorf("a same-class tie left canonical ID order at %d: %v", i, searchRefs(results))
		}
	}
	if ties == 0 {
		t.Fatal("the query produced no equal-score pair, so the tie-break was never exercised")
	}

	// The whole ordering, spelled out. Relevance first — three vocabulary entries precede a
	// session even though session is the first canonical class — then the class order and the ID.
	want := []string{
		"vocabulary/fixture-companion", "vocabulary/fixture-orphan-term", "vocabulary/fixture-term",
		"session/session-01-fixture", "source/session-01-fixture",
		"experiment/fixture-listening-exercise", "experiment/fixture-visualization-exercise",
		"session/session-02-unused", "source/session-02-unused",
		"source/fixture-archive-record", "source/fixture-reference-work",
		"source/fixture-attribution-source",
		"claim/alpha-carries-energy", "claim/alpha-may-extend-to-gamma",
		"claim/beta-was-observed-in-1999", "claim/gamma-follows-from-alpha-and-beta",
		"source/fixture-uncited-source",
	}
	if got := searchRefs(results); !reflect.DeepEqual(got, want) {
		t.Errorf("ordering = %v, want %v", got, want)
	}
}

// TestRankingIsStableAcrossRepeatedRuns is the determinism assertion, and the one that catches an
// ordering that depends on Go map iteration: a second index built from the same corpus must rank
// identically, not merely return the same set.
func TestRankingIsStableAcrossRepeatedRuns(t *testing.T) {
	first := evidenceIndex(t)
	second := evidenceIndex(t)

	queries := []service.SearchQuery{
		{Q: "fixture", Limit: 200},
		{Q: "alpha", Limit: 200},
		{Q: "e", Limit: 200},
		{Q: "fixture", EntityTypes: []string{"source", "vocabulary"}, Limit: 200},
		{Q: "fixture", IncludeContext: true, Limit: 200},
		allTerms("fixture term"),
		allTerms("synthetic entry"),
	}
	for _, query := range queries {
		t.Run(query.Q+"/"+query.Mode, func(t *testing.T) {
			a := mustSearch(t, first, query)
			for i := 0; i < 4; i++ {
				if got := mustSearch(t, first, query); !reflect.DeepEqual(a, got) {
					t.Fatalf("run %d differed on one index:\n%v\n%v", i, searchRefs(a), searchRefs(got))
				}
			}
			if got := mustSearch(t, second, query); !reflect.DeepEqual(a, got) {
				t.Fatalf("two indexes over one corpus disagreed:\n%v\n%v",
					searchRefs(a), searchRefs(got))
			}
		})
	}
}

// TestRankingScoreIsTheSumOfItsSignals asserts the arithmetic guarantee across the whole corpus,
// in both modes: a client can add up match_signals and recover relevance_score, so the
// explanation and the ordering can never describe different searches.
func TestRankingScoreIsTheSumOfItsSignals(t *testing.T) {
	k := evidenceIndex(t)
	for _, query := range []service.SearchQuery{
		{Q: "e", Limit: 200},
		{Q: "fixture", Limit: 200},
		{Q: "alpha", Limit: 200},
		allTerms("fixture term"),
		allTerms("synthetic entry"),
		allTerms("acoustics synthetic"),
	} {
		results := mustSearch(t, k, query)
		if len(results.Results) == 0 {
			t.Fatalf("%+v matched nothing, so nothing was checked", query)
		}
		for _, result := range results.Results {
			if want := domain.SearchRelevanceScore(result.MatchSignals); result.RelevanceScore != want {
				t.Errorf("%s: score %d is not the sum of its signals (%d)",
					signalsOf(result), result.RelevanceScore, want)
			}
			if result.RelevanceScore <= 0 {
				t.Errorf("%s scored nothing; a record that matched nothing is not a result",
					signalsOf(result))
			}
		}
	}
}

// TestRankingSignalsAreOrderedUniqueAndClosed holds the explanation itself to a contract: every
// name comes from the closed vocabulary, none repeats, and the list is always in the declared
// weight order so two results that matched the same way describe themselves identically.
func TestRankingSignalsAreOrderedUniqueAndClosed(t *testing.T) {
	k := evidenceIndex(t)
	rank := map[string]int{}
	for i, signal := range domain.SearchMatchSignals {
		rank[signal] = i
	}

	for _, query := range []service.SearchQuery{
		{Q: "e", Limit: 200},
		{Q: "fixture", Limit: 200},
		{Q: "Fixture Term", Limit: 200},
		allTerms("fixture term"),
		allTerms("synthetic entry"),
	} {
		for _, result := range mustSearch(t, k, query).Results {
			if len(result.MatchSignals) == 0 {
				t.Errorf("%s carries no signals", signalsOf(result))
			}
			seen := map[string]bool{}
			previous := -1
			for _, signal := range result.MatchSignals {
				position, known := rank[signal]
				if !known {
					t.Errorf("%s emitted %q, which is not in the closed signal set",
						signalsOf(result), signal)
					continue
				}
				if seen[signal] {
					t.Errorf("%s emitted %q twice", signalsOf(result), signal)
				}
				seen[signal] = true
				if position <= previous {
					t.Errorf("%s emitted its signals out of weight order", signalsOf(result))
				}
				previous = position
			}
			// The mutually exclusive tiers, asserted as tiers rather than one pair at a time.
			for _, tier := range [][]string{
				{domain.SignalIDExact, domain.SignalIDSubstring},
				{domain.SignalTitleExact, domain.SignalTitlePrefix, domain.SignalTitleSubstring},
				{domain.SignalTitleAllTerms, domain.SignalTitleTerms},
			} {
				var fired int
				for _, signal := range tier {
					if seen[signal] {
						fired++
					}
				}
				if fired > 1 {
					t.Errorf("%s fired %d signals of the mutually exclusive tier %v",
						signalsOf(result), fired, tier)
				}
			}
		}
	}
}

// TestRankingLiteralModeEmitsNoComposedSignals asserts the two composed-only signals never appear
// on a literal hit. A literal hit is a contiguous occurrence by definition, so reporting a phrase
// signal would restate the mode instead of distinguishing the result, and there are no terms to
// have covered anything.
func TestRankingLiteralModeEmitsNoComposedSignals(t *testing.T) {
	k := evidenceIndex(t)
	composedOnly := []string{
		domain.SignalPhraseMatch, domain.SignalTitleAllTerms, domain.SignalTitleTerms,
	}
	for _, query := range []string{"e", "fixture", "alpha", "Fixture Term", "synthetic entry"} {
		for _, result := range mustSearch(t, k, service.SearchQuery{Q: query, Limit: 200}).Results {
			for _, signal := range composedOnly {
				if containsString(result.MatchSignals, signal) {
					t.Errorf("literal query %q produced %s carrying the composed-only signal %q",
						query, signalsOf(result), signal)
				}
			}
		}
	}
}

// TestMatchKindIsDerivedFromSignals asserts the Phase 1E coarse class is now a summary of the
// signals rather than a second, independent classification that could drift away from them.
func TestMatchKindIsDerivedFromSignals(t *testing.T) {
	k := evidenceIndex(t)
	for _, query := range []string{"e", "fixture", "alpha", "Fixture Term", "listening", "resonant"} {
		results := mustSearch(t, k, service.SearchQuery{Q: query, Limit: 200})
		for _, result := range results.Results {
			var want string
			switch {
			case containsString(result.MatchSignals, domain.SignalIDExact):
				want = domain.MatchIDExact
			case containsString(result.MatchSignals, domain.SignalTitleExact):
				want = domain.MatchTitleExact
			case containsString(result.MatchSignals, domain.SignalTitlePrefix),
				containsString(result.MatchSignals, domain.SignalTitleSubstring):
				want = domain.MatchTitleSubstring
			default:
				want = domain.MatchFieldSubstring
			}
			if result.MatchKind != want {
				t.Errorf("%s: match_kind = %q, want %q from its signals",
					signalsOf(result), result.MatchKind, want)
			}
		}
		// The Phase 1E precedence still holds as an ordering, which is what makes the score a
		// refinement of the existing contract rather than a replacement for it.
		previous := -1
		for _, result := range results.Results {
			current := map[string]int{
				domain.MatchIDExact: 0, domain.MatchTitleExact: 1,
				domain.MatchTitleSubstring: 2, domain.MatchFieldSubstring: 3,
			}[result.MatchKind]
			if current < previous {
				t.Fatalf("query %q: match kinds left precedence order: %v", query, searchRefs(results))
			}
			previous = current
		}
	}
}

// TestRankingNormalisationMatchesTheQueryThatRan asserts ranking evaluates the same normalised
// query matching does. Case, surrounding and repeated whitespace, and a repeated term are all
// already normalised away before matching, and none of them may reach the score.
func TestRankingNormalisationMatchesTheQueryThatRan(t *testing.T) {
	k := evidenceIndex(t)

	literal := []struct {
		name    string
		queries []string
	}{
		{"case", []string{"fixture", "FIXTURE", "FiXtUrE"}},
		{"surrounding whitespace", []string{"fixture", "  fixture", "fixture  ", "\tfixture\n"}},
		{"exact name in another case", []string{"Fixture Term", "fixture term", "FIXTURE TERM"}},
	}
	for _, tc := range literal {
		t.Run(tc.name, func(t *testing.T) {
			base := mustSearch(t, k, service.SearchQuery{Q: tc.queries[0], Limit: 200})
			for _, query := range tc.queries[1:] {
				got := mustSearch(t, k, service.SearchQuery{Q: query, Limit: 200})
				if !reflect.DeepEqual(base, got) {
					t.Errorf("%q ranked differently from %q:\n%v\n%v",
						query, tc.queries[0], searchRefs(base), searchRefs(got))
				}
			}
		})
	}

	composed := []struct {
		name    string
		queries []string
	}{
		{"case and repeated whitespace", []string{"fixture term", "FIXTURE   term", " Fixture\tTerm "}},
		{"a repeated term", []string{"fixture term", "fixture fixture term", "fixture term term"}},
	}
	for _, tc := range composed {
		t.Run(tc.name, func(t *testing.T) {
			base := mustSearch(t, k, allTerms(tc.queries[0]))
			for _, query := range tc.queries[1:] {
				got := mustSearch(t, k, allTerms(query))
				if !reflect.DeepEqual(base, got) {
					t.Errorf("%q ranked differently from %q:\n%v\n%v",
						query, tc.queries[0], searchRefs(base), searchRefs(got))
				}
			}
		})
	}

	// Term order is not normalised away, and that is deliberate rather than an oversight. The
	// match set is order-independent — every term must occur somewhere in the record either way —
	// but the phrase-derived signals are statements about the words the caller wrote side by
	// side, and "term fixture" is not the phrase "fixture term". The response echoes the term
	// list in the caller's own order, so a client can always see which phrase was ranked.
	forward := mustSearch(t, k, allTerms("fixture term"))
	reversed := mustSearch(t, k, allTerms("term fixture"))
	forwardSet, reversedSet := searchRefs(forward), searchRefs(reversed)
	sort.Strings(forwardSet)
	sort.Strings(reversedSet)
	if !reflect.DeepEqual(forwardSet, reversedSet) {
		t.Errorf("reordering the terms changed which records matched: %v vs %v",
			forwardSet, reversedSet)
	}
	if !containsString(resultFor(t, forward, "vocabulary/fixture-term").MatchSignals,
		domain.SignalTitleExact) {
		t.Error(`"fixture term" no longer names vocabulary/fixture-term exactly`)
	}
	if containsString(resultFor(t, reversed, "vocabulary/fixture-term").MatchSignals,
		domain.SignalTitleExact) {
		t.Error(`"term fixture" was ranked as an exact name match for a name it does not spell`)
	}
	if echo := reversed.Query; echo != "term fixture" {
		t.Errorf("the response echoed %q rather than the term order that was ranked", echo)
	}

	// An empty and a whitespace-only query are still refused rather than ranked.
	for _, query := range []string{"", "   ", "\t\n"} {
		if _, err := k.Search(service.SearchQuery{Q: query, Limit: 200}); err == nil {
			t.Errorf("query %q was ranked rather than refused", query)
		}
	}
}

// TestRankingComposesWithScopeAndFacets is the Phase 1H integration. Scoping removes classes and
// changes nothing else: not a score, not a signal, and not the relative order of what it keeps.
// Facets keep describing the complete filtered set, which is what makes them stable across pages.
func TestRankingComposesWithScopeAndFacets(t *testing.T) {
	k := evidenceIndex(t)
	unscoped := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})

	for _, tc := range []struct {
		name  string
		query service.SearchQuery
		keep  []domain.SearchEntityType
	}{
		{
			"one class",
			service.SearchQuery{Q: "fixture", EntityTypes: []string{"source"}, Limit: 200},
			[]domain.SearchEntityType{domain.SearchSource},
		},
		{
			"several classes",
			service.SearchQuery{Q: "fixture", EntityTypes: []string{"vocabulary", "claim"}, Limit: 200},
			[]domain.SearchEntityType{domain.SearchClaim, domain.SearchVocabulary},
		},
		{
			"the single-class filter",
			service.SearchQuery{Q: "fixture", Type: "experiment", Limit: 200},
			[]domain.SearchEntityType{domain.SearchExperiment},
		},
		{
			"every class",
			service.SearchQuery{Q: "fixture", EntityTypes: []string{
				"session", "node", "claim", "source", "vocabulary", "experiment"}, Limit: 200},
			domain.SearchEntityTypes,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scoped := mustSearch(t, k, tc.query)
			if want := keepClasses(unscoped, tc.keep...); !reflect.DeepEqual(scoped.Results, want) {
				t.Errorf("scoped results are not the unscoped ones with other classes dropped:\n%v\n%v",
					searchRefs(scoped), refsOf(want))
			}
			if got := scoped.Facets.EntityTypes.Total(); got != scoped.Page.Total {
				t.Errorf("facet sum = %d, page.total = %d", got, scoped.Page.Total)
			}
			for _, class := range domain.SearchEntityTypes {
				var want int
				for _, result := range scoped.Results {
					if result.EntityType == class {
						want++
					}
				}
				if got := scoped.Facets.EntityTypes.Count(class); got != want {
					t.Errorf("%s facet = %d, want %d", class, got, want)
				}
			}
		})
	}

	// A scope that matches nothing still answers, with a zero-valued breakdown and no results to
	// rank. Ranking must not turn an empty answer into an error or a missing facet object.
	empty := mustSearch(t, k, service.SearchQuery{
		Q: "resonant", EntityTypes: []string{"experiment", "session"}, Limit: 200,
	})
	if len(empty.Results) != 0 || empty.Page.Total != 0 {
		t.Errorf("a zero-result scope returned %v", searchRefs(empty))
	}
	if empty.Facets.EntityTypes.Total() != 0 {
		t.Errorf("a zero-result scope reported facets %+v", empty.Facets)
	}
}

// TestRankingPrecedesPaging asserts the stage order: the whole match set is ranked before the
// page window is applied, so page one holds the best hits rather than the first ones the loader
// produced, and walking every page reproduces the ranked set exactly.
func TestRankingPrecedesPaging(t *testing.T) {
	k := evidenceIndex(t)
	full := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})
	if full.Page.Total < 9 {
		t.Fatalf("the fixture corpus matched %d records, too few to page", full.Page.Total)
	}

	// The first page is the top of the ranking, not an arbitrary window of it.
	firstPage := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 3})
	if got, want := searchRefs(firstPage), searchRefs(full)[:3]; !reflect.DeepEqual(got, want) {
		t.Errorf("page one = %v, want the top of the ranking %v", got, want)
	}
	best := full.Results[0].RelevanceScore
	for _, result := range full.Results[3:] {
		if result.RelevanceScore > best {
			t.Errorf("%s outscores page one and was paged out of reach", signalsOf(result))
		}
	}

	// Walking every page reproduces the ranking, and every page reports the same total and the
	// same facets: the counts describe the ranked set, never the window.
	var walked []string
	for offset := 0; offset < full.Page.Total; offset += 4 {
		page := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 4, Offset: offset})
		if page.Page.Total != full.Page.Total {
			t.Errorf("offset %d reported total %d, want %d", offset, page.Page.Total, full.Page.Total)
		}
		if page.Facets != full.Facets {
			t.Errorf("offset %d reported facets %+v, want %+v", offset, page.Facets, full.Facets)
		}
		walked = append(walked, searchRefs(page)...)
	}
	if want := searchRefs(full); !reflect.DeepEqual(walked, want) {
		t.Errorf("walked pages = %v, want %v", walked, want)
	}

	// Scores never ascend down the ranked set, on any page of it.
	if !sort.SliceIsSorted(full.Results, func(i, j int) bool {
		return full.Results[i].RelevanceScore > full.Results[j].RelevanceScore
	}) {
		t.Errorf("relevance ascended somewhere in the ranked set: %v", searchRefs(full))
	}
}

// TestRankingComposesWithContext is the Phase 1F integration. Context is resolved after ranking
// and paging, so requesting it may only add a Context object: it cannot change a score, a signal
// or a position, and it can never satisfy a match.
func TestRankingComposesWithContext(t *testing.T) {
	k := evidenceIndex(t)
	for _, query := range []service.SearchQuery{
		{Q: "fixture", Limit: 200},
		{Q: "alpha", Limit: 200},
		allTerms("fixture term"),
	} {
		plain := mustSearch(t, k, query)
		query.IncludeContext = true
		enriched := mustSearch(t, k, query)

		if len(plain.Results) != len(enriched.Results) {
			t.Fatalf("context changed the result count: %d vs %d",
				len(plain.Results), len(enriched.Results))
		}
		for i := range plain.Results {
			stripped := enriched.Results[i]
			if stripped.Context == nil && plain.Results[i].Context != nil {
				t.Errorf("result %d lost its context", i)
			}
			stripped.Context = nil
			if !reflect.DeepEqual(stripped, plain.Results[i]) {
				t.Errorf("context changed result %d:\n%s\n%s",
					i, signalsOf(stripped), signalsOf(plain.Results[i]))
			}
		}
	}
}

// TestRankingExplanationLeaksNothing is the provenance guard for the two fields Phase 1I added,
// and is named by TestSearchDoesNotFlattenProvenance. A signal name is drawn from a closed
// compile-time vocabulary and a score is an integer, so neither can carry text from this record
// or from any record it references. This asserts that rather than assuming it.
func TestRankingExplanationLeaksNothing(t *testing.T) {
	k := evidenceIndex(t)
	for _, query := range []service.SearchQuery{
		{Q: "e", Limit: 200, IncludeContext: true},
		{Q: "fixture", Limit: 200, IncludeContext: true},
		allTerms("fixture term"),
	} {
		for _, result := range mustSearch(t, k, query).Results {
			for _, signal := range result.MatchSignals {
				if !containsString(domain.SearchMatchSignals, signal) {
					t.Errorf("%s emitted %q, which is outside the closed vocabulary",
						signalsOf(result), signal)
				}
				// Nothing of the record, the query or the corpus may appear in an explanation.
				if strings.Contains(signal, result.ID) || strings.Contains(signal, result.Title) {
					t.Errorf("%s carries record content inside a signal name", signalsOf(result))
				}
			}
		}
	}
}
