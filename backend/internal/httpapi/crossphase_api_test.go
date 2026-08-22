package httpapi_test

import (
	"net/http"
	"testing"
)

// TestEveryPhaseRouteCoexistsOnOneHandler serves one representative route from each backend
// generation through a single router built over a single service instance.
//
// The phases were developed on branches that each added routes to the same mux, so the risk
// this covers is integration-shaped rather than handler-shaped: a route lost in a merge, or a
// pattern that shadows another. Go's ServeMux resolves by specificity rather than
// registration order, so `/vocabulary` and `/vocabulary/{id}` coexisting is not something to
// assume — it is something to assert, alongside the traversal routes nested under
// `/graph/entities/`, which is the one place a wildcard segment could swallow a sibling.
func TestEveryPhaseRouteCoexistsOnOneHandler(t *testing.T) {
	handler := newHandler(t)

	routes := []struct {
		phase  string
		target string
	}{
		{"1A", "/api/v1/nodes"},
		{"1A", "/api/v1/nodes/alpha"},
		{"1A", "/api/v1/sessions"},
		{"1A", "/api/v1/graph"},
		{"1B", "/api/v1/sources"},
		{"1B", "/api/v1/claims"},
		{"1B", "/api/v1/claims?node_id=alpha"},
		{"1C", "/api/v1/graph/entities/node/alpha/relationships"},
		{"1C", "/api/v1/graph/entities/node/alpha/traverse?depth=2"},
		{"1C", "/api/v1/graph/entities/session/session-01-fixture/traverse?depth=1"},
		{"1D", "/api/v1/vocabulary"},
		{"1D", "/api/v1/vocabulary/fixture-term"},
		{"1D", "/api/v1/experiments"},
		{"1D", "/api/v1/experiments/fixture-listening-exercise"},
		{"1D", "/api/v1/experiment-runs"},
		{"1D", "/api/v1/experiment-runs/fixture-listening-exercise-planned-a"},
		{"1E", "/api/v1/search?q=fixture"},
		{"1E", "/api/v1/search?q=fixture&type=vocabulary"},
		{"1E", "/api/v1/search?q=alpha&limit=5&offset=1"},
		{"shared", "/api/v1/project"},
		{"shared", "/api/v1/diagnostics"},
		{"shared", "/health"},
	}

	for _, route := range routes {
		t.Run(route.phase+" "+route.target, func(t *testing.T) {
			if rec := do(t, handler, http.MethodGet, route.target); rec.Code != http.StatusOK {
				t.Errorf("GET %s = %d, want 200: %s", route.target, rec.Code, rec.Body.String())
			}
			// HEAD is served by the same routes and must agree on status.
			if rec := do(t, handler, http.MethodHead, route.target); rec.Code != http.StatusOK {
				t.Errorf("HEAD %s = %d, want 200", route.target, rec.Code)
			}
		})
	}
}

// TestNoRouteShadowsAnother checks the pairs where one pattern could capture another's path.
//
// Each case asserts that the more specific route produced its own resource-specific error
// rather than the collection route's success or a generic code. A shadowed detail route would
// show up here as a 200 from the list handler, or as the wrong error code.
func TestNoRouteShadowsAnother(t *testing.T) {
	handler := newHandler(t)

	cases := []struct {
		target string
		code   string
	}{
		{"/api/v1/nodes/no-such-node", "node_not_found"},
		{"/api/v1/sessions/no-such-session", "session_not_found"},
		{"/api/v1/sources/no-such-source", "source_not_found"},
		{"/api/v1/claims/no-such-claim", "claim_not_found"},
		{"/api/v1/vocabulary/no-such-entry", "vocabulary_not_found"},
		{"/api/v1/experiments/no-such-experiment", "experiment_not_found"},
		{"/api/v1/experiment-runs/no-such-run", "experiment_run_not_found"},
		{"/api/v1/graph/entities/node/no-such-node/relationships", "entity_not_found"},
		{"/api/v1/graph/entities/node/no-such-node/traverse", "entity_not_found"},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, tc.target)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("GET %s = %d, want 404: %s", tc.target, rec.Code, rec.Body.String())
			}
			assertErrorCode(t, rec, tc.code)
		})
	}
}

// TestMutationIsRejectedOnEveryPhaseRoute asserts the read-only lock did not develop a hole
// at a route added by a later phase. The middleware runs before routing, so this is one
// guarantee rather than one per handler — which is exactly why it is worth checking that the
// newest routes are behind it too.
func TestMutationIsRejectedOnEveryPhaseRoute(t *testing.T) {
	handler := newHandler(t)

	targets := []string{
		"/api/v1/nodes",
		"/api/v1/claims",
		"/api/v1/graph/entities/node/alpha/traverse",
		"/api/v1/vocabulary",
		"/api/v1/experiments",
		"/api/v1/experiment-runs",
		"/api/v1/search?q=fixture",
	}
	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for _, target := range targets {
		for _, method := range methods {
			rec := do(t, handler, method, target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, target, rec.Code)
			}
			if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
				t.Errorf("%s %s Allow = %q, want %q", method, target, got, want)
			}
		}
	}
}

// TestDuplicateQueryParametersAreRejectedAcrossPhases covers the shared bound on every
// phase's list route at once. Duplicate parameters are refused rather than resolved to the
// first or last value, because either resolution would make one request mean two things.
func TestDuplicateQueryParametersAreRejectedAcrossPhases(t *testing.T) {
	handler := newHandler(t)

	targets := []string{
		"/api/v1/nodes?domain=acoustics&domain=dsp",
		"/api/v1/claims?node_id=alpha&node_id=beta",
		"/api/v1/graph/entities/node/alpha/traverse?depth=1&depth=2",
		"/api/v1/vocabulary?domain=acoustics&domain=dsp",
		"/api/v1/experiments?status=planned&status=validated",
		"/api/v1/experiment-runs?performed=true&performed=false",
		"/api/v1/search?q=alpha&q=beta",
		"/api/v1/search?q=alpha&type=node&type=claim",
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
