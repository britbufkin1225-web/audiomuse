package service_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// Phase 2C: the related-knowledge bounds and the two filters, tested where they meet.
//
// The Phase 2A and Phase 2B suites each pin their own contract against the fixture corpus, and the
// Phase 2C HTTP suite pins the whole pipeline as a client sees it. What neither reaches is the
// corner where the bounds interact at a scale the fixture cannot express: a start whose canonical
// context is wider than the scan ceiling, asked with a filter, at a limit, for an item whose
// evidence must be cut. Three caps then apply to one request, and the questions that only exist
// there are whether they can be told apart in the response, whether a filter can move any of them,
// and whether the answer is still the same on the next run.
//
// Everything below either needs a synthetic corpus larger than the fixture or needs to be asked of
// the service directly because HTTP cannot express it - a negative limit, a nil versus an empty
// filter, a caller that writes to the result it was handed.

// --------------------------------------------------------------------------------------------
// A. The bounds, composed
// --------------------------------------------------------------------------------------------

// relatedWorkflowFilters is the filter axis used against the wide corpus.
//
// Every case is a scope the wide corpus can answer or deliberately cannot: the hub's spokes reach
// it over one canonical field, so conceptual admits everything and every other class admits
// nothing. Both outcomes matter - a filter that admits nothing must still report the work the scan
// actually did, which is the accounting rule this file exists to pin.
func relatedWorkflowFilters() []struct {
	name  string
	query service.RelatedQuery
} {
	out := []struct {
		name  string
		query service.RelatedQuery
	}{
		{"unfiltered", service.RelatedQuery{}},
		{"limited", service.RelatedQuery{Limit: 7}},
		{"at-ceiling", service.RelatedQuery{Limit: service.MaxRelatedLimit}},
		{"over-ceiling", service.RelatedQuery{Limit: service.MaxRelatedLimit * 10}},
		{"destination-node", service.RelatedQuery{EntityTypes: []string{"node"}}},
		{"destination-claim", service.RelatedQuery{EntityTypes: []string{"claim"}}},
		{"destination-all", service.RelatedQuery{EntityTypes: domain.SearchEntityTypeNames()}},
	}
	for _, class := range domain.RelatedPriorities {
		out = append(out, struct {
			name  string
			query service.RelatedQuery
		}{"relationship-" + string(class), service.RelatedQuery{RelationshipTypes: []string{string(class)}}})
	}
	out = append(out, struct {
		name  string
		query service.RelatedQuery
	}{"relationship-all", service.RelatedQuery{RelationshipTypes: domain.RelatedPriorityNames()}})
	out = append(out, struct {
		name  string
		query service.RelatedQuery
	}{"both-and-limited", service.RelatedQuery{
		Limit:             5,
		EntityTypes:       []string{"node"},
		RelationshipTypes: []string{"conceptual"},
	}})
	return out
}

// TestRelatedWorkflowScanCeilingSurvivesEveryFilter is the accounting regression.
//
// The documented rule is that relations are counted as they are examined, before the destination
// scope, the relationship scope and the self-exclusion are applied, so relations_scanned describes
// the work done rather than the answer produced. The consequence a caller depends on is that no
// filter can buy a larger scan - a narrow scope that walked further while returning less would
// leave the ceiling bounding nothing.
//
// It is asked on a corpus one relation past the ceiling, which is the only size where the question
// has an answer: below it every scope reports the same number because there was nothing left to
// scan, and the invariant would hold vacuously.
func TestRelatedWorkflowScanCeilingSurvivesEveryFilter(t *testing.T) {
	k := indexFrom(t, wideRelatedCorpus(t, service.MaxRelatedRelationsScanned+1))

	baseline := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{})
	if !baseline.Bounds.RelationsTruncated {
		t.Fatalf("the wide corpus did not cut the scan: %+v", baseline.Bounds)
	}

	for _, tc := range relatedWorkflowFilters() {
		t.Run(tc.name, func(t *testing.T) {
			result := mustRelated(t, k, domain.SearchNode, "hub", tc.query)

			// The scan is a property of the start, never of the filters.
			if result.Bounds.RelationsScanned != baseline.Bounds.RelationsScanned {
				t.Errorf("relations_scanned = %d, want the unfiltered %d",
					result.Bounds.RelationsScanned, baseline.Bounds.RelationsScanned)
			}
			if result.Bounds.RelationsScanned != service.MaxRelatedRelationsScanned {
				t.Errorf("relations_scanned = %d, want the ceiling %d",
					result.Bounds.RelationsScanned, service.MaxRelatedRelationsScanned)
			}
			if !result.Bounds.RelationsTruncated {
				t.Error("a scan cut at the ceiling did not report itself as truncated")
			}

			// The three bounds stay distinguishable. A cut scan, a cut item list and a cut
			// evidence list are three different facts, and a caller has to be able to tell which
			// one shortened the answer they are holding.
			if result.Counts.Eligible > result.Bounds.RelationsScanned {
				t.Errorf("eligible = %d exceeds the %d relations scanned",
					result.Counts.Eligible, result.Bounds.RelationsScanned)
			}
			if result.Counts.Returned != len(result.Items) {
				t.Errorf("counts.returned = %d but %d items were built",
					result.Counts.Returned, len(result.Items))
			}
			if result.Counts.Returned > result.Limit {
				t.Errorf("returned %d items under a limit of %d", result.Counts.Returned, result.Limit)
			}
			if want := result.Counts.Returned < result.Counts.Eligible; result.Truncated != want {
				t.Errorf("truncated = %v, want %v for counts %+v", result.Truncated, want, result.Counts)
			}
			for _, item := range result.Items {
				if got := 1 + len(item.AdditionalEvidence); got > service.MaxRelatedEvidencePerItem {
					t.Errorf("item %s/%s carries %d reasons, past the cap of %d",
						item.EntityType, item.ID, got, service.MaxRelatedEvidencePerItem)
				}
			}
		})
	}
}

// TestRelatedWorkflowFilteredScanIsNotPostFiltered separates the two numbers a scan ceiling could
// plausibly report.
//
// A scope that admits nothing is the case where a pre-filter and a post-filter count differ most:
// pre-filter it is the ceiling, post-filter it is zero. The response must report the ceiling, and
// must report an eligible count of zero beside it, because those are answers to two different
// questions - how much of the corpus this request walked, and how much of it the caller asked
// about.
func TestRelatedWorkflowFilteredScanIsNotPostFiltered(t *testing.T) {
	k := indexFrom(t, wideRelatedCorpus(t, service.MaxRelatedRelationsScanned+1))

	// The hub's spokes reach it over node.relationships alone, so every class but conceptual
	// admits none of them, and no destination is a claim.
	empty := []service.RelatedQuery{
		{RelationshipTypes: []string{"evidential"}},
		{RelationshipTypes: []string{"navigational"}},
		{EntityTypes: []string{"claim"}},
		{EntityTypes: []string{"experiment"}, RelationshipTypes: []string{"conceptual"}},
	}
	for i, query := range empty {
		result := mustRelated(t, k, domain.SearchNode, "hub", query)
		if len(result.Items) != 0 || result.Counts.Eligible != 0 || result.Counts.Returned != 0 {
			t.Errorf("query %d admitted %d items with counts %+v, want an empty answer",
				i, len(result.Items), result.Counts)
		}
		if result.Truncated {
			t.Errorf("query %d: an empty answer reported item truncation", i)
		}
		if result.Bounds.RelationsScanned != service.MaxRelatedRelationsScanned {
			t.Errorf("query %d: relations_scanned = %d, want the ceiling %d - the scan was counted after the filter",
				i, result.Bounds.RelationsScanned, service.MaxRelatedRelationsScanned)
		}
		if !result.Bounds.RelationsTruncated {
			t.Errorf("query %d: an empty answer hid that its scan was cut", i)
		}
	}
}

// TestRelatedWorkflowTruncatedResultsAreStillDeterministic requires a bounded answer to be as
// reproducible as a complete one.
//
// A ceiling that cut the work is the one place where the answer depends on which relations were
// reached first, so it is the place a dependence on map iteration or on directory enumeration
// would surface while every unbounded test kept passing. The same request is asked repeatedly of
// one index and once of a second index built independently from the same corpus, and all of them
// must serialise to the same bytes.
func TestRelatedWorkflowTruncatedResultsAreStillDeterministic(t *testing.T) {
	corpus := wideRelatedCorpus(t, service.MaxRelatedRelationsScanned+1)
	first := indexFrom(t, corpus)
	second := indexFrom(t, corpus)

	for _, tc := range relatedWorkflowFilters() {
		t.Run(tc.name, func(t *testing.T) {
			want := relatedWorkflowJSON(t, mustRelated(t, first, domain.SearchNode, "hub", tc.query))
			for i := 0; i < 3; i++ {
				again := relatedWorkflowJSON(t, mustRelated(t, first, domain.SearchNode, "hub", tc.query))
				if again != want {
					t.Fatalf("repeat %d differs\n want %s\n  got %s", i, want, again)
				}
			}
			if other := relatedWorkflowJSON(t, mustRelated(t, second, domain.SearchNode, "hub", tc.query)); other != want {
				t.Errorf("a second index of the same corpus disagrees\n want %s\n  got %s", want, other)
			}
		})
	}
}

func relatedWorkflowJSON(t testing.TB, result domain.RelatedKnowledge) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal discovery result: %v", err)
	}
	return string(encoded)
}

// --------------------------------------------------------------------------------------------
// B. Evidence bounding where several precedence classes meet
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowEvidenceIsTheStrongestPrefixAcrossClasses pins which connections survive the
// evidence cap when a destination is reached by more than one kind of canonical field.
//
// The existing Phase 2A bound test uses six connections of one class, which checks the cap but not
// the selection: within one class the tie-breakers decide, and precedence never has to arbitrate.
// Here the two records are connected by six typed edges spanning both directions, and the kept
// five must be the front of the full precedence ordering rather than the first five the corpus
// happened to list. An item whose evidence was cut must also still report its complete count, so
// the caller can tell a short explanation from a small connection.
func TestRelatedWorkflowEvidenceIsTheStrongestPrefixAcrossClasses(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	edges := func(target string, kinds ...string) string {
		out := make([]string, 0, len(kinds))
		for _, kind := range kinds {
			out = append(out, fmt.Sprintf(`{"target": %q, "type": %q}`, target, kind))
		}
		return "[" + strings.Join(out, ", ") + "]"
	}
	testsupport.Write(corpus, "nodes/dsp/left.md", testsupport.ValidNode(
		"left", "Left", "dsp", "seed",
		edges("right", "produces", "characterized_by", "processes"), "[]"))
	testsupport.Write(corpus, "nodes/dsp/right.md", testsupport.ValidNode(
		"right", "Right", "dsp", "seed",
		edges("left", "produces", "characterized_by", "processes"), "[]"))
	k := indexFrom(t, corpus)

	result := mustRelated(t, k, domain.SearchNode, "left", service.RelatedQuery{})
	var item domain.RelatedItem
	for _, candidate := range result.Items {
		if candidate.ID == "right" {
			item = candidate
		}
	}
	if item.ID == "" {
		t.Fatalf("right is not among %v", relatedRefs(result))
	}

	reasons := append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...)
	if len(reasons) != service.MaxRelatedEvidencePerItem {
		t.Fatalf("carried %d reasons, want the cap of %d", len(reasons), service.MaxRelatedEvidencePerItem)
	}
	if item.EvidenceCount != 6 || !item.EvidenceTruncated {
		t.Errorf("evidence_count = %d truncated = %v, want 6 and true", item.EvidenceCount, item.EvidenceTruncated)
	}
	// Authored connections lead, then derived ones, and within each the relation name decides.
	// That is the documented order, written out here rather than read from the service.
	for i := 1; i < len(reasons); i++ {
		prev, cur := reasons[i-1], reasons[i]
		switch {
		case prev.PriorityRank > cur.PriorityRank:
			t.Errorf("reason %d ranks below reason %d", i-1, i)
		case prev.PriorityRank == cur.PriorityRank && prev.Derived && !cur.Derived:
			t.Errorf("reason %d is derived and precedes the authored reason %d", i-1, i)
		case prev.PriorityRank == cur.PriorityRank && prev.Derived == cur.Derived && prev.Relation > cur.Relation:
			t.Errorf("reasons %d and %d are out of relation order: %q before %q",
				i-1, i, prev.Relation, cur.Relation)
		}
	}
	// The primary reason is never repeated below itself, and no connection appears twice.
	seen := map[string]bool{}
	for _, reason := range reasons {
		key := reason.Relation + "|" + reason.Origin + "|" + fmt.Sprint(reason.Derived)
		if seen[key] {
			t.Errorf("connection %s is reported twice on one item", key)
		}
		seen[key] = true
	}

	// A relationship scope that admits the class still cuts at the same cap and reports the same
	// complete count, so the bound is not something a filter can widen.
	scoped := mustRelated(t, k, domain.SearchNode, "left",
		service.RelatedQuery{RelationshipTypes: []string{string(domain.PriorityConceptual)}})
	for _, candidate := range scoped.Items {
		if candidate.ID != "right" {
			continue
		}
		if got := 1 + len(candidate.AdditionalEvidence); got != service.MaxRelatedEvidencePerItem {
			t.Errorf("scoped item carried %d reasons, want the cap of %d", got, service.MaxRelatedEvidencePerItem)
		}
		if candidate.EvidenceCount != item.EvidenceCount {
			t.Errorf("scoped evidence_count = %d, want the unscoped %d", candidate.EvidenceCount, item.EvidenceCount)
		}
	}
}

// --------------------------------------------------------------------------------------------
// C. The service contract on its own terms
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowServiceContractIsCompleteWithoutHTTP pins the inputs a direct caller can send
// and the HTTP layer cannot.
//
// The handler refuses a negative limit before the service sees one, and it always passes a slice
// it built from a split. That leaves several shapes reachable only from Go, and each of them has a
// documented meaning the service is supposed to hold on its own: an unusable limit falls back to
// the default rather than returning nothing, a nil filter is the same request as an absent one,
// and a member with surrounding whitespace is trimmed before it is compared. If any of these were
// enforced only at the handler, a future non-HTTP caller would inherit a different contract.
func TestRelatedWorkflowServiceContractIsCompleteWithoutHTTP(t *testing.T) {
	k := evidenceIndex(t)
	reference := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{})

	sameAsDefault := []struct {
		name  string
		query service.RelatedQuery
	}{
		{"zero-limit", service.RelatedQuery{Limit: 0}},
		{"negative-limit", service.RelatedQuery{Limit: -1}},
		{"very-negative-limit", service.RelatedQuery{Limit: -1 << 30}},
		{"nil-filters", service.RelatedQuery{EntityTypes: nil, RelationshipTypes: nil}},
		{"empty-filters", service.RelatedQuery{EntityTypes: []string{}, RelationshipTypes: []string{}}},
	}
	for _, tc := range sameAsDefault {
		t.Run(tc.name, func(t *testing.T) {
			result := mustRelated(t, k, domain.SearchNode, "alpha", tc.query)
			if result.Limit != service.DefaultRelatedLimit {
				t.Errorf("limit = %d, want the default %d", result.Limit, service.DefaultRelatedLimit)
			}
			if !reflect.DeepEqual(result, reference) {
				t.Errorf("result differs from the default request:\n want %+v\n  got %+v", reference, result)
			}
		})
	}

	// A limit past the ceiling is clamped rather than refused, and the applied value is what the
	// response echoes, so the clamp is visible to the caller who triggered it.
	clamped := mustRelated(t, k, domain.SearchNode, "alpha",
		service.RelatedQuery{Limit: service.MaxRelatedLimit + 1})
	if clamped.Limit != service.MaxRelatedLimit {
		t.Errorf("limit = %d, want the ceiling %d", clamped.Limit, service.MaxRelatedLimit)
	}

	// Members are trimmed and then compared exactly, on both filters, so a caller who padded a
	// list is answering the same question rather than a different one.
	padded := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{
		EntityTypes:       []string{"  node  ", "\tclaim"},
		RelationshipTypes: []string{" conceptual ", "contextual  "},
	})
	tight := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{
		EntityTypes:       []string{"node", "claim"},
		RelationshipTypes: []string{"conceptual", "contextual"},
	})
	if !reflect.DeepEqual(padded, tight) {
		t.Error("a whitespace-padded filter answered a different question than the trimmed one")
	}

	// Trimming never becomes repair: a member that is only whitespace, or one whose case differs,
	// is still refused rather than folded into a working filter.
	for _, query := range []service.RelatedQuery{
		{RelationshipTypes: []string{"conceptual", "   "}},
		{RelationshipTypes: []string{"Conceptual"}},
		{RelationshipTypes: []string{"conceptual", " conceptual"}},
		{EntityTypes: []string{"node", "\t"}},
		{EntityTypes: []string{"Node"}},
	} {
		if _, err := k.RelatedKnowledgeFor("node", "alpha", query); err == nil {
			t.Errorf("%+v was accepted, want a refusal", query)
		}
	}
}

// TestRelatedWorkflowValidationPrecedesResolution pins the order the request is checked in.
//
// A request that is both malformed and names a record the corpus does not hold must be reported as
// malformed, on every combination of the two filters. The two failures ask different things of the
// caller - fix the query string, or accept that the record is not here - and a caller told only
// about the identifier would correct it, resend, and be refused a second time for a mistake that
// was already visible in what they sent.
func TestRelatedWorkflowValidationPrecedesResolution(t *testing.T) {
	k := evidenceIndex(t)
	malformed := []service.RelatedQuery{
		{RelationshipTypes: []string{"structural"}},
		{RelationshipTypes: []string{""}},
		{RelationshipTypes: []string{"conceptual", "conceptual"}},
		{RelationshipTypes: []string{"unclassified"}},
		{EntityTypes: []string{"widget"}},
		{EntityTypes: []string{"experiment_run"}},
		{EntityTypes: []string{"node"}, RelationshipTypes: []string{"structural"}},
	}
	for _, query := range malformed {
		if _, err := k.RelatedKnowledgeFor("node", "no-such-node", query); err == nil {
			t.Errorf("%+v against a missing start was accepted", query)
		} else if strings.Contains(err.Error(), service.ErrNotFound.Error()) {
			t.Errorf("%+v against a missing start was reported as a miss: %v", query, err)
		}
	}

	// An unsupported start class is decided before either filter, because a request that does not
	// name a discovery class is not a discovery request at all.
	if _, err := k.RelatedKnowledgeFor("experiment_run", "no-such-run",
		service.RelatedQuery{RelationshipTypes: []string{"structural"}}); err == nil {
		t.Error("an unsupported start class with a malformed filter was accepted")
	}
}

// --------------------------------------------------------------------------------------------
// D. Mutation safety and concurrency
// --------------------------------------------------------------------------------------------

// relatedWorkflowStarts is every start the fixture resolves, paired with the queries used to sweep
// it. It is the service-level equivalent of the HTTP matrix and is kept small on purpose: the wide
// combinations are covered over the wire, and what is needed here is one sweep broad enough that a
// shared buffer or a mutated projection would be reached by it.
func relatedWorkflowSweep() []service.RelatedQuery {
	return []service.RelatedQuery{
		{},
		{Limit: 1},
		{Limit: service.MaxRelatedLimit},
		{EntityTypes: []string{"node"}},
		{EntityTypes: []string{"node", "claim"}},
		{RelationshipTypes: []string{"conceptual"}},
		{RelationshipTypes: []string{"evidential", "attributive"}},
		{RelationshipTypes: domain.RelatedPriorityNames()},
		{Limit: 2, EntityTypes: []string{"node", "vocabulary"}, RelationshipTypes: []string{"conceptual", "referential"}},
	}
}

// TestRelatedWorkflowResultsAreOwnedByTheCaller extends the aliasing check to every part of the
// response a caller can write to.
//
// The existing Phase 2A test overwrites an item's title and one evidence field. Phase 2B added two
// echo slices, and an echo built by slicing something the index holds would be exactly the kind of
// aliasing a response-shaped test misses: it looks like a value, it serialises like a value, and it
// is a window onto the ranking policy. Everything writable is therefore written to here, and the
// index is required to serve the original bytes afterwards.
func TestRelatedWorkflowResultsAreOwnedByTheCaller(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		for i, query := range relatedWorkflowSweep() {
			before := relatedWorkflowJSON(t, mustRelated(t, k, start.class, start.id, query))

			mutated := mustRelated(t, k, start.class, start.id, query)
			mutated.Start.Title = "overwritten"
			for j := range mutated.EntityTypes {
				mutated.EntityTypes[j] = "overwritten"
			}
			for j := range mutated.RelationshipTypes {
				mutated.RelationshipTypes[j] = "overwritten"
			}
			for j := range mutated.Items {
				mutated.Items[j].Title = "overwritten"
				mutated.Items[j].Summary = "overwritten"
				mutated.Items[j].Reason.Relation = "overwritten"
				mutated.Items[j].Reason.Origin = "overwritten"
				mutated.Items[j].Reason.Explanation = "overwritten"
				mutated.Items[j].EvidenceCount = -1
				for e := range mutated.Items[j].AdditionalEvidence {
					mutated.Items[j].AdditionalEvidence[e].Origin = "overwritten"
					mutated.Items[j].AdditionalEvidence[e].Explanation = "overwritten"
				}
			}

			if after := relatedWorkflowJSON(t, mustRelated(t, k, start.class, start.id, query)); after != before {
				t.Fatalf("%s/%s query %d: writing to a result changed what the index serves\n want %s\n  got %s",
					start.class, start.id, i, before, after)
			}
		}
	}
}

// TestRelatedWorkflowConcurrentReadersSeeOneCorpus sweeps every start from several goroutines.
//
// Discovery reads projections that search, context and traversal also read, and it takes no lock
// because nothing writes to them after startup. That is a property of the design rather than of
// any one phase, so it needs a test that would fail if a request ever sorted a slice it did not
// own or cached a filter in index state. Under -race this is also where such a write would be
// reported; without a race detector available the equality check below is what stands in for it.
func TestRelatedWorkflowConcurrentReadersSeeOneCorpus(t *testing.T) {
	k := evidenceIndex(t)
	sweep := relatedWorkflowSweep()

	type key struct {
		start int
		query int
	}
	expected := map[key]string{}
	for s, start := range everyStart {
		for q, query := range sweep {
			expected[key{s, q}] = relatedWorkflowJSON(t, mustRelated(t, k, start.class, start.id, query))
		}
	}

	const readers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	var mismatches []string
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s, start := range everyStart {
				for q, query := range sweep {
					result, err := k.RelatedKnowledgeFor(string(start.class), start.id, query)
					if err != nil {
						mu.Lock()
						mismatches = append(mismatches, fmt.Sprintf("%s/%s query %d: %v", start.class, start.id, q, err))
						mu.Unlock()
						continue
					}
					encoded, err := json.Marshal(result)
					if err != nil {
						mu.Lock()
						mismatches = append(mismatches, fmt.Sprintf("%s/%s query %d: marshal: %v", start.class, start.id, q, err))
						mu.Unlock()
						continue
					}
					if string(encoded) != expected[key{s, q}] {
						mu.Lock()
						mismatches = append(mismatches, fmt.Sprintf("%s/%s query %d differs under load", start.class, start.id, q))
						mu.Unlock()
					}
				}
			}
		}()
	}
	wg.Wait()
	for _, mismatch := range mismatches {
		t.Error(mismatch)
	}

	// The sweep is repeated sequentially afterwards, so a reader that left something behind is
	// caught even if every concurrent answer happened to be right.
	for s, start := range everyStart {
		for q, query := range sweep {
			if again := relatedWorkflowJSON(t, mustRelated(t, k, start.class, start.id, query)); again != expected[key{s, q}] {
				t.Errorf("%s/%s query %d changed after the concurrent sweep", start.class, start.id, q)
			}
		}
	}
}

// TestRelatedWorkflowLeavesTheOtherProjectionsAlone requires the projections discovery reads to be
// unchanged by having read them.
//
// Discovery does not own the context projection, the search index or the graph adjacency - it
// reads all three. A request that sorted one of those in place would produce a correct-looking
// discovery response and corrupt the next search, so the damage would appear in a suite that never
// mentions this phase. Snapshotting the neighbouring projections either side of a full sweep is
// what makes that visible here instead.
func TestRelatedWorkflowLeavesTheOtherProjectionsAlone(t *testing.T) {
	k := evidenceIndex(t)

	snapshot := func() []string {
		out := []string{
			relatedWorkflowValue(t, k.Graph()),
			relatedWorkflowValue(t, k.Project()),
			relatedWorkflowValue(t, relatedWorkflowSearch(t, k, service.SearchQuery{Q: "fixture", Limit: service.MaxLimit})),
			relatedWorkflowValue(t, relatedWorkflowSearch(t, k, service.SearchQuery{
				Q: "fixture", Limit: service.MaxLimit, IncludeContext: true})),
			relatedWorkflowValue(t, k.ListNodes(service.NodeQuery{Limit: service.MaxLimit})),
			relatedWorkflowValue(t, k.ListSessions(service.SessionQuery{Limit: service.MaxLimit})),
		}
		return out
	}

	before := snapshot()
	for _, start := range everyStart {
		for _, query := range relatedWorkflowSweep() {
			mustRelated(t, k, start.class, start.id, query)
		}
	}
	after := snapshot()
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("projection %d changed after the discovery sweep", i)
		}
	}
}

// relatedWorkflowSearch runs one search and fails the test on a refusal, so a snapshot line is a
// projection rather than an error value.
func relatedWorkflowSearch(t testing.TB, k *service.Knowledge, q service.SearchQuery) service.SearchResults {
	t.Helper()
	results, err := k.Search(q)
	if err != nil {
		t.Fatalf("search %+v: %v", q, err)
	}
	return results
}

func relatedWorkflowValue(t testing.TB, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	return string(encoded)
}

// --------------------------------------------------------------------------------------------
// E. Generated builder artifacts are outside the discovery path
// --------------------------------------------------------------------------------------------

// TestRelatedWorkflowIgnoresGeneratedIndexArtifacts requires discovery to be unchanged by the
// output of the repository's index builders.
//
// The canonical builders under tools/ write generated views - a claim index, a vocabulary index, an
// experiment and run index, and the knowledge and coverage indexes - and the backend deliberately
// never reads any of them, because they are rebuildable projections whose value as an independent
// cross-check depends on not being a second input. That is stated in the documentation and is
// invisible in the code, which simply never opens those paths, so the way to keep it true is a test
// that puts a builder-shaped artifact in front of the loader and requires the discovery answer not
// to move.
//
// The content is deliberately hostile to the property: each file names real fixture records and
// asserts connections between them that no canonical record authored. A loader that started reading
// generated views would find those and the sweep would change.
func TestRelatedWorkflowIgnoresGeneratedIndexArtifacts(t *testing.T) {
	plain := evidenceIndex(t)

	corpus := testsupport.MutableCorpus(t)
	generated := map[string]string{
		"indexes/nodes-by-domain.md":       "# Nodes by domain\n\n- `alpha` relates to `gamma`\n",
		"indexes/node-connections.md":      "# Node connections\n\n- `alpha` -> `beta` -> `gamma`\n",
		"indexes/relationships-by-type.md": "# Relationships by type\n\n- produces: `alpha` -> `gamma`\n",
		"indexes/session-coverage.md":      "# Session coverage\n\n- `session-02-unused` covers `alpha`\n",
		"indexes/source-coverage.md":       "# Source coverage\n\n- `fixture-uncited-source` supports `alpha`\n",
		"indexes/knowledge-coverage.md":    "# Knowledge coverage\n\n- `fixture-orphan-term` relates to `alpha`\n",
		"indexes/knowledge-coverage.json":  `{"nodes":[{"id":"alpha","vocabulary":{"ids":["fixture-orphan-term"]}}]}`,
		"claims/index.md":                  "# Claim index\n\n- `alpha-carries-energy` cites `fixture-uncited-source`\n",
		"vocabulary/index.md":              "# Vocabulary index\n\n- `fixture-orphan-term` -> `alpha`\n",
		"experiments/index.md":             "# Experiment index\n\n- `fixture-visualization-exercise` -> `alpha`\n",
		"experiment-runs/index.md":         "# Experiment run index\n\n- `fixture-listening-exercise-planned-a` -> `alpha`\n",
	}
	for path, content := range generated {
		testsupport.Write(corpus, path, content)
	}
	withArtifacts := indexFrom(t, corpus)

	for _, start := range everyStart {
		for i, query := range relatedWorkflowSweep() {
			want := relatedWorkflowJSON(t, mustRelated(t, plain, start.class, start.id, query))
			got := relatedWorkflowJSON(t, mustRelated(t, withArtifacts, start.class, start.id, query))
			if want != got {
				t.Errorf("%s/%s query %d changed once generated builder output was present\n want %s\n  got %s",
					start.class, start.id, i, want, got)
			}
		}
	}
}
