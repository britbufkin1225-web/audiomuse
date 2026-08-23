package httpapi_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// Phase 1G over HTTP: the query_mode control, its bounds, and the shape of a composed response.
//
// The handler adds one parameter and maps two typed errors; everything else is asserted at the
// service. What is checked here is the wire contract a client actually sees — that a legacy
// request is byte-identical, that a composed one is unambiguous about what ran, and that no
// spelling of the mode is guessed at.

// resultRefs renders a decoded result set as type/id pairs.
func resultRefs(results service.SearchResults) []string {
	out := make([]string, 0, len(results.Results))
	for _, result := range results.Results {
		out = append(out, string(result.EntityType)+"/"+result.ID)
	}
	return out
}

// TestSearchRouteLegacyRequestIsByteIdentical is the compatibility assertion at the edge.
//
// A Phase 1E/1F request must serialise exactly what it always did: no query_mode key on the
// response, no term_matches key on any result, and whitespace in q still a phrase rather than a
// separator.
func TestSearchRouteLegacyRequestIsByteIdentical(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		"/api/v1/search?q=fixture",
		"/api/v1/search?q=fixture&type=vocabulary",
		"/api/v1/search?q=alpha&limit=5&offset=1",
		"/api/v1/search?q=fixture&include_context=true",
		"/api/v1/search?q=synthetic+outbound",
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if strings.Contains(body, "query_mode") {
				t.Errorf("a legacy response carries a query_mode key: %s", body)
			}
			if strings.Contains(body, "term_matches") {
				t.Errorf("a legacy response carries a term_matches key: %s", body)
			}
		})
	}

	// An explicit literal mode serialises to exactly the omitted-mode body, so the two are one
	// contract rather than two shapes.
	omitted := do(t, handler, http.MethodGet, "/api/v1/search?q=fixture&limit=200").Body.String()
	explicit := do(t, handler, http.MethodGet,
		"/api/v1/search?q=fixture&limit=200&query_mode=literal").Body.String()
	if omitted != explicit {
		t.Errorf("query_mode=literal differed from an omitted mode:\n%s\n%s", omitted, explicit)
	}

	// Whitespace in q is still part of the phrase: two words no canonical value carries side by
	// side match nothing under the default mode.
	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "synthetic outbound"}))
	var phrase service.SearchResults
	decode(t, rec, &phrase)
	if phrase.Page.Total != 0 {
		t.Errorf("a legacy two-word query matched %v; whitespace became a separator",
			resultRefs(phrase))
	}
}

// TestSearchRouteAllTermsMode covers the composed response over HTTP: the echoed mode, the
// normalised query, the composed match kind and the per-term evidence.
func TestSearchRouteAllTermsMode(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "alpha acoustics", "query_mode": "all_terms",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)

	if results.QueryMode != string(domain.SearchModeAllTerms) {
		t.Errorf("query_mode = %q, want all_terms", results.QueryMode)
	}
	if results.Query != "alpha acoustics" {
		t.Errorf("query = %q, want the normalised term list", results.Query)
	}
	if got := resultRefs(results); !reflect.DeepEqual(got, []string{"node/alpha"}) {
		t.Fatalf("results = %v, want node/alpha", got)
	}
	hit := results.Results[0]
	if hit.MatchKind != domain.MatchAllTerms {
		t.Errorf("match_kind = %q, want %q", hit.MatchKind, domain.MatchAllTerms)
	}
	wantTerms := []domain.SearchTermMatch{
		{Term: "alpha", MatchedFields: []string{"id", "title"}},
		{Term: "acoustics", MatchedFields: []string{"domain"}},
	}
	if !reflect.DeepEqual(hit.TermMatches, wantTerms) {
		t.Errorf("term_matches = %+v, want %+v", hit.TermMatches, wantTerms)
	}
	if want := []string{"id", "title", "domain"}; !reflect.DeepEqual(hit.MatchedFields, want) {
		t.Errorf("matched_fields = %v, want %v", hit.MatchedFields, want)
	}

	// The serialised keys are the documented ones and the composed evidence really is on the
	// wire rather than only in the decoded struct.
	body := rec.Body.String()
	for _, want := range []string{`"query_mode":"all_terms"`, `"term_matches"`, `"match_kind":"all_terms"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %s: %s", want, body)
		}
	}

	// The composed query is the whole of what changed: the same two words as a phrase still
	// match nothing.
	var phrase service.SearchResults
	decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "alpha acoustics"})), &phrase)
	if phrase.Page.Total != 0 {
		t.Errorf("the literal phrase matched %v", resultRefs(phrase))
	}
}

// TestSearchRouteAllTermsComposesWithTypeAndContext covers the two controls a composed query is
// most likely to be sent with.
func TestSearchRouteAllTermsComposesWithTypeAndContext(t *testing.T) {
	handler := newHandler(t)

	// type restricts the composed match set to one class and is echoed alongside the mode.
	var filtered service.SearchResults
	decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "acoustics synthetic", "query_mode": "all_terms", "type": "node",
	})), &filtered)
	if want := []string{"node/alpha", "node/beta"}; !reflect.DeepEqual(resultRefs(filtered), want) {
		t.Errorf("type=node = %v, want %v", resultRefs(filtered), want)
	}
	if filtered.Type != "node" || filtered.QueryMode != "all_terms" {
		t.Errorf("echoed type/mode = %q/%q", filtered.Type, filtered.QueryMode)
	}
	for _, result := range filtered.Results {
		if result.EntityType != domain.SearchNode {
			t.Errorf("type=node returned a %s result", result.EntityType)
		}
		if len(result.TermMatches) != 2 {
			t.Errorf("%s carries %d term matches", result.ID, len(result.TermMatches))
		}
	}

	// include_context composes: it adds a context object to each returned hit and changes
	// nothing about which hits there are, their order, or the paging totals.
	var plain, enriched service.SearchResults
	decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "acoustics synthetic", "query_mode": "all_terms",
	})), &plain)
	decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "acoustics synthetic", "query_mode": "all_terms", "include_context": "true",
	})), &enriched)

	if !reflect.DeepEqual(resultRefs(plain), resultRefs(enriched)) {
		t.Errorf("context changed the result set: %v vs %v", resultRefs(plain), resultRefs(enriched))
	}
	if plain.Page != enriched.Page {
		t.Errorf("context changed the paging metadata: %+v vs %+v", plain.Page, enriched.Page)
	}
	if !enriched.IncludeContext {
		t.Error("include_context was not echoed")
	}
	for i, result := range enriched.Results {
		if result.Context == nil {
			t.Errorf("result %d resolved no context", i)
			continue
		}
		if !reflect.DeepEqual(result.TermMatches, plain.Results[i].TermMatches) {
			t.Errorf("result %d evidence changed once context was resolved", i)
		}
	}

	// Context cannot supply a missing term. node/beta's context names the claim
	// beta-was-observed-in-1999, and node/beta must still not match "beta 1999".
	var contextual service.SearchResults
	decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "beta 1999", "query_mode": "all_terms", "include_context": "true",
	})), &contextual)
	if want := []string{"claim/beta-was-observed-in-1999"}; !reflect.DeepEqual(resultRefs(contextual), want) {
		t.Errorf("results = %v, want %v — context supplied a term", resultRefs(contextual), want)
	}
}

// TestSearchRouteAllTermsPaging asserts the page window runs over the whole composed match set.
func TestSearchRouteAllTermsPaging(t *testing.T) {
	handler := newHandler(t)
	const query = "acoustics synthetic"

	var full service.SearchResults
	decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": query, "query_mode": "all_terms", "limit": "200",
	})), &full)
	if full.Page.Total != 4 {
		t.Fatalf("baseline matched %d records: %v", full.Page.Total, resultRefs(full))
	}

	var walked []string
	for _, offset := range []string{"0", "2"} {
		var page service.SearchResults
		decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
			"q": query, "query_mode": "all_terms", "limit": "2", "offset": offset,
		})), &page)
		if page.Page.Total != full.Page.Total {
			t.Errorf("offset %s reported total %d, want %d", offset, page.Page.Total, full.Page.Total)
		}
		if page.Page.Count != 2 {
			t.Errorf("offset %s returned %d results", offset, page.Page.Count)
		}
		walked = append(walked, resultRefs(page)...)
	}
	if want := resultRefs(full); !reflect.DeepEqual(walked, want) {
		t.Errorf("walked pages = %v, want %v", walked, want)
	}
}

// TestSearchRouteAllTermsEmptyResultIsAnAnswer asserts a composed query that intersects to
// nothing is a successful empty collection, exactly as a literal one is.
func TestSearchRouteAllTermsEmptyResultIsAnAnswer(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "resonant dsp", "query_mode": "all_terms",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)
	if results.Page.Total != 0 || len(results.Results) != 0 {
		t.Errorf("results = %v, want empty", resultRefs(results))
	}
	if !strings.Contains(rec.Body.String(), `"results":[]`) {
		t.Errorf("body = %s, want an empty results array", rec.Body.String())
	}
	// The mode is still echoed, so an empty composed answer is distinguishable from an empty
	// literal one.
	if results.QueryMode != "all_terms" {
		t.Errorf("query_mode = %q, want all_terms", results.QueryMode)
	}
}

// TestSearchRouteRejectsInvalidQueryMode covers every way a caller can get the control wrong.
func TestSearchRouteRejectsInvalidQueryMode(t *testing.T) {
	handler := newHandler(t)

	targets := []string{
		// An unsupported spelling is refused rather than guessed at.
		"/api/v1/search?q=alpha+acoustics&query_mode=semantic",
		"/api/v1/search?q=alpha+acoustics&query_mode=AND",
		"/api/v1/search?q=alpha+acoustics&query_mode=and",
		"/api/v1/search?q=alpha+acoustics&query_mode=all",
		"/api/v1/search?q=alpha+acoustics&query_mode=ALL_TERMS",
		"/api/v1/search?q=alpha+acoustics&query_mode=fuzzy",
		"/api/v1/search?q=alpha+acoustics&query_mode=1",
		"/api/v1/search?q=alpha+acoustics&query_mode=true",
		// Present but blank is a caller mistake, not "absent".
		"/api/v1/search?q=alpha+acoustics&query_mode=",
		"/api/v1/search?q=alpha+acoustics&query_mode=%20",
		// Too few and too many terms for the mode that was named.
		"/api/v1/search?q=alpha&query_mode=all_terms",
		"/api/v1/search?q=alpha+alpha&query_mode=all_terms",
		"/api/v1/search?q=a+b+c+d+e+f+g+h+i&query_mode=all_terms",
		// The shared bounds still apply.
		"/api/v1/search?query_mode=all_terms",
		"/api/v1/search?q=&query_mode=all_terms",
		"/api/v1/search?q=%20%20&query_mode=all_terms",
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&type=experiment_run",
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&limit=-1",
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&offset=many",
		// Duplicated and unknown parameters are refused exactly as on every other route.
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&query_mode=literal",
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&mode=all_terms",
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&terms=2",
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms&operator=and",
		// Past the query character ceiling, in the new mode.
		"/api/v1/search?query_mode=all_terms&q=" + strings.Repeat("ab+", service.MaxQueryChars),
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("GET %s = %d, want 400: %s", target, rec.Code, rec.Body.String())
			}
			assertErrorCode(t, rec, "invalid_query")
		})
	}

	// The rejection lists the supported modes and does not echo the caller's own value.
	rec := do(t, handler, http.MethodGet, "/api/v1/search?q=alpha+acoustics&query_mode=semantic")
	body := rec.Body.String()
	for _, want := range []string{"literal", "all_terms"} {
		if !strings.Contains(body, want) {
			t.Errorf("error does not list %q: %s", want, body)
		}
	}
	if strings.Contains(body, "semantic") {
		t.Errorf("error echoed the caller's value: %s", body)
	}

	// The term-count refusals name the bound rather than the caller's terms.
	rec = do(t, handler, http.MethodGet, "/api/v1/search?q=alpha&query_mode=all_terms")
	if !strings.Contains(rec.Body.String(), "all_terms") {
		t.Errorf("the too-few-terms error does not name the mode: %s", rec.Body.String())
	}
}

// TestSearchRouteAllTermsIsDeterministic asserts two identical composed requests produce
// identical bytes, including across two indexes built from the same corpus.
func TestSearchRouteAllTermsIsDeterministic(t *testing.T) {
	handler := newHandler(t)

	targets := []string{
		"/api/v1/search?q=acoustics+synthetic&query_mode=all_terms",
		"/api/v1/search?q=acoustics+synthetic&query_mode=all_terms&type=node",
		"/api/v1/search?q=acoustics+synthetic&query_mode=all_terms&include_context=true",
		"/api/v1/search?q=synthetic+outbound&query_mode=all_terms&limit=2&offset=1",
	}
	for _, target := range targets {
		first := do(t, handler, http.MethodGet, target).Body.String()
		for i := 0; i < 4; i++ {
			if got := do(t, handler, http.MethodGet, target).Body.String(); got != first {
				t.Fatalf("GET %s returned different bytes on repeat:\n%s\n%s", target, first, got)
			}
		}
	}

	other := newHandler(t)
	for _, target := range targets {
		if a, b := do(t, handler, http.MethodGet, target).Body.String(),
			do(t, other, http.MethodGet, target).Body.String(); a != b {
			t.Errorf("two indexes over one corpus disagreed on %s:\n%s\n%s", target, a, b)
		}
	}

	// Spelling variants of one composed query serialise identically, so normalisation is
	// visible on the wire rather than only inside the service.
	canonical := do(t, handler, http.MethodGet,
		"/api/v1/search?q=alpha+acoustics&query_mode=all_terms").Body.String()
	for _, variant := range []string{
		"/api/v1/search?q=ALPHA++ACOUSTICS&query_mode=all_terms",
		"/api/v1/search?q=++alpha+++acoustics++&query_mode=all_terms",
		"/api/v1/search?q=alpha+alpha+acoustics&query_mode=all_terms",
	} {
		if got := do(t, handler, http.MethodGet, variant).Body.String(); got != canonical {
			t.Errorf("GET %s differed from the canonical spelling:\n%s\n%s", variant, got, canonical)
		}
	}
}

// TestSearchRouteAllTermsIsReadOnlyAndLeaksNothing puts the new mode behind the same two
// guarantees every other route carries.
func TestSearchRouteAllTermsIsReadOnlyAndLeaksNothing(t *testing.T) {
	handler := newHandler(t)
	const target = "/api/v1/search?q=acoustics+synthetic&query_mode=all_terms"

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := do(t, handler, method, target)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", method, target, rec.Code)
		}
		if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
			t.Errorf("%s Allow = %q, want %q", method, got, want)
		}
	}
	if rec := do(t, handler, http.MethodHead, target); rec.Code != http.StatusOK {
		t.Errorf("HEAD = %d, want 200", rec.Code)
	}

	bodies := []string{
		do(t, handler, http.MethodGet, target+"&include_context=true").Body.String(),
		do(t, handler, http.MethodGet, "/api/v1/search?q=alpha&query_mode=all_terms").Body.String(),
		do(t, handler, http.MethodGet, "/api/v1/search?q=alpha+beta&query_mode=semantic").Body.String(),
	}
	for _, body := range bodies {
		for _, leak := range []string{"C:\\", "/Users/", "testdata", ".md", ".yaml", "goroutine", "internal/service"} {
			if strings.Contains(body, leak) {
				t.Errorf("response contains %q: %s", leak, body)
			}
		}
	}
}

// TestSearchRouteAllTermsEvidenceIsBoundedJSON asserts the evidence carries exactly two keys per
// term and nothing resembling a snippet, an offset or a score.
func TestSearchRouteAllTermsEvidenceIsBoundedJSON(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "acoustics synthetic", "query_mode": "all_terms",
	}))
	var raw struct {
		Results []struct {
			TermMatches []map[string]json.RawMessage `json:"term_matches"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw.Results) == 0 {
		t.Fatal("no results to inspect")
	}
	for _, result := range raw.Results {
		if len(result.TermMatches) == 0 {
			t.Error("a composed result carries no term_matches")
		}
		for _, match := range result.TermMatches {
			if len(match) != 2 {
				t.Errorf("a term match carries %d keys: %v", len(match), match)
			}
			for _, forbidden := range []string{"score", "count", "snippet", "offset", "weight", "rank", "highlight"} {
				if _, present := match[forbidden]; present {
					t.Errorf("a term match carries a %q key", forbidden)
				}
			}
			if _, present := match["term"]; !present {
				t.Error("a term match carries no term")
			}
			if _, present := match["matched_fields"]; !present {
				t.Error("a term match carries no matched_fields")
			}
		}
	}
}
