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

// The Phase 1F context tests at the HTTP boundary. Resolution itself is tested in the service,
// where the exact canonical relations are asserted; what matters here is the wire contract: the
// control, the serialised shape, backwards compatibility, and how a malformed request is
// refused.

// searchWithContext runs one search over HTTP and returns the decoded results.
func searchWithContext(t testing.TB, handler http.Handler, params map[string]string) service.SearchResults {
	t.Helper()
	rec := do(t, handler, http.MethodGet, searchURL(params))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)
	return results
}

// TestSearchRouteContextIsOptIn is the backwards-compatibility assertion at the wire.
//
// The Phase 1E body is unchanged for a Phase 1E request: the raw JSON carries no context key
// and no include_context key at all. This is asserted on the bytes rather than on the decoded
// struct, because an omitted field and a field serialised as null decode identically and only
// one of them keeps an existing client's parser working the way it did.
func TestSearchRouteContextIsOptIn(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `"context"`) {
		t.Error("a search that did not ask for context serialised a context key")
	}
	if strings.Contains(body, `"include_context"`) {
		t.Error("a search that did not ask for context echoed the control")
	}

	// An explicit false is the same request as an absent parameter.
	explicit := do(t, handler, http.MethodGet,
		searchURL(map[string]string{"q": "fixture", "include_context": "false"}))
	if explicit.Body.String() != body {
		t.Error("include_context=false did not produce the same body as omitting it")
	}
}

// TestSearchRouteResolvesContext covers the served shape of a context item.
//
// Every field of the contract is checked on the wire: the relation, the typed identity of the
// entity, its canonical label, the field the relation was read from, and the counts that say
// whether anything was left out.
func TestSearchRouteResolvesContext(t *testing.T) {
	handler := newHandler(t)
	results := searchWithContext(t, handler, map[string]string{"q": "fixture", "include_context": "true"})

	if !results.IncludeContext {
		t.Error("a context request did not echo include_context")
	}
	if len(results.Results) == 0 {
		t.Fatal("the query matched nothing, so this test would pass vacuously")
	}

	resolved := 0
	for _, result := range results.Results {
		if result.Context == nil {
			t.Fatalf("result %s/%s carries no context object", result.EntityType, result.ID)
		}
		if result.Context.Returned != len(result.Context.Related) {
			t.Errorf("%s/%s reports %d returned but carries %d",
				result.EntityType, result.ID, result.Context.Returned, len(result.Context.Related))
		}
		if result.Context.Returned > result.Context.Count {
			t.Errorf("%s/%s returned more context than it counted: %+v",
				result.EntityType, result.ID, result.Context)
		}
		if result.Context.Truncated != (result.Context.Returned < result.Context.Count) {
			t.Errorf("%s/%s truncation flag disagrees with its counts: %+v",
				result.EntityType, result.ID, result.Context)
		}
		for _, relation := range result.Context.Related {
			resolved++
			if relation.Relation == "" || relation.Origin == "" {
				t.Errorf("context item %+v is missing its relation or origin", relation)
			}
			if relation.Entity.ID == "" || relation.Entity.Label == "" {
				t.Errorf("context entity %+v is missing its identity or label", relation.Entity)
			}
			if !domain.ValidSearchEntityType(string(relation.Entity.EntityType)) {
				t.Errorf("context named unsupported class %q", relation.Entity.EntityType)
			}
			// A context ref is an identity, not an embedded record and not a URL. The API's
			// identity conventions are what a client navigates by.
			if strings.Contains(relation.Entity.ID, "/") || strings.Contains(relation.Entity.ID, "http") {
				t.Errorf("context entity %q looks like a path or URL rather than a canonical ID",
					relation.Entity.ID)
			}
		}
	}
	if resolved == 0 {
		t.Fatal("no context was resolved at all, so this test would pass vacuously")
	}
}

// TestSearchRouteContextIsNavigable is the point of the phase at the API boundary.
//
// Every entity a context names must be fetchable through its own route by the identity the
// context gave. A ref that could not be followed would be decoration rather than navigation.
func TestSearchRouteContextIsNavigable(t *testing.T) {
	handler := newHandler(t)
	results := searchWithContext(t, handler, map[string]string{"q": "e", "include_context": "true"})

	routes := map[domain.SearchEntityType]string{
		domain.SearchSession:    "/api/v1/sessions/",
		domain.SearchNode:       "/api/v1/nodes/",
		domain.SearchClaim:      "/api/v1/claims/",
		domain.SearchSource:     "/api/v1/sources/",
		domain.SearchVocabulary: "/api/v1/vocabulary/",
		domain.SearchExperiment: "/api/v1/experiments/",
	}

	followed := map[domain.SearchEntityType]bool{}
	for _, result := range results.Results {
		for _, relation := range result.Context.Related {
			route, ok := routes[relation.Entity.EntityType]
			if !ok {
				t.Fatalf("context named class %q, which has no route", relation.Entity.EntityType)
			}
			rec := do(t, handler, http.MethodGet, route+relation.Entity.ID)
			if rec.Code != http.StatusOK {
				t.Errorf("following %s/%s returned %d: %s",
					relation.Entity.EntityType, relation.Entity.ID, rec.Code, rec.Body.String())
				continue
			}
			followed[relation.Entity.EntityType] = true
		}
	}
	if len(followed) < 5 {
		t.Errorf("context reached only %d classes: %v", len(followed), followed)
	}
}

// TestSearchRouteContextRejectsMalformedControl covers the error contract.
//
// include_context is a strict tri-state flag, exactly as the performed filter on
// /api/v1/experiment-runs is: only the two exact spellings are accepted. Guessing that "1",
// "yes" or "TRUE" means true would mean a caller who mistyped it silently received the opposite
// response shape. The refusal is the shared 400 invalid_query and leaks nothing.
func TestSearchRouteContextRejectsMalformedControl(t *testing.T) {
	handler := newHandler(t)

	for _, malformed := range []string{"1", "0", "yes", "no", "TRUE", "True", "on", "maybe", "-"} {
		rec := do(t, handler, http.MethodGet,
			searchURL(map[string]string{"q": "fixture", "include_context": malformed}))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("include_context=%s = %d, want 400: %s", malformed, rec.Code, rec.Body.String())
			continue
		}
		assertErrorCode(t, rec, "invalid_query")
		body := rec.Body.String()
		if strings.Contains(body, "goroutine") || strings.Contains(body, ".go:") ||
			strings.Contains(strings.ToLower(body), "testdata") {
			t.Errorf("include_context=%s leaked internal detail: %s", malformed, body)
		}
	}

	// A duplicated control is refused by the shared bound rather than silently taking the first
	// value, so a caller can never be handed a shape they did not ask for.
	rec := do(t, handler, http.MethodGet,
		"/api/v1/search?q=fixture&include_context=true&include_context=false")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("duplicated include_context = %d, want 400: %s", rec.Code, rec.Body.String())
	}

	// The control does not become valid on a request that is malformed for another reason: an
	// empty query is still refused before anything is resolved.
	rec = do(t, handler, http.MethodGet, "/api/v1/search?q=&include_context=true")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty q with context = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestSearchRouteContextIsDeterministic asserts the same request returns the same bytes.
//
// The context of one result is assembled from several canonical sources and from an adjacency
// index whose own map iteration is randomised, so byte equality across repeated requests is the
// assertion that the ordering contract is real rather than usually true.
func TestSearchRouteContextIsDeterministic(t *testing.T) {
	handler := newHandler(t)
	target := searchURL(map[string]string{"q": "e", "include_context": "true", "limit": "200"})

	first := do(t, handler, http.MethodGet, target).Body.String()
	if !strings.Contains(first, `"context"`) {
		t.Fatal("the request resolved no context, so this test would pass vacuously")
	}
	for i := 0; i < 5; i++ {
		if again := do(t, handler, http.MethodGet, target).Body.String(); again != first {
			t.Fatalf("response %d differed from the first", i)
		}
	}

	// The same handler serving a fresh index over the same corpus produces the same bytes too,
	// so determinism survives a restart rather than only a repeated read of one build.
	if rebuilt := do(t, newHandler(t), http.MethodGet, target).Body.String(); rebuilt != first {
		t.Error("a rebuilt index served a different context")
	}
}

// TestSearchRouteContextIsRefusedNowhereElse keeps the control on the one route that owns it.
//
// Phase 1F adds one contract, not a parameter every list endpoint quietly grows. A layer list
// asked for context is refused by the shared unknown-parameter bound, so a caller cannot
// believe they received context that was never resolved.
func TestSearchRouteContextIsRefusedNowhereElse(t *testing.T) {
	handler := newHandler(t)

	for _, route := range []string{
		"/api/v1/nodes", "/api/v1/sessions", "/api/v1/claims", "/api/v1/sources",
		"/api/v1/vocabulary", "/api/v1/experiments", "/api/v1/experiment-runs",
		"/api/v1/graph/entities/node/alpha/relationships",
	} {
		rec := do(t, handler, http.MethodGet, route+"?include_context=true")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s?include_context=true = %d, want 400: %s", route, rec.Code, rec.Body.String())
			continue
		}
		assertErrorCode(t, rec, "invalid_query")
	}
}

// TestSearchRouteContextStaysReadOnly asserts the layer stores nothing.
//
// Context resolution is a read of an immutable startup index. Two identical requests separated
// by a differently shaped one return identical bytes, which is what a cache, a materialised
// context or a query log written on the request path would break.
func TestSearchRouteContextStaysReadOnly(t *testing.T) {
	handler := newHandler(t)
	target := searchURL(map[string]string{"q": "fixture", "include_context": "true"})

	before := do(t, handler, http.MethodGet, target).Body.String()
	do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "alpha", "include_context": "true"}))
	do(t, handler, http.MethodGet, searchURL(map[string]string{"q": "fixture"}))
	if after := do(t, handler, http.MethodGet, target).Body.String(); after != before {
		t.Error("an intervening request changed a later context response")
	}

	// Every mutating method is still refused on the route, context or no context.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := do(t, handler, method, target)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/v1/search = %d, want 405", method, rec.Code)
		}
	}
}

// TestSearchRouteContextShapeIsExplicit asserts the serialised object is typed rather than a
// free-form map: exactly the documented keys, no more.
//
// The check is on the raw JSON because the contract a client codes against is the wire shape.
// An unstructured payload would be inspectable only by reading the implementation, which is
// what a stable API contract exists to avoid.
func TestSearchRouteContextShapeIsExplicit(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, searchURL(map[string]string{
		"q": "fixture-term", "include_context": "true",
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var envelope struct {
		Results []struct {
			Context *struct {
				Related []map[string]json.RawMessage `json:"related"`
			} `json:"context"`
		} `json:"results"`
	}
	decode(t, rec, &envelope)
	if len(envelope.Results) == 0 || envelope.Results[0].Context == nil {
		t.Fatal("no context was served, so this test would pass vacuously")
	}
	if len(envelope.Results[0].Context.Related) == 0 {
		t.Fatal("the first result resolved no context, so this test would pass vacuously")
	}

	want := []string{"derived", "entity", "origin", "relation"}
	for _, item := range envelope.Results[0].Context.Related {
		got := make([]string, 0, len(item))
		for key := range item {
			got = append(got, key)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("context item has keys %v, want exactly %v", got, want)
		}
	}

	// The entity inside one item is a typed identity too, not an embedded record.
	var entity map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Results[0].Context.Related[0]["entity"], &entity); err != nil {
		t.Fatalf("decode context entity: %v", err)
	}
	wantEntity := []string{"entity_type", "id", "label"}
	got := make([]string, 0, len(entity))
	for key := range entity {
		got = append(got, key)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, wantEntity) {
		t.Errorf("context entity has keys %v, want exactly %v", got, wantEntity)
	}
}
