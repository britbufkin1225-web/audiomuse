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

// Phase 1G: deterministic multi-term query composition.
//
// The distinction every test in this file protects is that a record containing two terms is a
// fact about text, not about meaning. all_terms composes several literal substring tests over
// one record; it does not stem, score, rank, expand or relate, and the assertions below are
// written so that any of those creeping in fails loudly rather than looking like an improvement.

// allTerms builds a composed query.
func allTerms(q string) service.SearchQuery {
	return service.SearchQuery{Q: q, Mode: string(domain.SearchModeAllTerms), Limit: 200}
}

// TestLiteralSearchIsUnchangedByQueryMode is the backward-compatibility assertion.
//
// An omitted mode and an explicit literal mode must both produce the Phase 1E result, and they
// must produce it identically: whitespace inside q is part of the phrase in both, no
// query_mode key is echoed by either, and no result carries multi-term evidence. A caller who
// never heard of Phase 1G must not be able to tell it shipped.
func TestLiteralSearchIsUnchangedByQueryMode(t *testing.T) {
	k := evidenceIndex(t)

	for _, q := range []string{
		"fixture", "alpha", "session-01-fixture", "Fixture Term",
		// The load-bearing case: two words. Under Phase 1E this is one phrase, and it must
		// stay one phrase. "synthetic outbound" appears contiguously in no canonical field,
		// while three records carry both words separately — so a whitespace-as-AND regression
		// turns this from an empty result set into three hits.
		"synthetic outbound", "outbound edges",
	} {
		t.Run(q, func(t *testing.T) {
			omitted := mustSearch(t, k, service.SearchQuery{Q: q, Limit: 200})
			explicit := mustSearch(t, k, service.SearchQuery{
				Q: q, Mode: string(domain.SearchModeLiteral), Limit: 200,
			})
			if !reflect.DeepEqual(omitted, explicit) {
				t.Fatalf("explicit literal mode differed from an omitted one:\n%+v\n%+v", omitted, explicit)
			}
			if omitted.QueryMode != "" {
				t.Errorf("literal response echoed query_mode = %q, want it absent", omitted.QueryMode)
			}
			for _, result := range omitted.Results {
				if result.TermMatches != nil {
					t.Errorf("literal result %s/%s carries term_matches", result.EntityType, result.ID)
				}
				if result.MatchKind == domain.MatchAllTerms {
					t.Errorf("literal result %s/%s carries the composed match kind", result.EntityType, result.ID)
				}
			}
		})
	}

	// The phrase is genuinely contiguous-only: two words that no single canonical value carries
	// side by side match nothing at all under literal search.
	if got := mustSearch(t, k, service.SearchQuery{Q: "synthetic outbound", Limit: 200}); got.Page.Total != 0 {
		t.Errorf("literal %q matched %d records: %v", "synthetic outbound", got.Page.Total, searchRefs(got))
	}
	// And the same two words compose into three hits once the caller asks for it, which is the
	// whole of what the phase adds.
	if got := mustSearch(t, k, allTerms("synthetic outbound")); got.Page.Total != 3 {
		t.Errorf("all_terms %q matched %d records, want 3: %v", "synthetic outbound", got.Page.Total, searchRefs(got))
	}
}

// TestLiteralMatchKindOrderingSurvives re-pins the Phase 1E precedence after the second mode
// was added, so a refactor that routed literal search through the composed path is caught here
// rather than by a client noticing its ordering changed.
func TestLiteralMatchKindOrderingSurvives(t *testing.T) {
	k := evidenceIndex(t)

	results := mustSearch(t, k, service.SearchQuery{Q: "fixture", Limit: 200})
	rank := map[string]int{
		domain.MatchIDExact: 0, domain.MatchTitleExact: 1,
		domain.MatchTitleSubstring: 2, domain.MatchFieldSubstring: 3,
	}
	previous := -1
	for _, result := range results.Results {
		current, ok := rank[result.MatchKind]
		if !ok {
			t.Fatalf("literal result carries match_kind %q", result.MatchKind)
		}
		if current < previous {
			t.Fatalf("match_kind %q followed a lower-precedence result: %v",
				result.MatchKind, searchRefs(results))
		}
		previous = current
	}

	// The four classes still classify the same records, and matched_fields is still the
	// record's own field order.
	exact := mustSearch(t, k, service.SearchQuery{Q: "fixture-term", Limit: 200})
	if exact.Results[0].MatchKind != domain.MatchIDExact {
		t.Errorf("exact id match_kind = %q", exact.Results[0].MatchKind)
	}
	for _, result := range results.Results {
		if string(result.EntityType)+"/"+result.ID != "vocabulary/fixture-term" {
			continue
		}
		want := []string{"id", "term", "digital_relationship", "technologies", "tags"}
		if !reflect.DeepEqual(result.MatchedFields, want) {
			t.Errorf("matched_fields = %v, want %v", result.MatchedFields, want)
		}
	}
}

// TestAllTermsMatchesWithinOneRecord covers the shapes a composed hit can take.
func TestAllTermsMatchesWithinOneRecord(t *testing.T) {
	k := evidenceIndex(t)

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{
			// Both terms inside one canonical field, non-contiguously. Each node definition
			// reads "A synthetic ... outbound ...", which no literal phrase reaches.
			"two terms in one field", "synthetic outbound",
			[]string{"node/alpha", "node/beta", "node/gamma"},
		},
		{
			// The case the phase exists for: one term in the record's identity fields and the
			// other in a different canonical field of the same record.
			"terms in separate fields", "alpha acoustics",
			[]string{"node/alpha"},
		},
		{
			// Terms spanning the class boundary: "acoustics" is a node domain and a vocabulary
			// domain, "synthetic" is prose in both.
			"terms across two classes", "acoustics synthetic",
			[]string{"node/alpha", "node/beta", "vocabulary/fixture-orphan-term", "vocabulary/fixture-term"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := searchRefs(mustSearch(t, k, allTerms(tc.query)))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("all_terms %q = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

// TestAllTermsRejectsARecordMissingATerm asserts a partial match is not a match.
func TestAllTermsRejectsARecordMissingATerm(t *testing.T) {
	k := evidenceIndex(t)

	// node/alpha carries "alpha" and "acoustics" and is a hit; add a third term it does not
	// carry and it must drop out entirely rather than being reported as a weaker hit.
	if got := searchRefs(mustSearch(t, k, allTerms("alpha acoustics"))); !reflect.DeepEqual(got, []string{"node/alpha"}) {
		t.Fatalf("baseline = %v, want node/alpha", got)
	}
	for _, absent := range []string{"resonant", "1999", "listening", "no-such-token"} {
		got := mustSearch(t, k, allTerms("alpha acoustics "+absent))
		if got.Page.Total != 0 {
			t.Errorf("all_terms with the unmatched term %q returned %v", absent, searchRefs(got))
		}
	}
}

// TestAllTermsDoesNotCombineSeparateRecords is the boundary the composition must not cross.
//
// Two terms that each match something, in different records, compose to nothing. A retrieval
// layer that returned a hit here would be asserting that two canonical records are one, which
// is a relationship claim the corpus did not make.
func TestAllTermsDoesNotCombineSeparateRecords(t *testing.T) {
	k := evidenceIndex(t)

	cases := []struct {
		a, b string
	}{
		// "resonant" is only in node/beta's definition; "dsp" is only in node/gamma's domain
		// and in a vocabulary entry's domain. No record carries both.
		{"resonant", "dsp"},
		// "acoustics" is a node and vocabulary domain; "1999" is only in a claim.
		{"acoustics", "1999"},
	}
	for _, tc := range cases {
		t.Run(tc.a+"+"+tc.b, func(t *testing.T) {
			// Each term alone finds something, so the empty composed result is a real
			// intersection rather than a typo.
			for _, term := range []string{tc.a, tc.b} {
				if mustSearch(t, k, service.SearchQuery{Q: term, Limit: 200}).Page.Total == 0 {
					t.Fatalf("term %q matches nothing on its own; the assertion would be vacuous", term)
				}
			}
			got := mustSearch(t, k, allTerms(tc.a+" "+tc.b))
			if got.Page.Total != 0 {
				t.Errorf("terms held by different records combined into %v", searchRefs(got))
			}
		})
	}
}

// TestAllTermsCannotBeSatisfiedByContext is the Phase 1F composition guardrail.
//
// Context is resolved after matching, for the returned page only, and it may never feed a term
// back into discovery. The case is chosen so the term really is one hop away: node/beta's
// context carries the claim beta-was-observed-in-1999, whose label contains both "beta" and
// "1999", and node/beta must still not match "beta 1999".
func TestAllTermsCannotBeSatisfiedByContext(t *testing.T) {
	k := evidenceIndex(t)

	// The premise: the term is genuinely present in the record's resolved context.
	neighbours := mustSearch(t, k, service.SearchQuery{
		Q: "beta", Type: string(domain.SearchNode), IncludeContext: true, Limit: 200,
	})
	found := false
	for _, result := range neighbours.Results {
		if result.ID != "beta" {
			continue
		}
		if result.Context == nil {
			t.Fatal("node/beta resolved no context, so this guardrail would be vacuous")
		}
		for _, related := range result.Context.Related {
			if strings.Contains(strings.ToLower(related.Entity.Label), "1999") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no context entity of node/beta mentions 1999; the assertion would be vacuous")
	}

	for _, includeContext := range []bool{false, true} {
		query := allTerms("beta 1999")
		query.IncludeContext = includeContext
		results, err := k.Search(query)
		if err != nil {
			t.Fatalf("include_context=%v: %v", includeContext, err)
		}
		for _, result := range results.Results {
			if result.EntityType == domain.SearchNode && result.ID == "beta" {
				t.Errorf("include_context=%v: node/beta matched a term only its context carries",
					includeContext)
			}
		}
		// The claim that carries both terms in its own fields is still a hit, so the exclusion
		// above is a boundary rather than the whole query failing.
		if got := searchRefs(results); !reflect.DeepEqual(got, []string{"claim/beta-was-observed-in-1999"}) {
			t.Errorf("include_context=%v results = %v, want only the claim", includeContext, got)
		}
	}
}

// TestAllTermsResultsAreIdenticalWithAndWithoutContext asserts context adds a field and
// changes nothing else: not which records matched, not their order, not the paging totals.
func TestAllTermsResultsAreIdenticalWithAndWithoutContext(t *testing.T) {
	k := evidenceIndex(t)

	plain := mustSearch(t, k, allTerms("acoustics synthetic"))
	withContext := allTerms("acoustics synthetic")
	withContext.IncludeContext = true
	enriched := mustSearch(t, k, withContext)

	if plain.Page != enriched.Page {
		t.Errorf("paging differed: %+v vs %+v", plain.Page, enriched.Page)
	}
	if plain.Query != enriched.Query || plain.QueryMode != enriched.QueryMode {
		t.Errorf("echoed query differed: %q/%q vs %q/%q",
			plain.Query, plain.QueryMode, enriched.Query, enriched.QueryMode)
	}
	if len(plain.Results) != len(enriched.Results) {
		t.Fatalf("result counts differ: %d vs %d", len(plain.Results), len(enriched.Results))
	}
	for i := range enriched.Results {
		if enriched.Results[i].Context == nil {
			t.Errorf("result %d resolved no context object", i)
		}
		stripped := enriched.Results[i]
		stripped.Context = nil
		if !reflect.DeepEqual(stripped, plain.Results[i]) {
			t.Errorf("result %d differed once context was stripped:\n%+v\n%+v",
				i, stripped, plain.Results[i])
		}
	}
}

// TestAllTermsNormalisation covers case, whitespace and duplicates: several spellings of one
// question that must all execute as the same query.
func TestAllTermsNormalisation(t *testing.T) {
	k := evidenceIndex(t)
	want := mustSearch(t, k, allTerms("alpha acoustics"))
	if want.Page.Total == 0 {
		t.Fatal("baseline composed query matched nothing")
	}

	for _, spelling := range []string{
		"ALPHA ACOUSTICS", "Alpha Acoustics", "aLpHa AcOuStIcS",
		"  alpha   acoustics  ", "alpha\tacoustics", "alpha\n\nacoustics",
		"alpha alpha acoustics", "alpha acoustics alpha acoustics", "alpha acoustics ALPHA",
	} {
		t.Run(strings.TrimSpace(spelling), func(t *testing.T) {
			got := mustSearch(t, k, allTerms(spelling))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%q produced a different response:\n%+v\n%+v", spelling, got, want)
			}
			if got.Query != "alpha acoustics" {
				t.Errorf("%q echoed query %q, want the normalised term list", spelling, got.Query)
			}
		})
	}

	// Duplicates collapse to their first occurrence, so the echoed order follows the query
	// rather than the repetition, and the evidence carries one entry per distinct term.
	repeated := mustSearch(t, k, allTerms("acoustics alpha acoustics"))
	if repeated.Query != "acoustics alpha" {
		t.Errorf("echoed query = %q, want %q", repeated.Query, "acoustics alpha")
	}
	if len(repeated.Results) != 1 {
		t.Fatalf("results = %v", searchRefs(repeated))
	}
	if got := len(repeated.Results[0].TermMatches); got != 2 {
		t.Errorf("term_matches has %d entries for two distinct terms", got)
	}
}

// TestAllTermsTermBounds covers the floor, the ceiling and the character ceiling, and covers
// them at the service rather than only at the HTTP edge.
func TestAllTermsTermBounds(t *testing.T) {
	k := evidenceIndex(t)

	// One term is refused rather than quietly becoming a second spelling of literal search.
	for _, single := range []string{"fixture", "  fixture  ", "fixture fixture", "a a a a a"} {
		results, err := k.Search(allTerms(single))
		if !errors.Is(err, service.ErrTooFewSearchTerms) {
			t.Errorf("all_terms %q error = %v, want ErrTooFewSearchTerms", single, err)
		}
		if len(results.Results) != 0 {
			t.Errorf("all_terms %q returned results alongside an error", single)
		}
	}

	// The exact boundary in both directions.
	terms := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	atCeiling := strings.Join(terms[:service.MaxSearchTerms], " ")
	if _, err := k.Search(allTerms(atCeiling)); err != nil {
		t.Errorf("%d terms was refused: %v", service.MaxSearchTerms, err)
	}
	overCeiling := strings.Join(terms[:service.MaxSearchTerms+1], " ")
	if _, err := k.Search(allTerms(overCeiling)); !errors.Is(err, service.ErrTooManySearchTerms) {
		t.Errorf("%d terms error = %v, want ErrTooManySearchTerms", service.MaxSearchTerms+1, err)
	}
	// Repetition cannot move either bound: they apply to the distinct terms that execute.
	if _, err := k.Search(allTerms(overCeiling + " " + overCeiling)); !errors.Is(err, service.ErrTooManySearchTerms) {
		t.Error("a repeated over-long query was not refused")
	}
	if _, err := k.Search(allTerms(atCeiling + " " + atCeiling)); err != nil {
		t.Errorf("a repeated at-ceiling query was refused: %v", err)
	}

	// The whole-query character ceiling still applies, and it applies before the term bounds:
	// the text is refused rather than truncated into a different composition.
	long := strings.Repeat("ab ", service.MaxQueryChars)
	if _, err := k.Search(allTerms(long)); !errors.Is(err, service.ErrSearchQueryTooLong) {
		t.Errorf("over-long composed query error = %v, want ErrSearchQueryTooLong", err)
	}

	// An empty or whitespace-only query is still the empty-query error rather than a term-count
	// one: the caller supplied no query at all, which is a different mistake.
	for _, blank := range []string{"", "   ", "\t\n"} {
		if _, err := k.Search(allTerms(blank)); !errors.Is(err, service.ErrEmptySearchQuery) {
			t.Errorf("all_terms q=%q error = %v, want ErrEmptySearchQuery", blank, err)
		}
	}
}

// TestSearchRejectsUnsupportedQueryMode asserts the mode vocabulary is closed and exact.
//
// Every value below is a plausible guess at "and these words together", and each is refused
// rather than resolved, because a mode that silently meant something else would answer a
// different question than the caller asked.
func TestSearchRejectsUnsupportedQueryMode(t *testing.T) {
	k := evidenceIndex(t)

	for _, unsupported := range []string{
		"AND", "and", "all", "ALL_TERMS", "All_Terms", "any", "any_terms",
		"semantic", "fuzzy", "phrase", "Literal", "LITERAL", "1", "true", "or",
	} {
		t.Run(unsupported, func(t *testing.T) {
			results, err := k.Search(service.SearchQuery{Q: "alpha acoustics", Mode: unsupported})
			var invalid *service.InvalidFilterError
			if !errors.As(err, &invalid) {
				t.Fatalf("mode=%q error = %v, want InvalidFilterError", unsupported, err)
			}
			if invalid.Param != "query_mode" {
				t.Errorf("param = %q, want query_mode", invalid.Param)
			}
			if !reflect.DeepEqual(invalid.Allowed, domain.SearchQueryModeNames()) {
				t.Errorf("allowed = %v, want %v", invalid.Allowed, domain.SearchQueryModeNames())
			}
			if len(results.Results) != 0 {
				t.Errorf("mode=%q returned results alongside an error", unsupported)
			}
		})
	}
}

// TestAllTermsEvidenceNamesOnlyTheHitsOwnFields is the evidence contract.
//
// Per-term matched fields follow the record's canonical field order, terms follow the
// normalised query order, matched_fields is their union in the same canonical order, and every
// name is a field of the hit's own record.
func TestAllTermsEvidenceNamesOnlyTheHitsOwnFields(t *testing.T) {
	k := evidenceIndex(t)

	results := mustSearch(t, k, allTerms("alpha acoustics"))
	if len(results.Results) != 1 {
		t.Fatalf("results = %v, want only node/alpha", searchRefs(results))
	}
	hit := results.Results[0]
	wantTerms := []domain.SearchTermMatch{
		{Term: "alpha", MatchedFields: []string{"id", "title"}},
		{Term: "acoustics", MatchedFields: []string{"domain"}},
	}
	if !reflect.DeepEqual(hit.TermMatches, wantTerms) {
		t.Errorf("term_matches = %+v, want %+v", hit.TermMatches, wantTerms)
	}
	// matched_fields is the union, in canonical field order — id, title, domain — not the
	// concatenation of the per-term lists and not one term's list.
	if want := []string{"id", "title", "domain"}; !reflect.DeepEqual(hit.MatchedFields, want) {
		t.Errorf("matched_fields = %v, want %v", hit.MatchedFields, want)
	}
	if hit.MatchKind != domain.MatchAllTerms {
		t.Errorf("match_kind = %q, want %q", hit.MatchKind, domain.MatchAllTerms)
	}

	// Term order follows the query, so reversing the query reverses the evidence and changes
	// nothing else.
	reversed := mustSearch(t, k, allTerms("acoustics alpha"))
	if got := searchRefs(reversed); !reflect.DeepEqual(got, searchRefs(results)) {
		t.Errorf("reversed query returned %v, want the same records", got)
	}
	wantReversed := []domain.SearchTermMatch{
		{Term: "acoustics", MatchedFields: []string{"domain"}},
		{Term: "alpha", MatchedFields: []string{"id", "title"}},
	}
	if !reflect.DeepEqual(reversed.Results[0].TermMatches, wantReversed) {
		t.Errorf("reversed term_matches = %+v, want %+v", reversed.Results[0].TermMatches, wantReversed)
	}
	if want := []string{"id", "title", "domain"}; !reflect.DeepEqual(reversed.Results[0].MatchedFields, want) {
		t.Errorf("reversed matched_fields = %v, want %v", reversed.Results[0].MatchedFields, want)
	}

	// Across every hit of a broader query: no term is reported against a field the record does
	// not have, every named field really contains the term, and the union is exactly the union.
	broad := allTerms("acoustics synthetic")
	broad.IncludeContext = true
	for _, result := range mustSearch(t, k, broad).Results {
		if len(result.TermMatches) != 2 {
			t.Errorf("%s/%s has %d term matches", result.EntityType, result.ID, len(result.TermMatches))
		}
		union := map[string]bool{}
		for _, match := range result.TermMatches {
			if len(match.MatchedFields) == 0 {
				t.Errorf("%s/%s term %q names no field", result.EntityType, result.ID, match.Term)
			}
			for _, field := range match.MatchedFields {
				union[field] = true
				if !containsField(result.MatchedFields, field) {
					t.Errorf("%s/%s term %q names %q, absent from matched_fields",
						result.EntityType, result.ID, match.Term, field)
				}
			}
			// Evidence never borrows from a related record. Context is resolved on this very
			// response, so a field name leaking in from it would show up here.
			if result.Context != nil {
				for _, related := range result.Context.Related {
					if containsField(match.MatchedFields, related.Origin) {
						t.Errorf("%s/%s evidence names the context origin %q",
							result.EntityType, result.ID, related.Origin)
					}
				}
			}
		}
		for _, field := range result.MatchedFields {
			if !union[field] {
				t.Errorf("%s/%s matched_fields names %q, which no term matched",
					result.EntityType, result.ID, field)
			}
		}
	}
}

// TestAllTermsOrderingIsDeterministic covers the composed ordering: canonical class order, then
// canonical ID, and nothing else.
func TestAllTermsOrderingIsDeterministic(t *testing.T) {
	first := evidenceIndex(t)
	second := evidenceIndex(t)

	// A query spanning two classes: nodes precede vocabulary entries because that is the
	// canonical class order, and inside each class the canonical IDs ascend.
	want := []string{
		"node/alpha", "node/beta", "vocabulary/fixture-orphan-term", "vocabulary/fixture-term",
	}
	if got := searchRefs(mustSearch(t, first, allTerms("acoustics synthetic"))); !reflect.DeepEqual(got, want) {
		t.Fatalf("ordering = %v, want %v", got, want)
	}

	// Every composed hit carries the one composed match class, so ordering cannot be coming
	// from a match-kind precedence smuggled in behind it.
	for _, result := range mustSearch(t, first, allTerms("acoustics synthetic")).Results {
		if result.MatchKind != domain.MatchAllTerms {
			t.Errorf("%s/%s match_kind = %q", result.EntityType, result.ID, result.MatchKind)
		}
	}

	// Repeat runs on one index, and a second index built from the same corpus: the second is
	// what catches an ordering that depends on Go map iteration.
	for _, query := range []string{
		"acoustics synthetic", "synthetic outbound", "alpha fixture", "fixture synthetic",
	} {
		a := mustSearch(t, first, allTerms(query))
		b := mustSearch(t, first, allTerms(query))
		c := mustSearch(t, second, allTerms(query))
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%q differed between two calls on one index", query)
		}
		if !reflect.DeepEqual(a, c) {
			t.Errorf("%q differed between two indexes over one corpus:\n%v\n%v",
				query, searchRefs(a), searchRefs(c))
		}
	}
}

// TestAllTermsComposesWithTypeAndPaging asserts the filter and the page window behave exactly
// as they do for literal search, and that paging runs after the whole match set is ordered.
func TestAllTermsComposesWithTypeAndPaging(t *testing.T) {
	k := evidenceIndex(t)
	const query = "acoustics synthetic"

	full := mustSearch(t, k, allTerms(query))
	if full.Page.Total != 4 {
		t.Fatalf("baseline matched %d records, want 4: %v", full.Page.Total, searchRefs(full))
	}

	// type selects from the composed match set rather than restricting what "all terms" means.
	nodes := allTerms(query)
	nodes.Type = string(domain.SearchNode)
	got := mustSearch(t, k, nodes)
	if want := []string{"node/alpha", "node/beta"}; !reflect.DeepEqual(searchRefs(got), want) {
		t.Errorf("type=node = %v, want %v", searchRefs(got), want)
	}
	if got.Type != string(domain.SearchNode) || got.QueryMode != string(domain.SearchModeAllTerms) {
		t.Errorf("echoed type/mode = %q/%q", got.Type, got.QueryMode)
	}

	// The filtered totals add up to the unfiltered one.
	sum := 0
	for _, entityType := range domain.SearchEntityTypes {
		filtered := allTerms(query)
		filtered.Type = string(entityType)
		sum += mustSearch(t, k, filtered).Page.Total
	}
	if sum != full.Page.Total {
		t.Errorf("filtered totals sum to %d, unfiltered total is %d", sum, full.Page.Total)
	}

	// An unsupported class is refused here exactly as it is for literal search.
	rejected := allTerms(query)
	rejected.Type = "experiment_run"
	var invalid *service.InvalidFilterError
	if _, err := k.Search(rejected); !errors.As(err, &invalid) || invalid.Param != "type" {
		t.Errorf("type=experiment_run error = %v, want an InvalidFilterError on type", err)
	}

	// Paging walks the complete ordered match set. Every page reports the full total, so a
	// caller can never read a page count as the size of the result.
	var walked []string
	for offset := 0; offset < full.Page.Total; offset += 2 {
		page := allTerms(query)
		page.Limit, page.Offset = 2, offset
		got := mustSearch(t, k, page)
		if got.Page.Total != full.Page.Total {
			t.Errorf("offset %d reported total %d, want %d", offset, got.Page.Total, full.Page.Total)
		}
		if got.Page.Limit != 2 || got.Page.Offset != offset {
			t.Errorf("page meta = %+v", got.Page)
		}
		walked = append(walked, searchRefs(got)...)
	}
	if want := searchRefs(full); !reflect.DeepEqual(walked, want) {
		t.Errorf("walked pages = %v, want %v", walked, want)
	}

	// An offset past the end is an empty page reporting the real total, not an error.
	past := allTerms(query)
	past.Offset = full.Page.Total + 10
	if got := mustSearch(t, k, past); len(got.Results) != 0 || got.Page.Total != full.Page.Total {
		t.Errorf("offset past end = %+v with %d results", got.Page, len(got.Results))
	}

	// The default and maximum limits are the shared ones.
	unbounded := service.SearchQuery{Q: query, Mode: string(domain.SearchModeAllTerms)}
	if got := mustSearch(t, k, unbounded); got.Page.Limit != service.DefaultLimit {
		t.Errorf("default limit = %d, want %d", got.Page.Limit, service.DefaultLimit)
	}
	oversized := allTerms(query)
	oversized.Limit = service.MaxLimit + 1000
	if got := mustSearch(t, k, oversized); got.Page.Limit != service.MaxLimit {
		t.Errorf("limit = %d, want it clamped to %d", got.Page.Limit, service.MaxLimit)
	}
}

// TestAllTermsResultsAreDefensiveCopies asserts a caller cannot write through a composed result
// — including through its per-term evidence — into the startup index.
func TestAllTermsResultsAreDefensiveCopies(t *testing.T) {
	k := evidenceIndex(t)
	const query = "acoustics synthetic"

	before := mustSearch(t, k, allTerms(query))
	if len(before.Results) == 0 {
		t.Fatal("baseline composed query matched nothing")
	}
	for i := range before.Results {
		before.Results[i].ID = "mutated"
		before.Results[i].MatchKind = "mutated"
		for j := range before.Results[i].MatchedFields {
			before.Results[i].MatchedFields[j] = "mutated"
		}
		for j := range before.Results[i].TermMatches {
			before.Results[i].TermMatches[j].Term = "mutated"
			for f := range before.Results[i].TermMatches[j].MatchedFields {
				before.Results[i].TermMatches[j].MatchedFields[f] = "mutated"
			}
		}
	}

	after := mustSearch(t, k, allTerms(query))
	for _, result := range after.Results {
		if result.ID == "mutated" || result.MatchKind == "mutated" {
			t.Fatalf("a returned result wrote through into the index: %+v", result)
		}
		for _, field := range result.MatchedFields {
			if field == "mutated" {
				t.Fatalf("matched_fields wrote through into the index: %+v", result)
			}
		}
		for _, match := range result.TermMatches {
			if match.Term == "mutated" {
				t.Fatalf("term_matches wrote through into the index: %+v", result)
			}
			for _, field := range match.MatchedFields {
				if field == "mutated" {
					t.Fatalf("term evidence wrote through into the index: %+v", result)
				}
			}
		}
	}

	// The literal path and the per-layer lists read the same documents, so they must be
	// unharmed too.
	if got := mustSearch(t, k, service.SearchQuery{Q: "acoustics", Limit: 200}); got.Page.Total == 0 {
		t.Error("literal search stopped matching after a composed result was mutated")
	}
	if got := k.ListNodes(service.NodeQuery{Q: "acoustics", Limit: 200}).Page.Total; got == 0 {
		t.Error("the node list stopped matching after a composed result was mutated")
	}
}

// TestAllTermsIsNotABooleanLanguage asserts the mode composes whitespace-separated terms and
// nothing else. Operators, wildcards, quotes and field selectors are terms like any other, so
// each is looked for literally and finds only a record that genuinely contains it.
func TestAllTermsIsNotABooleanLanguage(t *testing.T) {
	k := evidenceIndex(t)

	for _, query := range []string{
		"alpha AND acoustics", "alpha OR acoustics", "alpha NOT beta",
		"+alpha +acoustics", "alpha -beta", "(alpha acoustics)",
		"domain:acoustics title:alpha", "alph* acoustics", "alpha acousti?s",
		`"alpha" "acoustics"`, "alpha.* acoustics", "alpha && acoustics",
	} {
		t.Run(query, func(t *testing.T) {
			results, err := k.Search(allTerms(query))
			if errors.Is(err, service.ErrTooManySearchTerms) || errors.Is(err, service.ErrTooFewSearchTerms) {
				// A bounded refusal is an acceptable outcome; an interpreted operator is not.
				return
			}
			if err != nil {
				t.Fatalf("q=%q: %v", query, err)
			}
			// Every token is required as literal text, the operator-looking ones included. Some
			// of them genuinely occur in the corpus — "or" is inside "more", "-beta" is inside a
			// claim ID — so the assertion is not that the result is empty; it is that the
			// composed set is exactly the intersection of the literal searches for each token.
			// An interpreted operator would drop a token out of that intersection or negate it.
			terms := distinctTerms(query)
			for _, ref := range searchRefs(results) {
				for _, term := range terms {
					literal := mustSearch(t, k, service.SearchQuery{Q: term, Limit: 200})
					if !containsField(searchRefs(literal), ref) {
						t.Errorf("q=%q: %s is a hit but literal %q does not find it — the token "+
							"was interpreted rather than required", query, ref, term)
					}
				}
			}
			for _, result := range results.Results {
				if len(result.TermMatches) != len(terms) {
					t.Errorf("q=%q: %s/%s reports %d term matches for %d terms",
						query, result.EntityType, result.ID, len(result.TermMatches), len(terms))
				}
				for i, match := range result.TermMatches {
					if match.Term != terms[i] {
						t.Errorf("q=%q: term %d = %q, want %q", query, i, match.Term, terms[i])
					}
					if len(match.MatchedFields) == 0 {
						t.Errorf("q=%q: token %q matched no field yet the record was returned",
							query, match.Term)
					}
				}
			}
		})
	}

	// Punctuation stays part of a term, which is the documented tokenisation limit: the bare
	// word finds the record and the comma-suffixed spelling does not.
	if got := mustSearch(t, k, allTerms("alpha acoustics")); got.Page.Total == 0 {
		t.Fatal("baseline matched nothing")
	}
	if got := mustSearch(t, k, allTerms("alpha, acoustics")); got.Page.Total != 0 {
		t.Errorf("a comma was stripped from a term: %v", searchRefs(got))
	}
}

// TestAllTermsExcludesExperimentRuns re-asserts the corpus boundary under the new mode.
//
// Phase 1G composes queries; it does not widen what is searchable. A run has no search document
// at all, and no combination of terms may produce one.
func TestAllTermsExcludesExperimentRuns(t *testing.T) {
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

	queries := []string{
		"fixture planned", "fixture completed", "listening completed",
		"exercise observation", "fixture run",
	}
	for id := range runIDs {
		queries = append(queries, id+" fixture")
	}
	for _, query := range queries {
		results, err := k.Search(allTerms(query))
		if err != nil {
			t.Fatalf("q=%q: %v", query, err)
		}
		for _, result := range results.Results {
			if runIDs[result.ID] {
				t.Errorf("q=%q returned experiment run %q as a %s result",
					query, result.ID, result.EntityType)
			}
		}
	}

	// A run's observation prose is not searchable, so two of its own words cannot compose into
	// a run hit either.
	run, err := k.ExperimentRunByID("fixture-listening-exercise-completed-a")
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	if len(run.Observations) == 0 {
		t.Fatal("the completed fixture run has no observations, so this assertion is vacuous")
	}
	words := strings.Fields(strings.ToLower(run.Observations[0].Statement))
	if len(words) < 2 {
		t.Fatal("the fixture observation is too short to compose")
	}
	for _, result := range mustSearch(t, k, allTerms(strings.Join(words[:2], " "))).Results {
		if runIDs[result.ID] {
			t.Errorf("run observation text composed into a run hit: %s/%s", result.EntityType, result.ID)
		}
	}
}

// TestAllTermsAgreesWithLiteralOnASingleTerm is the anti-drift assertion between the modes.
//
// The two modes share one field projection and one substring test, so the composed match set
// must be exactly the intersection of what each term finds under literal search. A second,
// subtly different matching implementation would break one direction or the other.
func TestAllTermsAgreesWithLiteralOnASingleTerm(t *testing.T) {
	k := evidenceIndex(t)

	for _, query := range []string{
		"acoustics synthetic", "synthetic outbound", "alpha fixture", "fixture synthetic",
	} {
		terms := strings.Fields(query)
		perTerm := make([]map[string]bool, 0, len(terms))
		for _, term := range terms {
			set := map[string]bool{}
			for _, ref := range searchRefs(mustSearch(t, k, service.SearchQuery{Q: term, Limit: 200})) {
				set[ref] = true
			}
			perTerm = append(perTerm, set)
		}

		composed := searchRefs(mustSearch(t, k, allTerms(query)))
		for _, ref := range composed {
			for i, set := range perTerm {
				if !set[ref] {
					t.Errorf("q=%q: %s is a composed hit but literal %q does not find it",
						query, ref, terms[i])
				}
			}
		}

		var intersection []string
		for ref := range perTerm[0] {
			inAll := true
			for _, set := range perTerm[1:] {
				if !set[ref] {
					inAll = false
					break
				}
			}
			if inAll {
				intersection = append(intersection, ref)
			}
		}
		sortStrings(intersection)
		got := append([]string(nil), composed...)
		sortStrings(got)
		if !reflect.DeepEqual(got, intersection) {
			t.Errorf("q=%q: composed set %v, intersection of literal sets %v",
				query, got, intersection)
		}
	}
}

// TestAllTermsIsSafeForConcurrentReaders exercises the composed path from many goroutines at
// once against one index.
//
// The index is built in New and never written to again, and composed search adds a second
// per-request pass over it, so this is where a caching or memoisation shortcut would most
// plausibly be introduced later. Every goroutine must observe the same response. Under `go test
// -race` this is also the detector's target for the new code path; without the detector it still
// asserts that concurrent readers cannot see each other's work.
func TestAllTermsIsSafeForConcurrentReaders(t *testing.T) {
	k := evidenceIndex(t)

	queries := []string{
		"acoustics synthetic", "synthetic outbound", "alpha fixture",
		"fixture synthetic", "beta 1999", "alpha acoustics",
	}
	want := make([]service.SearchResults, len(queries))
	for i, query := range queries {
		q := allTerms(query)
		q.IncludeContext = i%2 == 0
		want[i] = mustSearch(t, k, q)
	}

	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan string, readers*len(queries))
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pass := 0; pass < 4; pass++ {
				for i, query := range queries {
					q := allTerms(query)
					q.IncludeContext = i%2 == 0
					got, err := k.Search(q)
					if err != nil {
						errs <- "search " + query + ": " + err.Error()
						continue
					}
					if !reflect.DeepEqual(got, want[i]) {
						errs <- "concurrent reader saw a different response for " + query
					}
				}
				// A literal search over the same documents runs alongside, so a shared
				// structure touched by one mode would be observed by the other.
				if _, err := k.Search(service.SearchQuery{Q: "fixture", Limit: 200}); err != nil {
					errs <- "literal search: " + err.Error()
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for message := range errs {
		t.Error(message)
	}

	// The index still answers exactly as it did before the readers ran.
	for i, query := range queries {
		q := allTerms(query)
		q.IncludeContext = i%2 == 0
		if got := mustSearch(t, k, q); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("%q changed after concurrent reads", query)
		}
	}
}

// distinctTerms mirrors the service's normalisation for a test's own expectations: lower-case,
// whitespace-separated, first occurrence wins.
func distinctTerms(query string) []string {
	var out []string
	for _, term := range strings.Fields(strings.ToLower(strings.TrimSpace(query))) {
		if !containsField(out, term) {
			out = append(out, term)
		}
	}
	return out
}

// containsField is a local membership test; the service's own is unexported.
func containsField(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
