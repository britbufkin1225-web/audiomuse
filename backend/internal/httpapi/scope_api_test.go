package httpapi_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// Phase 1H over HTTP: the entity_types scope filter and the facet object.
//
// The handler splits one parameter and maps three typed errors; which records a scope keeps is
// asserted at the service. What is checked here is the wire contract a client actually sees —
// how the list is parsed, what the facet object serialises as, that a malformed list is refused
// without leaking anything, and that an existing request is unchanged apart from the additive
// facets key.

// searchBody decodes a successful search response into the typed contract.
func searchBody(t testing.TB, handler http.Handler, target string) service.SearchResults {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)
	return results
}

// rejectedScope requires a 400 carrying the stable error envelope, and requires that envelope
// to disclose nothing about the process serving it.
func rejectedScope(t testing.TB, handler http.Handler, target string) string {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET %s = %d, want 400: %s", target, rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	if body.Error.Code != "invalid_query" {
		t.Errorf("GET %s error code = %q, want invalid_query", target, body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Errorf("GET %s returned an empty error message", target)
	}
	for _, leak := range []string{
		"service.", "domain.", "httpapi.", "goroutine", ".go:", "C:\\", "/Users/", "internal/",
	} {
		if strings.Contains(body.Error.Message, leak) {
			t.Errorf("GET %s error message leaked %q: %s", target, leak, body.Error.Message)
		}
	}
	return body.Error.Message
}

// TestSearchRouteEntityTypesParsesTheList covers the accepted spellings: one class, several
// comma-separated classes, the whole set, and members padded with whitespace.
func TestSearchRouteEntityTypesParsesTheList(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		name  string
		value string
		want  []string
	}{
		{"one class", "node", []string{"node"}},
		{"two classes", "node,claim", []string{"node", "claim"}},
		{"caller order is not canonical order", "experiment,session", []string{"session", "experiment"}},
		{"padded members", " node , claim ", []string{"node", "claim"}},
		{
			"every searchable class",
			strings.Join(domain.SearchEntityTypeNames(), ","),
			domain.SearchEntityTypeNames(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results := searchBody(t, handler, searchURL(map[string]string{
				"q": "fixture", "entity_types": tc.value, "limit": "200",
			}))
			if !reflect.DeepEqual(results.EntityTypes, tc.want) {
				t.Fatalf("echoed entity_types = %v, want %v", results.EntityTypes, tc.want)
			}
			allowed := map[string]bool{}
			for _, name := range tc.want {
				allowed[name] = true
			}
			for _, result := range results.Results {
				if !allowed[string(result.EntityType)] {
					t.Errorf("%s/%s is outside the requested scope", result.EntityType, result.ID)
				}
			}
			if got := results.Facets.EntityTypes.Total(); got != results.Page.Total {
				t.Errorf("facet sum = %d, page.total = %d", got, results.Page.Total)
			}
		})
	}

	// A scope naming one class returns the same records as the single-class type filter, so the
	// two spellings agree over the wire as well as in the service.
	list := searchBody(t, handler, searchURL(map[string]string{
		"q": "fixture", "entity_types": "vocabulary", "limit": "200",
	}))
	single := searchBody(t, handler, searchURL(map[string]string{
		"q": "fixture", "type": "vocabulary", "limit": "200",
	}))
	if !reflect.DeepEqual(resultRefs(list), resultRefs(single)) {
		t.Errorf("entity_types=vocabulary = %v, type=vocabulary = %v",
			resultRefs(list), resultRefs(single))
	}
}

// TestSearchRouteFacetShape pins the serialised facet contract: one facets object, one
// entity_types member, exactly the six searchable classes as keys, every one present with an
// integer value even at zero, and no experiment_run key.
func TestSearchRouteFacetShape(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		searchURL(map[string]string{"q": "fixture", "limit": "200"}),
		searchURL(map[string]string{"q": "fixture", "entity_types": "claim", "limit": "200"}),
		searchURL(map[string]string{"q": "no-canonical-record-contains-this-text"}),
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var envelope struct {
				Facets map[string]json.RawMessage `json:"facets"`
			}
			decode(t, rec, &envelope)
			if len(envelope.Facets) != 1 {
				t.Fatalf("facets carries %d keys, want exactly entity_types: %v",
					len(envelope.Facets), envelope.Facets)
			}
			raw, ok := envelope.Facets["entity_types"]
			if !ok {
				t.Fatal("facets carries no entity_types member")
			}
			var counts map[string]int
			if err := json.Unmarshal(raw, &counts); err != nil {
				t.Fatalf("decode facets.entity_types: %v", err)
			}
			want := domain.SearchEntityTypeNames()
			got := make([]string, 0, len(counts))
			for key := range counts {
				got = append(got, key)
			}
			sort.Strings(got)
			sorted := append([]string(nil), want...)
			sort.Strings(sorted)
			if !reflect.DeepEqual(got, sorted) {
				t.Errorf("facet keys = %v, want exactly %v", got, sorted)
			}
			if _, present := counts["experiment_run"]; present {
				t.Error("facets carry an experiment_run key")
			}

			// Serialised key order is the canonical class order, not an alphabetisation of it.
			body := rec.Body.String()
			at := -1
			for _, class := range want {
				next := strings.Index(body, `"`+class+`":`)
				if next < 0 {
					t.Fatalf("facet key %q is absent from the body: %s", class, body)
				}
				if next < at {
					t.Fatalf("facet keys are not in canonical class order: %s", body)
				}
				at = next
			}
		})
	}

	// An empty result set serialises the complete zero-valued structure rather than an empty
	// object, so "nothing matched" is an answer a client can read directly.
	empty := do(t, handler, http.MethodGet,
		searchURL(map[string]string{"q": "no-canonical-record-contains-this-text"})).Body.String()
	for _, class := range domain.SearchEntityTypeNames() {
		if !strings.Contains(empty, `"`+class+`":0`) {
			t.Errorf("an empty search omitted the zero facet for %s: %s", class, empty)
		}
	}
}

// TestSearchRouteFacetsSurvivePaging: paging changes the page and nothing else. The facet
// object is byte-identical across every page of one search.
func TestSearchRouteFacetsSurvivePaging(t *testing.T) {
	handler := newHandler(t)
	full := searchBody(t, handler, searchURL(map[string]string{
		"q": "e", "entity_types": "node,claim,vocabulary", "limit": "200",
	}))
	if full.Page.Total < 3 {
		t.Fatalf("scoped baseline matched %d records; too few to page", full.Page.Total)
	}

	for _, paging := range []map[string]string{
		{"limit": "1"},
		{"limit": "1", "offset": "1"},
		{"limit": "2", "offset": "2"},
		{"limit": "50", "offset": "500"},
	} {
		params := map[string]string{"q": "e", "entity_types": "node,claim,vocabulary"}
		for key, value := range paging {
			params[key] = value
		}
		page := searchBody(t, handler, searchURL(params))
		if page.Facets != full.Facets {
			t.Errorf("%v facets = %+v, want %+v", paging, page.Facets, full.Facets)
		}
		if page.Page.Total != full.Page.Total {
			t.Errorf("%v page.total = %d, want %d", paging, page.Page.Total, full.Page.Total)
		}
		if got := page.Facets.EntityTypes.Total(); got != page.Page.Total {
			t.Errorf("%v facet sum = %d, page.total = %d", paging, got, page.Page.Total)
		}
	}
}

// TestSearchRouteEntityTypesComposesWithEveryControl requires the scope to be orthogonal to the
// controls that already exist: composition, context and paging.
func TestSearchRouteEntityTypesComposesWithEveryControl(t *testing.T) {
	handler := newHandler(t)

	// query_mode=all_terms.
	composed := searchBody(t, handler, searchURL(map[string]string{
		"q": "fixture alpha", "query_mode": "all_terms",
		"entity_types": "node,claim", "limit": "200",
	}))
	if composed.QueryMode != string(domain.SearchModeAllTerms) {
		t.Errorf("query_mode echo = %q under a scoped composed search", composed.QueryMode)
	}
	if !reflect.DeepEqual(composed.EntityTypes, []string{"node", "claim"}) {
		t.Errorf("entity_types echo = %v under a composed search", composed.EntityTypes)
	}
	for _, result := range composed.Results {
		if result.EntityType != domain.SearchNode && result.EntityType != domain.SearchClaim {
			t.Errorf("%s/%s is outside the composed scope", result.EntityType, result.ID)
		}
		if result.MatchKind != domain.MatchAllTerms {
			t.Errorf("%s/%s match_kind = %q under all_terms", result.EntityType, result.ID, result.MatchKind)
		}
		if len(result.TermMatches) == 0 {
			t.Errorf("%s/%s lost its per-term evidence under a scope", result.EntityType, result.ID)
		}
	}
	if got := composed.Facets.EntityTypes.Total(); got != composed.Page.Total {
		t.Errorf("composed facet sum = %d, page.total = %d", got, composed.Page.Total)
	}

	// include_context.
	contextual := searchBody(t, handler, searchURL(map[string]string{
		"q": "fixture", "entity_types": "claim", "include_context": "true", "limit": "200",
	}))
	if !contextual.IncludeContext {
		t.Error("include_context echo lost under a scoped search")
	}
	if len(contextual.Results) == 0 {
		t.Fatal("no scoped claim hit to resolve context for")
	}
	for _, result := range contextual.Results {
		if result.Context == nil {
			t.Errorf("%s/%s carries no context under include_context", result.EntityType, result.ID)
		}
	}
	if got := contextual.Facets.EntityTypes.Total(); got != contextual.Page.Total {
		t.Errorf("contextual facet sum = %d, page.total = %d", got, contextual.Page.Total)
	}
}

// TestSearchRouteRejectsMalformedEntityTypes walks every malformed spelling a query string can
// carry. Each is a 400 in the stable envelope; none is repaired into a working search.
func TestSearchRouteRejectsMalformedEntityTypes(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		name  string
		value string
	}{
		{"unknown class", "widget"},
		{"unknown class among valid ones", "node,widget"},
		{"experiment run", "experiment_run"},
		{"experiment run among valid ones", "experiment,experiment_run"},
		{"wrong case", "Node"},
		{"upper case", "NODE"},
		{"plural", "nodes"},
		{"empty parameter", ""},
		{"whitespace-only parameter", "   "},
		{"leading comma", ",node"},
		{"trailing comma", "node,"},
		{"consecutive commas", "node,,claim"},
		{"only a comma", ","},
		{"whitespace-only member", "node, ,claim"},
		{"duplicate", "node,node"},
		{"duplicate after trimming", "node, node"},
		{"duplicate among valid values", "claim,node,claim"},
		{"space separated rather than comma separated", "node claim"},
		{"semicolon separated", "node;claim"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := rejectedScope(t, handler, searchURL(map[string]string{
				"q": "fixture", "entity_types": tc.value,
			}))
			if strings.Contains(message, tc.value) && tc.value != "" && strings.TrimSpace(tc.value) != "" {
				t.Errorf("the error echoed the caller's value %q: %s", tc.value, message)
			}
		})
	}

	// A malformed scope is refused even when the rest of the request is valid and would have
	// succeeded, so the filter is never silently dropped from an otherwise working search.
	for _, params := range []map[string]string{
		{"q": "fixture", "entity_types": "node,node", "limit": "10"},
		{"q": "fixture alpha", "query_mode": "all_terms", "entity_types": "node,node"},
		{"q": "fixture", "include_context": "true", "entity_types": "node,widget"},
		{"q": "fixture", "entity_types": "node,widget", "limit": "5", "offset": "5"},
	} {
		rejectedScope(t, handler, searchURL(params))
	}

	// The two class filters together are refused rather than intersected or silently reduced
	// to one of them.
	for _, params := range []map[string]string{
		{"q": "fixture", "type": "node", "entity_types": "node"},
		{"q": "fixture", "type": "node", "entity_types": "claim"},
	} {
		message := rejectedScope(t, handler, searchURL(params))
		if !strings.Contains(message, "entity_types") || !strings.Contains(message, "type") {
			t.Errorf("the conflict message names neither parameter: %s", message)
		}
	}

	// An unsupported class lists the closed searchable set, so a caller learns what is accepted
	// without reading the source.
	message := rejectedScope(t, handler, searchURL(map[string]string{
		"q": "fixture", "entity_types": "experiment_run",
	}))
	for _, class := range domain.SearchEntityTypeNames() {
		if !strings.Contains(message, class) {
			t.Errorf("the rejection message omits the searchable class %q: %s", class, message)
		}
	}
	if strings.Contains(message, "experiment_run") {
		t.Errorf("the rejection message echoed experiment_run: %s", message)
	}

	// The parameter is still bound by the shared query-string rules: supplied twice is refused
	// exactly as every other parameter is, so two lists can never be silently reduced to one.
	rejectedScope(t, handler, "/api/v1/search?q=fixture&entity_types=node&entity_types=claim")
}

// TestSearchRouteEntityTypesIsDeterministic requires two identical scoped requests, and two
// spellings of one scope, to produce identical bytes.
func TestSearchRouteEntityTypesIsDeterministic(t *testing.T) {
	handler := newHandler(t)
	target := searchURL(map[string]string{
		"q": "e", "entity_types": "node,claim", "limit": "200",
	})

	first := do(t, handler, http.MethodGet, target).Body.String()
	// A differently shaped request in between must not change the next response.
	do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "alpha"}))
	second := do(t, handler, http.MethodGet, target).Body.String()
	if first != second {
		t.Errorf("two identical scoped requests returned different bytes:\n%s\n%s", first, second)
	}

	reordered := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "e", "entity_types": "claim,node", "limit": "200",
	})).Body.String()
	if reordered != first {
		t.Errorf("class-list order changed the serialised response:\n%s\n%s", first, reordered)
	}

	padded := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "e", "entity_types": " node , claim ", "limit": "200",
	})).Body.String()
	if padded != first {
		t.Errorf("padding changed the serialised response:\n%s\n%s", first, padded)
	}
}

// TestSearchRouteWithoutEntityTypesIsUnchanged is the regression assertion at the edge. A
// request that names no scope serialises no entity_types key, keeps every established field,
// and differs from its Phase 1G self only by the additive facets object.
func TestSearchRouteWithoutEntityTypesIsUnchanged(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		searchURL(map[string]string{"q": "fixture"}),
		searchURL(map[string]string{"q": "fixture", "type": "vocabulary"}),
		searchURL(map[string]string{"q": "alpha", "limit": "5", "offset": "1"}),
		searchURL(map[string]string{"q": "fixture", "include_context": "true"}),
		searchURL(map[string]string{"q": "fixture alpha", "query_mode": "all_terms"}),
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var envelope map[string]json.RawMessage
			decode(t, rec, &envelope)
			if _, present := envelope["entity_types"]; present {
				t.Error("an unscoped response carries an entity_types key")
			}
			for _, required := range []string{"query", "page", "results", "facets"} {
				if _, present := envelope[required]; !present {
					t.Errorf("the response lost its %s key: %s", required, rec.Body.String())
				}
			}
			var results service.SearchResults
			decode(t, rec, &results)
			if got := results.Facets.EntityTypes.Total(); got != results.Page.Total {
				t.Errorf("facet sum = %d, page.total = %d", got, results.Page.Total)
			}
		})
	}

	// The one key Phase 1H adds to an existing response is facets: strip it, and an unscoped
	// body is exactly the Phase 1G body it always was.
	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture", "limit": "200"}))
	var envelope map[string]json.RawMessage
	decode(t, rec, &envelope)
	delete(envelope, "facets")
	for key := range envelope {
		switch key {
		case "query", "query_mode", "type", "include_context", "page", "results":
		default:
			t.Errorf("an unscoped response carries an unexpected key %q", key)
		}
	}
}
