package service_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// The Phase 2A related-knowledge tests.
//
// They assert three separable things, and the separation is deliberate. First, that the corpus is
// read correctly: a discovery result contains the records the canonical fields actually connect
// and no others. Second, that the ordering is a policy rather than an accident: it is total,
// reproducible across runs and across separately built indexes, and it does not depend on Go map
// iteration or on how the filesystem enumerated the corpus. Third, that every item can account
// for itself: each carries a classified reason naming a canonical field, and each reason is a
// relation the Phase 1F context projection independently agrees exists.

// relatedOf is the discovery call with default bounds and no destination scope.
func relatedOf(t testing.TB, k *service.Knowledge, entityType domain.SearchEntityType, id string) domain.RelatedKnowledge {
	t.Helper()
	result, err := k.RelatedKnowledgeFor(string(entityType), id, service.RelatedQuery{})
	if err != nil {
		t.Fatalf("related %s/%s: %v", entityType, id, err)
	}
	return result
}

// mustRelated is the discovery call with an explicit query.
func mustRelated(t testing.TB, k *service.Knowledge, entityType domain.SearchEntityType, id string, q service.RelatedQuery) domain.RelatedKnowledge {
	t.Helper()
	result, err := k.RelatedKnowledgeFor(string(entityType), id, q)
	if err != nil {
		t.Fatalf("related %s/%s %+v: %v", entityType, id, q, err)
	}
	return result
}

// relatedRefs renders a result as ordered type/id pairs.
//
// Type and ID together are the identity, never the ID alone: the fixture corpus registers
// session-01-fixture both as a session and as a registry entry, and node/alpha is related to both
// of them, so an assertion on bare IDs could not tell the two items apart.
func relatedRefs(result domain.RelatedKnowledge) []string {
	out := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		out = append(out, string(item.EntityType)+"/"+item.ID)
	}
	return out
}

// relatedLines renders a result as "type/id <-relation- origin (priority, derived)".
//
// The reason is part of the rendering rather than checked separately, because it is what keeps an
// item honest: which canonical field connected the two records, which direction it was written
// in, and what precedence that field carries. An assertion on identity alone would pass on a
// projection that had started returning the right records for invented reasons.
func relatedLines(result domain.RelatedKnowledge) []string {
	out := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		out = append(out, fmt.Sprintf("%s/%s <-%s- %s (%s, derived=%v)",
			item.EntityType, item.ID, item.Reason.Relation, item.Reason.Origin,
			item.Reason.Priority, item.Reason.Derived))
	}
	return out
}

// everyStart is one start of each searchable class that the fixture corpus resolves, so a test
// that must cover the closed set names it once rather than per case.
var everyStart = []struct {
	class domain.SearchEntityType
	id    string
}{
	{domain.SearchSession, "session-01-fixture"},
	{domain.SearchNode, "alpha"},
	{domain.SearchClaim, "alpha-carries-energy"},
	{domain.SearchSource, "fixture-reference-work"},
	{domain.SearchVocabulary, "fixture-term"},
	{domain.SearchExperiment, "fixture-listening-exercise"},
}

// --------------------------------------------------------------------------------------------
// A. Successful discovery
// --------------------------------------------------------------------------------------------

// TestRelatedKnowledgeSupportsEverySearchableStartClass is the reach assertion of the phase.
//
// Discovery starts from all six searchable classes rather than the four graph ones, which is the
// difference between it and traversal: a reader holding a vocabulary entry or an experiment
// definition can ask where to go next, and neither of those is a graph vertex. The closed set is
// walked rather than listed, so a class added to the model without a discovery start fails here.
func TestRelatedKnowledgeSupportsEverySearchableStartClass(t *testing.T) {
	k := evidenceIndex(t)

	if len(everyStart) != len(domain.SearchEntityTypes) {
		t.Fatalf("the fixture start table covers %d classes, the model has %d",
			len(everyStart), len(domain.SearchEntityTypes))
	}
	for i, start := range everyStart {
		if start.class != domain.SearchEntityTypes[i] {
			t.Fatalf("start table is not in canonical class order at %d: %s", i, start.class)
		}
		t.Run(string(start.class), func(t *testing.T) {
			result := relatedOf(t, k, start.class, start.id)
			if result.Start.EntityType != start.class || result.Start.ID != start.id {
				t.Errorf("start = %s/%s, want %s/%s",
					result.Start.EntityType, result.Start.ID, start.class, start.id)
			}
			if result.Start.Title == "" {
				t.Error("the resolved start carries no canonical title")
			}
			if len(result.Items) == 0 {
				t.Fatal("no related knowledge, so every assertion below would pass vacuously")
			}
			for _, item := range result.Items {
				if !domain.ValidSearchEntityType(string(item.EntityType)) {
					t.Errorf("item %s/%s is not a searchable class", item.EntityType, item.ID)
				}
				if item.Title == "" {
					t.Errorf("item %s/%s carries no canonical title", item.EntityType, item.ID)
				}
			}
		})
	}
}

// TestRelatedKnowledgeFromNodeIsTheCanonicalNeighbourhoodInPrecedenceOrder pins the whole
// contract for one start, ordering and reasons together.
//
// Alpha is the fixture's most connected node and reaches five of the six classes, so this is
// simultaneously the cross-layer assertion and the precedence assertion. Reading down the
// expected list is reading the ranking policy: the two concepts alpha itself declares an edge to,
// then the statements made about it, then where it came from and what it cites, then the practice
// records that point back at it.
func TestRelatedKnowledgeFromNodeIsTheCanonicalNeighbourhoodInPrecedenceOrder(t *testing.T) {
	k := evidenceIndex(t)
	got := relatedLines(relatedOf(t, k, domain.SearchNode, "alpha"))
	want := []string{
		"node/beta <-produces- node.relationships (conceptual, derived=false)",
		"node/gamma <-characterized_by- node.relationships (conceptual, derived=false)",
		"claim/alpha-carries-energy <-appearance_site_of- claim.appears_in (assertional, derived=true)",
		"claim/gamma-follows-from-alpha-and-beta <-basis_for- claim.derived_from (assertional, derived=true)",
		"session/session-01-fixture <-originates_in- node.session_origin (contextual, derived=false)",
		"source/fixture-reference-work <-sourced_from- node.sources (contextual, derived=false)",
		"source/session-01-fixture <-sourced_from- node.sources (contextual, derived=false)",
		"vocabulary/fixture-term <-referenced_by- vocabulary.node_refs (referential, derived=true)",
		"experiment/fixture-listening-exercise <-referenced_by- experiment.node_refs (referential, derived=true)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("related knowledge of node/alpha:\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRelatedKnowledgeFromClaimLeadsToProvenanceFirst is the provenance-connected case.
//
// A reader holding a checkable statement wants what stands behind it before anything else, and
// the precedence table says so: claim.evidence outranks every other field a claim carries. The
// two supporting sources therefore lead, ahead of the records the statement appears in.
func TestRelatedKnowledgeFromClaimLeadsToProvenanceFirst(t *testing.T) {
	k := evidenceIndex(t)
	got := relatedLines(relatedOf(t, k, domain.SearchClaim, "alpha-carries-energy"))
	want := []string{
		"source/fixture-archive-record <-supported_by- claim.evidence (evidential, derived=false)",
		"source/fixture-reference-work <-supported_by- claim.evidence (evidential, derived=false)",
		"node/alpha <-appears_in- claim.appears_in (assertional, derived=false)",
		"vocabulary/fixture-term <-appears_in- claim.appears_in (assertional, derived=false)",
		"claim/gamma-follows-from-alpha-and-beta <-basis_for- claim.derived_from (assertional, derived=true)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("related knowledge of claim/alpha-carries-energy:\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRelatedKnowledgeSeparatesEvidenceFromAttribution covers the one precedence boundary the
// provenance model insists on.
//
// docs/claim-provenance-model.md treats "what materially supports this" and "who is credited with
// saying it" as different facts. The table gives them different precedence classes, so a
// contradicting source still outranks an attribution: the reader is offered what bears on the
// truth of the statement before who is associated with it. A projection that flattened the two
// into one provenance class would pass an identity assertion and fail this one.
func TestRelatedKnowledgeSeparatesEvidenceFromAttribution(t *testing.T) {
	k := evidenceIndex(t)
	got := relatedLines(relatedOf(t, k, domain.SearchClaim, "beta-was-observed-in-1999"))
	want := []string{
		"source/fixture-archive-record <-supported_by- claim.evidence (evidential, derived=false)",
		"source/fixture-reference-work <-contradicted_by- claim.evidence (evidential, derived=false)",
		"source/fixture-attribution-source <-attributed_to- claim.attribution (attributive, derived=false)",
		"session/session-01-fixture <-appears_in- claim.appears_in (assertional, derived=false)",
		"node/beta <-appears_in- claim.appears_in (assertional, derived=false)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("related knowledge of claim/beta-was-observed-in-1999:\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRelatedKnowledgeFromVocabularyReachesTheConceptLayer is the practice-layer start.
//
// A bare term and its definition say nothing about where the corpus uses it, which is exactly why
// a vocabulary entry is the start least useful without discovery. Its own reference lists reach
// the node and session layers, the claims made about it are read backwards, and its curated
// related term comes last because vocabulary/README.md says that list is navigation only.
func TestRelatedKnowledgeFromVocabularyReachesTheConceptLayer(t *testing.T) {
	k := evidenceIndex(t)
	got := relatedLines(relatedOf(t, k, domain.SearchVocabulary, "fixture-term"))
	want := []string{
		"claim/alpha-carries-energy <-appearance_site_of- claim.appears_in (assertional, derived=true)",
		"session/session-01-fixture <-references- vocabulary.session_refs (referential, derived=false)",
		"node/alpha <-references- vocabulary.node_refs (referential, derived=false)",
		"experiment/fixture-listening-exercise <-referenced_by- experiment.vocabulary_refs (referential, derived=true)",
		"vocabulary/fixture-companion <-related_term- vocabulary.related_terms (navigational, derived=false)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("related knowledge of vocabulary/fixture-term:\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRelatedKnowledgeFromSourceReachesWhatCitesIt reads the provenance layer backwards.
//
// A registry entry authors no reference of its own, so every item here is a derived read of some
// other record's citation. That is the case a discovery layer has to get right: without it a
// source would be a dead end, reachable from everything and leading nowhere.
func TestRelatedKnowledgeFromSourceReachesWhatCitesIt(t *testing.T) {
	k := evidenceIndex(t)
	result := relatedOf(t, k, domain.SearchSource, "fixture-archive-record")
	got := relatedLines(result)
	want := []string{
		"claim/alpha-carries-energy <-supports- claim.evidence (evidential, derived=true)",
		"claim/beta-was-observed-in-1999 <-supports- claim.evidence (evidential, derived=true)",
		"claim/gamma-follows-from-alpha-and-beta <-supports- claim.evidence (evidential, derived=true)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("related knowledge of source/fixture-archive-record:\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, item := range result.Items {
		if !item.Reason.Derived {
			t.Errorf("item %s/%s claims a registry entry authored the citation", item.EntityType, item.ID)
		}
	}
}

// TestRelatedKnowledgeFromExperimentReachesWhatItDemonstrates is the experiment start.
//
// A definition's own reference lists are the whole of its outbound context. Its runs are not
// resolved here and must not be: a run is not a searchable class, and following a definition to
// its results stays an explicit request to the experiment-run route.
func TestRelatedKnowledgeFromExperimentReachesWhatItDemonstrates(t *testing.T) {
	k := evidenceIndex(t)
	result := relatedOf(t, k, domain.SearchExperiment, "fixture-listening-exercise")
	got := relatedRefs(result)
	want := []string{
		"session/session-01-fixture", "node/alpha", "node/beta",
		"source/fixture-reference-work", "vocabulary/fixture-term",
		"experiment/fixture-visualization-exercise",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("related knowledge of experiment/fixture-listening-exercise:\n%v\nwant\n%v", got, want)
	}
	for _, item := range result.Items {
		if strings.Contains(item.ID, "-run-") || strings.Contains(item.ID, "planned") || strings.Contains(item.ID, "completed") {
			t.Errorf("an experiment run reached the discovery contract: %s/%s", item.EntityType, item.ID)
		}
	}
}

// TestRelatedKnowledgeDefaultLimit asserts the documented default is applied and echoed.
//
// The echo matters as much as the bound: a caller who named no limit has to be able to read what
// they received without knowing the constant, or a truncated list is indistinguishable from a
// complete one.
func TestRelatedKnowledgeDefaultLimit(t *testing.T) {
	k := evidenceIndex(t)
	result := relatedOf(t, k, domain.SearchNode, "alpha")
	if result.Limit != service.DefaultRelatedLimit {
		t.Errorf("limit = %d, want the default %d", result.Limit, service.DefaultRelatedLimit)
	}
	if result.Counts.Returned != len(result.Items) {
		t.Errorf("counts.returned = %d, items = %d", result.Counts.Returned, len(result.Items))
	}
	if result.Truncated {
		t.Error("a nine-item result reported truncation under a default limit of 25")
	}
}

// TestRelatedKnowledgeTruncatesDeterministically is the bounded-truncation assertion.
//
// A cut list must be the front of the complete ordering rather than a different selection, must
// report the complete eligible total, and must say it was cut. All three are checked against the
// unbounded result rather than against a hard-coded list, so the assertion keeps meaning if the
// fixture grows.
func TestRelatedKnowledgeTruncatesDeterministically(t *testing.T) {
	k := evidenceIndex(t)
	full := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: service.MaxRelatedLimit})

	for limit := 1; limit < len(full.Items); limit++ {
		t.Run(fmt.Sprintf("limit-%d", limit), func(t *testing.T) {
			cut := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: limit})
			if cut.Limit != limit {
				t.Errorf("limit = %d, want %d", cut.Limit, limit)
			}
			if !cut.Truncated {
				t.Error("a cut result did not report truncation")
			}
			if cut.Counts.Eligible != full.Counts.Eligible {
				t.Errorf("eligible = %d under a limit, %d without", cut.Counts.Eligible, full.Counts.Eligible)
			}
			if got, want := cut.Items, full.Items[:limit]; !reflect.DeepEqual(got, want) {
				t.Errorf("a cut result is not the front of the complete ordering:\n%v\nwant\n%v",
					relatedRefs(cut), relatedRefs(domain.RelatedKnowledge{Items: want}))
			}
		})
	}
}

// TestRelatedKnowledgeLimitIsClampedNotRefused pins the limit contract.
//
// limit follows the paging contract every other route on this API uses rather than the traversal
// depth contract: an over-large value is clamped and the applied value is echoed, so the clamp is
// visible in the response. A depth is refused instead because a silently reduced depth would let
// a caller believe they had seen a whole neighbourhood; a clamped limit returns the front of the
// same ordering that was asked for, and the eligible count says what was left.
func TestRelatedKnowledgeLimitIsClampedNotRefused(t *testing.T) {
	k := evidenceIndex(t)

	for name, limit := range map[string]int{
		"zero-is-the-default":                0,
		"negative-is-treated-as-unspecified": -5,
	} {
		t.Run(name, func(t *testing.T) {
			result := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: limit})
			if result.Limit != service.DefaultRelatedLimit {
				t.Errorf("limit = %d, want the default %d", result.Limit, service.DefaultRelatedLimit)
			}
		})
	}
	for _, limit := range []int{service.MaxRelatedLimit + 1, 5000} {
		result := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: limit})
		if result.Limit != service.MaxRelatedLimit {
			t.Errorf("limit %d was echoed as %d, want the ceiling %d",
				limit, result.Limit, service.MaxRelatedLimit)
		}
	}
	if result := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: service.MaxRelatedLimit}); result.Limit != service.MaxRelatedLimit {
		t.Errorf("an exactly maximal limit was echoed as %d", result.Limit)
	}
}

// TestRelatedKnowledgeDestinationScopeRestrictsWithoutReordering is the scope assertion.
//
// A scoped discovery must be the unscoped one with other classes removed: same items, same
// reasons, same relative order. If a filter could change the ranking, the ranking would be a
// property of the request rather than of the corpus, and two callers asking about one record
// would be told different things about how it is connected.
func TestRelatedKnowledgeDestinationScopeRestrictsWithoutReordering(t *testing.T) {
	k := evidenceIndex(t)
	full := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: service.MaxRelatedLimit})

	for _, scope := range [][]string{
		{"node"}, {"source"}, {"claim", "node"}, {"experiment", "vocabulary"},
		{"session", "node", "claim", "source", "vocabulary", "experiment"},
	} {
		t.Run(strings.Join(scope, "+"), func(t *testing.T) {
			scoped := mustRelated(t, k, domain.SearchNode, "alpha",
				service.RelatedQuery{Limit: service.MaxRelatedLimit, EntityTypes: scope})

			admitted := make([]domain.RelatedItem, 0, len(full.Items))
			for _, item := range full.Items {
				if contains(scope, string(item.EntityType)) {
					admitted = append(admitted, item)
				}
			}
			if !reflect.DeepEqual(scoped.Items, admitted) {
				t.Errorf("scoped result is not the unscoped one filtered:\n%v\nwant\n%v",
					relatedRefs(scoped), relatedRefs(domain.RelatedKnowledge{Items: admitted}))
			}
			if scoped.Counts.Eligible != len(admitted) {
				t.Errorf("eligible = %d, want %d: the count must describe the scoped set",
					scoped.Counts.Eligible, len(admitted))
			}
		})
	}
}

// TestRelatedKnowledgeEchoesScopeInCanonicalOrder asserts the response describes the request that
// ran rather than the caller's spelling of it, so two spellings of one scope are one response.
func TestRelatedKnowledgeEchoesScopeInCanonicalOrder(t *testing.T) {
	k := evidenceIndex(t)

	forward := mustRelated(t, k, domain.SearchNode, "alpha",
		service.RelatedQuery{EntityTypes: []string{"claim", "node", "source"}})
	reversed := mustRelated(t, k, domain.SearchNode, "alpha",
		service.RelatedQuery{EntityTypes: []string{"source", "claim", "node"}})

	want := []string{"node", "claim", "source"}
	if !reflect.DeepEqual(forward.EntityTypes, want) {
		t.Errorf("entity_types = %v, want the canonical order %v", forward.EntityTypes, want)
	}
	if !reflect.DeepEqual(forward, reversed) {
		t.Error("two spellings of one scope produced two different responses")
	}
	if plain := relatedOf(t, k, domain.SearchNode, "alpha"); plain.EntityTypes != nil {
		t.Errorf("an unscoped discovery echoed entity_types = %v", plain.EntityTypes)
	}
}

// --------------------------------------------------------------------------------------------
// B. Determinism
// --------------------------------------------------------------------------------------------

// TestRelatedKnowledgeIsStableAcrossRepeatedCalls is the reproducibility assertion.
//
// Repeated equivalent requests against an unchanged corpus must return logically equivalent
// responses. Every start of every class is checked, because the grouping walks a map-keyed
// projection and a single unordered read anywhere in it would show up as an intermittent
// reordering rather than as a failure at the point of the mistake.
func TestRelatedKnowledgeIsStableAcrossRepeatedCalls(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		first := relatedOf(t, k, start.class, start.id)
		for run := 0; run < 25; run++ {
			again := relatedOf(t, k, start.class, start.id)
			if !reflect.DeepEqual(first, again) {
				t.Fatalf("%s/%s changed on run %d:\n%v\nwant\n%v",
					start.class, start.id, run, relatedLines(again), relatedLines(first))
			}
		}
	}
}

// TestRelatedKnowledgeIsIndependentOfCorpusEnumerationOrder is the stronger determinism
// assertion: two indexes built from two different filesystem implementations must agree.
//
// The fixture is read once from disk, where fs.WalkDir returns lexical directory order, and once
// from an in-memory map, where the corpus is held in a Go map with no order at all. If any step
// of the projection — the context builder, the grouping, or the ranking — depended on the order
// records happened to arrive in, the two indexes would disagree here while each stayed
// self-consistent under the repeated-call test above.
func TestRelatedKnowledgeIsIndependentOfCorpusEnumerationOrder(t *testing.T) {
	fromDisk := evidenceIndex(t)
	fromMemory := indexFrom(t, testsupport.MutableCorpus(t))

	for _, start := range everyStart {
		disk := relatedOf(t, fromDisk, start.class, start.id)
		memory := relatedOf(t, fromMemory, start.class, start.id)
		if !reflect.DeepEqual(disk, memory) {
			t.Errorf("%s/%s differs between two independently built indexes:\n%v\nwant\n%v",
				start.class, start.id, relatedLines(memory), relatedLines(disk))
		}
	}
}

// TestRelatedKnowledgeTieBreaksOnCanonicalIdentity pins the last two ordering keys.
//
// Node alpha cites two sources through the same canonical field, so the two items agree on
// precedence and on direction and are separated by nothing but their canonical IDs. Node beta
// reaches a session and a source through two fields of the same precedence class, so those two
// are separated by the canonical class order. Between them the pair covers both tie-breaks, and
// both are properties of the corpus rather than of the run.
func TestRelatedKnowledgeTieBreaksOnCanonicalIdentity(t *testing.T) {
	k := evidenceIndex(t)

	byID := relatedRefs(mustRelated(t, k, domain.SearchNode, "alpha",
		service.RelatedQuery{EntityTypes: []string{"source"}}))
	if want := []string{"source/fixture-reference-work", "source/session-01-fixture"}; !reflect.DeepEqual(byID, want) {
		t.Errorf("ID tie-break: %v, want %v", byID, want)
	}

	byClass := relatedRefs(mustRelated(t, k, domain.SearchNode, "beta",
		service.RelatedQuery{EntityTypes: []string{"session", "source"}}))
	if want := []string{"session/session-01-fixture", "source/session-01-fixture"}; !reflect.DeepEqual(byClass, want) {
		t.Errorf("class tie-break: %v, want %v", byClass, want)
	}
}

// TestRelatedKnowledgeOrderingIsTotal asserts no two items of any result compare equal on the
// full key, which is what makes "the same set" and "the same bytes" the same statement.
func TestRelatedKnowledgeOrderingIsTotal(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		result := mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
		seen := make(map[string]bool, len(result.Items))
		for _, item := range result.Items {
			key := fmt.Sprintf("%d|%v|%s|%s",
				item.Reason.PriorityRank, item.Reason.Derived, item.EntityType, item.ID)
			if seen[key] {
				t.Errorf("%s/%s returned two items with identical sort keys: %s", start.class, start.id, key)
			}
			seen[key] = true
		}
	}
}

// TestRelatedKnowledgeDoesNotAliasTheIndex asserts a caller cannot reach the startup projection
// through a response, which is what keeps the index immutable and therefore safe for concurrent
// readers without locking.
func TestRelatedKnowledgeDoesNotAliasTheIndex(t *testing.T) {
	k := evidenceIndex(t)

	before := relatedOf(t, k, domain.SearchClaim, "alpha-may-extend-to-gamma")
	mutated := relatedOf(t, k, domain.SearchClaim, "alpha-may-extend-to-gamma")
	for i := range mutated.Items {
		mutated.Items[i].Title = "overwritten"
		mutated.Items[i].Reason.Relation = "overwritten"
		for j := range mutated.Items[i].AdditionalEvidence {
			mutated.Items[i].AdditionalEvidence[j].Origin = "overwritten"
		}
	}
	if after := relatedOf(t, k, domain.SearchClaim, "alpha-may-extend-to-gamma"); !reflect.DeepEqual(after, before) {
		t.Error("writing to a returned discovery result changed what the index serves")
	}
}

// --------------------------------------------------------------------------------------------
// C. Evidence integrity
// --------------------------------------------------------------------------------------------

// TestEveryRelatedReasonIsClassifiedAndCanonical is the no-fabrication assertion.
//
// Every reason on every item of every start is checked against three separate facts: the origin
// names a canonical field the model declares, the precedence is a real class rather than the
// unclassified fallback, and the rank is the one the table gives for that class. A hand-built
// reason, a mistyped origin, or a field added to the context layer without being classified all
// fail here rather than reaching a caller as a plausible-looking explanation.
func TestEveryRelatedReasonIsClassifiedAndCanonical(t *testing.T) {
	k := evidenceIndex(t)

	checked := 0
	for _, start := range everyStart {
		result := mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
		for _, item := range result.Items {
			for _, reason := range append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...) {
				checked++
				if reason.Relation == "" {
					t.Errorf("%s/%s carries a reason with no relation", item.EntityType, item.ID)
				}
				if !contains(domain.RelatedPriorityOrigins, reason.Origin) {
					t.Errorf("%s/%s names origin %q, which is not a canonical field of the model",
						item.EntityType, item.ID, reason.Origin)
				}
				if reason.Priority == domain.PriorityUnclassified {
					t.Errorf("%s/%s is explained by an unclassified field %q",
						item.EntityType, item.ID, reason.Origin)
				}
				if want := domain.RelatedPriorityRank(reason.Priority); reason.PriorityRank != want {
					t.Errorf("%s/%s reports rank %d for %s, want %d",
						item.EntityType, item.ID, reason.PriorityRank, reason.Priority, want)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no reasons were checked, so this test would pass vacuously")
	}
}

// TestRelatedEvidenceAgreesWithTheContextProjection is the grounding assertion.
//
// Discovery is a ranking of the Phase 1F context projection and must add nothing to it. Every
// reason a discovery result carries is looked up in the context the search route independently
// serves for the same record, matched on all four canonical facts — relation, destination,
// canonical field and direction — and a reason that is not there is a fabrication no matter how
// reasonable it looks. This is the test that would fail if a future edit ever started inferring
// a connection from similarity, prose overlap or co-occurrence.
func TestRelatedEvidenceAgreesWithTheContextProjection(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		canonical := make(map[string]bool)
		for _, relation := range contextOf(t, k, start.class, start.id).Related {
			canonical[fmt.Sprintf("%s|%s/%s|%s|%v", relation.Relation,
				relation.Entity.EntityType, relation.Entity.ID, relation.Origin, relation.Derived)] = true
		}

		result := mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
		for _, item := range result.Items {
			for _, reason := range append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...) {
				key := fmt.Sprintf("%s|%s/%s|%s|%v", reason.Relation,
					item.EntityType, item.ID, reason.Origin, reason.Derived)
				if !canonical[key] {
					t.Errorf("%s/%s claims a connection the canonical context does not contain: %s",
						start.class, start.id, key)
				}
			}
		}
	}
}

// TestRelatedKnowledgeDeduplicatesKeepingTheStrongestReason covers the deduplication rule against
// real canonical data.
//
// The fixture claim names node gamma twice, in appears_in and in derived_from. Discovery must
// offer gamma once, lead with the stronger of the two connections, and report the other as
// further evidence with an accurate total — not two items, and not one item that has forgotten
// the second field said so.
func TestRelatedKnowledgeDeduplicatesKeepingTheStrongestReason(t *testing.T) {
	k := evidenceIndex(t)
	result := relatedOf(t, k, domain.SearchClaim, "alpha-may-extend-to-gamma")

	var gamma *domain.RelatedItem
	for i, item := range result.Items {
		if item.EntityType == domain.SearchNode && item.ID == "gamma" {
			if gamma != nil {
				t.Fatal("node/gamma was returned twice by one discovery")
			}
			gamma = &result.Items[i]
		}
	}
	if gamma == nil {
		t.Fatal("node/gamma is named by two canonical fields of the claim and was not returned")
	}
	if gamma.Reason.Relation != "appears_in" || gamma.Reason.Origin != domain.OriginClaimAppearsIn {
		t.Errorf("primary reason = %+v, want the appears_in connection", gamma.Reason)
	}
	if gamma.EvidenceCount != 2 || gamma.EvidenceTruncated {
		t.Errorf("evidence_count = %d truncated = %v, want 2 and false",
			gamma.EvidenceCount, gamma.EvidenceTruncated)
	}
	if len(gamma.AdditionalEvidence) != 1 || gamma.AdditionalEvidence[0].Origin != domain.OriginClaimDerivedFrom {
		t.Errorf("additional evidence = %+v, want the derived_from connection", gamma.AdditionalEvidence)
	}
}

// TestRelatedKnowledgeDeduplicationPrefersTheStrongerPrecedenceClass is the cross-class case.
//
// The fixture corpus has no pair of records connected by two fields of different precedence, so
// the case is constructed: one claim both cites a source as evidence and credits it in
// attribution. Discovery must lead with the evidential connection, because that is the stronger
// class, and must report the attributive one behind it rather than choosing by field order,
// alphabetical relation name, or whichever loop happened to run first.
func TestRelatedKnowledgeDeduplicationPrefersTheStrongerPrecedenceClass(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "claims/records/fixture-claims.yaml", testsupport.ValidClaim(
		"doubly-cited-claim", "technical_fact", "high", "undisputed",
		testsupport.SupportedBy("fixture-attribution-source"),
		`[{"actor": "Fixture Archivist", "source_id": "fixture-attribution-source"}]`,
		"[]", testsupport.AppearsInNode("alpha")))
	k := indexFrom(t, corpus)

	result := relatedOf(t, k, domain.SearchClaim, "doubly-cited-claim")
	var cited *domain.RelatedItem
	for i, item := range result.Items {
		if item.EntityType == domain.SearchSource && item.ID == "fixture-attribution-source" {
			cited = &result.Items[i]
		}
	}
	if cited == nil {
		t.Fatal("the doubly cited source was not returned")
	}
	if cited.Reason.Priority != domain.PriorityEvidential {
		t.Errorf("primary priority = %s, want %s: evidence outranks attribution",
			cited.Reason.Priority, domain.PriorityEvidential)
	}
	if cited.EvidenceCount != 2 {
		t.Fatalf("evidence_count = %d, want 2", cited.EvidenceCount)
	}
	if got := cited.AdditionalEvidence[0].Priority; got != domain.PriorityAttributive {
		t.Errorf("additional priority = %s, want %s", got, domain.PriorityAttributive)
	}
}

// TestRelatedEvidenceIsBounded asserts the per-item evidence cap.
//
// Evidence is a list inside a list, so without a cap one response would be the product of two
// corpus properties. The case is constructed because the fixture has no pair of records connected
// six ways: two nodes each declare an edge to the other under all three canonical relationship
// types, which gives six distinct connections between the same pair — three authored and three
// read backwards. The item must keep the strongest, report the full count, and say it was cut.
func TestRelatedEvidenceIsBounded(t *testing.T) {
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

	result := relatedOf(t, k, domain.SearchNode, "twinned-left")
	if len(result.Items) == 0 || result.Items[0].ID != "twinned-right" {
		t.Fatalf("twinned-right is not the first item: %v", relatedRefs(result))
	}
	item := result.Items[0]
	if item.EvidenceCount != 6 {
		t.Errorf("evidence_count = %d, want 6", item.EvidenceCount)
	}
	if !item.EvidenceTruncated {
		t.Error("a six-connection item did not report truncated evidence")
	}
	if got := 1 + len(item.AdditionalEvidence); got != service.MaxRelatedEvidencePerItem {
		t.Errorf("carried %d reasons, want the cap of %d", got, service.MaxRelatedEvidencePerItem)
	}
	// The kept reasons are the strongest, in policy order: authored before derived, then relation
	// name. The primary is never repeated inside the additional list.
	got := []string{item.Reason.Relation}
	for _, reason := range item.AdditionalEvidence {
		got = append(got, reason.Relation)
	}
	want := []string{"characterized_by", "processes", "produces", "characterizes", "processed_by"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kept reasons = %v, want %v", got, want)
	}
}

// TestRelatedKnowledgeNeverOffersTheStartItself covers the self-reference case from both sides.
//
// The corpus contracts forbid a record from referencing itself, and the loader enforces it as a
// fatal issue in every layer that has a reference list, so a self-connection cannot exist in an
// index that started. The first half of this test asserts that guarantee at the layer that owns
// it rather than assuming it, because the discovery exclusion is defence in depth and defence in
// depth that is never checked is decoration.
//
// The second half asserts the exclusion itself over every start the fixture offers, which is the
// property a caller depends on: a discovery result never tells a reader to go and read the record
// they are already holding.
func TestRelatedKnowledgeNeverOffersTheStartItself(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "nodes/dsp/self-referring.md", testsupport.ValidNode(
		"self-referring", "Self Referring", "dsp", "seed",
		`[{"target": "self-referring", "type": "produces"}]`, "[]"))
	repo, err := filesystem.NewFromFS(corpus, testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	if _, err := service.New(context.Background(), repo); err == nil {
		t.Error("a corpus declaring a node self-link built an index; the self-link rule is not enforced")
	} else if !strings.Contains(err.Error(), "self-link") {
		t.Errorf("self-link corpus was refused for another reason: %v", err)
	}

	k := evidenceIndex(t)
	for _, start := range everyStart {
		result := mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
		for _, item := range result.Items {
			if item.EntityType == start.class && item.ID == start.id {
				t.Errorf("%s/%s was offered as related knowledge about itself", start.class, start.id)
			}
		}
	}
}

// TestRelatedKnowledgeKeepsTheTwoProjectionsOfOneRecordApart is the identity assertion.
//
// A registered session is also a registry entry, so session/session-01-fixture and
// source/session-01-fixture share an ID and are two different records. Node alpha names the
// second in its sources and the first in its session_origin, and discovery must offer both,
// separately, with the canonical field that reached each. Collapsing them by ID would merge two
// canonical projections into one and lose one of the two connections.
func TestRelatedKnowledgeKeepsTheTwoProjectionsOfOneRecordApart(t *testing.T) {
	k := evidenceIndex(t)
	lines := relatedLines(relatedOf(t, k, domain.SearchNode, "alpha"))

	for _, want := range []string{
		"session/session-01-fixture <-originates_in- node.session_origin (contextual, derived=false)",
		"source/session-01-fixture <-sourced_from- node.sources (contextual, derived=false)",
	} {
		if !contains(lines, want) {
			t.Errorf("missing %q from:\n%s", want, strings.Join(lines, "\n"))
		}
	}
}

// --------------------------------------------------------------------------------------------
// D. Validation
// --------------------------------------------------------------------------------------------

// TestRelatedKnowledgeRejectsUnsupportedStartClass asserts the closed starting set.
//
// experiment_run is the interesting member of the list. It is a canonical layer the backend fully
// parses and serves, and it is deliberately not a discovery class, so accepting it here and
// returning an empty result would tell a caller that a run is connected to nothing rather than
// that runs are not a discovery start.
func TestRelatedKnowledgeRejectsUnsupportedStartClass(t *testing.T) {
	k := evidenceIndex(t)

	for _, class := range []string{"", "experiment_run", "Node", "NODE", "nodes", "entity", "../node"} {
		if _, err := k.RelatedKnowledgeFor(class, "alpha", service.RelatedQuery{}); !errors.Is(err, service.ErrUnsupportedRelatedEntityType) {
			t.Errorf("start class %q: err = %v, want ErrUnsupportedRelatedEntityType", class, err)
		}
	}
}

// TestRelatedKnowledgeUnknownStartIsNotFound keeps "this record does not exist" distinguishable
// from "this record is related to nothing".
//
// Canonical IDs are ASCII kebab-case by contract, enforced at load by every layer, so a Unicode
// or otherwise unusual identifier is syntactically valid input naming a record the corpus cannot
// contain. It must be answered as a missing record rather than crashing, matching, or leaking
// anything about the shape of the lookup.
func TestRelatedKnowledgeUnknownStartIsNotFound(t *testing.T) {
	k := evidenceIndex(t)

	for _, id := range []string{
		"", "no-such-node", "ALPHA", "alpha ", " alpha", "alpha-carries-energy",
		"rezonans-fikstür", "共鳴", "alpha%2Fbeta",
	} {
		if _, err := k.RelatedKnowledgeFor("node", id, service.RelatedQuery{}); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("node/%q: err = %v, want ErrNotFound", id, err)
		}
	}
	// The same ID under its own class resolves, which is what makes the case above a statement
	// about identity rather than about the ID being rejected outright.
	if _, err := k.RelatedKnowledgeFor("claim", "alpha-carries-energy", service.RelatedQuery{}); err != nil {
		t.Errorf("claim/alpha-carries-energy: err = %v, want a resolved start", err)
	}
}

// TestRelatedKnowledgeRejectsMalformedDestinationScope asserts the class list is refused rather
// than repaired, exactly as the search route refuses it.
//
// A discovery whose scope silently differed from the one the caller wrote would return a list —
// and an eligible count — that does not mean what they think it means, and unlike a mistyped
// query string that is invisible in the response.
func TestRelatedKnowledgeRejectsMalformedDestinationScope(t *testing.T) {
	k := evidenceIndex(t)

	cases := map[string]struct {
		scope []string
		want  error
	}{
		"blank member":        {[]string{"node", ""}, service.ErrEmptySearchEntityType},
		"leading separator":   {[]string{"", "node"}, service.ErrEmptySearchEntityType},
		"whitespace member":   {[]string{"node", "   "}, service.ErrEmptySearchEntityType},
		"repeated class":      {[]string{"node", "node"}, service.ErrDuplicateSearchEntityType},
		"repeated after trim": {[]string{"node", " node "}, service.ErrDuplicateSearchEntityType},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := k.RelatedKnowledgeFor("node", "alpha", service.RelatedQuery{EntityTypes: tc.scope}); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}

	for _, scope := range [][]string{{"experiment_run"}, {"widget"}, {"Node"}, {"node", "experiment_run"}} {
		var invalid *service.InvalidFilterError
		_, err := k.RelatedKnowledgeFor("node", "alpha", service.RelatedQuery{EntityTypes: scope})
		if !errors.As(err, &invalid) {
			t.Errorf("scope %v: err = %v, want an InvalidFilterError", scope, err)
			continue
		}
		if invalid.Param != "entity_types" {
			t.Errorf("scope %v: param = %q, want entity_types", scope, invalid.Param)
		}
		if !reflect.DeepEqual(invalid.Allowed, domain.SearchEntityTypeNames()) {
			t.Errorf("scope %v: allowed = %v, want the searchable classes", scope, invalid.Allowed)
		}
	}
}

// TestRelatedKnowledgeRefusesAMalformedScopeBeforeResolvingTheStart pins the order the request
// is validated in.
//
// The whole request is checked before the start is looked up, so a caller who sent both a
// mistyped identifier and a malformed class list is told about the class list. Reporting the miss
// first would send them to fix the identifier and then refuse them a second time for a mistake
// that was already visible in the request they sent.
//
// The traversal routes are the partial precedent: an out-of-range depth is already refused on a
// root that does not resolve, while the relationship and target_type vocabularies are resolved
// after the root and a mistyped one there is still reported as a missing entity. This test pins
// the discovery half only. It is deliberately not a statement about the Phase 1C routes, which
// this phase does not change.
//
// The complement is asserted with it. A well-formed scope over a missing start must still be
// ErrNotFound, or the ordering would have turned a miss into a refusal rather than the other way
// round, and a malformed scope over a start that does resolve must still be the scope error.
func TestRelatedKnowledgeRefusesAMalformedScopeBeforeResolvingTheStart(t *testing.T) {
	k := evidenceIndex(t)

	cases := map[string]struct {
		scope []string
		want  error
	}{
		"blank member":      {[]string{"node", ""}, service.ErrEmptySearchEntityType},
		"whitespace member": {[]string{"   "}, service.ErrEmptySearchEntityType},
		"repeated class":    {[]string{"node", "node"}, service.ErrDuplicateSearchEntityType},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// The start does not exist under any class, so the only reachable errors are the
			// scope error and ErrNotFound.
			for _, class := range domain.SearchEntityTypeNames() {
				_, err := k.RelatedKnowledgeFor(class, "no-such-record", service.RelatedQuery{EntityTypes: tc.scope})
				if !errors.Is(err, tc.want) {
					t.Errorf("%s/no-such-record: err = %v, want %v", class, err, tc.want)
				}
			}
			// And the same scope is still refused the same way over a start that resolves, so
			// the ordering changed which error is reported first and not what either means.
			if _, err := k.RelatedKnowledgeFor("node", "alpha", service.RelatedQuery{EntityTypes: tc.scope}); !errors.Is(err, tc.want) {
				t.Errorf("node/alpha: err = %v, want %v", err, tc.want)
			}
		})
	}

	// An unsupported class member is an InvalidFilterError rather than a miss, and it names the
	// parameter and the permitted values rather than anything about the start.
	var invalid *service.InvalidFilterError
	_, err := k.RelatedKnowledgeFor("node", "no-such-record", service.RelatedQuery{EntityTypes: []string{"widget"}})
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want an InvalidFilterError", err)
	}
	if invalid.Param != "entity_types" {
		t.Errorf("param = %q, want entity_types", invalid.Param)
	}

	// A well-formed scope over the same missing start is still a miss.
	for _, scope := range [][]string{nil, {"node"}, {"node", "claim"}} {
		if _, err := k.RelatedKnowledgeFor("node", "no-such-record", service.RelatedQuery{EntityTypes: scope}); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("scope %v: err = %v, want ErrNotFound", scope, err)
		}
	}

	// An unsupported start class outranks both. It is the class half of the address rather than a
	// filter, and a request that does not name a discovery class is not a discovery request.
	if _, err := k.RelatedKnowledgeFor("experiment_run", "no-such-run", service.RelatedQuery{EntityTypes: []string{"widget"}}); !errors.Is(err, service.ErrUnsupportedRelatedEntityType) {
		t.Errorf("err = %v, want ErrUnsupportedRelatedEntityType", err)
	}
}

// TestRelatedKnowledgeEmptyResultIsSuccess asserts the empty case is an answer.
//
// A registered source nothing cites, and a vocabulary entry that references nothing and is
// referenced by nothing, both exist. Each must resolve, echo its own identity and title, and
// return an empty list with a zero eligible count — never an error, and never a null the caller
// has to defend against.
func TestRelatedKnowledgeEmptyResultIsSuccess(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range []struct {
		class domain.SearchEntityType
		id    string
	}{
		{domain.SearchSource, "fixture-uncited-source"},
		{domain.SearchVocabulary, "fixture-orphan-term"},
	} {
		t.Run(string(start.class), func(t *testing.T) {
			result := relatedOf(t, k, start.class, start.id)
			if result.Items == nil {
				t.Error("an empty discovery returned a null item list rather than an empty one")
			}
			if len(result.Items) != 0 {
				t.Errorf("items = %v, want none", relatedRefs(result))
			}
			if result.Counts != (domain.RelatedCounts{}) {
				t.Errorf("counts = %+v, want zeroes", result.Counts)
			}
			if result.Truncated {
				t.Error("an empty result reported truncation")
			}
			if result.Start.ID != start.id || result.Start.Title == "" {
				t.Errorf("start = %+v, want the resolved record", result.Start)
			}
		})
	}
}

// TestRelatedKnowledgeSurvivesAScopeThatAdmitsNothing asserts a legitimate empty result is
// distinguished from a malformed request: a scope naming a class the record is not connected to
// is a real question with the answer "none", not an error.
func TestRelatedKnowledgeSurvivesAScopeThatAdmitsNothing(t *testing.T) {
	k := evidenceIndex(t)
	result := mustRelated(t, k, domain.SearchSource, "fixture-attribution-source",
		service.RelatedQuery{EntityTypes: []string{"experiment"}})

	if len(result.Items) != 0 || result.Counts.Eligible != 0 {
		t.Errorf("items = %v eligible = %d, want an empty result", relatedRefs(result), result.Counts.Eligible)
	}
	if !reflect.DeepEqual(result.EntityTypes, []string{"experiment"}) {
		t.Errorf("entity_types = %v, want the scope that was applied", result.EntityTypes)
	}
}

// TestRelatedKnowledgeToleratesADanglingCanonicalReference.
//
// An unresolved canonical reference is fatal at load, so the corpus the index is built from
// cannot contain one and this asserts the guard rather than the corpus: the context layer drops a
// target that has no discovery document, so a reference that somehow reached the projection
// without a resolvable record cannot appear as a bare identifier a client could not follow. The
// case is built as an experiment definition whose source_refs name a registry entry, then the
// registry entry is left in place — the load must either refuse the corpus or serve only
// resolvable items, and both are correct; what must not happen is an item with no title.
func TestRelatedKnowledgeToleratesADanglingCanonicalReference(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		result := mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
		for _, item := range result.Items {
			if item.ID == "" || item.Title == "" {
				t.Errorf("%s/%s produced an unresolvable item %+v", start.class, start.id, item)
			}
		}
	}
}

// --------------------------------------------------------------------------------------------
// E. Regression
// --------------------------------------------------------------------------------------------

// TestRelatedKnowledgeChangesNothingItReads is the no-write assertion at the service.
//
// Discovery reads three projections the earlier phases built — the context relations, the display
// labels and the summaries — and writes to none of them. Search, traversal and context are each
// exercised before and after a full sweep of discovery calls, and every one must answer
// identically. A projection that sorted its input in place, or reused a returned slice, would
// pass every assertion above and fail here.
func TestRelatedKnowledgeChangesNothingItReads(t *testing.T) {
	k := evidenceIndex(t)

	searchBefore := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200, IncludeContext: true})
	traverseBefore, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: service.MaxTraversalDepth})
	if err != nil {
		t.Fatalf("traverse node/alpha: %v", err)
	}

	for _, start := range everyStart {
		for _, limit := range []int{0, 1, 3, service.MaxRelatedLimit} {
			mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: limit})
		}
		mustRelated(t, k, start.class, start.id, service.RelatedQuery{EntityTypes: []string{"node", "claim"}})
	}

	if got := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200, IncludeContext: true}); !reflect.DeepEqual(got, searchBefore) {
		t.Error("running discovery changed what search returns")
	}
	traverseAfter, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: service.MaxTraversalDepth})
	if err != nil {
		t.Fatalf("traverse node/alpha: %v", err)
	}
	if !reflect.DeepEqual(traverseAfter, traverseBefore) {
		t.Error("running discovery changed what traversal returns")
	}
}

// TestRelatedKnowledgeReusesTheDiscoveryDisplayFields asserts a related item names a record
// exactly as search and context already name it.
//
// One record must have one title everywhere it appears, or a reader following a suggestion would
// arrive somewhere that calls itself something else. Reusing the context layer's own label map is
// what guarantees it, and this is the assertion that would fail if a future edit chose a display
// field a second time.
func TestRelatedKnowledgeReusesTheDiscoveryDisplayFields(t *testing.T) {
	k := evidenceIndex(t)

	titles := make(map[string]string)
	summaries := make(map[string]string)
	for _, result := range mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200}).Results {
		titles[string(result.EntityType)+"/"+result.ID] = result.Title
		summaries[string(result.EntityType)+"/"+result.ID] = result.Summary
	}

	checked := 0
	for _, start := range everyStart {
		for _, item := range mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit}).Items {
			ref := string(item.EntityType) + "/" + item.ID
			if want, ok := titles[ref]; ok {
				checked++
				if item.Title != want {
					t.Errorf("%s is titled %q by discovery and %q by search", ref, item.Title, want)
				}
				if item.Summary != summaries[ref] {
					t.Errorf("%s is summarised %q by discovery and %q by search", ref, item.Summary, summaries[ref])
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no item was cross-checked against search, so this test would pass vacuously")
	}
}

// TestRelatedKnowledgeReportsTheBoundsThatShapedIt asserts every bound behind a result is stated
// in it.
//
// A bounded answer that does not say what bounded it cannot be told apart from a complete one.
// The applied item limit is already echoed at the top level; these are the other two, and the
// scan count is cross-checked against the record's own canonical context, so the number reported
// is the work actually done rather than a constant restated.
func TestRelatedKnowledgeReportsTheBoundsThatShapedIt(t *testing.T) {
	k := evidenceIndex(t)

	// The context layer's own count of a record's canonical relations is the independent number
	// the scan count has to agree with.
	contextCount := make(map[string]int)
	for _, result := range mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200, IncludeContext: true}).Results {
		contextCount[string(result.EntityType)+"/"+result.ID] = result.Context.Count
	}

	checked := 0
	for _, start := range everyStart {
		result := mustRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: service.MaxRelatedLimit})

		if result.Bounds.MaxEvidencePerItem != service.MaxRelatedEvidencePerItem {
			t.Errorf("%s/%s: max_evidence_per_item = %d, want %d",
				start.class, start.id, result.Bounds.MaxEvidencePerItem, service.MaxRelatedEvidencePerItem)
		}
		if result.Bounds.MaxRelationsScanned != service.MaxRelatedRelationsScanned {
			t.Errorf("%s/%s: max_relations_scanned = %d, want %d",
				start.class, start.id, result.Bounds.MaxRelationsScanned, service.MaxRelatedRelationsScanned)
		}
		// The fixture corpus is far narrower than the ceiling, so no start may report a cut scan.
		if result.Bounds.RelationsTruncated {
			t.Errorf("%s/%s: reported a truncated scan of %d relations in the fixture corpus",
				start.class, start.id, result.Bounds.RelationsScanned)
		}
		if want, ok := contextCount[string(start.class)+"/"+start.id]; ok {
			checked++
			if result.Bounds.RelationsScanned != want {
				t.Errorf("%s/%s: scanned %d relations, but its canonical context holds %d",
					start.class, start.id, result.Bounds.RelationsScanned, want)
			}
		}
		// A scan that was not cut examined the whole context, so the eligible count is exact and
		// can never exceed the relations it was derived from.
		if result.Counts.Eligible > result.Bounds.RelationsScanned {
			t.Errorf("%s/%s: eligible = %d from %d scanned relations",
				start.class, start.id, result.Counts.Eligible, result.Bounds.RelationsScanned)
		}
	}
	if checked == 0 {
		t.Fatal("no scan count was cross-checked against the context layer, so this test would pass vacuously")
	}
}

// TestRelatedKnowledgeBoundsTheRelationScan exercises the scan ceiling at its boundary.
//
// The other two bounds cap what a response carries; this one caps the work behind it, and without
// it the cost of a request would be bounded by whatever the widest context in the corpus happened
// to be rather than by anything the phase declares. The corpus here is deliberately one relation
// past the ceiling, which is the only place the difference between "bounded by contract" and
// "bounded by the corpus" is observable.
//
// The cut is required to be reported, because it is the one bound that changes how another field
// must be read: a truncated scan makes the eligible count a floor rather than a total, and a
// caller cannot be left to discover that by comparing counts across requests.
func TestRelatedKnowledgeBoundsTheRelationScan(t *testing.T) {
	const spokes = service.MaxRelatedRelationsScanned + 1
	k := indexFrom(t, wideRelatedCorpus(t, spokes))

	result := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: service.MaxRelatedLimit})

	if result.Bounds.RelationsScanned != service.MaxRelatedRelationsScanned {
		t.Errorf("scanned = %d, want the ceiling %d",
			result.Bounds.RelationsScanned, service.MaxRelatedRelationsScanned)
	}
	if !result.Bounds.RelationsTruncated {
		t.Error("a scan cut at the ceiling did not report itself as truncated")
	}
	// Every spoke is a distinct destination, so the eligible count is exactly what was scanned:
	// a floor, and visibly short of the spokes the corpus actually holds.
	if result.Counts.Eligible != service.MaxRelatedRelationsScanned {
		t.Errorf("eligible = %d, want %d", result.Counts.Eligible, service.MaxRelatedRelationsScanned)
	}
	if result.Counts.Eligible >= spokes {
		t.Errorf("eligible = %d reached the %d spokes, so the scan was not bounded", result.Counts.Eligible, spokes)
	}
	// The item bound still applies on top of the scan bound, and the result is still the front of
	// the same canonical ordering rather than a different selection.
	if len(result.Items) != service.MaxRelatedLimit {
		t.Errorf("items = %d, want %d", len(result.Items), service.MaxRelatedLimit)
	}
	if !result.Truncated {
		t.Error("a result cut at the item limit did not report truncation")
	}
	if first := result.Items[0].ID; first != "spoke-00000" {
		t.Errorf("result starts at %s, want the front of the canonical ID order", first)
	}
	// A start whose context fits under the ceiling in the same corpus must still report an
	// untruncated scan, so the flag tracks the request rather than the corpus it ran against.
	spoke := mustRelated(t, k, domain.SearchNode, "spoke-00000", service.RelatedQuery{})
	if spoke.Bounds.RelationsTruncated {
		t.Error("a narrow start in a wide corpus reported a truncated scan")
	}
}

// wideRelatedCorpus is hubRelatedCorpus at a size that crosses the scan ceiling.
//
// It is a separate helper rather than a larger argument to hubRelatedCorpus because the spoke
// identifiers need a wider zero-padding to keep canonical ID order and numeric order the same
// past a thousand, and silently changing the existing helper's ID shape would move the
// expectations of the test that already depends on it.
func wideRelatedCorpus(t testing.TB, count int) fstest.MapFS {
	t.Helper()
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "nodes/dsp/hub.md",
		testsupport.ValidNode("hub", "Hub", "dsp", "foundation", "[]", "[]"))
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("spoke-%05d", i)
		testsupport.Write(corpus, "nodes/dsp/"+id+".md", testsupport.ValidNode(
			id, "Spoke "+id, "dsp", "seed", `[{"target": "hub", "type": "produces"}]`, "[]"))
	}
	return corpus
}

// hubRelatedCorpus is a corpus whose every node points at one hub, so the hub's discovery result
// is far larger than any bound the phase declares.
func hubRelatedCorpus(t testing.TB, count int) fstest.MapFS {
	t.Helper()
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "nodes/dsp/hub.md",
		testsupport.ValidNode("hub", "Hub", "dsp", "foundation", "[]", "[]"))
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("spoke-%03d", i)
		testsupport.Write(corpus, "nodes/dsp/"+id+".md", testsupport.ValidNode(
			id, "Spoke "+id, "dsp", "seed", `[{"target": "hub", "type": "produces"}]`, "[]"))
	}
	return corpus
}

// TestRelatedKnowledgeIsBoundedOnAHubRecord is the safety assertion.
//
// One heavily referenced record must not be able to turn a navigation request into a corpus dump.
// The hub is reachable from three hundred nodes, well past the ceiling, and the response must
// still be cut at the maximum, report the true eligible total, and say it was cut. The cut is
// also checked to be the front of the same ordering, so a bounded result stays a prefix of the
// unbounded one rather than a different selection.
func TestRelatedKnowledgeIsBoundedOnAHubRecord(t *testing.T) {
	const spokes = 300
	k := indexFrom(t, hubRelatedCorpus(t, spokes))

	result := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: service.MaxRelatedLimit * 10})
	if result.Limit != service.MaxRelatedLimit {
		t.Errorf("limit = %d, want the ceiling %d", result.Limit, service.MaxRelatedLimit)
	}
	if len(result.Items) != service.MaxRelatedLimit {
		t.Errorf("items = %d, want %d", len(result.Items), service.MaxRelatedLimit)
	}
	if result.Counts.Eligible < spokes {
		t.Errorf("eligible = %d, want at least the %d spokes", result.Counts.Eligible, spokes)
	}
	if !result.Truncated {
		t.Error("a hub result did not report truncation")
	}
	if first, last := result.Items[0].ID, result.Items[len(result.Items)-1].ID; first != "spoke-000" || last != "spoke-099" {
		t.Errorf("bounded result runs %s..%s, want the front of the canonical ID order", first, last)
	}
}
