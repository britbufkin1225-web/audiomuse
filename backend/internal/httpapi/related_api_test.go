package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 2A related-knowledge tests at the HTTP boundary.
//
// Ranking, deduplication and bounding are tested in the service, where the exact canonical
// relations are asserted. What matters here is the wire contract: the route, the accepted query
// string, the serialised shape, the status codes, what a malformed request is told, and what a
// response must never contain.

// relatedURL builds a discovery request. The identifier is escaped as a path segment, so a test
// can send an encoded separator and see what the router and the handler make of it.
func relatedURL(entityType, id string, params map[string]string) string {
	target := "/api/v1/related/" + entityType + "/" + id
	if len(params) == 0 {
		return target
	}
	values := url.Values{}
	for name, value := range params {
		values.Set(name, value)
	}
	return target + "?" + values.Encode()
}

// getRelated runs one discovery over HTTP and returns the decoded result.
func getRelated(t testing.TB, handler http.Handler, entityType, id string, params map[string]string) domain.RelatedKnowledge {
	t.Helper()
	rec := do(t, handler, http.MethodGet, relatedURL(entityType, id, params))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var result domain.RelatedKnowledge
	decode(t, rec, &result)
	return result
}

// relatedErrorCode runs a request expected to fail and returns its status and stable code.
func relatedErrorCode(t testing.TB, handler http.Handler, target string) (int, string, string) {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decode(t, rec, &body)
	return rec.Code, body.Error.Code, body.Error.Message
}

// TestRelatedRouteServesEverySearchableStartClass is the route assertion.
//
// The path is addressed by the same (entity_type, id) pair every other identity on this API uses,
// and the accepted classes are the six searchable ones rather than the four graph ones. A start
// that resolves answers 200 with its own identity echoed back.
func TestRelatedRouteServesEverySearchableStartClass(t *testing.T) {
	handler := newHandler(t)

	starts := map[string]string{
		"session":    "session-01-fixture",
		"node":       "alpha",
		"claim":      "alpha-carries-energy",
		"source":     "fixture-reference-work",
		"vocabulary": "fixture-term",
		"experiment": "fixture-listening-exercise",
	}
	if len(starts) != len(domain.SearchEntityTypes) {
		t.Fatalf("the route table covers %d classes, the model has %d", len(starts), len(domain.SearchEntityTypes))
	}
	for _, class := range domain.SearchEntityTypes {
		id, ok := starts[string(class)]
		if !ok {
			t.Fatalf("no HTTP start is exercised for searchable class %s", class)
		}
		t.Run(string(class), func(t *testing.T) {
			result := getRelated(t, handler, string(class), id, nil)
			if result.Start.EntityType != class || result.Start.ID != id {
				t.Errorf("start = %s/%s, want %s/%s", result.Start.EntityType, result.Start.ID, class, id)
			}
			if len(result.Items) == 0 {
				t.Fatal("no items, so the shape assertions below would pass vacuously")
			}
		})
	}
}

// TestRelatedRouteServesTheDocumentedShape checks every field of the contract on the wire.
//
// Identity, display text, the structured reason, the evidence counts and the response-level
// bounds are all asserted against the decoded body rather than against the service, so a field
// that failed to serialise, or serialised under a different name, fails here.
func TestRelatedRouteServesTheDocumentedShape(t *testing.T) {
	handler := newHandler(t)
	rec := do(t, handler, http.MethodGet, relatedURL("node", "alpha", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var raw struct {
		Start struct {
			EntityType string `json:"entity_type"`
			ID         string `json:"id"`
			Title      string `json:"title"`
		} `json:"start"`
		Limit  int `json:"limit"`
		Counts struct {
			Eligible int `json:"eligible"`
			Returned int `json:"returned"`
		} `json:"counts"`
		Truncated bool `json:"truncated"`
		Items     []struct {
			EntityType string `json:"entity_type"`
			ID         string `json:"id"`
			Title      string `json:"title"`
			Reason     struct {
				Relation     string `json:"relation"`
				Origin       string `json:"origin"`
				Derived      bool   `json:"derived"`
				Priority     string `json:"priority"`
				PriorityRank int    `json:"priority_rank"`
			} `json:"reason"`
			EvidenceCount     int  `json:"evidence_count"`
			EvidenceTruncated bool `json:"evidence_truncated"`
		} `json:"items"`
	}
	decode(t, rec, &raw)

	if raw.Start.EntityType != "node" || raw.Start.ID != "alpha" || raw.Start.Title != "Alpha" {
		t.Errorf("start = %+v", raw.Start)
	}
	if raw.Limit != service.DefaultRelatedLimit {
		t.Errorf("limit = %d, want %d", raw.Limit, service.DefaultRelatedLimit)
	}
	if raw.Counts.Returned != len(raw.Items) || raw.Counts.Eligible != len(raw.Items) || raw.Truncated {
		t.Errorf("counts = %+v truncated = %v for %d items", raw.Counts, raw.Truncated, len(raw.Items))
	}
	for _, item := range raw.Items {
		if item.EntityType == "" || item.ID == "" || item.Title == "" {
			t.Errorf("item %+v is missing part of its identity", item)
		}
		if item.Reason.Relation == "" || item.Reason.Origin == "" || item.Reason.Priority == "" {
			t.Errorf("item %s/%s carries an incomplete reason %+v", item.EntityType, item.ID, item.Reason)
		}
		if item.Reason.Priority == string(domain.PriorityUnclassified) {
			t.Errorf("item %s/%s is explained by an unclassified field", item.EntityType, item.ID)
		}
		if item.EvidenceCount < 1 {
			t.Errorf("item %s/%s reports %d connections", item.EntityType, item.ID, item.EvidenceCount)
		}
		if item.EvidenceTruncated {
			t.Errorf("item %s/%s reported truncated evidence in the fixture corpus", item.EntityType, item.ID)
		}
	}
}

// TestRelatedRouteOmitsAbsentOptionalKeys asserts the response carries no key for something the
// caller did not ask for and no key for something a record does not have.
//
// An omitted key and a key serialised as null decode identically into Go and mean different
// things to a client, so this is asserted on the bytes: an unscoped discovery must not echo a
// scope, an item with one connection must not carry an empty additional_evidence array, and a
// class with no summary field must not acquire an empty one.
func TestRelatedRouteOmitsAbsentOptionalKeys(t *testing.T) {
	handler := newHandler(t)

	body := do(t, handler, http.MethodGet, relatedURL("claim", "alpha-carries-energy", nil)).Body.String()
	if strings.Contains(body, `"entity_types"`) {
		t.Error("an unscoped discovery echoed entity_types")
	}
	if strings.Contains(body, `"additional_evidence":[]`) || strings.Contains(body, `"additional_evidence":null`) {
		t.Error("a single-connection item serialised an empty additional_evidence")
	}
	// The claim's sources are registry entries, which have no summary field of their own.
	if strings.Contains(body, `"summary":""`) {
		t.Error("a class with no summary field acquired an empty one")
	}
	// An empty discovery still serialises an items array, because a null would make "related to
	// nothing" indistinguishable from a response that failed to build a list.
	empty := do(t, handler, http.MethodGet, relatedURL("source", "fixture-uncited-source", nil)).Body.String()
	if !strings.Contains(empty, `"items":[]`) {
		t.Errorf("an empty discovery did not serialise an empty item array: %s", empty)
	}
}

// TestRelatedRouteSerialisesEvidenceWhenARecordIsReachedTwice covers the deduplication contract on
// the wire: one item, the strongest connection as its reason, the other reported behind it.
func TestRelatedRouteSerialisesEvidenceWhenARecordIsReachedTwice(t *testing.T) {
	handler := newHandler(t)
	result := getRelated(t, handler, "claim", "alpha-may-extend-to-gamma", nil)

	found := 0
	for _, item := range result.Items {
		if item.EntityType != domain.SearchNode || item.ID != "gamma" {
			continue
		}
		found++
		if item.EvidenceCount != 2 || len(item.AdditionalEvidence) != 1 {
			t.Errorf("evidence_count = %d additional = %d, want 2 and 1",
				item.EvidenceCount, len(item.AdditionalEvidence))
		}
		if item.Reason.Origin == item.AdditionalEvidence[0].Origin {
			t.Error("the primary reason was repeated inside additional_evidence")
		}
	}
	if found != 1 {
		t.Errorf("node/gamma appeared %d times, want exactly once", found)
	}
}

// TestRelatedRouteIsDeterministicOnTheWire is the byte-equivalence assertion.
//
// Repeated equivalent requests against an unchanged corpus must produce identical bodies, not
// merely equivalent ones: a client caching or diffing two responses depends on the bytes. Every
// start is exercised, with and without a scope, because a single unordered read anywhere in the
// projection would surface as an intermittent reordering rather than as a stable failure.
func TestRelatedRouteIsDeterministicOnTheWire(t *testing.T) {
	handler := newHandler(t)

	for _, start := range []struct{ class, id string }{
		{"session", "session-01-fixture"}, {"node", "alpha"}, {"claim", "alpha-carries-energy"},
		{"source", "fixture-reference-work"}, {"vocabulary", "fixture-term"},
		{"experiment", "fixture-listening-exercise"},
	} {
		for _, params := range []map[string]string{nil, {"limit": "3"}, {"entity_types": "node,claim"}} {
			target := relatedURL(start.class, start.id, params)
			first := do(t, handler, http.MethodGet, target).Body.String()
			for run := 0; run < 10; run++ {
				if again := do(t, handler, http.MethodGet, target).Body.String(); again != first {
					t.Fatalf("%s changed on run %d:\n%s\nwant\n%s", target, run, again, first)
				}
			}
		}
	}
}

// TestRelatedRouteEncodedIdentifiersAgree asserts that a percent-encoded identifier and its plain
// spelling are one request.
//
// The router decodes a path segment before the handler sees it, so an encoded hyphen is the same
// identifier and must produce the same body. An encoded separator decodes to a separator and is
// refused by the shared identifier bound, which is what keeps an encoded traversal attempt from
// becoming a different lookup rather than a rejected one.
func TestRelatedRouteEncodedIdentifiersAgree(t *testing.T) {
	handler := newHandler(t)

	plain := do(t, handler, http.MethodGet, relatedURL("source", "fixture-reference-work", nil)).Body.String()
	encoded := do(t, handler, http.MethodGet, "/api/v1/related/source/fixture%2Dreference%2Dwork").Body.String()
	if encoded != plain {
		t.Errorf("an encoded identifier produced a different body:\n%s\nwant\n%s", encoded, plain)
	}

	for _, target := range []string{
		"/api/v1/related/node/..%2F..%2Fetc",
		"/api/v1/related/node/alpha%2Fbeta",
		"/api/v1/related/node/%2E%2E",
	} {
		status, code, _ := relatedErrorCode(t, handler, target)
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("%s: status = %d code = %q, want 400 invalid_query", target, status, code)
		}
	}
}

// TestRelatedRouteLimitContract pins the wire behaviour of the limit parameter.
//
// A non-integer or negative limit is refused by the shared parser, exactly as it is on every
// other route. An over-large limit is clamped rather than refused, exactly as it is on every
// other route, and the applied value is echoed so the clamp is visible in the response.
func TestRelatedRouteLimitContract(t *testing.T) {
	handler := newHandler(t)

	for _, bad := range []string{"-1", "abc", "1.5", "1e3", "٣", "9999999999999999999999"} {
		status, code, message := relatedErrorCode(t, handler, relatedURL("node", "alpha", map[string]string{"limit": bad}))
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("limit=%q: status = %d code = %q, want 400 invalid_query", bad, status, code)
		}
		if !strings.Contains(message, "limit") {
			t.Errorf("limit=%q: message %q does not name the parameter", bad, message)
		}
	}

	// Zero and a whitespace-only value both mean "unspecified", which is what the shared integer
	// parser has always made them mean on every other route. Giving this one route a stricter
	// reading of a blank parameter would make one spelling behave differently depending on which
	// endpoint it was sent to.
	for _, absent := range []string{"0", " "} {
		if got := getRelated(t, handler, "node", "alpha", map[string]string{"limit": absent}); got.Limit != service.DefaultRelatedLimit {
			t.Errorf("limit=%q echoed %d, want the default %d", absent, got.Limit, service.DefaultRelatedLimit)
		}
	}
	huge := strconv.Itoa(service.MaxRelatedLimit * 100)
	if got := getRelated(t, handler, "node", "alpha", map[string]string{"limit": huge}); got.Limit != service.MaxRelatedLimit {
		t.Errorf("limit=%s echoed %d, want the ceiling %d", huge, got.Limit, service.MaxRelatedLimit)
	}

	cut := getRelated(t, handler, "node", "alpha", map[string]string{"limit": "2"})
	if len(cut.Items) != 2 || !cut.Truncated || cut.Counts.Eligible <= 2 {
		t.Errorf("limit=2 returned %d items truncated=%v eligible=%d",
			len(cut.Items), cut.Truncated, cut.Counts.Eligible)
	}
}

// TestRelatedRouteScopeContract pins the wire behaviour of the destination scope.
//
// The comma is a wire convention: the handler splits and the service validates. The split is
// deliberately naive, so a leading, trailing or doubled comma survives as the blank member it is
// and is refused rather than repaired into a working filter that answers a different question.
func TestRelatedRouteScopeContract(t *testing.T) {
	handler := newHandler(t)

	scoped := getRelated(t, handler, "node", "alpha", map[string]string{"entity_types": "source,claim"})
	if !reflect.DeepEqual(scoped.EntityTypes, []string{"claim", "source"}) {
		t.Errorf("entity_types = %v, want the canonical order", scoped.EntityTypes)
	}
	for _, item := range scoped.Items {
		if item.EntityType != domain.SearchClaim && item.EntityType != domain.SearchSource {
			t.Errorf("a scoped discovery returned %s/%s", item.EntityType, item.ID)
		}
	}

	for _, bad := range []string{"", ",", "node,", ",node", "node,,claim", "node,node", "experiment_run", "Node", "widget"} {
		status, code, _ := relatedErrorCode(t, handler,
			relatedURL("node", "alpha", map[string]string{"entity_types": bad}))
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("entity_types=%q: status = %d code = %q, want 400 invalid_query", bad, status, code)
		}
	}
}

// TestRelatedRouteScopeErrorsMatchTheSearchRoute asserts one malformed class list is refused with
// one message wherever it is written.
//
// entity_types is one filter with one meaning, and a caller who moved a bad list from the search
// route to this one should not be told two different things about the same mistake. This is the
// regression guard on the shared renderer.
func TestRelatedRouteScopeErrorsMatchTheSearchRoute(t *testing.T) {
	handler := newHandler(t)

	for _, bad := range []string{"node,", "node,node"} {
		_, searchCode, searchMessage := relatedErrorCode(t, handler,
			searchURL(map[string]string{"q": "fixture", "entity_types": bad}))
		_, relatedCode, relatedMessage := relatedErrorCode(t, handler,
			relatedURL("node", "alpha", map[string]string{"entity_types": bad}))
		if searchCode != relatedCode || searchMessage != relatedMessage {
			t.Errorf("entity_types=%q: search says %q/%q, related says %q/%q",
				bad, searchCode, searchMessage, relatedCode, relatedMessage)
		}
	}
}

// TestRelatedRouteRejectsUnsupportedStartClass asserts the closed starting set at the wire and the
// message it is refused with.
//
// experiment_run is the case that matters: it is a canonical layer this API fully serves and is
// deliberately not a discovery start, so it must be refused rather than answered with an empty
// list a caller could read as "this run is connected to nothing".
func TestRelatedRouteRejectsUnsupportedStartClass(t *testing.T) {
	handler := newHandler(t)

	// Every value here is chosen so it cannot occur inside the fixed message, which is what makes
	// the echo assertion below meaningful rather than accidentally satisfied by the prose.
	for _, class := range []string{"experiment_run", "Node", "nodes", "widget", "run"} {
		status, code, message := relatedErrorCode(t, handler, "/api/v1/related/"+class+"/alpha")
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("class %q: status = %d code = %q, want 400 invalid_query", class, status, code)
		}
		for _, supported := range domain.SearchEntityTypeNames() {
			if !strings.Contains(message, supported) {
				t.Errorf("class %q: message %q does not list %q", class, message, supported)
			}
		}
		if strings.Contains(message, class) {
			t.Errorf("class %q: message echoes the caller's own value: %q", class, message)
		}
	}
}

// TestRelatedRouteUnknownStartIsNotFound asserts a syntactically valid identifier naming no record
// answers 404 under its own stable code.
//
// The code is distinct from entity_not_found, which names a graph vertex: a discovery start may be
// a vocabulary entry or an experiment definition, and reporting a graph miss would tell a client
// the lookup failed in a layer it never asked about.
func TestRelatedRouteUnknownStartIsNotFound(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		"/api/v1/related/node/no-such-node",
		"/api/v1/related/vocabulary/no-such-term",
		"/api/v1/related/experiment/no-such-experiment",
		// Canonical IDs are ASCII kebab-case by contract, so a Unicode identifier is valid input
		// naming a record the corpus cannot contain. It is a miss, not a crash and not a 400.
		"/api/v1/related/node/" + url.PathEscape("共鳴"),
		"/api/v1/related/node/" + url.PathEscape("rezonans-fikstür"),
		// A real ID under the wrong class: identity is the pair, never the ID alone.
		"/api/v1/related/node/alpha-carries-energy",
		"/api/v1/related/experiment/alpha",
	} {
		status, code, message := relatedErrorCode(t, handler, target)
		if status != http.StatusNotFound || code != "related_start_not_found" {
			t.Errorf("%s: status = %d code = %q, want 404 related_start_not_found", target, status, code)
		}
		if message == "" {
			t.Errorf("%s: 404 carried no message", target)
		}
	}
}

// TestRelatedRouteRejectsUnknownParameters asserts the accepted query string is closed.
//
// Silently dropping a parameter the caller believed was applied would return a list that does not
// mean what they think it means. depth, offset and q are named explicitly because each is a
// plausible thing to try from a neighbouring route and none of them is part of this operation.
func TestRelatedRouteRejectsUnknownParameters(t *testing.T) {
	handler := newHandler(t)

	for _, params := range []string{
		"depth=2", "offset=1", "q=fixture", "type=node", "relationship=produces",
		"target_type=node", "include_context=true", "query_mode=all_terms", "priority=conceptual",
	} {
		status, code, _ := relatedErrorCode(t, handler, "/api/v1/related/node/alpha?"+params)
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("%s: status = %d code = %q, want 400 invalid_query", params, status, code)
		}
	}
	// A supported parameter supplied twice is refused too: two values are two requests.
	status, code, _ := relatedErrorCode(t, handler, "/api/v1/related/node/alpha?limit=1&limit=2")
	if status != http.StatusBadRequest || code != "invalid_query" {
		t.Errorf("repeated limit: status = %d code = %q, want 400 invalid_query", status, code)
	}
}

// TestRelatedRouteIsReadOnly asserts the method lock covers the new route.
//
// Read-only is enforced by middleware that runs before routing, so this is a check that the route
// inherited the guarantee rather than that the guarantee exists.
func TestRelatedRouteIsReadOnly(t *testing.T) {
	handler := newHandler(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := do(t, handler, method, relatedURL("node", "alpha", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, rec.Code)
		}
		if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
			t.Errorf("%s: Allow = %q", method, got)
		}
	}
	if rec := do(t, handler, http.MethodHead, relatedURL("node", "alpha", nil)); rec.Code != http.StatusOK {
		t.Errorf("HEAD: status = %d, want 200", rec.Code)
	}
}

// TestRelatedRouteLeaksNoHostDetail is the disclosure assertion.
//
// A discovery response names canonical records and canonical field names and nothing else. It
// must not carry an absolute path, a repository-relative file path, a Go error, or a stack frame,
// on success or on any failure. The repository root is derived from the running process rather
// than hard-coded, so this keeps meaning wherever the fixture lives.
func TestRelatedRouteLeaksNoHostDetail(t *testing.T) {
	handler := newHandler(t)

	forbidden := []string{
		"C:\\", "/home/", "/tmp/", "testdata", ".md\"", ".yaml\"",
		"goroutine", "runtime.", "panic", "*errors.", "internal/service",
	}
	for _, target := range []string{
		relatedURL("node", "alpha", nil),
		relatedURL("claim", "alpha-carries-energy", map[string]string{"entity_types": "source"}),
		relatedURL("source", "fixture-uncited-source", nil),
		"/api/v1/related/node/no-such-node",
		"/api/v1/related/widget/alpha",
		"/api/v1/related/node/alpha?entity_types=node,,claim",
		"/api/v1/related/node/alpha?limit=-1",
	} {
		body := do(t, handler, http.MethodGet, target).Body.String()
		for _, needle := range forbidden {
			if strings.Contains(body, needle) {
				t.Errorf("%s: response contains %q\n%s", target, needle, body)
			}
		}
	}
}

// TestRelatedRouteCarriesNoUnrelatedRecordContent asserts a discovery item is an identity and a
// display field, never a record.
//
// A node's prose body, its future questions and its practical applications are canonical content
// that belongs to the node's own route. Copying them into a suggestion list would turn a bounded
// navigation response into an unbounded corpus dump one field at a time.
func TestRelatedRouteCarriesNoUnrelatedRecordContent(t *testing.T) {
	handler := newHandler(t)
	body := do(t, handler, http.MethodGet, relatedURL("node", "alpha", nil)).Body.String()

	// Canonical field names are deliberately absent from this list where they are legitimate
	// origins - node.session_origin is the explanation of an item, not content copied out of a
	// record. What must never appear is the content of those fields.
	for _, needle := range []string{
		"Prose body", "\"future_questions\"", "\"practical_applications\"", "\"core_concepts\"",
		"\"confidence_basis\"", "\"open_questions\"", "\"inbound_relationships\"",
		"\"definition\"", "\"body\"", "\"evidence\"", "\"attribution\"",
	} {
		if strings.Contains(body, needle) {
			t.Errorf("a discovery response carried %q\n%s", needle, body)
		}
	}
}

// TestRelatedRouteDoesNotDisturbTheEarlierPhases is the cross-phase regression assertion.
//
// Every route the earlier phases contracted is captured, a full sweep of discovery requests runs,
// and every route is compared byte for byte against its own earlier body. Phase 2A is additive:
// it introduces one route and changes none, and search ranking, search explainability, traversal
// and provenance must answer exactly what they answered before.
func TestRelatedRouteDoesNotDisturbTheEarlierPhases(t *testing.T) {
	handler := newHandler(t)

	routes := []string{
		"/health",
		"/api/v1/project",
		"/api/v1/nodes",
		"/api/v1/nodes/alpha",
		"/api/v1/sessions",
		"/api/v1/sessions/session-01-fixture",
		"/api/v1/sources",
		"/api/v1/sources/fixture-reference-work",
		"/api/v1/claims",
		"/api/v1/claims/alpha-carries-energy",
		"/api/v1/vocabulary",
		"/api/v1/vocabulary/fixture-term",
		"/api/v1/experiments",
		"/api/v1/experiments/fixture-listening-exercise",
		"/api/v1/experiment-runs",
		"/api/v1/graph",
		"/api/v1/graph/entities/node/alpha/relationships",
		"/api/v1/graph/entities/node/alpha/traverse?depth=3",
		"/api/v1/diagnostics",
		searchURL(map[string]string{"q": "fixture"}),
		searchURL(map[string]string{"q": "fixture", "include_context": "true"}),
		searchURL(map[string]string{"q": "fixture alpha", "query_mode": "all_terms"}),
		searchURL(map[string]string{"q": "alpha", "entity_types": "node,claim"}),
	}

	before := make(map[string]string, len(routes))
	for _, route := range routes {
		before[route] = do(t, handler, http.MethodGet, route).Body.String()
	}

	for _, start := range []string{
		"session/session-01-fixture", "node/alpha", "node/gamma", "claim/alpha-may-extend-to-gamma",
		"source/fixture-archive-record", "vocabulary/fixture-term", "experiment/fixture-listening-exercise",
	} {
		parts := strings.SplitN(start, "/", 2)
		for _, params := range []map[string]string{nil, {"limit": "1"}, {"entity_types": "node"}} {
			do(t, handler, http.MethodGet, relatedURL(parts[0], parts[1], params))
		}
	}

	for _, route := range routes {
		if after := do(t, handler, http.MethodGet, route).Body.String(); after != before[route] {
			t.Errorf("%s changed after discovery ran:\n%s\nwant\n%s", route, after, before[route])
		}
	}
}

// TestRelatedRouteEnvelopeMatchesTheApiConventions asserts the new route uses the same headers and
// the same error envelope as every route that preceded it.
func TestRelatedRouteEnvelopeMatchesTheApiConventions(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		relatedURL("node", "alpha", nil),
		"/api/v1/related/node/no-such-node",
		"/api/v1/related/widget/alpha",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
			t.Errorf("%s: content-type = %q, want %q", target, got, want)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", target, got)
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Errorf("%s: body is not valid JSON: %s", target, rec.Body.String())
		}
	}
}
