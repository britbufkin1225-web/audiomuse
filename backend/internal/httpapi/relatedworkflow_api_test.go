package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/httpapi"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// Phase 2C: the merged related-knowledge workflow, tested as one system rather than as two
// features.
//
// Phase 2A introduced discovery and Phase 2B introduced the relationship scope, and each pinned
// its own contract thoroughly. What neither could pin is the property that only exists once both
// are present and reached over HTTP: that route binding, query-string admission, entity
// resolution, destination filtering, relationship filtering, grouping, precedence ranking,
// evidence bounding, the scan ceiling, the item limit and serialisation compose into one
// deterministic, bounded, explainable request-to-response pipeline. A per-feature suite can be
// complete and still leave that unowned, because each of its tests is entitled to assume the
// other behaves.
//
// So the tests here are deliberately not more feature tests. They follow the Phase 1J pattern:
// each invariant is stated once and required of a shared matrix of requests that exercises the
// controls together.
//
//	counts.returned == len(items) <= limit <= MaxRelatedLimit   no request produces an unbounded response
//	truncated == (returned < eligible)                          a cut answer always says it was cut
//	len(additional_evidence) == min(evidence_count, cap) - 1    the evidence bound is exact, not approximate
//	explanation == the fixed sentence for its own priority      the prose cannot drift from the class
//	the ordering is total and matches the documented four keys  no pair of items compares equal
//	relations_scanned is the same under every filter spelling   no filter can buy a larger scan
//	byte-identical repeats, concurrently and across indexes     nothing read a map, a clock or a directory order
//
// Nothing here is a new discovery feature, a new endpoint or a new fixture. Every request below is
// one a client could already send, and every record it reads is the fixture corpus the earlier
// phases already ship.

// --------------------------------------------------------------------------------------------
// The shared matrix
// --------------------------------------------------------------------------------------------

// relatedWorkflowCase is one request in the shared matrix, named so a failure reports which
// combination of controls broke rather than only which URL.
type relatedWorkflowCase struct {
	name   string
	start  relatedWorkflowStart
	target string
}

type relatedWorkflowStart struct {
	class string
	id    string
}

func (s relatedWorkflowStart) path() string {
	return httpapi.APIBase + "/related/" + s.class + "/" + s.id
}

// relatedWorkflowStarts is every discovery start the fixture corpus resolves, one line per record
// rather than one per class.
//
// The list is deliberately wider than "one of each class": it includes the records that are
// connected to nothing (an uncited source, an orphan vocabulary term, an unused session registry
// entry), because an empty discovery is an answer this contract has to keep making correctly and
// is exactly the case a matrix of well-connected records would never reach.
var relatedWorkflowStarts = []relatedWorkflowStart{
	{"session", "session-01-fixture"},
	{"session", "session-02-unused"},
	{"node", "alpha"},
	{"node", "beta"},
	{"node", "gamma"},
	{"claim", "alpha-carries-energy"},
	{"claim", "beta-was-observed-in-1999"},
	{"claim", "gamma-follows-from-alpha-and-beta"},
	{"claim", "alpha-may-extend-to-gamma"},
	{"source", "fixture-reference-work"},
	{"source", "fixture-uncited-source"},
	{"source", "fixture-archive-record"},
	{"source", "fixture-attribution-source"},
	{"vocabulary", "fixture-term"},
	{"vocabulary", "fixture-companion"},
	{"vocabulary", "fixture-orphan-term"},
	{"experiment", "fixture-listening-exercise"},
	{"experiment", "fixture-visualization-exercise"},
}

// relatedWorkflowScopes is the relationship-filter axis: unfiltered, each declared class alone,
// two genuine subsets, and the complete set.
//
// The single-class entries are generated from domain.RelatedPriorities rather than listed, so a
// class added to the precedence model enters this matrix without anybody remembering to add it.
func relatedWorkflowScopes() []struct{ name, param string } {
	out := []struct{ name, param string }{{"unfiltered", ""}}
	for _, p := range domain.RelatedPriorities {
		out = append(out, struct{ name, param string }{"rel-" + string(p), "relationship_types=" + string(p)})
	}
	out = append(out,
		struct{ name, param string }{"rel-pair", "relationship_types=evidential,attributive"},
		struct{ name, param string }{"rel-mixed", "relationship_types=navigational,conceptual"},
		struct{ name, param string }{"rel-all", "relationship_types=" + strings.Join(domain.RelatedPriorityNames(), ",")},
	)
	return out
}

// relatedWorkflowDestinations is the destination-filter axis, generated from the closed searchable
// set for the same reason.
func relatedWorkflowDestinations() []struct{ name, param string } {
	out := []struct{ name, param string }{{"anywhere", ""}}
	for _, class := range domain.SearchEntityTypeNames() {
		out = append(out, struct{ name, param string }{"to-" + class, "entity_types=" + class})
	}
	out = append(out,
		struct{ name, param string }{"to-subset", "entity_types=node,claim"},
		struct{ name, param string }{"to-all", "entity_types=" + strings.Join(domain.SearchEntityTypeNames(), ",")},
	)
	return out
}

// relatedWorkflowMatrix is the shared request set: the whole pipeline, exercised with its controls
// in combination rather than one at a time.
//
// It is built in a fixed order from fixed literals, so the matrix is the same list on every run
// and a failure names a reproducible request.
func relatedWorkflowMatrix() []relatedWorkflowCase {
	var cases []relatedWorkflowCase
	add := func(name string, start relatedWorkflowStart, params ...string) {
		target := start.path()
		var supplied []string
		for _, p := range params {
			if p != "" {
				supplied = append(supplied, p)
			}
		}
		if len(supplied) > 0 {
			target += "?" + strings.Join(supplied, "&")
		}
		cases = append(cases, relatedWorkflowCase{name: name, start: start, target: target})
	}

	scopes := relatedWorkflowScopes()
	destinations := relatedWorkflowDestinations()
	for _, start := range relatedWorkflowStarts {
		label := start.class + "/" + start.id
		for _, scope := range scopes {
			add(label+"/"+scope.name, start, scope.param)
		}
		for _, dest := range destinations {
			add(label+"/"+dest.name, start, dest.param)
		}
		// Both axes at once, which is the combination neither phase's own suite exercises across
		// every start.
		add(label+"/both", start, "relationship_types=conceptual,referential", "entity_types=node,vocabulary")
		// The limit axis, including the default, the minimum, the ceiling and past it.
		for _, limit := range []string{"limit=1", "limit=2", "limit=" + strconv.Itoa(service.MaxRelatedLimit),
			"limit=" + strconv.Itoa(service.MaxRelatedLimit*10), "limit=0"} {
			add(label+"/"+limit, start, limit)
		}
		add(label+"/everything", start, "relationship_types=conceptual,evidential,referential",
			"entity_types=node,claim,source,vocabulary", "limit=3")
	}
	return cases
}

// getRelatedWorkflow runs one matrix request and decodes the typed contract. A non-200 is fatal:
// every request in the matrix is well formed, so a refusal is a defect rather than a case.
func getRelatedWorkflow(t testing.TB, handler http.Handler, target string) domain.RelatedKnowledge {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
	}
	var result domain.RelatedKnowledge
	decode(t, rec, &result)
	return result
}

// relatedBody runs one request and returns the raw bytes, which is what a determinism check has to
// compare: two structurally equal results can still serialise differently.
// withoutContinuationToken removes the opaque cursor's value from a serialised response.
//
// It removes the value and leaves the key, so a scan run over the result still sees every other
// byte the server wrote, including the field name itself. It fails rather than returning the body
// unchanged if the response does not decode, so a malformed body cannot slip past a check by way
// of this helper.
func withoutContinuationToken(t testing.TB, body string) string {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("decode body: %v: %s", err, body)
	}
	if _, present := raw["next_continuation_token"]; !present {
		return body
	}
	raw["next_continuation_token"] = json.RawMessage(`""`)
	out, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encode body: %v", err)
	}
	return string(out)
}

func relatedBody(t testing.TB, handler http.Handler, target string) string {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// refusedRelated requires the established validation response: 400, the stable envelope, the
// existing invalid_query code, and a message that discloses nothing about the process. It returns
// the message so a case may additionally assert which rule it cited.
//
// It is written against the related route rather than reusing the Phase 1J search helper, because
// this route has a second refusal code of its own and a test that accepted either would not be
// checking which one a case earns.
func refusedRelated(t testing.TB, handler http.Handler, target string) string {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET %s = %d, want 400: %s", target, rec.Code, rec.Body.String())
	}
	return relatedErrorEnvelope(t, target, rec, httpapi.CodeInvalidQuery)
}

// relatedErrorEnvelope asserts the envelope structurally as well as by type, so an error response
// cannot grow a field - a hint, a caller echo, a trace id - without this failing.
func relatedErrorEnvelope(t testing.TB, target string, rec *httptest.ResponseRecorder, wantCode string) string {
	t.Helper()
	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("GET %s: error content-type = %q, want %q", target, got, want)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("GET %s: X-Content-Type-Options = %q, want nosniff", target, got)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("GET %s: decode envelope: %v", target, err)
	}
	if len(raw) != 1 || raw["error"] == nil {
		t.Fatalf("GET %s: error envelope has keys %v, want exactly [error]", target, sortedRawKeys(raw))
	}
	var detail map[string]string
	if err := json.Unmarshal(raw["error"], &detail); err != nil {
		t.Fatalf("GET %s: decode error detail: %v", target, err)
	}
	if len(detail) != 2 {
		t.Errorf("GET %s: error detail has keys %v, want exactly [code message]", target, sortedTextKeys(detail))
	}
	if detail["code"] != wantCode {
		t.Errorf("GET %s: error code = %q, want %q", target, detail["code"], wantCode)
	}
	if strings.TrimSpace(detail["message"]) == "" {
		t.Errorf("GET %s: empty error message", target)
	}
	assertNoInternalDisclosure(t, target, detail["message"])
	return detail["message"]
}

// --------------------------------------------------------------------------------------------
// A. The invariants, over the whole matrix
// --------------------------------------------------------------------------------------------

// relatedItemKey identifies one destination. Two classes may share an ID, so identity is the pair.
func relatedItemKey(item domain.RelatedItem) string {
	return string(item.EntityType) + "/" + item.ID
}

// relatedReasonKey identifies one canonical connection within an item, on the four fields the
// context layer deduplicates by. It is what makes "no item reports the same connection twice"
// checkable.
func relatedReasonKey(reason domain.RelatedReason) string {
	return reason.Relation + "|" + reason.Origin + "|" + strconv.FormatBool(reason.Derived)
}

// relatedReasons flattens an item's primary reason and its additional evidence into the order the
// response presents them, which is the order the precedence policy is supposed to have produced.
func relatedReasons(item domain.RelatedItem) []domain.RelatedReason {
	out := make([]domain.RelatedReason, 0, 1+len(item.AdditionalEvidence))
	out = append(out, item.Reason)
	out = append(out, item.AdditionalEvidence...)
	return out
}

// lessRelatedItemContract is the documented four-key ordering, written out independently of the
// service so the test states the contract rather than calling the code it is checking.
func lessRelatedItemContract(a, b domain.RelatedItem) bool {
	if a.Reason.PriorityRank != b.Reason.PriorityRank {
		return a.Reason.PriorityRank < b.Reason.PriorityRank
	}
	if a.Reason.Derived != b.Reason.Derived {
		return !a.Reason.Derived
	}
	if a.EntityType != b.EntityType {
		return domain.SearchEntityRank(a.EntityType) < domain.SearchEntityRank(b.EntityType)
	}
	return a.ID < b.ID
}

// TestRelatedWorkflowInvariantsHoldAcrossTheMatrix is the phase's central regression.
//
// Every property below is a statement about the pipeline as a whole rather than about one stage,
// and each is required of every request in the matrix. A per-feature test could not state most of
// them: "the additional evidence list is exactly the capped remainder of the evidence count" is
// only meaningful once grouping, ranking and bounding are all present, and "every reason's
// explanation is the fixed sentence for its own class" is only checkable once Phase 2A's structure
// and Phase 2B's prose are serialised together.
func TestRelatedWorkflowInvariantsHoldAcrossTheMatrix(t *testing.T) {
	handler := newHandler(t)
	priorityRank := map[domain.RelatedPriority]int{}
	for i, p := range domain.RelatedPriorities {
		priorityRank[p] = i
	}

	for _, tc := range relatedWorkflowMatrix() {
		t.Run(tc.name, func(t *testing.T) {
			result := getRelatedWorkflow(t, handler, tc.target)

			// 1. The start is the record that was asked for, resolved rather than echoed raw.
			if string(result.Start.EntityType) != tc.start.class || result.Start.ID != tc.start.id {
				t.Errorf("start = %s/%s, want %s/%s",
					result.Start.EntityType, result.Start.ID, tc.start.class, tc.start.id)
			}
			if strings.TrimSpace(result.Start.Title) == "" {
				t.Error("resolved start carries no title")
			}

			// 2. Nothing is unbounded. The applied limit is real, within the ceiling, and the
			// item list never exceeds it.
			if result.Limit < 1 || result.Limit > service.MaxRelatedLimit {
				t.Errorf("limit = %d, want 1..%d", result.Limit, service.MaxRelatedLimit)
			}
			if len(result.Items) > result.Limit {
				t.Errorf("items = %d exceeds the applied limit %d", len(result.Items), result.Limit)
			}
			if result.Counts.Returned != len(result.Items) {
				t.Errorf("counts.returned = %d, but %d items were serialised",
					result.Counts.Returned, len(result.Items))
			}
			if result.Counts.Eligible < result.Counts.Returned {
				t.Errorf("counts = %+v: eligible is below returned", result.Counts)
			}

			// 3. A cut answer always says it was cut, and an uncut one never claims to be.
			if want := result.Counts.Returned < result.Counts.Eligible; result.Truncated != want {
				t.Errorf("truncated = %v, want %v for counts %+v",
					result.Truncated, want, result.Counts)
			}

			// 4. The declared bounds are reported as constants, and the work actually done stays
			// inside them. A truncated scan must have reached the ceiling: those are one fact.
			if result.Bounds.MaxEvidencePerItem != service.MaxRelatedEvidencePerItem {
				t.Errorf("bounds.max_evidence_per_item = %d, want %d",
					result.Bounds.MaxEvidencePerItem, service.MaxRelatedEvidencePerItem)
			}
			if result.Bounds.MaxRelationsScanned != service.MaxRelatedRelationsScanned {
				t.Errorf("bounds.max_relations_scanned = %d, want %d",
					result.Bounds.MaxRelationsScanned, service.MaxRelatedRelationsScanned)
			}
			if result.Bounds.RelationsScanned < 0 || result.Bounds.RelationsScanned > result.Bounds.MaxRelationsScanned {
				t.Errorf("bounds.relations_scanned = %d, outside 0..%d",
					result.Bounds.RelationsScanned, result.Bounds.MaxRelationsScanned)
			}
			if result.Bounds.RelationsTruncated &&
				result.Bounds.RelationsScanned != result.Bounds.MaxRelationsScanned {
				t.Errorf("bounds = %+v: a truncated scan did not stop at the ceiling", result.Bounds)
			}
			// The eligible set is drawn from the relations that were scanned, so it can never
			// exceed them however the corpus is shaped.
			if result.Counts.Eligible > result.Bounds.RelationsScanned {
				t.Errorf("eligible = %d exceeds the %d relations scanned",
					result.Counts.Eligible, result.Bounds.RelationsScanned)
			}

			// 5. The echoes describe the request that ran, normalised, and are present exactly
			// when the caller supplied that filter.
			assertRelatedEchoes(t, tc.target, result)

			seen := map[string]bool{}
			for i, item := range result.Items {
				key := relatedItemKey(item)
				if seen[key] {
					t.Errorf("item %d: %s appears twice; grouping is per destination", i, key)
				}
				seen[key] = true

				// 6. An item is never the record the caller is already holding.
				if key == tc.start.class+"/"+tc.start.id {
					t.Errorf("item %d: the start was offered as a place to go next", i)
				}
				// 7. Experiment runs are not a discovery destination, as they are not a search class.
				if item.EntityType == domain.SearchEntityType("experiment_run") {
					t.Errorf("item %d: an experiment run entered discovery", i)
				}
				if strings.TrimSpace(item.Title) == "" {
					t.Errorf("item %d (%s): no title", i, key)
				}

				// 8. The evidence bound is exact rather than approximate: the additional list is
				// the capped remainder of the count, and the flag says whether anything was cut.
				reasons := relatedReasons(item)
				kept := item.EvidenceCount
				if kept > service.MaxRelatedEvidencePerItem {
					kept = service.MaxRelatedEvidencePerItem
				}
				if item.EvidenceCount < 1 {
					t.Errorf("item %d (%s): evidence_count = %d, want at least the reason itself",
						i, key, item.EvidenceCount)
				}
				if len(reasons) != kept {
					t.Errorf("item %d (%s): %d serialised reasons, want min(%d, %d) = %d",
						i, key, len(reasons), item.EvidenceCount, service.MaxRelatedEvidencePerItem, kept)
				}
				if want := item.EvidenceCount > service.MaxRelatedEvidencePerItem; item.EvidenceTruncated != want {
					t.Errorf("item %d (%s): evidence_truncated = %v, want %v for count %d",
						i, key, item.EvidenceTruncated, want, item.EvidenceCount)
				}

				// 9. No connection is reported twice inside one item, so a client counting what it
				// received cannot double-count, and the primary reason is not repeated below itself.
				reasonSeen := map[string]bool{}
				for j, reason := range reasons {
					rk := relatedReasonKey(reason)
					if reasonSeen[rk] {
						t.Errorf("item %d (%s): reason %d repeats connection %s", i, key, j, rk)
					}
					reasonSeen[rk] = true

					// 10. Every reason is classified, ranked and explained by one closed table.
					// unclassified is a fallback no corpus can produce, so seeing it here means a
					// canonical field reached the contract without a precedence decision.
					rank, known := priorityRank[reason.Priority]
					if !known {
						t.Errorf("item %d (%s): reason %d has priority %q, outside the declared model",
							i, key, j, reason.Priority)
						continue
					}
					if reason.PriorityRank != rank {
						t.Errorf("item %d (%s): reason %d rank = %d, want %d for %q",
							i, key, j, reason.PriorityRank, rank, reason.Priority)
					}
					if want := domain.RelatedPriorityExplanation(reason.Priority); reason.Explanation != want {
						t.Errorf("item %d (%s): reason %d explanation = %q, want the fixed sentence %q",
							i, key, j, reason.Explanation, want)
					}
					if strings.TrimSpace(reason.Relation) == "" || strings.TrimSpace(reason.Origin) == "" {
						t.Errorf("item %d (%s): reason %d names no canonical relation or field", i, key, j)
					}
					// 11. An explanation never names either endpoint: it says what kind of field
					// connected two records, which is a fact about the vocabulary rather than a
					// statement generated about this pair.
					for _, endpoint := range []string{tc.start.id, item.ID, result.Start.Title, item.Title} {
						if endpoint != "" && strings.Contains(reason.Explanation, endpoint) {
							t.Errorf("item %d (%s): reason %d explanation names an endpoint: %q",
								i, key, j, reason.Explanation)
						}
					}
				}

				// 12. An item's own reasons are in precedence order, so the strongest connection is
				// the one the item is explained by.
				for j := 1; j < len(reasons); j++ {
					if reasons[j-1].PriorityRank > reasons[j].PriorityRank {
						t.Errorf("item %d (%s): reason %d ranks below reason %d", i, key, j-1, j)
					}
				}
			}

			// 13. The item ordering is exactly the documented four keys, and it is total: no two
			// items compare equal in either direction.
			for i := 1; i < len(result.Items); i++ {
				prev, cur := result.Items[i-1], result.Items[i]
				if lessRelatedItemContract(cur, prev) {
					t.Errorf("items %d and %d are out of the documented order: %s before %s",
						i-1, i, relatedItemKey(prev), relatedItemKey(cur))
				}
				if !lessRelatedItemContract(prev, cur) {
					t.Errorf("items %d and %d compare equal; the ordering is not total", i-1, i)
				}
			}

			// 14. Nothing in a successful body describes the operator's machine.
			//
			// The Phase 2D continuation token is removed before the scan rather than exempted
			// from it, and the distinction matters. The scan looks for literal fragments that
			// betray a Go process - a package qualifier, a source path, a pointer's 0x - in prose
			// the server wrote. A token is not prose: it is base64 over an opaque payload, and
			// base64 produces those two characters by arithmetic. The claim identifier
			// beta-was-observed-in-1999 encodes through "bi0xOTk5", which contains 0x and means
			// nothing. Leaving it in the scan would make this invariant fail on the contents of
			// the corpus rather than on anything the backend disclosed.
			//
			// What the token actually carries is asserted directly instead, against the decoded
			// payload rather than against its encoding, by the Phase 2D service suite: no title,
			// no summary, no evidence, no path, no host, no internal name, and no field outside
			// the documented set.
			assertNoInternalDisclosure(t, tc.target,
				withoutContinuationToken(t, relatedBody(t, handler, tc.target)))
		})
	}
}

// assertRelatedEchoes requires each filter echo to be present exactly when that filter was
// supplied, to carry the normalised set rather than the caller's spelling, and to actually
// constrain the result it accompanies.
func assertRelatedEchoes(t testing.TB, target string, result domain.RelatedKnowledge) {
	t.Helper()
	query := ""
	if at := strings.Index(target, "?"); at >= 0 {
		query = target[at+1:]
	}

	askedRelationships := strings.Contains(query, "relationship_types=")
	if askedRelationships != (len(result.RelationshipTypes) > 0) {
		t.Errorf("relationship_types echo = %v for query %q", result.RelationshipTypes, query)
	}
	if askedRelationships {
		// The echo is in precedence order regardless of how the caller wrote it, and every
		// serialised reason belongs to a class the caller admitted.
		allowed := map[domain.RelatedPriority]bool{}
		for i, name := range result.RelationshipTypes {
			class := domain.RelatedPriority(name)
			if !domain.ValidRelatedPriority(name) {
				t.Errorf("relationship_types echo carries %q, outside the model", name)
				continue
			}
			allowed[class] = true
			if i > 0 && domain.RelatedPriorityRank(domain.RelatedPriority(result.RelationshipTypes[i-1])) >=
				domain.RelatedPriorityRank(class) {
				t.Errorf("relationship_types echo %v is not in precedence order", result.RelationshipTypes)
			}
		}
		for i, item := range result.Items {
			for j, reason := range relatedReasons(item) {
				if !allowed[reason.Priority] {
					t.Errorf("item %d reason %d is %q, a class the request excluded (%v)",
						i, j, reason.Priority, result.RelationshipTypes)
				}
			}
		}
	}

	askedDestinations := strings.Contains(query, "entity_types=")
	if askedDestinations != (len(result.EntityTypes) > 0) {
		t.Errorf("entity_types echo = %v for query %q", result.EntityTypes, query)
	}
	if askedDestinations {
		allowed := map[string]bool{}
		for i, name := range result.EntityTypes {
			allowed[name] = true
			if i > 0 && domain.SearchEntityRank(domain.SearchEntityType(result.EntityTypes[i-1])) >=
				domain.SearchEntityRank(domain.SearchEntityType(name)) {
				t.Errorf("entity_types echo %v is not in canonical class order", result.EntityTypes)
			}
		}
		for i, item := range result.Items {
			if !allowed[string(item.EntityType)] {
				t.Errorf("item %d is a %s, a class the request excluded (%v)",
					i, item.EntityType, result.EntityTypes)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// B. Cross-request invariants: how one request relates to another
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowNamingEveryClassEqualsNamingNone pins the one documented case where two
// different requests must produce the same answer.
//
// The README states that an unfiltered request and a request naming all seven classes return the
// same items, reasons, counts and ordering, and differ only in the echo. That is a claim about the
// filter being genuinely a set restriction rather than a second ranking input, and it is only
// checkable by comparing two responses.
func TestRelatedWorkflowNamingEveryClassEqualsNamingNone(t *testing.T) {
	handler := newHandler(t)
	allClasses := strings.Join(domain.RelatedPriorityNames(), ",")
	allDestinations := strings.Join(domain.SearchEntityTypeNames(), ",")

	for _, start := range relatedWorkflowStarts {
		plain := getRelatedWorkflow(t, handler, start.path())

		scoped := getRelatedWorkflow(t, handler, start.path()+"?relationship_types="+allClasses)
		if len(scoped.RelationshipTypes) != len(domain.RelatedPriorities) {
			t.Errorf("%s: echo = %v, want all declared classes", start.path(), scoped.RelationshipTypes)
		}
		scoped.RelationshipTypes = nil
		assertRelatedResultsEqual(t, start.path()+" [all relationship classes]", plain, scoped)

		everywhere := getRelatedWorkflow(t, handler, start.path()+"?entity_types="+allDestinations)
		if len(everywhere.EntityTypes) != len(domain.SearchEntityTypeNames()) {
			t.Errorf("%s: echo = %v, want all searchable classes", start.path(), everywhere.EntityTypes)
		}
		everywhere.EntityTypes = nil
		assertRelatedResultsEqual(t, start.path()+" [all destination classes]", plain, everywhere)
	}
}

// assertRelatedResultsEqual compares two results by their serialisation, which is what a client
// actually receives. Comparing field by field would let a key that is present in one and absent in
// the other pass as equal.
func assertRelatedResultsEqual(t testing.TB, label string, want, got domain.RelatedKnowledge) {
	t.Helper()
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("%s: marshal expected: %v", label, err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("%s: marshal actual: %v", label, err)
	}
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("%s differs:\n want %s\n  got %s", label, wantJSON, gotJSON)
	}
}

// TestRelatedWorkflowFilterOrderIsNormalisedNotHonoured requires two spellings of one scope to be
// byte-identical.
//
// It is the guardrail against the filter quietly becoming a ranking input. If the order a caller
// wrote the classes in ever reached the ordering, this is where it would show, and it checks the
// raw bytes rather than the decoded values so an echo re-ordered in place would fail too.
func TestRelatedWorkflowFilterOrderIsNormalisedNotHonoured(t *testing.T) {
	handler := newHandler(t)
	permutations := [][2]string{
		{"relationship_types=conceptual,navigational", "relationship_types=navigational,conceptual"},
		{"relationship_types=evidential,attributive", "relationship_types=attributive,evidential"},
		{"relationship_types=referential,conceptual,assertional", "relationship_types=assertional,referential,conceptual"},
		{"entity_types=node,claim", "entity_types=claim,node"},
		{"entity_types=vocabulary,source,node", "entity_types=node,vocabulary,source"},
	}
	for _, start := range relatedWorkflowStarts {
		for _, pair := range permutations {
			first := relatedBody(t, handler, start.path()+"?"+pair[0])
			second := relatedBody(t, handler, start.path()+"?"+pair[1])
			if first != second {
				t.Errorf("%s: %q and %q differ\n %s\n %s", start.path(), pair[0], pair[1], first, second)
			}
		}
	}
}

// TestRelatedWorkflowSmallerLimitsArePrefixes requires bounding to happen after ranking.
//
// The documented contract is that a bounded result is the front of the complete ordering and never
// a different selection. Comparing the serialised items rather than their identifiers means a
// limit that changed an item's reason, evidence or summary would fail here too, which is the
// failure a bare identifier comparison would miss.
func TestRelatedWorkflowSmallerLimitsArePrefixes(t *testing.T) {
	handler := newHandler(t)
	scopes := []string{"", "&relationship_types=conceptual", "&entity_types=node,claim"}
	for _, start := range relatedWorkflowStarts {
		for _, scope := range scopes {
			full := getRelatedWorkflow(t, handler,
				start.path()+"?limit="+strconv.Itoa(service.MaxRelatedLimit)+scope)
			for limit := 1; limit <= len(full.Items)+1; limit++ {
				page := getRelatedWorkflow(t, handler,
					start.path()+"?limit="+strconv.Itoa(limit)+scope)
				want := len(full.Items)
				if limit < want {
					want = limit
				}
				if len(page.Items) != want {
					t.Errorf("%s%s limit=%d: %d items, want %d",
						start.path(), scope, limit, len(page.Items), want)
					continue
				}
				for i := range page.Items {
					assertRelatedItemsEqual(t,
						start.path()+scope+" limit="+strconv.Itoa(limit),
						i, full.Items[i], page.Items[i])
				}
				// The eligible total is a property of the corpus and the filters, never of how
				// many items the caller asked to see.
				if page.Counts.Eligible != full.Counts.Eligible {
					t.Errorf("%s%s limit=%d: eligible = %d, want %d",
						start.path(), scope, limit, page.Counts.Eligible, full.Counts.Eligible)
				}
			}
		}
	}
}

func assertRelatedItemsEqual(t testing.TB, label string, at int, want, got domain.RelatedItem) {
	t.Helper()
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("%s: item %d differs\n want %s\n  got %s", label, at, wantJSON, gotJSON)
	}
}

// TestRelatedWorkflowNoFilterCanEnlargeTheScan pins the scan accounting.
//
// The documented rule is that relations are counted as they are examined, before any exclusion, so
// relations_scanned reports the work done rather than the answer produced. If it were ever counted
// after filtering, a narrow scope would appear cheap while walking exactly as much of the corpus,
// and the ceiling would stop bounding the request it is supposed to bound.
func TestRelatedWorkflowNoFilterCanEnlargeTheScan(t *testing.T) {
	handler := newHandler(t)
	for _, start := range relatedWorkflowStarts {
		baseline := getRelatedWorkflow(t, handler, start.path())
		for _, tc := range relatedWorkflowMatrix() {
			if tc.start != start {
				continue
			}
			result := getRelatedWorkflow(t, handler, tc.target)
			if result.Bounds.RelationsScanned != baseline.Bounds.RelationsScanned {
				t.Errorf("%s: relations_scanned = %d, want the unfiltered %d - a filter changed the work done",
					tc.target, result.Bounds.RelationsScanned, baseline.Bounds.RelationsScanned)
			}
			if result.Bounds.RelationsTruncated != baseline.Bounds.RelationsTruncated {
				t.Errorf("%s: relations_truncated = %v, want the unfiltered %v",
					tc.target, result.Bounds.RelationsTruncated, baseline.Bounds.RelationsTruncated)
			}
		}
	}
}

// TestRelatedWorkflowScopeRestrictsWithoutReordering requires a filtered result to be the
// unfiltered one restricted, rather than a separately computed answer.
//
// Both axes are checked, and they are checked differently because they mean different things. A
// destination scope removes whole items and must leave the survivors untouched. A relationship
// scope removes connections, so an item survives if any admitted connection reaches it and is then
// explained by the strongest of those - which is exactly the unfiltered item's reason list with the
// excluded classes struck out.
func TestRelatedWorkflowScopeRestrictsWithoutReordering(t *testing.T) {
	handler := newHandler(t)
	for _, start := range relatedWorkflowStarts {
		plain := getRelatedWorkflow(t, handler, start.path()+"?limit="+strconv.Itoa(service.MaxRelatedLimit))

		for _, class := range domain.SearchEntityTypeNames() {
			scoped := getRelatedWorkflow(t, handler,
				start.path()+"?limit="+strconv.Itoa(service.MaxRelatedLimit)+"&entity_types="+class)
			var want []domain.RelatedItem
			for _, item := range plain.Items {
				if string(item.EntityType) == class {
					want = append(want, item)
				}
			}
			if len(scoped.Items) != len(want) {
				t.Errorf("%s entity_types=%s: %d items, want the %d of that class in the unscoped result",
					start.path(), class, len(scoped.Items), len(want))
				continue
			}
			for i := range want {
				assertRelatedItemsEqual(t, start.path()+" entity_types="+class, i, want[i], scoped.Items[i])
			}
			if scoped.Counts.Eligible != len(want) {
				t.Errorf("%s entity_types=%s: eligible = %d, want %d",
					start.path(), class, scoped.Counts.Eligible, len(want))
			}
		}

		for _, class := range domain.RelatedPriorities {
			scoped := getRelatedWorkflow(t, handler,
				start.path()+"?limit="+strconv.Itoa(service.MaxRelatedLimit)+"&relationship_types="+string(class))
			// Rebuild the expected items from the unfiltered answer: every destination that has at
			// least one connection of this class, explained by its strongest such connection, in
			// the same relative order the unfiltered result placed those connections in.
			wantKeys := []string{}
			wantReason := map[string]domain.RelatedReason{}
			wantCount := map[string]int{}
			for _, item := range plain.Items {
				if item.EvidenceTruncated {
					// The unfiltered evidence list was cut, so it is not a complete statement of
					// what this destination is connected by and cannot be used as the expectation.
					continue
				}
				key := relatedItemKey(item)
				for _, reason := range relatedReasons(item) {
					if reason.Priority != class {
						continue
					}
					if wantCount[key] == 0 {
						wantKeys = append(wantKeys, key)
						wantReason[key] = reason
					}
					wantCount[key]++
				}
			}
			got := map[string]domain.RelatedItem{}
			for _, item := range scoped.Items {
				got[relatedItemKey(item)] = item
			}
			for _, key := range wantKeys {
				item, present := got[key]
				if !present {
					t.Errorf("%s relationship_types=%s: %s is missing, though a %s connection reaches it",
						start.path(), class, key, class)
					continue
				}
				if item.Reason != wantReason[key] {
					t.Errorf("%s relationship_types=%s: %s explained by %+v, want %+v",
						start.path(), class, key, item.Reason, wantReason[key])
				}
				if item.EvidenceCount != wantCount[key] {
					t.Errorf("%s relationship_types=%s: %s evidence_count = %d, want the %d admitted connections",
						start.path(), class, key, item.EvidenceCount, wantCount[key])
				}
			}
			// The scoped ordering is the unfiltered ordering restricted: the items that appear in
			// both keep their relative positions.
			var scopedOrder, plainOrder []string
			plainAt := map[string]int{}
			for i, item := range plain.Items {
				plainAt[relatedItemKey(item)] = i
			}
			for _, item := range scoped.Items {
				key := relatedItemKey(item)
				if _, known := plainAt[key]; known {
					scopedOrder = append(scopedOrder, key)
				}
			}
			plainOrder = append(plainOrder, scopedOrder...)
			sort.SliceStable(plainOrder, func(i, j int) bool { return plainAt[plainOrder[i]] < plainAt[plainOrder[j]] })
			for i := range scopedOrder {
				if scopedOrder[i] != plainOrder[i] {
					t.Errorf("%s relationship_types=%s: scoped order %v is a reordering of the unscoped one %v",
						start.path(), class, scopedOrder, plainOrder)
					break
				}
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// C. Determinism
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowRepeatedRequestsAreByteIdentical is the determinism check at the wire.
//
// Structural equality is not enough: two results that decode the same can still serialise
// differently if a key's presence or an array's order came from a map. The whole matrix is
// therefore replayed and compared as raw bytes.
func TestRelatedWorkflowRepeatedRequestsAreByteIdentical(t *testing.T) {
	handler := newHandler(t)
	for _, tc := range relatedWorkflowMatrix() {
		first := relatedBody(t, handler, tc.target)
		for i := 0; i < 3; i++ {
			if again := relatedBody(t, handler, tc.target); again != first {
				t.Fatalf("%s: repeat %d differs\n first %s\n again %s", tc.target, i, first, again)
			}
		}
	}
}

// TestRelatedWorkflowIsStableUnderConcurrentReaders replays the matrix from several goroutines.
//
// The index is built once and never written to afterwards, and the discovery path is supposed to
// read it without taking a copy of anything it could mutate. A shared slice reused between
// requests, or a filter scope written into index-owned state, would show up here as a body that
// differs from the sequential one.
func TestRelatedWorkflowIsStableUnderConcurrentReaders(t *testing.T) {
	handler := newHandler(t)
	matrix := relatedWorkflowMatrix()
	expected := make([]string, len(matrix))
	for i, tc := range matrix {
		expected[i] = relatedBody(t, handler, tc.target)
	}

	const readers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, tc := range matrix {
				rec := do(t, handler, http.MethodGet, tc.target)
				if rec.Code != http.StatusOK || rec.Body.String() != expected[i] {
					mu.Lock()
					failures = append(failures, tc.target)
					mu.Unlock()
					return
				}
			}
		}()
	}
	wg.Wait()
	for _, target := range failures {
		t.Errorf("concurrent reader disagreed on %s", target)
	}

	// Sequential requests after the concurrent replay must still produce the original bytes, so a
	// reader cannot have left anything behind for the next one.
	for i, tc := range matrix {
		if again := relatedBody(t, handler, tc.target); again != expected[i] {
			t.Errorf("%s changed after concurrent reads", tc.target)
		}
	}
}

// TestRelatedWorkflowDoesNotDependOnHowTheCorpusWasEnumerated answers the matrix from a second
// index built over an in-memory copy of the same corpus.
//
// It rules out a dependence on directory enumeration order rather than only on the request path:
// the two indexes read the same records through different filesystem implementations, so a
// projection that had absorbed the order os.ReadDir happened to return would disagree here while
// passing every repeat check.
func TestRelatedWorkflowDoesNotDependOnHowTheCorpusWasEnumerated(t *testing.T) {
	fromDisk := newHandler(t)

	repo, err := filesystem.NewFromFS(testsupport.MutableCorpus(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open in-memory fixture corpus: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build in-memory index: %v", err)
	}
	fromMemory := httpapi.NewServer(knowledge, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, tc := range relatedWorkflowMatrix() {
		want := relatedBody(t, fromDisk, tc.target)
		got := relatedBody(t, fromMemory, tc.target)
		if want != got {
			t.Errorf("%s differs between two indexes of one corpus\n disk   %s\n memory %s", tc.target, want, got)
		}
	}
}

// TestRelatedWorkflowRefusalsAreDeterministicToo requires an error body to be as reproducible as a
// successful one.
//
// An error body is part of the response, so "an identical request returns an identical response"
// has to cover it. The cases that matter are the ones carrying more than one violation at once:
// which rule the message cites must be decided by the request rather than by the order a map
// happened to be walked in.
func TestRelatedWorkflowRefusalsAreDeterministicToo(t *testing.T) {
	handler := newHandler(t)
	base := httpapi.APIBase + "/related/node/alpha"
	targets := []string{
		base + "?bogus=1&other=2",
		base + "?bogus=1&limit=1&limit=2",
		base + "?limit=1&limit=2&entity_types=node&entity_types=claim",
		base + "?relationship_types=conceptual&relationship_types=evidential",
		base + "?relationship_types=,&entity_types=,",
		base + "?relationship_types=BOGUS&entity_types=BOGUS",
		base + "?relationship_types=conceptual,conceptual&limit=-1",
	}
	for _, target := range targets {
		first := refusedRelated(t, handler, target)
		for i := 0; i < 40; i++ {
			if again := refusedRelated(t, handler, target); again != first {
				t.Fatalf("%s: refusal is not deterministic\n first %q\n again %q", target, first, again)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// D. The validation surface
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowMalformedRequestsAreRefusedConsistently is the whole refusal surface as one
// table.
//
// Each row is a request a caller could plausibly write and the rule it must be told about. They are
// gathered here rather than spread across the feature suites so that the surface can be read as a
// list of decisions, and so that a rule enforced for one parameter but not the other is visible as
// a gap in the table rather than as a test nobody wrote.
func TestRelatedWorkflowMalformedRequestsAreRefusedConsistently(t *testing.T) {
	handler := newHandler(t)
	base := httpapi.APIBase + "/related/node/alpha"
	cases := []struct{ name, target, wants string }{
		// The relationship scope, in every spelling of wrong.
		{"relationship/empty", base + "?relationship_types=", "relationship_types"},
		{"relationship/whitespace", base + "?relationship_types=%20", "relationship_types"},
		{"relationship/leading-comma", base + "?relationship_types=,conceptual", "empty value"},
		{"relationship/trailing-comma", base + "?relationship_types=conceptual,", "empty value"},
		{"relationship/doubled-comma", base + "?relationship_types=conceptual,,evidential", "empty value"},
		{"relationship/blank-member", base + "?relationship_types=conceptual,%20,evidential", "empty value"},
		{"relationship/duplicate", base + "?relationship_types=conceptual,conceptual", "repeat"},
		{"relationship/duplicate-after-trim", base + "?relationship_types=conceptual,%20conceptual", "repeat"},
		{"relationship/wrong-case", base + "?relationship_types=Conceptual", "relationship_types"},
		{"relationship/upper-case", base + "?relationship_types=CONCEPTUAL", "relationship_types"},
		{"relationship/unknown", base + "?relationship_types=structural", "relationship_types"},
		{"relationship/unclassified", base + "?relationship_types=unclassified", "relationship_types"},
		{"relationship/mixed-valid-invalid", base + "?relationship_types=conceptual,structural", "relationship_types"},
		{"relationship/plural", base + "?relationship_types=conceptuals", "relationship_types"},
		{"relationship/semicolon-delimited", base + "?relationship_types=conceptual;evidential", ""},
		{"relationship/repeated-key", base + "?relationship_types=conceptual&relationship_types=evidential", "exactly once"},
		{"relationship/singular-alias", base + "?relationship_type=conceptual", "Unsupported query parameter"},
		// The destination scope, which must be refused by the same rules.
		{"destination/empty", base + "?entity_types=", "entity_types"},
		{"destination/whitespace", base + "?entity_types=%20", "entity_types"},
		{"destination/trailing-comma", base + "?entity_types=node,", "empty value"},
		{"destination/duplicate", base + "?entity_types=node,node", "repeat"},
		{"destination/wrong-case", base + "?entity_types=Node", "entity_types"},
		{"destination/unknown", base + "?entity_types=widget", "entity_types"},
		{"destination/experiment-run", base + "?entity_types=experiment_run", "entity_types"},
		{"destination/repeated-key", base + "?entity_types=node&entity_types=claim", "exactly once"},
		{"destination/search-only-alias", base + "?type=node", "Unsupported query parameter"},
		// The limit.
		{"limit/negative", base + "?limit=-1", "non-negative integer"},
		{"limit/text", base + "?limit=abc", "non-negative integer"},
		{"limit/fractional", base + "?limit=1.0", "non-negative integer"},
		{"limit/overflow", base + "?limit=99999999999999999999", "non-negative integer"},
		{"limit/repeated-key", base + "?limit=1&limit=2", "exactly once"},
		// The query string itself.
		{"query/unknown-parameter", base + "?depth=2", "Unsupported query parameter"},
		{"query/offset", base + "?offset=1", "Unsupported query parameter"},
		{"query/q", base + "?q=alpha", "Unsupported query parameter"},
		{"query/malformed-escape", base + "?relationship_types=%zz", "query string"},
		{"query/malformed-escape-destination", base + "?entity_types=%zz", "query string"},
		{"query/malformed-escape-limit", base + "?limit=%zz", "query string"},
		{"query/malformed-escape-name", base + "?%zz=1", "query string"},
		// The route values.
		{"start/unsupported-class", httpapi.APIBase + "/related/widget/alpha", "entity type"},
		{"start/experiment-run", httpapi.APIBase + "/related/experiment_run/fixture-listening-exercise-completed-a", "entity type"},
		{"start/wrong-case-class", httpapi.APIBase + "/related/Node/alpha", "entity type"},
		{"start/traversal-identifier", httpapi.APIBase + "/related/node/%2e%2e%2fclaims", "Identifier"},
		{"start/encoded-separator", httpapi.APIBase + "/related/node/alpha%2Fbeta", "Identifier"},
		{"start/oversized-identifier", httpapi.APIBase + "/related/node/" + strings.Repeat("a", 129), "Identifier"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := refusedRelated(t, handler, tc.target)
			if tc.wants != "" && !strings.Contains(message, tc.wants) {
				t.Errorf("GET %s: message %q does not cite %q", tc.target, message, tc.wants)
			}
			// A refusal never echoes what the caller sent, so a 400 cannot be used to reflect
			// content back or to learn whether a record exists.
			for _, secret := range []string{"structural", "widget", "conceptuals", "BOGUS", "CONCEPTUAL"} {
				if strings.Contains(tc.target, secret) && strings.Contains(message, secret) {
					t.Errorf("GET %s: message echoes the rejected value %q: %s", tc.target, secret, message)
				}
			}
		})
	}
}

// TestRelatedWorkflowMalformedQueryStringsNeverFallBackToAnUnfilteredAnswer is the regression for
// the Phase 2C defect.
//
// A query string carrying an invalid percent-escape or a semicolon separator cannot be parsed by
// url.ParseQuery. r.URL.Query() discards that error and returns the pairs it could read, so the
// malformed ones simply vanished - and a vanished filter is an unfiltered request. A caller who
// wrote entity_types=%zz received the complete unrestricted result set under a 200, with no echo
// of the filter they believed they had applied. Each case here is required to be a refusal and,
// separately, to be nothing like the unfiltered answer.
func TestRelatedWorkflowMalformedQueryStringsNeverFallBackToAnUnfilteredAnswer(t *testing.T) {
	handler := newHandler(t)
	for _, start := range relatedWorkflowStarts {
		unfiltered := relatedBody(t, handler, start.path())
		for _, query := range []string{
			"?entity_types=%zz",
			"?relationship_types=%zz",
			"?limit=%zz",
			"?%zz=1",
			"?relationship_types=conceptual&entity_types=%zz",
			"?relationship_types=conceptual;evidential",
			"?entity_types=node;claim",
		} {
			target := start.path() + query
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s = %d, want 400: %s", target, rec.Code, rec.Body.String())
				continue
			}
			if rec.Body.String() == unfiltered {
				t.Errorf("GET %s returned the unfiltered discovery result", target)
			}
			relatedErrorEnvelope(t, target, rec, httpapi.CodeInvalidQuery)
		}
	}
}

// TestRelatedWorkflowUnsatisfiableRequestsAreAnswered keeps the well-formed-but-empty cases apart
// from the malformed ones.
//
// A valid scope a record has no connection of is a fact about the corpus, not a mistake in the
// request, and it must answer 200 with the complete zero-valued structure rather than an error or a
// partially populated body. The distinction is the reason the refusal table above can be read as a
// list of actual defects.
func TestRelatedWorkflowUnsatisfiableRequestsAreAnswered(t *testing.T) {
	handler := newHandler(t)
	targets := []string{
		httpapi.APIBase + "/related/source/fixture-uncited-source",
		httpapi.APIBase + "/related/vocabulary/fixture-orphan-term",
		httpapi.APIBase + "/related/session/session-02-unused",
		httpapi.APIBase + "/related/node/alpha?relationship_types=attributive&entity_types=experiment",
		httpapi.APIBase + "/related/vocabulary/fixture-orphan-term?relationship_types=conceptual",
	}
	for _, target := range targets {
		result := getRelatedWorkflow(t, handler, target)
		if len(result.Items) == 0 {
			if result.Counts.Eligible != 0 || result.Counts.Returned != 0 || result.Truncated {
				t.Errorf("GET %s: empty item list with counts %+v truncated=%v",
					target, result.Counts, result.Truncated)
			}
			if result.Limit != service.DefaultRelatedLimit {
				t.Errorf("GET %s: limit = %d, want the default %d", target, result.Limit, service.DefaultRelatedLimit)
			}
			if result.Bounds.MaxEvidencePerItem != service.MaxRelatedEvidencePerItem {
				t.Errorf("GET %s: an empty answer dropped its bounds: %+v", target, result.Bounds)
			}
		}
		// The empty list is serialised as an empty array rather than as null, so a client can
		// iterate it without a nil check.
		body := relatedBody(t, handler, target)
		if strings.Contains(body, `"items":null`) {
			t.Errorf("GET %s serialised items as null: %s", target, body)
		}
	}
}

// TestRelatedWorkflowMissingStartsAreNotFound keeps "this record is related to nothing" and "this
// record does not exist" apart, which is the distinction that lets an empty list be an answer.
func TestRelatedWorkflowMissingStartsAreNotFound(t *testing.T) {
	handler := newHandler(t)
	targets := []string{
		httpapi.APIBase + "/related/node/no-such-node",
		httpapi.APIBase + "/related/claim/no-such-claim",
		httpapi.APIBase + "/related/vocabulary/no-such-term",
		// A canonical ID is ASCII kebab-case by contract, so a Unicode identifier is well-formed
		// input naming a record the corpus cannot contain.
		httpapi.APIBase + "/related/node/caf%C3%A9",
		// A record that exists in another class is still absent from this one: identity is the pair.
		httpapi.APIBase + "/related/node/fixture-term",
		httpapi.APIBase + "/related/claim/alpha",
	}
	for _, target := range targets {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404: %s", target, rec.Code, rec.Body.String())
			continue
		}
		relatedErrorEnvelope(t, target, rec, httpapi.CodeRelatedStartNotFound)
	}

	// A request that is both malformed and names no record is answered as malformed, so a caller
	// is told about every mistake that was visible in what they sent.
	message := refusedRelated(t, handler,
		httpapi.APIBase+"/related/node/no-such-node?relationship_types=structural")
	if !strings.Contains(message, "relationship_types") {
		t.Errorf("a malformed request naming no record cited %q rather than the filter", message)
	}
}

// --------------------------------------------------------------------------------------------
// E. HTTP method behaviour
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowHeadServesHeadersWithoutABody is the one check that needs a real connection.
//
// Every other HEAD assertion in this package runs through httptest.NewRecorder, which invokes the
// handler directly and therefore records the body the handler wrote. Discarding it is net/http's
// job and happens in the server, so the claim that HEAD carries no body has never actually been
// exercised. This serves the requests over a real listener and requires the status, the content
// type and the content length of the matching GET, with zero bytes of body.
func TestRelatedWorkflowHeadServesHeadersWithoutABody(t *testing.T) {
	server := httptest.NewServer(newHandler(t))
	defer server.Close()

	targets := []string{
		httpapi.APIBase + "/related/node/alpha",
		httpapi.APIBase + "/related/node/alpha?relationship_types=conceptual",
		httpapi.APIBase + "/related/source/fixture-uncited-source",
		httpapi.APIBase + "/related/node/no-such-node",
		httpapi.APIBase + "/related/node/alpha?relationship_types=structural",
	}
	for _, target := range targets {
		get, err := http.Get(server.URL + target)
		if err != nil {
			t.Fatalf("GET %s: %v", target, err)
		}
		body, err := io.ReadAll(get.Body)
		get.Body.Close()
		if err != nil {
			t.Fatalf("GET %s: read body: %v", target, err)
		}

		request, err := http.NewRequest(http.MethodHead, server.URL+target, nil)
		if err != nil {
			t.Fatalf("HEAD %s: %v", target, err)
		}
		head, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("HEAD %s: %v", target, err)
		}
		headBody, err := io.ReadAll(head.Body)
		head.Body.Close()
		if err != nil {
			t.Fatalf("HEAD %s: read body: %v", target, err)
		}

		if head.StatusCode != get.StatusCode {
			t.Errorf("HEAD %s = %d, want the GET status %d", target, head.StatusCode, get.StatusCode)
		}
		if got, want := head.Header.Get("Content-Type"), get.Header.Get("Content-Type"); got != want {
			t.Errorf("HEAD %s content-type = %q, want %q", target, got, want)
		}
		if got, want := head.Header.Get("X-Content-Type-Options"), "nosniff"; got != want {
			t.Errorf("HEAD %s X-Content-Type-Options = %q, want %q", target, got, want)
		}
		if len(headBody) != 0 {
			t.Errorf("HEAD %s returned %d bytes of body", target, len(headBody))
		}
		// A declared length must be the length the matching GET would have sent. It is not
		// required to be declared at all: net/http only sets Content-Length when the whole
		// response fits its write buffer, and chunks it otherwise, which is a transport decision
		// rather than part of this API's contract.
		if declared := head.Header.Get("Content-Length"); declared != "" {
			if want := strconv.Itoa(len(body)); declared != want {
				t.Errorf("HEAD %s content-length = %q, want the GET body length %q", target, declared, want)
			}
		}
	}
}

// TestRelatedWorkflowRejectsEveryMutatingMethod pins the read-only contract at the edge for this
// route specifically, including the methods a proxy or a browser preflight would send.
//
// The refusal is required to happen before routing, so it must be identical on a path that
// resolves and on one that does not: a caller cannot use a mutating method to learn whether a
// record exists.
func TestRelatedWorkflowRejectsEveryMutatingMethod(t *testing.T) {
	handler := newHandler(t)
	targets := []string{
		httpapi.APIBase + "/related/node/alpha",
		httpapi.APIBase + "/related/node/no-such-node",
		httpapi.APIBase + "/related/widget/alpha",
		httpapi.APIBase + "/related/node/alpha?relationship_types=conceptual",
	}
	methods := []string{
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		http.MethodOptions, http.MethodConnect, http.MethodTrace,
	}
	for _, target := range targets {
		for _, method := range methods {
			rec := do(t, handler, method, target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, target, rec.Code)
				continue
			}
			if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
				t.Errorf("%s %s: Allow = %q, want %q", method, target, got, "GET, HEAD")
			}
			relatedErrorEnvelope(t, method+" "+target, rec, httpapi.CodeMethodNotAllow)
		}
	}

	// The index still answers exactly as before, so a refused request left nothing behind.
	for _, tc := range relatedWorkflowMatrix() {
		if tc.name != "node/alpha/unfiltered" {
			continue
		}
		before := relatedBody(t, handler, tc.target)
		for _, method := range methods {
			do(t, handler, method, tc.target)
		}
		if after := relatedBody(t, handler, tc.target); after != before {
			t.Errorf("%s changed after a run of refused mutating requests", tc.target)
		}
	}
}

// TestRelatedWorkflowSuccessResponsesCarryTheDocumentedHeaders keeps the success envelope pinned
// alongside the error one, so the two cannot drift apart.
func TestRelatedWorkflowSuccessResponsesCarryTheDocumentedHeaders(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{
		httpapi.APIBase + "/related/node/alpha",
		httpapi.APIBase + "/related/node/alpha?relationship_types=conceptual&entity_types=node&limit=2",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", target, rec.Code)
		}
		if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
			t.Errorf("GET %s content-type = %q, want %q", target, got, want)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("GET %s X-Content-Type-Options = %q, want nosniff", target, got)
		}
		// The response carries exactly the documented top-level keys and nothing else, so a field
		// cannot be added to this contract without a deliberate decision.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("GET %s: decode: %v", target, err)
		}
		// has_more and next_continuation_token are the Phase 2D additions. has_more joins the
		// required list below rather than only this one, because a boolean that disappears when
		// false cannot be told apart from a server that does not implement it;
		// next_continuation_token is documented but optional, since it is present exactly when
		// has_more is true and a final page must not carry one.
		documented := map[string]bool{
			"start": true, "entity_types": true, "relationship_types": true,
			"limit": true, "bounds": true, "counts": true, "truncated": true, "items": true,
			"has_more": true, "next_continuation_token": true,
		}
		for key := range raw {
			if !documented[key] {
				t.Errorf("GET %s: undocumented top-level key %q", target, key)
			}
		}
		for _, required := range []string{"start", "limit", "bounds", "counts", "truncated", "has_more", "items"} {
			if raw[required] == nil {
				t.Errorf("GET %s: required key %q is absent", target, required)
			}
		}
	}
}

// TestRelatedWorkflowAcceptsOnlyTheDocumentedParameters walks the accepted query string as a set
// rather than checking each parameter where it happens to be used.
//
// Every documented parameter must be accepted on its own, and the parameters this route
// deliberately does not have - the search and traversal ones a caller might reasonably try - must
// each be refused rather than ignored.
func TestRelatedWorkflowAcceptsOnlyTheDocumentedParameters(t *testing.T) {
	handler := newHandler(t)
	base := httpapi.APIBase + "/related/node/alpha"
	for _, accepted := range []string{
		"entity_types=node", "relationship_types=conceptual", "limit=5",
	} {
		if rec := do(t, handler, http.MethodGet, base+"?"+accepted); rec.Code != http.StatusOK {
			t.Errorf("GET %s?%s = %d, want 200: %s", base, accepted, rec.Code, rec.Body.String())
		}
	}
	for _, refused := range []string{
		"q=alpha", "type=node", "offset=1", "depth=2", "include_context=true",
		"query_mode=all_terms", "relationship=produces", "target_type=node",
		"relationship_type=conceptual", "entity_type=node", "sort=priority", "order=desc",
	} {
		message := refusedRelated(t, handler, base+"?"+refused)
		if !strings.Contains(message, "Unsupported query parameter") {
			t.Errorf("GET %s?%s was refused for %q rather than as an unsupported parameter",
				base, refused, message)
		}
		// The message lists what is supported, so a caller can correct the request from the
		// response alone.
		for _, supported := range []string{"entity_types", "limit", "relationship_types"} {
			if !strings.Contains(message, supported) {
				t.Errorf("GET %s?%s: message does not list %q: %s", base, refused, supported, message)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// F. Coexistence with the phases either side of it
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowLeavesTheOtherRoutesAlone requires the earlier phases to answer identically
// before and after the whole related-knowledge matrix has run.
//
// Discovery reads projections that search, context and traversal also read. If it ever took a
// slice of one of them and sorted it, or wrote a filter into index-owned state, the damage would
// not show in the discovery response - it would show in the next request to whichever route owns
// that projection.
func TestRelatedWorkflowLeavesTheOtherRoutesAlone(t *testing.T) {
	handler := newHandler(t)
	neighbours := []string{
		httpapi.APIBase + "/project",
		httpapi.APIBase + "/graph",
		httpapi.APIBase + "/nodes",
		httpapi.APIBase + "/nodes/alpha",
		httpapi.APIBase + "/claims",
		httpapi.APIBase + "/vocabulary",
		httpapi.APIBase + "/experiments",
		httpapi.APIBase + "/experiment-runs",
		httpapi.APIBase + "/search?q=fixture",
		httpapi.APIBase + "/search?q=fixture&include_context=true",
		httpapi.APIBase + "/graph/entities/node/alpha/relationships",
		httpapi.APIBase + "/graph/entities/node/alpha/traverse?depth=3",
		httpapi.APIBase + "/diagnostics",
	}
	before := make([]string, len(neighbours))
	for i, target := range neighbours {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", target, rec.Code)
		}
		before[i] = rec.Body.String()
	}

	for _, tc := range relatedWorkflowMatrix() {
		do(t, handler, http.MethodGet, tc.target)
	}

	for i, target := range neighbours {
		rec := do(t, handler, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d after the discovery matrix", target, rec.Code)
		}
		if rec.Body.String() != before[i] {
			t.Errorf("GET %s changed after the discovery matrix ran", target)
		}
	}
}
