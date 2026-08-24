package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// Phase 1I at the edge. The service tests own the ranking policy; these own its wire contract —
// that every result serialises its score and its explanation, that the two agree, that the
// ordering survives the JSON round trip, and that none of it moved when the caller paged, scoped
// or composed the request.

// containsString reports membership in a signal list. The signal set is small and closed, so a
// linear scan is the whole of what is needed and a map would only add an allocation.
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// rankingKeys decodes the results array as raw objects so a key's presence can be asserted
// without the typed struct silently supplying a zero value for a key the response never sent.
func rankingKeys(t testing.TB, body []byte) []map[string]json.RawMessage {
	t.Helper()
	var envelope struct {
		Results []map[string]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode results: %v", err)
	}
	return envelope.Results
}

// TestSearchRouteServesTheRankingExplanation asserts the two additive keys are present on every
// result of every mode, and that a client can recompute the score from the signals it was given.
func TestSearchRouteServesTheRankingExplanation(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		searchURL(map[string]string{"q": "fixture", "limit": "200"}),
		searchURL(map[string]string{"q": "alpha", "limit": "200"}),
		searchURL(map[string]string{"q": "fixture term", "query_mode": "all_terms", "limit": "200"}),
		searchURL(map[string]string{"q": "fixture", "entity_types": "source,vocabulary", "limit": "200"}),
		searchURL(map[string]string{"q": "fixture", "include_context": "true", "limit": "200"}),
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}

			raw := rankingKeys(t, rec.Body.Bytes())
			if len(raw) == 0 {
				t.Fatal("the request matched nothing, so no explanation was checked")
			}
			for i, result := range raw {
				for _, key := range []string{"relevance_score", "match_signals", "match_kind", "matched_fields"} {
					if _, present := result[key]; !present {
						t.Errorf("result %d lost its %s key: %s", i, key, rec.Body.String())
					}
				}
			}

			var decoded service.SearchResults
			decode(t, rec, &decoded)
			previous := -1
			for _, result := range decoded.Results {
				if want := domain.SearchRelevanceScore(result.MatchSignals); result.RelevanceScore != want {
					t.Errorf("%s/%s: relevance_score %d is not the sum of %v (%d)",
						result.EntityType, result.ID, result.RelevanceScore, result.MatchSignals, want)
				}
				if len(result.MatchSignals) == 0 {
					t.Errorf("%s/%s serialised an empty explanation", result.EntityType, result.ID)
				}
				for _, signal := range result.MatchSignals {
					if !containsString(domain.SearchMatchSignals, signal) {
						t.Errorf("%s/%s emitted the unknown signal %q",
							result.EntityType, result.ID, signal)
					}
				}
				if previous >= 0 && result.RelevanceScore > previous {
					t.Errorf("relevance ascended at %s/%s: the wire order is not the ranking",
						result.EntityType, result.ID)
				}
				previous = result.RelevanceScore
			}
		})
	}
}

// TestSearchRouteRankingIsDeterministic asserts the whole point of the phase over HTTP: the same
// request against the same corpus serialises the same bytes, ordering and explanations included.
func TestSearchRouteRankingIsDeterministic(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		searchURL(map[string]string{"q": "fixture", "limit": "200"}),
		searchURL(map[string]string{"q": "e", "limit": "200"}),
		searchURL(map[string]string{"q": "fixture term", "query_mode": "all_terms", "limit": "200"}),
		searchURL(map[string]string{"q": "synthetic entry", "query_mode": "all_terms", "limit": "200"}),
	} {
		t.Run(target, func(t *testing.T) {
			first := do(t, handler, http.MethodGet, target).Body.String()
			for i := 0; i < 4; i++ {
				if got := do(t, handler, http.MethodGet, target).Body.String(); got != first {
					t.Fatalf("run %d differed:\n%s\n%s", i, first, got)
				}
			}
		})
	}

	// A second handler over a second index of the same corpus, which is what catches an ordering
	// that depends on Go map iteration rather than on the ranking policy.
	target := searchURL(map[string]string{"q": "fixture", "limit": "200"})
	if a, b := do(t, newHandler(t), http.MethodGet, target).Body.String(),
		do(t, newHandler(t), http.MethodGet, target).Body.String(); a != b {
		t.Errorf("two handlers over one corpus disagreed:\n%s\n%s", a, b)
	}
}

// TestSearchRouteRankingPrecedesPaging asserts the stage order at the edge: the page window is
// applied to the ranked set, so page one carries the best hits and the facets keep describing the
// whole set rather than the window.
func TestSearchRouteRankingPrecedesPaging(t *testing.T) {
	handler := newHandler(t)

	var full service.SearchResults
	decode(t, do(t, handler, http.MethodGet,
		searchURL(map[string]string{"q": "fixture", "limit": "200"})), &full)
	if full.Page.Total < 9 {
		t.Fatalf("the fixture corpus matched %d records, too few to page", full.Page.Total)
	}

	var walked []string
	for offset := 0; offset < full.Page.Total; offset += 4 {
		var page service.SearchResults
		decode(t, do(t, handler, http.MethodGet, searchURL(map[string]string{
			"q": "fixture", "limit": "4", "offset": itoaHTTP(offset),
		})), &page)

		if page.Page.Total != full.Page.Total {
			t.Errorf("offset %d reported total %d, want %d", offset, page.Page.Total, full.Page.Total)
		}
		if page.Facets != full.Facets {
			t.Errorf("offset %d reported facets %+v, want %+v", offset, page.Facets, full.Facets)
		}
		for i, result := range page.Results {
			want := full.Results[offset+i]
			if result.RelevanceScore != want.RelevanceScore ||
				string(result.EntityType)+"/"+result.ID != string(want.EntityType)+"/"+want.ID {
				t.Errorf("offset %d position %d = %s/%s (%d), want %s/%s (%d)",
					offset, i, result.EntityType, result.ID, result.RelevanceScore,
					want.EntityType, want.ID, want.RelevanceScore)
			}
			walked = append(walked, string(result.EntityType)+"/"+result.ID)
		}
	}
	if len(walked) != full.Page.Total {
		t.Errorf("walking the pages produced %d results, want %d", len(walked), full.Page.Total)
	}
}

// TestSearchRouteRankingIsAdditive asserts Phase 1I changed nothing a Phase 1E-1H client already
// read: the envelope keeps exactly its keys, a literal response still carries no composed keys,
// and the two new keys live on the result rather than reshaping anything around it.
func TestSearchRouteRankingIsAdditive(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "limit": "200"}))
	var envelope map[string]json.RawMessage
	decode(t, rec, &envelope)
	for key := range envelope {
		switch key {
		case "query", "page", "results", "facets":
		default:
			t.Errorf("a literal response carries an unexpected top-level key %q", key)
		}
	}

	for _, result := range rankingKeys(t, rec.Body.Bytes()) {
		for key := range result {
			switch key {
			case "entity_type", "id", "title", "summary", "match_kind", "matched_fields",
				"relevance_score", "match_signals":
			default:
				t.Errorf("a literal result carries an unexpected key %q", key)
			}
		}
		if _, present := result["term_matches"]; present {
			t.Error("a literal result carries multi-term evidence")
		}
		if _, present := result["context"]; present {
			t.Error("a literal result carries context that was never requested")
		}
	}

	// A composed response adds term_matches to the same result shape and nothing else.
	composed := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "fixture term", "query_mode": "all_terms", "limit": "200",
	}))
	for _, result := range rankingKeys(t, composed.Body.Bytes()) {
		for key := range result {
			switch key {
			case "entity_type", "id", "title", "summary", "match_kind", "matched_fields",
				"relevance_score", "match_signals", "term_matches":
			default:
				t.Errorf("a composed result carries an unexpected key %q", key)
			}
		}
	}
}

// TestSearchRouteRankingLeaksNothing asserts the explanation cannot become a channel for the
// operator's filesystem or the corpus's internals, which is the standing rule for every field
// this API adds.
func TestSearchRouteRankingLeaksNothing(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		searchURL(map[string]string{"q": "e", "limit": "200", "include_context": "true"}),
		searchURL(map[string]string{"q": "fixture term", "query_mode": "all_terms", "limit": "200"}),
	} {
		var decoded service.SearchResults
		decode(t, do(t, handler, http.MethodGet, target), &decoded)
		for _, result := range decoded.Results {
			for _, signal := range result.MatchSignals {
				if !containsString(domain.SearchMatchSignals, signal) {
					t.Errorf("%s/%s emitted %q, which is outside the closed vocabulary",
						result.EntityType, result.ID, signal)
				}
			}
		}
	}
}

// itoaHTTP renders a page offset for a query string without pulling strconv into this file for
// one call site.
func itoaHTTP(n int) string {
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
