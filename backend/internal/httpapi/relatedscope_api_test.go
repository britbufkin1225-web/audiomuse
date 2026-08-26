package httpapi_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 2B relationship-scope and explainability tests at the HTTP boundary.
//
// Filtering, ranking and explanation content are tested in the service and the domain, where the
// exact canonical relations and the exact sentences are asserted. What matters here is the wire
// contract the parameter added: its name and spelling, that it is closed and refused rather than
// repaired, that a refusal never falls back to an unfiltered discovery, that the applied scope and
// the per-result explanation are visible in the serialised body, and that none of it disturbed the
// Phase 2A shape.

// relatedScopeParam is the Phase 2B parameter name, written once so a test that asserts the
// spelling and a test that uses it cannot drift apart.
const relatedScopeParam = "relationship_types"

// TestRelatedRouteAcceptsTheRelationshipScopeParameter asserts the accepted spelling.
//
// It is plural and comma-separated, matching entity_types, because that is this API's only
// multi-value query convention. The singular form, the repeated-parameter form and the
// neighbouring routes' spellings are all refused, so a caller cannot believe a filter was applied
// that was not.
func TestRelatedRouteAcceptsTheRelationshipScopeParameter(t *testing.T) {
	handler := newHandler(t)

	result := getRelated(t, handler, "node", "alpha", map[string]string{relatedScopeParam: "conceptual"})
	if !reflect.DeepEqual(result.RelationshipTypes, []string{"conceptual"}) {
		t.Errorf("relationship_types = %v, want [conceptual]", result.RelationshipTypes)
	}

	// The singular is not an alias. /api/v1/search carries both spellings of its class filter only
	// because the singular predates the list; a new filter has no such history, so accepting one
	// would give this route two ways to say one thing from the day it shipped.
	for _, rejected := range []string{
		"relationship_type=conceptual", "priority=conceptual", "priorities=conceptual",
		"relationship=conceptual", "relation=conceptual", "origin=node.relationships",
	} {
		status, code, _ := relatedErrorCode(t, handler, "/api/v1/related/node/alpha?"+rejected)
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("%s: status = %d code = %q, want 400 invalid_query", rejected, status, code)
		}
	}

	// Supplied twice is two requests, and is refused by the same guard that refuses a repeated
	// limit. This is what makes the comma the only multi-value spelling on this route.
	status, code, _ := relatedErrorCode(t, handler,
		"/api/v1/related/node/alpha?relationship_types=conceptual&relationship_types=evidential")
	if status != http.StatusBadRequest || code != "invalid_query" {
		t.Errorf("repeated relationship_types: status = %d code = %q, want 400 invalid_query", status, code)
	}
}

// TestRelatedRouteServesEveryDeclaredRelationshipClass walks the closed allowlist over HTTP.
//
// Every class the model declares must be a scope the route accepts and echoes normalised, and no
// response under a one-class scope may carry a connection of any other class. A class added to the
// model without becoming reachable here fails.
func TestRelatedRouteServesEveryDeclaredRelationshipClass(t *testing.T) {
	handler := newHandler(t)

	for _, class := range domain.RelatedPriorityNames() {
		t.Run(class, func(t *testing.T) {
			result := getRelated(t, handler, "node", "alpha", map[string]string{relatedScopeParam: class})
			if !reflect.DeepEqual(result.RelationshipTypes, []string{class}) {
				t.Fatalf("relationship_types = %v, want [%s]", result.RelationshipTypes, class)
			}
			for _, item := range result.Items {
				for _, reason := range append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...) {
					if string(reason.Priority) != class {
						t.Errorf("%s/%s carries a %s connection under scope %s",
							item.EntityType, item.ID, reason.Priority, class)
					}
				}
			}
		})
	}
}

// TestRelatedRouteRelationshipScopeContract covers the accepted and refused spellings of the list
// itself, on the wire, where the comma is the caller's own separator rather than a Go slice.
//
// The refusals are the point: a leading, trailing or doubled comma produces a blank member and is
// refused rather than repaired, an unknown or mis-cased class is refused rather than folded, and a
// present-but-empty parameter is refused rather than read as "no filter".
func TestRelatedRouteRelationshipScopeContract(t *testing.T) {
	handler := newHandler(t)

	scoped := getRelated(t, handler, "node", "alpha",
		map[string]string{relatedScopeParam: "referential,conceptual"})
	if !reflect.DeepEqual(scoped.RelationshipTypes, []string{"conceptual", "referential"}) {
		t.Errorf("relationship_types = %v, want precedence order", scoped.RelationshipTypes)
	}
	// Whitespace around a member is trimmed, which is the entity_types contract, and the response
	// is identical to the tight spelling: a caller's spacing is not part of their question.
	spaced := getRelated(t, handler, "node", "alpha",
		map[string]string{relatedScopeParam: " conceptual , referential "})
	if !reflect.DeepEqual(spaced, scoped) {
		t.Error("spacing inside the class list changed the response")
	}

	for _, bad := range []string{
		"", " ", ",", "conceptual,", ",conceptual", "conceptual,,referential",
		"conceptual,conceptual", "conceptual, conceptual", "unclassified", "Conceptual",
		"CONCEPTUAL", "widget", "conceptual,widget", "widget,conceptual",
		"node.relationships", "produces", "conceptual;referential", "conceptual referential",
	} {
		status, code, message := relatedErrorCode(t, handler,
			relatedURL("node", "alpha", map[string]string{relatedScopeParam: bad}))
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("%s=%q: status = %d code = %q, want 400 invalid_query",
				relatedScopeParam, bad, status, code)
		}
		if !strings.Contains(message, relatedScopeParam) {
			t.Errorf("%s=%q: message %q does not name the parameter", relatedScopeParam, bad, message)
		}
		// The caller's own value is never echoed: it is the one part of a response an attacker
		// controls. The invented values below cannot occur inside any fixed message.
		for _, secret := range []string{"widget", "node.relationships", "CONCEPTUAL"} {
			if bad == secret && strings.Contains(message, secret) {
				t.Errorf("%s=%q: message echoed the rejected value: %q", relatedScopeParam, bad, message)
			}
		}
	}
}

// TestRelatedRouteRelationshipScopeErrorsNameTheirOwnParameter asserts a caller who wrote one comma
// too many is told which of the two lists on this route was wrong.
//
// The two filters take the same kinds of mistake, so the messages parallel each other; what must
// not happen is a relationship mistake reported as an entity_types mistake, which would send a
// caller to fix a list that was already correct.
func TestRelatedRouteRelationshipScopeErrorsNameTheirOwnParameter(t *testing.T) {
	handler := newHandler(t)

	for _, bad := range []string{"conceptual,", "conceptual,conceptual"} {
		_, _, message := relatedErrorCode(t, handler,
			relatedURL("node", "alpha", map[string]string{relatedScopeParam: bad}))
		if strings.Contains(message, "entity_types") {
			t.Errorf("%s=%q was reported against entity_types: %q", relatedScopeParam, bad, message)
		}
		if !strings.Contains(message, relatedScopeParam) {
			t.Errorf("%s=%q: message %q does not name the parameter", relatedScopeParam, bad, message)
		}
	}

	// A malformed destination scope is still reported against entity_types even when a
	// well-formed relationship scope accompanies it, so adding the second list did not capture
	// the first list's errors.
	_, _, message := relatedErrorCode(t, handler, relatedURL("node", "alpha", map[string]string{
		"entity_types":    "node,node",
		relatedScopeParam: "conceptual",
	}))
	if !strings.Contains(message, "entity_types") || strings.Contains(message, relatedScopeParam) {
		t.Errorf("a malformed entity_types list was reported as %q", message)
	}
}

// TestRelatedRouteRefusedScopeCarriesNoResult is the no-silent-fallback assertion on the wire.
//
// A refused filter must produce the error envelope and nothing else. A body carrying both an error
// and a discovery would let a client that reads the payload first treat an unfiltered result as a
// filtered one.
func TestRelatedRouteRefusedScopeCarriesNoResult(t *testing.T) {
	handler := newHandler(t)

	for _, bad := range []string{"", "widget", "Conceptual", "conceptual,conceptual", "conceptual,"} {
		rec := do(t, handler, http.MethodGet,
			relatedURL("node", "alpha", map[string]string{relatedScopeParam: bad}))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s=%q: status = %d, want 400", relatedScopeParam, bad, rec.Code)
			continue
		}
		body := rec.Body.String()
		for _, key := range []string{`"items"`, `"start"`, `"counts"`, `"bounds"`, `"limit"`} {
			if strings.Contains(body, key) {
				t.Errorf("%s=%q: the error body carries %s: %s", relatedScopeParam, bad, key, body)
			}
		}
	}
}

// TestRelatedRouteScopedEmptyResultIsSuccess asserts a valid filter matching nothing is a 200 with
// an empty list rather than a 404 or a 400.
//
// The distinction is the whole reason the empty case is tested at the boundary: a status code is
// what a client branches on, and answering "this record has no connection of that kind" with an
// error would make a true statement about the corpus look like a broken request.
func TestRelatedRouteScopedEmptyResultIsSuccess(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, relatedURL("source", "fixture-attribution-source",
		map[string]string{relatedScopeParam: "conceptual"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"items":[]`) {
		t.Errorf("an empty scoped discovery did not serialise an empty item array: %s", body)
	}
	if !strings.Contains(body, `"relationship_types":["conceptual"]`) {
		t.Errorf("an empty scoped discovery did not echo the scope that produced it: %s", body)
	}
	if !strings.Contains(body, `"eligible":0`) {
		t.Errorf("an empty scoped discovery did not report a zero eligible count: %s", body)
	}
}

// TestRelatedRouteSerialisesTheAppliedScopeAndExplanation is the additive-shape assertion.
//
// The two things Phase 2B adds must appear in the serialised body under the documented keys: the
// applied relationship scope at the top level, next to the destination scope and spelled the same
// way, and an explanation on every reason. Both are decoded from the raw JSON rather than through
// the domain type, so a renamed tag fails here rather than passing silently.
func TestRelatedRouteSerialisesTheAppliedScopeAndExplanation(t *testing.T) {
	handler := newHandler(t)
	rec := do(t, handler, http.MethodGet, relatedURL("node", "gamma",
		map[string]string{relatedScopeParam: "conceptual,assertional"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var raw struct {
		RelationshipTypes []string `json:"relationship_types"`
		Items             []struct {
			EntityType string `json:"entity_type"`
			ID         string `json:"id"`
			Reason     struct {
				Priority     string `json:"priority"`
				PriorityRank int    `json:"priority_rank"`
				Origin       string `json:"origin"`
				Explanation  string `json:"explanation"`
			} `json:"reason"`
			AdditionalEvidence []struct {
				Priority    string `json:"priority"`
				Explanation string `json:"explanation"`
			} `json:"additional_evidence"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !reflect.DeepEqual(raw.RelationshipTypes, []string{"conceptual", "assertional"}) {
		t.Errorf("relationship_types = %v, want the scope in precedence order", raw.RelationshipTypes)
	}
	if len(raw.Items) == 0 {
		t.Fatal("the scoped request returned nothing, so the explanation assertions are vacuous")
	}
	for _, item := range raw.Items {
		want := domain.RelatedPriorityExplanation(domain.RelatedPriority(item.Reason.Priority))
		if item.Reason.Explanation == "" {
			t.Errorf("%s/%s carries no explanation", item.EntityType, item.ID)
		}
		if item.Reason.Explanation != want {
			t.Errorf("%s/%s: explanation = %q, want %q for class %s",
				item.EntityType, item.ID, item.Reason.Explanation, want, item.Reason.Priority)
		}
		if item.Reason.PriorityRank != domain.RelatedPriorityRank(domain.RelatedPriority(item.Reason.Priority)) {
			t.Errorf("%s/%s: priority_rank %d disagrees with class %s",
				item.EntityType, item.ID, item.Reason.PriorityRank, item.Reason.Priority)
		}
		// Further evidence explains itself too, so a client rendering an item's full connection
		// list never has to leave one line unexplained.
		for _, extra := range item.AdditionalEvidence {
			if extra.Explanation != domain.RelatedPriorityExplanation(domain.RelatedPriority(extra.Priority)) {
				t.Errorf("%s/%s: additional evidence %s explained as %q",
					item.EntityType, item.ID, extra.Priority, extra.Explanation)
			}
		}
	}
}

// TestRelatedRouteOmitsTheScopeEchoWhenUnfiltered asserts absent means unrestricted on the bytes.
//
// An omitted key and a key serialised as an empty array decode identically into some clients and
// mean different things: one says "every class", the other says "these classes". The distinction is
// the documented convention for entity_types and must hold for the new list the same way.
func TestRelatedRouteOmitsTheScopeEchoWhenUnfiltered(t *testing.T) {
	handler := newHandler(t)

	plain := do(t, handler, http.MethodGet, relatedURL("node", "alpha", nil)).Body.String()
	if strings.Contains(plain, `"relationship_types"`) {
		t.Error("an unfiltered discovery echoed relationship_types")
	}
	// A destination scope alone must not bring the relationship echo with it, and the reverse.
	byClass := do(t, handler, http.MethodGet,
		relatedURL("node", "alpha", map[string]string{"entity_types": "node"})).Body.String()
	if strings.Contains(byClass, `"relationship_types"`) {
		t.Error("a destination-scoped discovery echoed relationship_types")
	}
	byRelation := do(t, handler, http.MethodGet,
		relatedURL("node", "alpha", map[string]string{relatedScopeParam: "conceptual"})).Body.String()
	if strings.Contains(byRelation, `"entity_types"`) {
		t.Error("a relationship-scoped discovery echoed entity_types")
	}
	if !strings.Contains(byRelation, `"relationship_types":["conceptual"]`) {
		t.Errorf("a relationship-scoped discovery did not echo its scope: %s", byRelation)
	}
}

// TestRelatedRouteScopedResponsesAreDeterministicOnTheWire is the byte-equivalence assertion for
// the filtered path.
//
// Repeated equivalent requests must produce identical bodies, and so must two spellings of one
// scope: a client caching or diffing responses depends on the bytes, and a normalised filter that
// only normalised the echo would fail here while looking correct in a decoded response.
func TestRelatedRouteScopedResponsesAreDeterministicOnTheWire(t *testing.T) {
	handler := newHandler(t)

	for _, spelling := range [][2]string{
		{"conceptual,referential", "referential,conceptual"},
		{"assertional,contextual", " contextual , assertional "},
		{"evidential", "evidential"},
	} {
		first := do(t, handler, http.MethodGet,
			relatedURL("node", "alpha", map[string]string{relatedScopeParam: spelling[0]})).Body.String()
		for run := 0; run < 5; run++ {
			again := do(t, handler, http.MethodGet,
				relatedURL("node", "alpha", map[string]string{relatedScopeParam: spelling[0]})).Body.String()
			if again != first {
				t.Fatalf("scope %q produced two different bodies", spelling[0])
			}
		}
		other := do(t, handler, http.MethodGet,
			relatedURL("node", "alpha", map[string]string{relatedScopeParam: spelling[1]})).Body.String()
		if other != first {
			t.Errorf("scopes %q and %q produced different bodies", spelling[0], spelling[1])
		}
	}
}

// TestRelatedRouteScopedResponsesReportTheAppliedBounds asserts every bound stays visible under a
// filter, and that a filter cannot move one.
//
// A bounded answer that does not say what bounded it cannot be told apart from a complete one, and
// that is exactly as true of a filtered answer as of an unfiltered one. The scan count is asserted
// equal to the unfiltered request's, because it describes the work done rather than the answer
// produced: a filter that changed it would be buying or saving work in the caller's name.
func TestRelatedRouteScopedResponsesReportTheAppliedBounds(t *testing.T) {
	handler := newHandler(t)
	base := getRelated(t, handler, "node", "alpha", nil)

	for _, class := range domain.RelatedPriorityNames() {
		result := getRelated(t, handler, "node", "alpha", map[string]string{relatedScopeParam: class})
		if result.Bounds != base.Bounds {
			t.Errorf("scope %s: bounds = %+v, want the unfiltered %+v", class, result.Bounds, base.Bounds)
		}
		if result.Bounds.MaxEvidencePerItem != service.MaxRelatedEvidencePerItem ||
			result.Bounds.MaxRelationsScanned != service.MaxRelatedRelationsScanned {
			t.Errorf("scope %s: applied bounds = %+v, want the declared constants", class, result.Bounds)
		}
		if result.Limit != service.DefaultRelatedLimit {
			t.Errorf("scope %s: limit = %d, want the default %d", class, result.Limit, service.DefaultRelatedLimit)
		}
	}

	// The limit contract is unchanged under a filter: an over-large value is clamped and echoed
	// rather than refused, so the applied bound is always the one the response states.
	clamped := getRelated(t, handler, "node", "alpha", map[string]string{
		relatedScopeParam: "conceptual",
		"limit":           strconv.Itoa(service.MaxRelatedLimit + 1),
	})
	if clamped.Limit != service.MaxRelatedLimit {
		t.Errorf("limit = %d, want the ceiling %d", clamped.Limit, service.MaxRelatedLimit)
	}
	// An invalid limit is still refused alongside a valid filter, so one well-formed parameter
	// cannot rescue another that is not.
	status, code, _ := relatedErrorCode(t, handler, relatedURL("node", "alpha", map[string]string{
		relatedScopeParam: "conceptual",
		"limit":           "-1",
	}))
	if status != http.StatusBadRequest || code != "invalid_query" {
		t.Errorf("negative limit with a valid filter: status = %d code = %q", status, code)
	}
}

// TestRelatedRouteScopeDoesNotDisturbTheUnfilteredContract is the Phase 2A regression guard at the
// boundary.
//
// Every Phase 2A request must still answer exactly as it did apart from the additive explanation:
// same items, same order, same reasons, same counts and same bounds. The comparison is made against
// a request naming every declared class, which is the strongest form of the claim — admitting
// everything must be indistinguishable from filtering nothing.
func TestRelatedRouteScopeDoesNotDisturbTheUnfilteredContract(t *testing.T) {
	handler := newHandler(t)
	every := strings.Join(domain.RelatedPriorityNames(), ",")

	for _, start := range [][2]string{
		{"session", "session-01-fixture"}, {"node", "alpha"}, {"claim", "alpha-carries-energy"},
		{"source", "fixture-reference-work"}, {"vocabulary", "fixture-term"},
		{"experiment", "fixture-listening-exercise"},
	} {
		plain := getRelated(t, handler, start[0], start[1], nil)
		scoped := getRelated(t, handler, start[0], start[1], map[string]string{relatedScopeParam: every})

		if !reflect.DeepEqual(scoped.Items, plain.Items) {
			t.Errorf("%s/%s: naming every class changed the items", start[0], start[1])
		}
		if scoped.Counts != plain.Counts || scoped.Bounds != plain.Bounds || scoped.Limit != plain.Limit {
			t.Errorf("%s/%s: naming every class changed the counts or bounds", start[0], start[1])
		}
		if plain.RelationshipTypes != nil {
			t.Errorf("%s/%s: an unfiltered request echoed a scope", start[0], start[1])
		}
	}
}

// TestRelatedRouteScopeLeaksNoHostDetail asserts the new parameter did not open a disclosure path.
//
// A rejected value is never echoed and no message names a file, a package or a local path, which
// is the same guarantee every other refused filter on this API carries. The hostile values here
// are chosen to be things a naive echo would reproduce verbatim.
func TestRelatedRouteScopeLeaksNoHostDetail(t *testing.T) {
	handler := newHandler(t)

	for _, hostile := range []string{
		"../../etc/passwd", "C:\\Windows\\System32", "<script>alert(1)</script>",
		"conceptual\x00evidential", strings.Repeat("a", 200),
	} {
		_, _, message := relatedErrorCode(t, handler,
			relatedURL("node", "alpha", map[string]string{relatedScopeParam: hostile}))
		for _, fragment := range []string{"passwd", "System32", "<script", "\x00", strings.Repeat("a", 40)} {
			if strings.Contains(message, fragment) {
				t.Errorf("value %q was echoed into the message: %q", hostile, message)
			}
		}
		for _, leak := range []string{".go", "internal/", "goroutine", "C:\\Users"} {
			if strings.Contains(message, leak) {
				t.Errorf("message %q leaks %q", message, leak)
			}
		}
	}
}
