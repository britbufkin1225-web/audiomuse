package httpapi_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// searchURL builds a request target with the query string escaped, so a test can send a term
// containing spaces, punctuation or a percent sign without the target itself being ambiguous.
func searchURL(params map[string]string) string {
	values := url.Values{}
	for name, value := range params {
		values.Set(name, value)
	}
	return "/api/v1/search?" + values.Encode()
}

func TestSearchRoute(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)

	if results.Query != "fixture" {
		t.Errorf("query = %q, want the normalised needle", results.Query)
	}
	if results.Page.Total == 0 || results.Page.Count != len(results.Results) {
		t.Errorf("page = %+v with %d results", results.Page, len(results.Results))
	}

	// Every result must be self-describing: a class, an ID, a display value and the evidence
	// for why it matched.
	seen := map[domain.SearchEntityType]bool{}
	for _, result := range results.Results {
		seen[result.EntityType] = true
		if !domain.ValidSearchEntityType(string(result.EntityType)) {
			t.Errorf("result carries unsupported entity_type %q", result.EntityType)
		}
		if result.ID == "" || result.Title == "" {
			t.Errorf("result %+v is missing its identity or display value", result)
		}
		if len(result.MatchedFields) == 0 {
			t.Errorf("result %s/%s carries no matched_fields", result.EntityType, result.ID)
		}
		if result.MatchKind == "" {
			t.Errorf("result %s/%s carries no match_kind", result.EntityType, result.ID)
		}
	}
	if len(seen) < 3 {
		t.Errorf("one query reached only %d classes: %v", len(seen), seen)
	}
}

// TestSearchRouteTypeFilter covers the class filter over HTTP, including the two ways a
// caller can get it wrong.
func TestSearchRouteTypeFilter(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "type": "vocabulary"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)
	if results.Type != "vocabulary" {
		t.Errorf("echoed type = %q, want vocabulary", results.Type)
	}
	if len(results.Results) == 0 {
		t.Fatal("type=vocabulary matched nothing")
	}
	for _, result := range results.Results {
		if result.EntityType != domain.SearchVocabulary {
			t.Errorf("type=vocabulary returned a %s result", result.EntityType)
		}
	}

	// An unsupported class is refused rather than answered with an empty set. experiment_run is
	// the case that matters: it is a real canonical class the API serves elsewhere, and a
	// caller must not be able to read "no results" as "no run mentions this".
	for _, unsupported := range []string{"experiment_run", "experiment-run", "runs", "Node", "everything"} {
		rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "type": unsupported}))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("type=%s = %d, want 400: %s", unsupported, rec.Code, rec.Body.String())
			continue
		}
		assertErrorCode(t, rec, "invalid_query")
		if !strings.Contains(rec.Body.String(), "vocabulary") {
			t.Errorf("type=%s error does not list the supported classes: %s", unsupported, rec.Body.String())
		}
		// The caller's own value is never echoed back into the response.
		if strings.Contains(rec.Body.String(), unsupported) && unsupported != "Node" {
			t.Errorf("type=%s error echoed the caller's value: %s", unsupported, rec.Body.String())
		}
	}
}

// TestSearchRouteRequiresAQuery asserts /search never becomes a whole-corpus dump.
func TestSearchRouteRequiresAQuery(t *testing.T) {
	handler := newHandler(t)

	targets := []string{
		"/api/v1/search",
		"/api/v1/search?q=",
		"/api/v1/search?q=%20",
		"/api/v1/search?q=%09%0A",
		"/api/v1/search?type=node",
		"/api/v1/search?q=&type=node&limit=10",
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
}

// TestSearchRouteEmptyResultIsNotA404 asserts an unmatched query is a successful answer.
func TestSearchRouteEmptyResultIsNotA404(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "no-record-contains-this"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)
	if results.Page.Total != 0 || len(results.Results) != 0 {
		t.Errorf("results = %+v, want empty", results)
	}
	// An empty collection, not a null: a client iterating the field must not have to nil-check.
	if !strings.Contains(rec.Body.String(), `"results":[]`) {
		t.Errorf("body = %s, want an empty results array", rec.Body.String())
	}
}

// TestSearchRoutePaging covers the bounds at the edge of the service.
func TestSearchRoutePaging(t *testing.T) {
	handler := newHandler(t)

	var full service.SearchResults
	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "limit": "200"}))
	decode(t, rec, &full)
	if full.Page.Total < 6 {
		t.Fatalf("fixture corpus matched %d records, too few to page", full.Page.Total)
	}

	rec = do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "limit": "2"}))
	var first service.SearchResults
	decode(t, rec, &first)
	if len(first.Results) != 2 || first.Page.Limit != 2 || first.Page.Total != full.Page.Total {
		t.Errorf("first page = %+v with %d results", first.Page, len(first.Results))
	}

	rec = do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "limit": "2", "offset": "2"}))
	var second service.SearchResults
	decode(t, rec, &second)
	if len(second.Results) != 2 || second.Page.Offset != 2 {
		t.Errorf("second page = %+v with %d results", second.Page, len(second.Results))
	}
	if first.Results[0].ID == second.Results[0].ID && first.Results[0].EntityType == second.Results[0].EntityType {
		t.Error("the second page repeated the first")
	}

	// An oversized limit is clamped rather than honoured, so no single request can be talked
	// into serialising the whole projection.
	rec = do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "limit": "100000"}))
	var clamped service.SearchResults
	decode(t, rec, &clamped)
	if clamped.Page.Limit != service.MaxLimit {
		t.Errorf("limit = %d, want it clamped to %d", clamped.Page.Limit, service.MaxLimit)
	}

	// A negative or non-numeric paging value is a caller mistake and is refused.
	for _, target := range []string{
		"/api/v1/search?q=fixture&limit=-1",
		"/api/v1/search?q=fixture&offset=-5",
		"/api/v1/search?q=fixture&limit=many",
		"/api/v1/search?q=fixture&offset=1.5",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, rec.Code)
			continue
		}
		assertErrorCode(t, rec, "invalid_query")
	}
}

// TestSearchRouteRejectsMalformedQueryStrings covers the shared query-string bounds on the new
// route: an unknown parameter, a duplicated one, and a term past the service ceiling.
func TestSearchRouteRejectsMalformedQueryStrings(t *testing.T) {
	handler := newHandler(t)

	targets := []string{
		// Unknown parameters are refused rather than ignored, so a caller is never handed a
		// result set that silently dropped the filter they believed was applied.
		"/api/v1/search?q=fixture&entity_type=node",
		"/api/v1/search?q=fixture&sort=relevance",
		"/api/v1/search?query=fixture",
		// Duplicates are refused rather than resolved to the first or last value.
		"/api/v1/search?q=fixture&q=alpha",
		"/api/v1/search?q=fixture&type=node&type=claim",
		"/api/v1/search?q=fixture&limit=1&limit=2",
		// Past the service query ceiling.
		"/api/v1/search?q=" + strings.Repeat("a", service.MaxQueryChars+1),
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
}

// TestSearchRouteIsReadOnly asserts the new route is behind the same method lock as every
// other. The lock runs before routing, so this is a check that the route did not somehow
// arrive in front of it.
func TestSearchRouteIsReadOnly(t *testing.T) {
	handler := newHandler(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := do(t, handler, method, "/api/v1/search?q=fixture")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/v1/search = %d, want 405", method, rec.Code)
		}
		if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
			t.Errorf("%s Allow = %q, want %q", method, got, want)
		}
	}

	// HEAD is served by the same handler and must agree on status.
	if rec := do(t, handler, http.MethodHead, "/api/v1/search?q=fixture"); rec.Code != http.StatusOK {
		t.Errorf("HEAD = %d, want 200", rec.Code)
	}
}

// TestSearchRouteIsDeterministic asserts two identical requests produce identical bytes.
//
// This is the property a client caching or diffing a response depends on, and it is the one
// that would break first if ordering ever fell back on Go map iteration.
func TestSearchRouteIsDeterministic(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		"/api/v1/search?q=fixture",
		"/api/v1/search?q=fixture&type=source",
		"/api/v1/search?q=a&limit=5&offset=2",
		"/api/v1/search?q=session-01-fixture",
	} {
		first := do(t, handler, http.MethodGet, target).Body.String()
		for i := 0; i < 4; i++ {
			if got := do(t, handler, http.MethodGet, target).Body.String(); got != first {
				t.Fatalf("GET %s returned different bytes on repeat:\n%s\n%s", target, first, got)
			}
		}
	}

	// A second handler over a second index of the same corpus must agree too.
	other := newHandler(t)
	target := "/api/v1/search?q=fixture&limit=200"
	if a, b := do(t, handler, http.MethodGet, target).Body.String(),
		do(t, other, http.MethodGet, target).Body.String(); a != b {
		t.Errorf("two indexes over one corpus disagreed:\n%s\n%s", a, b)
	}
}

// TestSearchRouteAdversarialQueries asserts a hostile query string is data and nothing else.
//
// None of these may be interpreted as a path, a pattern, a program or a command; each is
// either a literal substring of a canonical field or it matches nothing, and either way the
// response is a well-formed JSON envelope with the right content type.
func TestSearchRouteAdversarialQueries(t *testing.T) {
	handler := newHandler(t)

	queries := []string{
		".*", "^.*$", "[a-z]+", "(", ")", "\\", "|", "+",
		"*", "**/*", "?", "../../../etc/passwd", "..\\..\\..\\windows\\system32",
		"/etc/passwd", "C:\\Users\\britb", "nodes/acoustics/alpha.md",
		"http://127.0.0.1:8788/api/v1/nodes", "https://example.com/?a=b#c",
		"'; DROP TABLE nodes; --", "<script>alert(1)</script>", "${jndi:ldap://x}",
		"$(whoami)", "`id`", "&& rm -rf /", "%00", "%2e%2e%2f",
		"1999", "-1", "0", "3.14159", "1e400",
		"共鳴", "résonance", "Ω", "🎵🎶", "ＡＬＰＨＡ",
		strings.Repeat("alpha ", 20),
	}
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": query}))
			if rec.Code != http.StatusOK {
				t.Fatalf("q=%q = %d, want 200: %s", query, rec.Code, rec.Body.String())
			}
			var results service.SearchResults
			decode(t, rec, &results)
			if results.Results == nil {
				t.Errorf("q=%q returned a null results field", query)
			}
			if results.Page.Count != len(results.Results) {
				t.Errorf("q=%q page count %d, %d results", query, results.Page.Count, len(results.Results))
			}
			for _, result := range results.Results {
				if len(result.MatchedFields) == 0 {
					t.Errorf("q=%q matched %s/%s with no field named", query, result.EntityType, result.ID)
				}
			}
		})
	}

	// The pattern that would match every record if it were ever compiled matches nothing.
	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": ".*"}))
	var wildcard service.SearchResults
	decode(t, rec, &wildcard)
	if wildcard.Page.Total != 0 {
		t.Errorf(".* matched %d records, so the query was not treated as literal text", wildcard.Page.Total)
	}

	// A path-shaped query cannot reach the filesystem; it is looked for as text and found only
	// where a canonical field genuinely contains it.
	rec = do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "../../../etc/passwd"}))
	var traversal service.SearchResults
	decode(t, rec, &traversal)
	if traversal.Page.Total != 0 {
		t.Errorf("a path-shaped query matched %d records", traversal.Page.Total)
	}
}

// TestSearchRouteDoesNotLeakInternals asserts the response discloses corpus data and nothing
// about the machine serving it.
func TestSearchRouteDoesNotLeakInternals(t *testing.T) {
	handler := newHandler(t)

	bodies := []string{
		do(t, handler, http.MethodGet, "/api/v1/search?q=fixture&limit=200").Body.String(),
		do(t, handler, http.MethodGet, "/api/v1/search?q=fixture&type=nonsense").Body.String(),
		do(t, handler, http.MethodGet, "/api/v1/search").Body.String(),
	}
	for _, body := range bodies {
		for _, leak := range []string{"C:\\", "/Users/", "testdata", ".md", ".yaml", "goroutine", "internal/service"} {
			if strings.Contains(body, leak) {
				t.Errorf("response contains %q: %s", leak, body)
			}
		}
	}
}
