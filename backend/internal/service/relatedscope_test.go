package service_test

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// The Phase 2B relationship-scope and explainability tests.
//
// Phase 2A proved that discovery reads the corpus correctly, orders it by a policy and explains
// each item by the canonical field it came from. Phase 2B adds one filter over that model and one
// sentence to each explanation, and these tests assert the three things that addition must not
// break and the three it must provide.
//
// Must not break: an unfiltered request answers exactly as it did; the precedence order among the
// items that survive a filter is the order they always had; and every bound still binds.
//
// Must provide: the filter is closed and strictly validated, with no spelling of a malformed
// filter falling back to an unfiltered discovery; it restricts the eligible relations rather than
// post-filtering a page, so a caller receives up to their requested limit from the scope they
// asked about; and every result explains itself deterministically.

// relatedScopeOf runs one discovery under a relationship scope with the ceiling limit, so a test
// about filtering is never accidentally also a test about the default limit.
func relatedScopeOf(
	t testing.TB, k *service.Knowledge, class domain.SearchEntityType, id string, scope ...string,
) domain.RelatedKnowledge {
	t.Helper()
	return mustRelated(t, k, class, id, service.RelatedQuery{
		Limit:             service.MaxRelatedLimit,
		RelationshipTypes: scope,
	})
}

// relatedReasons flattens one item's reported connections: the winning reason first, then its
// bounded further evidence, which is the order the contract serialises them in.
func relatedReasons(item domain.RelatedItem) []domain.RelatedReason {
	out := make([]domain.RelatedReason, 0, 1+len(item.AdditionalEvidence))
	out = append(out, item.Reason)
	return append(out, item.AdditionalEvidence...)
}

// assertRelatedOrder checks the documented total order directly on a result.
//
// It is written out here rather than borrowed, because the comparator is unexported and a test
// that reused it could not tell an ordering policy from an ordering bug. The keys are the ones
// domain.RelatedKnowledge documents: the winning reason's precedence rank, then authored before
// derived, then the destination's canonical class in model order, then its canonical ID. The
// order is total, so no adjacent pair may compare equal on every key.
func assertRelatedOrder(t testing.TB, result domain.RelatedKnowledge, label string) {
	t.Helper()
	for i := 1; i < len(result.Items); i++ {
		prev, next := result.Items[i-1], result.Items[i]
		if prev.Reason.PriorityRank != next.Reason.PriorityRank {
			if prev.Reason.PriorityRank > next.Reason.PriorityRank {
				t.Errorf("%s: item %d (%s) outranks item %d (%s) but sorts after it",
					label, i, next.Reason.Priority, i-1, prev.Reason.Priority)
			}
			continue
		}
		if prev.Reason.Derived != next.Reason.Derived {
			if prev.Reason.Derived {
				t.Errorf("%s: a derived reason sorts before an authored one at item %d", label, i)
			}
			continue
		}
		prevRank := domain.SearchEntityRank(prev.EntityType)
		nextRank := domain.SearchEntityRank(next.EntityType)
		if prevRank != nextRank {
			if prevRank > nextRank {
				t.Errorf("%s: class %s sorts before %s at item %d",
					label, prev.EntityType, next.EntityType, i)
			}
			continue
		}
		if prev.ID >= next.ID {
			t.Errorf("%s: items %d and %d are not separated by canonical ID (%s, %s)",
				label, i-1, i, prev.ID, next.ID)
		}
	}
}

// expectedUnderScope rebuilds what a scoped discovery must return from an unfiltered one.
//
// It is the filtering contract written as data: keep the relations whose precedence class the
// scope admits, drop a destination that keeps none, and let the strongest survivor become the
// reason. Deriving the expectation from the unfiltered response rather than from a hand-written
// list is what makes the assertion a statement about the rule instead of about the fixture, and
// it is possible only because the fixture's evidence lists sit well under the per-item cap, so an
// unfiltered response reports every connection it has.
//
// The result is keyed by destination rather than ordered, because a filter may change which class
// wins for a destination and therefore where it sorts. Ordering is asserted separately, against
// the documented comparator, so a reordering bug and a filtering bug fail different tests.
func expectedUnderScope(
	t testing.TB, unfiltered domain.RelatedKnowledge, scope []string,
) map[string][]domain.RelatedReason {
	t.Helper()
	want := make(map[string][]domain.RelatedReason, len(unfiltered.Items))
	for _, item := range unfiltered.Items {
		if item.EvidenceTruncated {
			t.Fatalf("%s/%s reports truncated evidence, so the unfiltered response is not a "+
				"complete basis for this expectation", item.EntityType, item.ID)
		}
		kept := make([]domain.RelatedReason, 0, item.EvidenceCount)
		for _, reason := range relatedReasons(item) {
			if len(scope) == 0 || contains(scope, string(reason.Priority)) {
				kept = append(kept, reason)
			}
		}
		if len(kept) == 0 {
			continue
		}
		want[string(item.EntityType)+"/"+item.ID] = kept
	}
	return want
}

// assertScopedResult checks one scoped response against the rule above, end to end.
func assertScopedResult(t testing.TB, result domain.RelatedKnowledge, want map[string][]domain.RelatedReason, label string) {
	t.Helper()
	got := make(map[string][]domain.RelatedReason, len(result.Items))
	for _, item := range result.Items {
		ref := string(item.EntityType) + "/" + item.ID
		if _, twice := got[ref]; twice {
			t.Errorf("%s: destination %s appeared more than once", label, ref)
		}
		got[ref] = relatedReasons(item)
		if item.EvidenceCount != len(got[ref]) {
			t.Errorf("%s: %s reports evidence_count %d for %d carried reasons",
				label, ref, item.EvidenceCount, len(got[ref]))
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: scoped result is not the unfiltered one restricted\n got %v\nwant %v",
			label, sortedRefs(got), sortedRefs(want))
	}
	if result.Counts.Eligible != len(want) {
		t.Errorf("%s: eligible = %d, want %d", label, result.Counts.Eligible, len(want))
	}
	assertRelatedOrder(t, result, label)
}

// sortedRefs renders a keyed expectation for a failure message in a stable order.
func sortedRefs(byRef map[string][]domain.RelatedReason) []string {
	out := make([]string, 0, len(byRef))
	for ref, reasons := range byRef {
		classes := make([]string, 0, len(reasons))
		for _, reason := range reasons {
			classes = append(classes, string(reason.Priority))
		}
		out = append(out, ref+" ["+strings.Join(classes, " ")+"]")
	}
	sort.Strings(out)
	return out
}

// --------------------------------------------------------------------------------------------
// A. Filter validation
// --------------------------------------------------------------------------------------------

// TestRelatedRelationshipScopeAcceptsEveryDeclaredClass walks the closed allowlist.
//
// Every class the ranking model declares must be a scope a caller may ask for, and the echo must
// report it normalised. A class added to the model without becoming filterable fails here, which
// is the same guard TestEveryRelatedPriorityOriginIsClassified provides one layer down.
func TestRelatedRelationshipScopeAcceptsEveryDeclaredClass(t *testing.T) {
	k := evidenceIndex(t)

	for _, class := range domain.RelatedPriorityNames() {
		t.Run(class, func(t *testing.T) {
			for _, start := range everyStart {
				result := relatedScopeOf(t, k, start.class, start.id, class)
				if !reflect.DeepEqual(result.RelationshipTypes, []string{class}) {
					t.Fatalf("%s/%s: relationship_types = %v, want [%s]",
						start.class, start.id, result.RelationshipTypes, class)
				}
				for _, item := range result.Items {
					for _, reason := range relatedReasons(item) {
						if string(reason.Priority) != class {
							t.Errorf("%s/%s: item %s/%s carries a %s connection under scope %s",
								start.class, start.id, item.EntityType, item.ID, reason.Priority, class)
						}
					}
				}
			}
		})
	}
}

// TestRelatedKnowledgeRejectsMalformedRelationshipScope is the strict-validation assertion.
//
// A discovery whose relationship scope silently differed from the one the caller wrote would
// return a list, an eligible count and a set of explanations that do not mean what they think
// they mean. Each malformed spelling is refused with the error that names what was wrong, and the
// unsupported values are refused with an InvalidFilterError naming this parameter and listing the
// declared classes rather than echoing the caller's own value.
func TestRelatedKnowledgeRejectsMalformedRelationshipScope(t *testing.T) {
	k := evidenceIndex(t)

	typed := map[string]struct {
		scope []string
		want  error
	}{
		"blank member":        {[]string{"conceptual", ""}, service.ErrEmptyRelatedPriority},
		"leading separator":   {[]string{"", "conceptual"}, service.ErrEmptyRelatedPriority},
		"whitespace member":   {[]string{"conceptual", "   "}, service.ErrEmptyRelatedPriority},
		"only whitespace":     {[]string{"\t"}, service.ErrEmptyRelatedPriority},
		"repeated class":      {[]string{"conceptual", "conceptual"}, service.ErrDuplicateRelatedPriority},
		"repeated after trim": {[]string{"conceptual", " conceptual "}, service.ErrDuplicateRelatedPriority},
		"repeated out of order": {
			[]string{"referential", "conceptual", "referential"},
			service.ErrDuplicateRelatedPriority,
		},
	}
	for name, tc := range typed {
		t.Run(name, func(t *testing.T) {
			_, err := k.RelatedKnowledgeFor("node", "alpha",
				service.RelatedQuery{RelationshipTypes: tc.scope})
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	// Unknown, mis-cased, invented and internal-only values are all one refusal. unclassified is
	// the case that matters most: it is a real member of the model's own type and is deliberately
	// not a scope, because a canonical field the model does not name is a gap rather than a thing
	// to filter by.
	for _, scope := range [][]string{
		{"unclassified"}, {"Conceptual"}, {"CONCEPTUAL"}, {"conceptuals"},
		{"widget"}, {"node.relationships"}, {"produces"}, {"explicit_related_node"},
		{"conceptual", "widget"}, {"widget", "conceptual"}, {"conceptual", "unclassified"},
	} {
		var invalid *service.InvalidFilterError
		_, err := k.RelatedKnowledgeFor("node", "alpha", service.RelatedQuery{RelationshipTypes: scope})
		if !errors.As(err, &invalid) {
			t.Errorf("scope %v: err = %v, want an InvalidFilterError", scope, err)
			continue
		}
		if invalid.Param != "relationship_types" {
			t.Errorf("scope %v: param = %q, want relationship_types", scope, invalid.Param)
		}
		if !reflect.DeepEqual(invalid.Allowed, domain.RelatedPriorityNames()) {
			t.Errorf("scope %v: allowed = %v, want the declared precedence classes",
				scope, invalid.Allowed)
		}
	}
}

// TestRelatedMalformedRelationshipScopeNeverFallsBack is the no-silent-fallback assertion.
//
// A refused filter must produce nothing at all. Returning the unfiltered discovery alongside an
// error would let a caller who checked the payload before the error read an unrestricted result
// as a restricted one, which is the failure the whole strict-validation contract exists to
// prevent.
func TestRelatedMalformedRelationshipScopeNeverFallsBack(t *testing.T) {
	k := evidenceIndex(t)
	unfiltered := relatedScopeOf(t, k, domain.SearchNode, "alpha")
	if len(unfiltered.Items) == 0 {
		t.Fatal("the fixture start returned nothing, so a fallback would be invisible here")
	}

	for _, scope := range [][]string{
		{""}, {"   "}, {"widget"}, {"Conceptual"}, {"unclassified"},
		{"conceptual", "conceptual"}, {"conceptual", "widget"},
	} {
		result, err := k.RelatedKnowledgeFor("node", "alpha", service.RelatedQuery{RelationshipTypes: scope})
		if err == nil {
			t.Errorf("scope %v was accepted", scope)
			continue
		}
		if !reflect.DeepEqual(result, domain.RelatedKnowledge{}) {
			t.Errorf("scope %v returned %d items alongside its error", scope, len(result.Items))
		}
	}
}

// TestRelatedKnowledgeRefusesAMalformedRelationshipScopeBeforeResolvingTheStart pins where the new
// filter sits in the validation order.
//
// It is validated with the rest of the request and before the start is looked up, exactly as the
// destination scope is, so a caller who sent both a mistyped identifier and a malformed
// relationship list is told about the list. The complements are asserted with it: a well-formed
// scope over a missing start is still a miss, and an unsupported start class still outranks both,
// because a request that does not name a discovery class is not a discovery request.
func TestRelatedKnowledgeRefusesAMalformedRelationshipScopeBeforeResolvingTheStart(t *testing.T) {
	k := evidenceIndex(t)

	for name, tc := range map[string]struct {
		scope []string
		want  error
	}{
		"blank member":   {[]string{"conceptual", ""}, service.ErrEmptyRelatedPriority},
		"repeated class": {[]string{"conceptual", "conceptual"}, service.ErrDuplicateRelatedPriority},
	} {
		t.Run(name, func(t *testing.T) {
			for _, class := range domain.SearchEntityTypeNames() {
				_, err := k.RelatedKnowledgeFor(class, "no-such-record",
					service.RelatedQuery{RelationshipTypes: tc.scope})
				if !errors.Is(err, tc.want) {
					t.Errorf("%s/no-such-record: err = %v, want %v", class, err, tc.want)
				}
			}
			if _, err := k.RelatedKnowledgeFor("node", "alpha",
				service.RelatedQuery{RelationshipTypes: tc.scope}); !errors.Is(err, tc.want) {
				t.Errorf("node/alpha: err = %v, want %v", err, tc.want)
			}
		})
	}

	var invalid *service.InvalidFilterError
	_, err := k.RelatedKnowledgeFor("node", "no-such-record",
		service.RelatedQuery{RelationshipTypes: []string{"widget"}})
	if !errors.As(err, &invalid) || invalid.Param != "relationship_types" {
		t.Errorf("err = %v, want an InvalidFilterError naming relationship_types", err)
	}

	for _, scope := range [][]string{nil, {"conceptual"}, {"conceptual", "evidential"}} {
		if _, err := k.RelatedKnowledgeFor("node", "no-such-record",
			service.RelatedQuery{RelationshipTypes: scope}); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("scope %v: err = %v, want ErrNotFound", scope, err)
		}
	}
	if _, err := k.RelatedKnowledgeFor("experiment_run", "no-such-run",
		service.RelatedQuery{RelationshipTypes: []string{"widget"}}); !errors.Is(err, service.ErrUnsupportedRelatedEntityType) {
		t.Errorf("err = %v, want ErrUnsupportedRelatedEntityType", err)
	}

	// The destination scope is validated first, so a request carrying two malformed lists reports
	// the same one on every run. That is a determinism property of the error, not a claim that
	// one mistake matters more than the other.
	_, err = k.RelatedKnowledgeFor("node", "alpha", service.RelatedQuery{
		EntityTypes:       []string{"node", "node"},
		RelationshipTypes: []string{"conceptual", "conceptual"},
	})
	if !errors.Is(err, service.ErrDuplicateSearchEntityType) {
		t.Errorf("err = %v, want the destination-scope error to be reported first", err)
	}
}

// --------------------------------------------------------------------------------------------
// B. Filtering
// --------------------------------------------------------------------------------------------

// TestRelatedUnfilteredDiscoveryIsUnchanged is the Phase 2A compatibility assertion.
//
// A request that names no relationship scope must return what it always returned: the same items,
// the same order, the same reasons, the same evidence and the same counts, with no echo of a
// filter nobody applied. The only permitted difference is the additive explanation field, which is
// checked separately.
//
// The equivalence is asserted against a scope naming every declared class rather than against a
// stored expectation, which makes it a statement about the filter: admitting everything must be
// indistinguishable from filtering nothing, item for item, or the filter is doing something other
// than restricting.
func TestRelatedUnfilteredDiscoveryIsUnchanged(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		unfiltered := relatedScopeOf(t, k, start.class, start.id)
		if unfiltered.RelationshipTypes != nil {
			t.Errorf("%s/%s: an unfiltered discovery echoed relationship_types = %v",
				start.class, start.id, unfiltered.RelationshipTypes)
		}

		everything := relatedScopeOf(t, k, start.class, start.id, domain.RelatedPriorityNames()...)
		if !reflect.DeepEqual(everything.Items, unfiltered.Items) {
			t.Errorf("%s/%s: naming every class changed the result\n%v\nwant\n%v",
				start.class, start.id, relatedLines(everything), relatedLines(unfiltered))
		}
		if everything.Counts != unfiltered.Counts || everything.Bounds != unfiltered.Bounds {
			t.Errorf("%s/%s: counts/bounds = %+v %+v, want %+v %+v", start.class, start.id,
				everything.Counts, everything.Bounds, unfiltered.Counts, unfiltered.Bounds)
		}
		// The one documented difference: the all-classes request echoes what it pinned.
		if !reflect.DeepEqual(everything.RelationshipTypes, domain.RelatedPriorityNames()) {
			t.Errorf("%s/%s: relationship_types = %v, want every declared class",
				start.class, start.id, everything.RelationshipTypes)
		}
	}
}

// TestRelatedRelationshipScopeIsTheUnfilteredResultRestricted is the central filtering assertion.
//
// For every start and a spread of scopes, the scoped result must be exactly the unfiltered one
// with the ineligible relations removed: the same destinations minus those that keep nothing, each
// explained by its strongest surviving connection and carrying only surviving connections as
// evidence, counted over the restricted set and ordered by the unchanged precedence policy.
//
// Deriving the expectation from the unfiltered response is what makes this a test of the rule
// rather than of the fixture. It also covers the case a post-filter over finished items would get
// wrong: a destination whose strongest connection is excluded but which a weaker admitted
// connection still reaches must remain, explained by that weaker connection.
func TestRelatedRelationshipScopeIsTheUnfilteredResultRestricted(t *testing.T) {
	k := evidenceIndex(t)

	scopes := [][]string{
		{"conceptual"}, {"evidential"}, {"attributive"}, {"assertional"},
		{"contextual"}, {"referential"}, {"navigational"},
		{"conceptual", "referential"},
		{"evidential", "attributive"},
		{"assertional", "contextual", "navigational"},
	}
	for _, start := range everyStart {
		unfiltered := relatedScopeOf(t, k, start.class, start.id)
		for _, scope := range scopes {
			label := fmt.Sprintf("%s/%s %s", start.class, start.id, strings.Join(scope, "+"))
			result := relatedScopeOf(t, k, start.class, start.id, scope...)
			assertScopedResult(t, result, expectedUnderScope(t, unfiltered, scope), label)
		}
	}
}

// TestRelatedRelationshipScopeKeepsADestinationItsWeakerConnectionStillReaches is the case above,
// isolated so it fails on its own terms.
//
// node/gamma reaches claim/alpha-may-extend-to-gamma through two assertional connections and
// nothing else, while claim/gamma-follows-from-alpha-and-beta is reached by one. Under an
// assertional scope both must survive with their evidence intact; under a conceptual scope neither
// may appear, because the filter removes the relations rather than reranking them.
func TestRelatedRelationshipScopeKeepsADestinationItsWeakerConnectionStillReaches(t *testing.T) {
	k := evidenceIndex(t)

	// claim/beta-was-observed-in-1999 reaches node/beta assertionally and reaches two sources
	// evidentially and one attributively, so restricting to the weakest of those three classes
	// must still return the record that class reaches.
	attributive := relatedScopeOf(t, k, domain.SearchClaim, "beta-was-observed-in-1999", "attributive")
	if got := relatedRefs(attributive); !reflect.DeepEqual(got, []string{"source/fixture-attribution-source"}) {
		t.Errorf("attributive scope = %v, want only the attributed source", got)
	}
	if len(attributive.Items) == 1 && attributive.Items[0].Reason.Priority != domain.PriorityAttributive {
		t.Errorf("the surviving item is explained as %s", attributive.Items[0].Reason.Priority)
	}

	// A destination reached twice in one class keeps both connections; the same destination under
	// a scope that admits neither disappears rather than appearing with an empty explanation.
	assertional := relatedScopeOf(t, k, domain.SearchNode, "gamma", "assertional")
	found := false
	for _, item := range assertional.Items {
		if item.EntityType != domain.SearchClaim || item.ID != "alpha-may-extend-to-gamma" {
			continue
		}
		found = true
		if item.EvidenceCount != 2 || len(item.AdditionalEvidence) != 1 {
			t.Errorf("evidence_count = %d additional = %d, want 2 and 1",
				item.EvidenceCount, len(item.AdditionalEvidence))
		}
	}
	if !found {
		t.Errorf("claim/alpha-may-extend-to-gamma is missing under an assertional scope: %v",
			relatedRefs(assertional))
	}
	conceptual := relatedScopeOf(t, k, domain.SearchNode, "gamma", "conceptual")
	for _, item := range conceptual.Items {
		if item.EntityType == domain.SearchClaim {
			t.Errorf("a conceptual scope returned claim/%s", item.ID)
		}
	}
}

// TestRelatedMultiClassScopeIsTheUnionOfItsParts asserts a multi-class filter composes rather than
// intersects, which is what a scope over a closed vocabulary means everywhere else on this API.
func TestRelatedMultiClassScopeIsTheUnionOfItsParts(t *testing.T) {
	k := evidenceIndex(t)
	classes := domain.RelatedPriorityNames()

	for _, start := range everyStart {
		for i := 0; i < len(classes); i++ {
			for j := i + 1; j < len(classes); j++ {
				pair := relatedScopeOf(t, k, start.class, start.id, classes[i], classes[j])
				union := make(map[string]bool)
				for _, single := range []string{classes[i], classes[j]} {
					for _, ref := range relatedRefs(relatedScopeOf(t, k, start.class, start.id, single)) {
						union[ref] = true
					}
				}
				got := relatedRefs(pair)
				if len(got) != len(union) {
					t.Errorf("%s/%s %s+%s: %v, want the union of %d destinations",
						start.class, start.id, classes[i], classes[j], got, len(union))
					continue
				}
				for _, ref := range got {
					if !union[ref] {
						t.Errorf("%s/%s %s+%s: %s is in neither single-class result",
							start.class, start.id, classes[i], classes[j], ref)
					}
				}
			}
		}
	}
}

// TestRelatedRelationshipScopeFiltersBeforeTheLimit is the stage-order assertion.
//
// A caller asking for one relationship class and n items must receive up to n items of that class,
// not whatever survives of the first n items of the unfiltered ranking. node/alpha is the case:
// its two strongest items are conceptual, so a referential scope with a limit of two returns
// nothing at all if the filter runs after the cut and two referential records if it runs before.
func TestRelatedRelationshipScopeFiltersBeforeTheLimit(t *testing.T) {
	k := evidenceIndex(t)

	head := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: 2})
	for _, item := range head.Items {
		if item.Reason.Priority != domain.PriorityConceptual {
			t.Fatalf("the fixture no longer leads with conceptual items: %v", relatedLines(head))
		}
	}

	scoped := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{
		Limit:             2,
		RelationshipTypes: []string{"referential"},
	})
	if len(scoped.Items) != 2 {
		t.Fatalf("items = %v, want two referential records", relatedRefs(scoped))
	}
	for _, item := range scoped.Items {
		if item.Reason.Priority != domain.PriorityReferential {
			t.Errorf("%s/%s is explained as %s", item.EntityType, item.ID, item.Reason.Priority)
		}
	}
	// The eligible total describes the filtered set before the cut, so the caller can tell a
	// complete short list from the front of a longer one without a second request.
	full := relatedScopeOf(t, k, domain.SearchNode, "alpha", "referential")
	if scoped.Counts.Eligible != len(full.Items) || !scoped.Truncated == (scoped.Counts.Eligible > 2) {
		t.Errorf("counts = %+v truncated = %v against %d eligible referential records",
			scoped.Counts, scoped.Truncated, len(full.Items))
	}
}

// TestRelatedRelationshipScopeCombinesWithTheDestinationScope asserts the two filters restrict
// independently and compose, rather than one overriding the other.
func TestRelatedRelationshipScopeCombinesWithTheDestinationScope(t *testing.T) {
	k := evidenceIndex(t)

	result := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{
		Limit:             service.MaxRelatedLimit,
		EntityTypes:       []string{"source"},
		RelationshipTypes: []string{"contextual"},
	})
	if !reflect.DeepEqual(result.EntityTypes, []string{"source"}) {
		t.Errorf("entity_types = %v", result.EntityTypes)
	}
	if !reflect.DeepEqual(result.RelationshipTypes, []string{"contextual"}) {
		t.Errorf("relationship_types = %v", result.RelationshipTypes)
	}
	if len(result.Items) == 0 {
		t.Fatal("the combined scope returned nothing, so it proves neither filter")
	}
	for _, item := range result.Items {
		if item.EntityType != domain.SearchSource {
			t.Errorf("%s/%s is outside the destination scope", item.EntityType, item.ID)
		}
		for _, reason := range relatedReasons(item) {
			if reason.Priority != domain.PriorityContextual {
				t.Errorf("%s/%s carries a %s connection", item.EntityType, item.ID, reason.Priority)
			}
		}
	}
	// A combination that admits nothing is still a question with the answer "none".
	empty := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{
		EntityTypes:       []string{"source"},
		RelationshipTypes: []string{"conceptual"},
	})
	if len(empty.Items) != 0 || empty.Counts.Eligible != 0 {
		t.Errorf("items = %v eligible = %d, want an empty result",
			relatedRefs(empty), empty.Counts.Eligible)
	}
}

// TestRelatedRelationshipScopeThatAdmitsNothingIsSuccess asserts a valid filter matching nothing is
// an answer rather than an error, and echoes the scope that produced it.
//
// The distinction matters: an error would tell a caller their request was wrong, when what the
// corpus is actually saying is that this record has no connection of that kind.
func TestRelatedRelationshipScopeThatAdmitsNothingIsSuccess(t *testing.T) {
	k := evidenceIndex(t)

	// A source reached only through a claim's attribution list has no conceptual connection at
	// all, because only nodes author typed concept edges.
	result := relatedScopeOf(t, k, domain.SearchSource, "fixture-attribution-source", "conceptual")
	if result.Items == nil {
		t.Error("an empty scoped discovery returned a null item list rather than an empty one")
	}
	if len(result.Items) != 0 || result.Counts != (domain.RelatedCounts{}) {
		t.Errorf("items = %v counts = %+v, want an empty result", relatedRefs(result), result.Counts)
	}
	if result.Truncated {
		t.Error("an empty scoped result reported truncation")
	}
	if !reflect.DeepEqual(result.RelationshipTypes, []string{"conceptual"}) {
		t.Errorf("relationship_types = %v, want the scope that was applied", result.RelationshipTypes)
	}
	if result.Start.ID != "fixture-attribution-source" || result.Start.Title == "" {
		t.Errorf("start = %+v, want the resolved record", result.Start)
	}
	// The scan still ran and still reports what it examined, so an empty answer is visibly an
	// answer about a record with context rather than a record with none.
	if result.Bounds.RelationsScanned == 0 {
		t.Error("an empty scoped result reported that nothing was scanned")
	}
}

// TestRelatedRelationshipScopeIntroducesNoInferredConnection is the no-fabrication assertion at the
// filtering layer.
//
// Every connection a scoped response reports must be one the Phase 1F context projection
// independently holds, with the same relation, canonical field and direction. A filter can only
// ever remove; if one of these responses carried a connection the context layer does not have,
// the filter would have invented a relationship rather than selected one.
func TestRelatedRelationshipScopeIntroducesNoInferredConnection(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		known := make(map[string]bool)
		for _, relation := range contextOf(t, k, start.class, start.id).Related {
			known[fmt.Sprintf("%s/%s|%s|%s|%v", relation.Entity.EntityType, relation.Entity.ID,
				relation.Relation, relation.Origin, relation.Derived)] = true
		}
		for _, class := range domain.RelatedPriorityNames() {
			result := relatedScopeOf(t, k, start.class, start.id, class)
			for _, item := range result.Items {
				for _, reason := range relatedReasons(item) {
					key := fmt.Sprintf("%s/%s|%s|%s|%v", item.EntityType, item.ID,
						reason.Relation, reason.Origin, reason.Derived)
					if !known[key] {
						t.Errorf("%s/%s scope %s: %s is not a canonical context relation",
							start.class, start.id, class, key)
					}
					if domain.RelatedPriorityFor(reason.Origin) != reason.Priority {
						t.Errorf("%s/%s: %s claims class %s, but its canonical field classifies as %s",
							start.class, start.id, key, reason.Priority,
							domain.RelatedPriorityFor(reason.Origin))
					}
				}
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// C. Determinism and ranking
// --------------------------------------------------------------------------------------------

// TestRelatedRelationshipScopeIsOrderIndependent asserts the filter is a set rather than a
// sequence.
//
// Every permutation of one scope must produce one byte-identical response, including the echo,
// because the scope is normalised to precedence order before anything reads it. A caller writing
// their classes in a different order is asking the same question.
func TestRelatedRelationshipScopeIsOrderIndependent(t *testing.T) {
	k := evidenceIndex(t)

	for _, scope := range [][]string{
		{"conceptual", "evidential", "referential"},
		{"navigational", "assertional"},
		domain.RelatedPriorityNames(),
	} {
		want := relatedScopeOf(t, k, domain.SearchNode, "alpha", scope...)
		for _, permuted := range permutations(scope) {
			got := relatedScopeOf(t, k, domain.SearchNode, "alpha", permuted...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("scope %v produced a different response than %v", permuted, scope)
			}
		}
		// The echo is the normalised scope in precedence order, never the caller's spelling.
		if !sortedByPrecedence(want.RelationshipTypes) {
			t.Errorf("relationship_types = %v, want precedence order", want.RelationshipTypes)
		}
	}
}

// TestRelatedScopedDiscoveryIsStableAcrossRepeatedCalls is the reproducibility assertion for the
// filtered path, matching the Phase 2A one for the unfiltered path.
func TestRelatedScopedDiscoveryIsStableAcrossRepeatedCalls(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		for _, class := range domain.RelatedPriorityNames() {
			first := relatedScopeOf(t, k, start.class, start.id, class)
			for run := 0; run < 10; run++ {
				if again := relatedScopeOf(t, k, start.class, start.id, class); !reflect.DeepEqual(again, first) {
					t.Fatalf("%s/%s scope %s changed on run %d", start.class, start.id, class, run)
				}
			}
		}
	}
}

// TestRelatedScopedDiscoveryIsIndependentOfCorpusEnumerationOrder is the stronger determinism
// assertion: two indexes built from two different filesystem implementations must agree under a
// filter as they do without one.
func TestRelatedScopedDiscoveryIsIndependentOfCorpusEnumerationOrder(t *testing.T) {
	fromDisk := evidenceIndex(t)
	fromMemory := indexFrom(t, testsupport.MutableCorpus(t))

	for _, start := range everyStart {
		for _, class := range domain.RelatedPriorityNames() {
			disk := relatedScopeOf(t, fromDisk, start.class, start.id, class)
			memory := relatedScopeOf(t, fromMemory, start.class, start.id, class)
			if !reflect.DeepEqual(disk, memory) {
				t.Errorf("%s/%s scope %s differs between indexes:\n%v\n%v",
					start.class, start.id, class, relatedLines(disk), relatedLines(memory))
			}
		}
	}
}

// TestRelatedRelationshipScopeDoesNotChangePrecedence asserts the filter cannot promote a class.
//
// Naming a class does not raise it: within any scope, an item explained by a stronger class must
// still sort above one explained by a weaker class, in the order the model declares and never in
// the order the caller wrote. This is the assertion that keeps relationship_types a filter rather
// than a ranking parameter.
func TestRelatedRelationshipScopeDoesNotChangePrecedence(t *testing.T) {
	k := evidenceIndex(t)
	classes := domain.RelatedPriorityNames()

	for _, start := range everyStart {
		for i := 0; i < len(classes); i++ {
			for j := i + 1; j < len(classes); j++ {
				strong, weak := classes[i], classes[j]
				// Written weakest-first, which is the spelling that would reorder the result if
				// the caller's sequence meant anything.
				result := relatedScopeOf(t, k, start.class, start.id, weak, strong)
				assertRelatedOrder(t, result, fmt.Sprintf("%s/%s %s+%s", start.class, start.id, weak, strong))

				seenWeak := false
				for _, item := range result.Items {
					switch string(item.Reason.Priority) {
					case weak:
						seenWeak = true
					case strong:
						if seenWeak {
							t.Errorf("%s/%s: %s item %s/%s sorts after a %s item",
								start.class, start.id, strong, item.EntityType, item.ID, weak)
						}
					}
				}
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// D. Explainability
// --------------------------------------------------------------------------------------------

// TestEveryRelatedResultExplainsItself is the completeness assertion for the Phase 2B explanation.
//
// Every reason a response carries — the winning one and every piece of further evidence, filtered
// or not — must expose the normalised class, its declared precedence rank, the canonical field it
// was read from and the fixed sentence that class renders. A response with a ranked item that
// cannot account for itself is what the phase exists to prevent.
func TestEveryRelatedResultExplainsItself(t *testing.T) {
	k := evidenceIndex(t)

	checked := 0
	for _, start := range everyStart {
		for _, scope := range append([][]string{nil}, singleClassScopes()...) {
			result := relatedScopeOf(t, k, start.class, start.id, scope...)
			for _, item := range result.Items {
				for _, reason := range relatedReasons(item) {
					checked++
					if reason.Priority == "" || reason.Origin == "" || reason.Relation == "" {
						t.Errorf("%s/%s: incomplete reason %+v", item.EntityType, item.ID, reason)
					}
					if reason.Priority == domain.PriorityUnclassified {
						t.Errorf("%s/%s is explained by an unclassified field %q",
							item.EntityType, item.ID, reason.Origin)
					}
					if want := domain.RelatedPriorityRank(reason.Priority); reason.PriorityRank != want {
						t.Errorf("%s/%s: rank %d for class %s, want %d",
							item.EntityType, item.ID, reason.PriorityRank, reason.Priority, want)
					}
					if want := domain.RelatedPriorityExplanation(reason.Priority); reason.Explanation != want {
						t.Errorf("%s/%s: explanation %q for class %s, want %q",
							item.EntityType, item.ID, reason.Explanation, reason.Priority, want)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no reason was inspected, so this test would pass vacuously")
	}
}

// TestRelatedExplanationsAreStableAndLeakNothing is the determinism and disclosure assertion on
// the explanation text as it appears in a real response.
//
// The sentence must be one of the fixed strings the model declares — never a record's own words,
// never a local path, never a filesystem detail — and repeated identical requests must produce it
// identically. The corpus-level check complements the table-level one in the domain package: this
// one runs against text that has travelled through the projection.
func TestRelatedExplanationsAreStableAndLeakNothing(t *testing.T) {
	k := evidenceIndex(t)

	allowed := make(map[string]bool, len(domain.RelatedPriorities))
	for _, p := range domain.RelatedPriorities {
		allowed[domain.RelatedPriorityExplanation(p)] = true
	}

	for _, start := range everyStart {
		first := relatedScopeOf(t, k, start.class, start.id)
		for run := 0; run < 5; run++ {
			again := relatedScopeOf(t, k, start.class, start.id)
			for i := range first.Items {
				if first.Items[i].Reason.Explanation != again.Items[i].Reason.Explanation {
					t.Fatalf("%s/%s item %d explained itself two ways", start.class, start.id, i)
				}
			}
		}
		for _, item := range first.Items {
			for _, reason := range relatedReasons(item) {
				if !allowed[reason.Explanation] {
					t.Errorf("%s/%s: %q is not one of the declared explanations",
						item.EntityType, item.ID, reason.Explanation)
				}
				for _, leak := range []string{
					testsupport.CorpusRoot(t), "C:\\", "/home/", "\\", "://",
					item.Title, string(item.EntityType) + "/" + item.ID,
				} {
					if leak != "" && strings.Contains(reason.Explanation, leak) {
						t.Errorf("%s/%s: explanation %q leaks %q",
							item.EntityType, item.ID, reason.Explanation, leak)
					}
				}
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// E. Bounds
// --------------------------------------------------------------------------------------------

// TestRelatedScopeCannotExpandTheRelationScan is the safety assertion for the new parameter.
//
// A filter restricts what an answer contains and must never enlarge the work behind it. The scan
// is counted before any exclusion, so the number of relations examined must be identical whether
// a request names no class, one class or every class — and a scope that admits almost nothing must
// not be able to walk further in exchange.
func TestRelatedScopeCannotExpandTheRelationScan(t *testing.T) {
	const spokes = service.MaxRelatedRelationsScanned + 1
	k := indexFrom(t, wideRelatedCorpus(t, spokes))

	base := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: service.MaxRelatedLimit})
	if !base.Bounds.RelationsTruncated {
		t.Fatal("the wide fixture no longer crosses the scan ceiling")
	}

	for _, scope := range append([][]string{nil, domain.RelatedPriorityNames()}, singleClassScopes()...) {
		result := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{
			Limit:             service.MaxRelatedLimit,
			RelationshipTypes: scope,
		})
		if result.Bounds.RelationsScanned != base.Bounds.RelationsScanned {
			t.Errorf("scope %v scanned %d relations, want the unfiltered %d",
				scope, result.Bounds.RelationsScanned, base.Bounds.RelationsScanned)
		}
		if result.Bounds.RelationsScanned > service.MaxRelatedRelationsScanned {
			t.Errorf("scope %v scanned past the ceiling", scope)
		}
		if result.Bounds.MaxRelationsScanned != service.MaxRelatedRelationsScanned ||
			result.Bounds.MaxEvidencePerItem != service.MaxRelatedEvidencePerItem {
			t.Errorf("scope %v: applied bounds = %+v, want the declared constants", scope, result.Bounds)
		}
		if len(result.Items) > service.MaxRelatedLimit {
			t.Errorf("scope %v returned %d items, past the ceiling", scope, len(result.Items))
		}
	}
}

// TestRelatedScopedEvidenceIsStillCapped asserts the per-item evidence cap binds under a filter,
// and that a filtered item's truncation flag and count describe the filtered evidence.
//
// The twinned corpus connects one pair of nodes through six conceptual relations, so a conceptual
// scope must report the same capped explanation the unfiltered request does, while a scope that
// admits none of them must drop the item rather than report it with an empty explanation.
func TestRelatedScopedEvidenceIsStillCapped(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	edges := func(target string) string {
		out := make([]string, 0, 3)
		for _, kind := range []string{"produces", "characterized_by", "processes"} {
			out = append(out, fmt.Sprintf(`{"target": %q, "type": %q}`, target, kind))
		}
		return "[" + strings.Join(out, ", ") + "]"
	}
	testsupport.Write(corpus, "nodes/dsp/twinned-left.md",
		testsupport.ValidNode("twinned-left", "Twinned Left", "dsp", "seed", edges("twinned-right"), "[]"))
	testsupport.Write(corpus, "nodes/dsp/twinned-right.md",
		testsupport.ValidNode("twinned-right", "Twinned Right", "dsp", "seed", edges("twinned-left"), "[]"))
	k := indexFrom(t, corpus)

	scoped := relatedScopeOf(t, k, domain.SearchNode, "twinned-left", "conceptual")
	if len(scoped.Items) == 0 || scoped.Items[0].ID != "twinned-right" {
		t.Fatalf("twinned-right is not the first item: %v", relatedRefs(scoped))
	}
	item := scoped.Items[0]
	if item.EvidenceCount != 6 || !item.EvidenceTruncated {
		t.Errorf("evidence_count = %d truncated = %v, want 6 and true",
			item.EvidenceCount, item.EvidenceTruncated)
	}
	if got := 1 + len(item.AdditionalEvidence); got != service.MaxRelatedEvidencePerItem {
		t.Errorf("carried %d reasons, want the cap of %d", got, service.MaxRelatedEvidencePerItem)
	}
	// Generating an explanation for each kept reason is a lookup per reason, so the explanation
	// work is bounded by the same cap the evidence is: an item cannot carry more sentences than
	// it carries reasons.
	for _, reason := range relatedReasons(item) {
		if reason.Explanation != domain.RelatedPriorityExplanation(domain.PriorityConceptual) {
			t.Errorf("a conceptual connection explained itself as %q", reason.Explanation)
		}
	}

	// The same pair under a scope that admits nothing they are connected by is absent entirely.
	none := relatedScopeOf(t, k, domain.SearchNode, "twinned-left", "navigational")
	for _, other := range none.Items {
		if other.ID == "twinned-right" {
			t.Error("a navigational scope returned a pair connected only conceptually")
		}
	}
}

// TestRelatedScopedResultRespectsTheItemCeiling asserts the item bound still binds under a filter,
// and that the bounded result is still the front of the same ordering.
func TestRelatedScopedResultRespectsTheItemCeiling(t *testing.T) {
	const spokes = 300
	k := indexFrom(t, hubRelatedCorpus(t, spokes))

	result := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{
		Limit:             service.MaxRelatedLimit * 10,
		RelationshipTypes: []string{"conceptual"},
	})
	if result.Limit != service.MaxRelatedLimit || len(result.Items) != service.MaxRelatedLimit {
		t.Errorf("limit = %d items = %d, want the ceiling %d",
			result.Limit, len(result.Items), service.MaxRelatedLimit)
	}
	if result.Counts.Eligible < spokes || !result.Truncated {
		t.Errorf("counts = %+v truncated = %v, want the full spoke count and a truncation flag",
			result.Counts, result.Truncated)
	}
	if first, last := result.Items[0].ID, result.Items[len(result.Items)-1].ID; first != "spoke-000" || last != "spoke-099" {
		t.Errorf("bounded result runs %s..%s, want the front of the canonical ID order", first, last)
	}
}

// --------------------------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------------------------

// singleClassScopes renders each declared class as a one-member scope.
func singleClassScopes() [][]string {
	out := make([][]string, 0, len(domain.RelatedPriorities))
	for _, name := range domain.RelatedPriorityNames() {
		out = append(out, []string{name})
	}
	return out
}

// sortedByPrecedence reports whether a rendered scope is in the model's declared order.
func sortedByPrecedence(names []string) bool {
	for i := 1; i < len(names); i++ {
		prev := domain.RelatedPriorityRank(domain.RelatedPriority(names[i-1]))
		next := domain.RelatedPriorityRank(domain.RelatedPriority(names[i]))
		if prev >= next {
			return false
		}
	}
	return true
}

// permutations returns every ordering of a short list.
//
// It is bounded by the caller: the longest list any test permutes is the seven declared classes,
// which is 5,040 orderings of a seven-element slice and runs in milliseconds. It is written here
// rather than approximated by a reversal because "order-independent" is a claim about every
// spelling, and a reversal only checks two of them.
func permutations(values []string) [][]string {
	if len(values) <= 1 {
		return [][]string{append([]string(nil), values...)}
	}
	out := make([][]string, 0)
	for i := range values {
		rest := make([]string, 0, len(values)-1)
		rest = append(rest, values[:i]...)
		rest = append(rest, values[i+1:]...)
		for _, tail := range permutations(rest) {
			out = append(out, append([]string{values[i]}, tail...))
		}
	}
	return out
}
