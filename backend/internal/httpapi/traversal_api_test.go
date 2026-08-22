package httpapi_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// The Phase 1C HTTP surface. These tests assert status codes, error codes and the bounds
// the handlers own; the graph semantics themselves are asserted in the service package
// against the same fixture corpus.

const graphBase = "/api/v1/graph/entities/"

func TestEntityRelationshipsEndpoint(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		target        string
		wantRelations int
		wantNeighbors int
	}{
		{graphBase + "node/alpha/relationships", 7, 7},
		{graphBase + "session/session-01-fixture/relationships", 3, 3},
		{graphBase + "claim/beta-was-observed-in-1999/relationships", 5, 5},
		{graphBase + "source/fixture-reference-work/relationships", 6, 6},
		// Exists, but nothing in the corpus references it.
		{graphBase + "source/fixture-uncited-source/relationships", 0, 0},
	} {
		rec := do(t, handler, http.MethodGet, tc.target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200 (%s)", tc.target, rec.Code, rec.Body.String())
		}
		var body service.EntityRelationships
		decode(t, rec, &body)
		if len(body.Relationships) != tc.wantRelations {
			t.Errorf("%s relationships = %d, want %d", tc.target, len(body.Relationships), tc.wantRelations)
		}
		if len(body.Neighbors) != tc.wantNeighbors {
			t.Errorf("%s neighbours = %d, want %d", tc.target, len(body.Neighbors), tc.wantNeighbors)
		}
		if body.Counts.Relationships != len(body.Relationships) || body.Counts.Entities != len(body.Neighbors) {
			t.Errorf("%s counts = %#v", tc.target, body.Counts)
		}
		if body.Partial {
			t.Errorf("%s reported partial over the fixture corpus", tc.target)
		}
	}
}

func TestTraverseEndpoint(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		target       string
		wantDepth    int
		wantEntities int
	}{
		{graphBase + "session/session-01-fixture/traverse", 1, 4},
		{graphBase + "session/session-01-fixture/traverse?depth=1", 1, 4},
		{graphBase + "session/session-01-fixture/traverse?depth=2", 2, 11},
		{graphBase + "session/session-01-fixture/traverse?depth=3", 3, 12},
	} {
		rec := do(t, handler, http.MethodGet, tc.target)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200 (%s)", tc.target, rec.Code, rec.Body.String())
		}
		var body service.Traversal
		decode(t, rec, &body)
		if body.Depth != tc.wantDepth {
			t.Errorf("%s depth = %d, want %d", tc.target, body.Depth, tc.wantDepth)
		}
		if len(body.Entities) != tc.wantEntities {
			t.Errorf("%s entities = %d, want %d", tc.target, len(body.Entities), tc.wantEntities)
		}
		if body.Root.ID != "session-01-fixture" || string(body.Root.Type) != "session" {
			t.Errorf("%s root = %#v", tc.target, body.Root)
		}
		if body.Entities[0].Distance != 0 || body.Entities[0].ID != "session-01-fixture" {
			t.Errorf("%s first entity = %#v", tc.target, body.Entities[0])
		}
		if body.Partial || body.TruncationReason != "" {
			t.Errorf("%s reported truncation over the fixture corpus", tc.target)
		}
	}
}

// TestTraverseFilters covers the two supported controls at the HTTP edge.
func TestTraverseFilters(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, graphBase+"node/alpha/traverse?depth=3&relationship=sourced_from")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s)", rec.Code, rec.Body.String())
	}
	var body service.Traversal
	decode(t, rec, &body)
	if len(body.Entities) != 3 {
		t.Errorf("filtered entities = %d, want 3", len(body.Entities))
	}
	if body.Filters.Relationship != "sourced_from" {
		t.Errorf("filters = %#v", body.Filters)
	}

	rec = do(t, handler, http.MethodGet, graphBase+"node/alpha/relationships?target_type=source")
	var direct service.EntityRelationships
	decode(t, rec, &direct)
	if len(direct.Relationships) != 2 {
		t.Errorf("target_type filtered relationships = %d, want 2", len(direct.Relationships))
	}
}

// TestTraverseRejectsBadInput asserts every validation path answers 400 with the stable
// invalid_query code rather than coercing nonsense into a valid request.
func TestTraverseRejectsBadInput(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		// Unsupported entity classes, including layers the backend does not parse.
		graphBase + "banana/123/traverse",
		graphBase + "banana/123/relationships",
		graphBase + "vocabulary/fixture-term/traverse",
		graphBase + "experiment_run/anything/traverse",
		graphBase + "Node/alpha/traverse",
		graphBase + "nodes/alpha/traverse",
		// Depth outside the accepted range, or not an integer at all.
		graphBase + "node/alpha/traverse?depth=0",
		graphBase + "node/alpha/traverse?depth=-1",
		graphBase + "node/alpha/traverse?depth=4",
		graphBase + "node/alpha/traverse?depth=999999",
		graphBase + "node/alpha/traverse?depth=banana",
		graphBase + "node/alpha/traverse?depth=1.5",
		graphBase + "node/alpha/traverse?depth=" + url.QueryEscape("1 OR 1=1"),
		// Filter values outside the closed vocabularies.
		graphBase + "node/alpha/traverse?relationship=whatever-i-want",
		graphBase + "node/alpha/traverse?target_type=banana",
		graphBase + "node/alpha/relationships?relationship=whatever-i-want",
		graphBase + "node/alpha/relationships?target_type=vocabulary",
		// Parameters this endpoint does not accept, and repeated parameters.
		graphBase + "node/alpha/traverse?limit=5",
		graphBase + "node/alpha/relationships?depth=2",
		graphBase + "node/alpha/traverse?depth=1&depth=2",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want 400 (%s)", target, rec.Code, rec.Body.String())
			continue
		}
		var body struct {
			Error struct{ Code, Message string }
		}
		decode(t, rec, &body)
		if body.Error.Code != "invalid_query" {
			t.Errorf("%s error code = %q, want invalid_query", target, body.Error.Code)
		}
	}
}

// TestTraverseUnknownEntityIsNotFound asserts that a well-formed request for a record the
// corpus does not contain answers 404, so a caller can tell it apart from an entity that
// exists and has no relationships.
func TestTraverseUnknownEntityIsNotFound(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		graphBase + "node/not-a-real-node/relationships",
		graphBase + "node/not-a-real-node/traverse",
		graphBase + "claim/not-a-real-claim/traverse",
		graphBase + "source/not-a-real-source/traverse",
		graphBase + "session/not-a-real-session/traverse",
		// A real node ID addressed as a claim: identity is (type, id).
		graphBase + "claim/alpha/traverse",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404 (%s)", target, rec.Code, rec.Body.String())
		}
		var body struct {
			Error struct{ Code, Message string }
		}
		decode(t, rec, &body)
		if body.Error.Code != "entity_not_found" {
			t.Errorf("%s error code = %q, want entity_not_found", target, body.Error.Code)
		}
	}

	// The distinction the 404 protects: this entity exists and simply has no edges.
	rec := do(t, handler, http.MethodGet, graphBase+"source/fixture-uncited-source/traverse")
	if rec.Code != http.StatusOK {
		t.Fatalf("existing entity with no relationships = %d, want 200", rec.Code)
	}
	var body service.Traversal
	decode(t, rec, &body)
	if len(body.Entities) != 1 || len(body.Relationships) != 0 {
		t.Errorf("isolated entity traversal = %#v", body.Counts)
	}
}

// TestTraverseHostileIdentifiers asserts that a traversal identifier can never reach the
// filesystem. Every value below is refused or resolves to nothing; none returns file
// content and none discloses a local path.
func TestTraverseHostileIdentifiers(t *testing.T) {
	handler := newHandler(t)

	hostile := []string{
		"../../README.md",
		"..%2f..%2fREADME.md",
		"%2e%2e%2f%2e%2e%2fREADME.md",
		"..\\..\\README.md",
		"..%5c..%5cREADME.md",
		"C:\\Windows\\System32",
		"C%3A%5CWindows%5CSystem32",
		"/etc/passwd",
		"%2fetc%2fpasswd",
		"....//....//README.md",
		"alpha%00.md",
		"nodes/acoustics/alpha.md",
		strings.Repeat("a", 4096),
	}

	for _, id := range hostile {
		for _, suffix := range []string{"/relationships", "/traverse"} {
			target := graphBase + "node/" + id + suffix
			rec := do(t, handler, http.MethodGet, target)

			// A traversal separator is normalised away by net/http before routing, which
			// answers a redirect to the cleaned path. That is safe, but only if the cleaned
			// path is still inside this API and still does not resolve, so the redirect is
			// followed rather than accepted on trust.
			if rec.Code >= 300 && rec.Code < 400 {
				location := rec.Header().Get("Location")
				if !strings.HasPrefix(location, "/api/v1/") {
					t.Errorf("hostile id %q %s redirected outside the API: %q", id, suffix, location)
					continue
				}
				rec = do(t, handler, http.MethodGet, location)
			}

			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Errorf("hostile id %q %s status = %d, want 400 or 404 (%s)",
					id, suffix, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, leak := range []string{"# Fixture", "Synthetic", "root:", "id: alpha", "AudioMuse is a repository-first"} {
				if strings.Contains(body, leak) {
					t.Errorf("hostile id %q %s leaked file content: %s", id, suffix, body)
				}
			}
		}
	}

	// The same hostile values as an entity type must never be treated as a graph class.
	for _, entityType := range []string{"../../README.md", "%2e%2e%2f", "C:\\Windows", "node%00"} {
		rec := do(t, handler, http.MethodGet, graphBase+entityType+"/alpha/traverse")
		if rec.Code == http.StatusOK {
			t.Errorf("hostile entity type %q was accepted: %s", entityType, rec.Body.String())
		}
	}
}

// TestTraverseIsReadOnly asserts the method lock covers the new routes. It runs outside the
// router, so this is a regression guard rather than a restatement of Phase 1A.
func TestTraverseIsReadOnly(t *testing.T) {
	handler := newHandler(t)

	for _, method := range []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions,
	} {
		for _, target := range []string{
			graphBase + "node/alpha/relationships",
			graphBase + "node/alpha/traverse",
		} {
			rec := do(t, handler, method, target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s status = %d, want 405", method, target, rec.Code)
			}
		}
	}

	// HEAD is served from the GET handler with the body discarded.
	rec := do(t, handler, http.MethodHead, graphBase+"node/alpha/traverse")
	if rec.Code != http.StatusOK {
		t.Errorf("HEAD status = %d, want 200", rec.Code)
	}
}

// TestTraverseResponseDoesNotDiscloseLocalPaths guards the Phase 1A property that no
// response body names the operator's filesystem.
func TestTraverseResponseDoesNotDiscloseLocalPaths(t *testing.T) {
	handler := newHandler(t)
	rec := do(t, handler, http.MethodGet, graphBase+"node/alpha/traverse?depth=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), testsupport.CorpusRoot(t)) {
		t.Error("traversal response disclosed the absolute repository path")
	}
}
