package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/httpapi"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

func newHandler(t testing.TB) http.Handler {
	t.Helper()
	repo, err := filesystem.NewFromFS(testsupport.CorpusFS(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open fixture corpus: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	return httpapi.NewServer(knowledge, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func do(t testing.TB, handler http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func decode(t testing.TB, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("content-type = %q, want %q", got, want)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
}

func TestHealth(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct{ Status, Service, Mode string }
	decode(t, rec, &body)
	if body.Status != "ok" || body.Service != "audiomuse-api" || body.Mode != "read-only" {
		t.Errorf("health = %#v", body)
	}
}

func TestProject(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/project")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body service.ProjectSummary
	decode(t, rec, &body)
	if body.Counts.Nodes != 3 || body.Counts.Edges != 3 || body.Counts.Sessions != 2 {
		t.Errorf("counts = %#v", body.Counts)
	}
	if body.Mode != "read-only" {
		t.Errorf("mode = %q", body.Mode)
	}

	// The absolute repository path must never reach a response body.
	if strings.Contains(rec.Body.String(), testsupport.CorpusRoot(t)) {
		t.Error("project response disclosed the absolute repository path")
	}
}

func TestNodesList(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/nodes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body service.NodeList
	decode(t, rec, &body)
	if got, want := body.Page.Total, 3; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := body.Nodes[0].ID, "alpha"; got != want {
		t.Errorf("first node = %q, want %q", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/nodes?domain=dsp")
	decode(t, rec, &body)
	if got, want := body.Page.Total, 1; got != want {
		t.Fatalf("filtered total = %d, want %d", got, want)
	}
	if got, want := body.Nodes[0].ID, "gamma"; got != want {
		t.Errorf("filtered node = %q, want %q", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/nodes?q=resonant")
	decode(t, rec, &body)
	if got, want := body.Page.Total, 1; got != want {
		t.Fatalf("search total = %d, want %d", got, want)
	}
}

func TestNodeByID(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/nodes/gamma")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var node struct {
		ID                   string `json:"id"`
		Relationships        []any  `json:"relationships"`
		InboundRelationships []any  `json:"inbound_relationships"`
	}
	decode(t, rec, &node)
	if node.ID != "gamma" {
		t.Errorf("id = %q", node.ID)
	}
	if node.Relationships == nil {
		t.Error("relationships serialised as null, want []")
	}
	if got, want := len(node.InboundRelationships), 2; got != want {
		t.Errorf("inbound = %d, want %d", got, want)
	}
}

func TestNodeNotFound(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/nodes/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec, "node_not_found")
}

func TestSessions(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/sessions")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body service.SessionList
	decode(t, rec, &body)
	if got, want := body.Page.Total, 2; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := body.Sessions[0].NodeIDs, 2; len(got) != want {
		t.Errorf("session node_ids = %v, want %d entries", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/sessions/session-01-fixture")
	if rec.Code != http.StatusOK {
		t.Fatalf("session detail status = %d, want 200", rec.Code)
	}
	rec = do(t, handler, http.MethodGet, "/api/v1/sessions/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing session status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec, "session_not_found")
}

func TestGraph(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/graph")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Nodes    []map[string]any `json:"nodes"`
		Edges    []map[string]any `json:"edges"`
		Metadata struct {
			NodeCount int `json:"node_count"`
			EdgeCount int `json:"edge_count"`
		} `json:"metadata"`
	}
	decode(t, rec, &body)
	if body.Metadata.NodeCount != 3 || body.Metadata.EdgeCount != 3 {
		t.Errorf("metadata = %#v", body.Metadata)
	}
	if len(body.Nodes) != 3 || len(body.Edges) != 3 {
		t.Errorf("nodes = %d, edges = %d", len(body.Nodes), len(body.Edges))
	}
}

func TestGraphResponseIsByteIdenticalAcrossRequests(t *testing.T) {
	handler := newHandler(t)
	first := do(t, handler, http.MethodGet, "/api/v1/graph").Body.String()
	second := do(t, handler, http.MethodGet, "/api/v1/graph").Body.String()
	if first != second {
		t.Error("graph response differed between two identical requests")
	}
}

func TestDiagnostics(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/diagnostics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body service.Diagnostics
	decode(t, rec, &body)
	if body.Counts.Fatal != 0 {
		t.Errorf("fatal = %d, want 0", body.Counts.Fatal)
	}
	if body.Counts.Warning == 0 {
		t.Error("want the fixture corpus warnings")
	}
}

// TestMutatingMethodsAreRejected is the read-only guarantee, asserted at the edge.
func TestMutatingMethodsAreRejected(t *testing.T) {
	handler := newHandler(t)
	methods := []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		http.MethodConnect, http.MethodOptions, http.MethodTrace,
	}
	targets := []string{
		"/health",
		"/api/v1/project",
		"/api/v1/nodes",
		"/api/v1/nodes/alpha",
		"/api/v1/sessions",
		"/api/v1/graph",
		"/api/v1/diagnostics",
		"/api/v1/anything-else",
		"/",
	}

	for _, method := range methods {
		for _, target := range targets {
			rec := do(t, handler, method, target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status = %d, want 405", method, target, rec.Code)
			}
			if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
				t.Errorf("%s %s: Allow = %q, want %q", method, target, got, want)
			}
			assertErrorCode(t, rec, "method_not_allowed")
		}
	}
}

func TestHeadIsAllowed(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodHead, "/api/v1/project")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestUnknownRouteReturnsJSONNotFound(t *testing.T) {
	// Phase 1B implemented /api/v1/claims, which this test previously used as its unrouted
	// placeholder. Experiments remain a canonical layer the backend does not serve.
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/experiments")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec, "not_found")
}

func TestInvalidQueryParameters(t *testing.T) {
	handler := newHandler(t)
	cases := []string{
		"/api/v1/nodes?limit=abc",
		"/api/v1/nodes?limit=-1",
		"/api/v1/nodes?offset=-1",
		"/api/v1/nodes?limit=1&limit=2",
		"/api/v1/nodes?q=alpha&q=beta",
		"/api/v1/nodes?unknown=1",
		"/api/v1/nodes?q=" + strings.Repeat("z", service.MaxQueryChars+1),
		"/api/v1/sessions?domain=acoustics",
		"/api/v1/graph?limit=5",
		"/health?verbose=1",
	}
	for _, target := range cases {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d, want 400", target, rec.Code)
			continue
		}
		assertErrorCode(t, rec, "invalid_query")
	}
}

// TestPathTraversalIsRejected confirms no route can be turned into a file reader. The ID is
// only ever a map key, and separators are refused before it even gets that far.
func TestPathTraversalIsRejected(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{
		"/api/v1/nodes/..%2f..%2fsources%2fsource-registry.yaml",
		"/api/v1/nodes/..%5c..%5cwindows%5csystem32",
		"/api/v1/nodes/" + strings.Repeat("a", 200),
		"/api/v1/sessions/..%2f..%2fetc%2fpasswd",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 400 or 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "audiomuse-source-registry") {
			t.Fatalf("GET %s served file contents", target)
		}
	}
}

// TestErrorsDoNotLeakInternals checks the error envelope stays free of filesystem and Go
// internals a client cannot act on.
func TestErrorsDoNotLeakInternals(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{"/api/v1/nodes/missing", "/api/v1/nope", "/api/v1/nodes?limit=x"} {
		body := do(t, handler, http.MethodGet, target).Body.String()
		for _, forbidden := range []string{"goroutine", ".go:", "C:\\", "/Users/", "panic"} {
			if strings.Contains(body, forbidden) {
				t.Errorf("GET %s leaked %q: %s", target, forbidden, body)
			}
		}
	}
}

func assertErrorCode(t testing.TB, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	if body.Error.Code != want {
		t.Errorf("error code = %q, want %q", body.Error.Code, want)
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
}
