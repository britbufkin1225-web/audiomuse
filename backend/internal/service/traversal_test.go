package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// indexFrom builds a Knowledge index over an arbitrary in-memory corpus, so a traversal
// test that needs a shape the fixture does not have can synthesise exactly that shape
// without weakening the shared fixture for every other test.
func indexFrom(t testing.TB, corpus fstest.MapFS) *service.Knowledge {
	t.Helper()
	repo, err := filesystem.NewFromFS(corpus, testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open corpus: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	return knowledge
}

// relationshipsOf is the direct-neighbour call with no filters.
func relationshipsOf(t testing.TB, k *service.Knowledge, entityType, id string) service.EntityRelationships {
	t.Helper()
	result, err := k.EntityRelationshipsFor(entityType, id, service.TraversalQuery{})
	if err != nil {
		t.Fatalf("relationships for %s/%s: %v", entityType, id, err)
	}
	return result
}

// edgeStrings renders relationships as "from -relationship-> to" for readable assertions.
func edgeStrings(edges []domain.GraphRelationship) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, fmt.Sprintf("%s:%s -%s-> %s:%s", e.From.Type, e.From.ID, e.Relationship, e.To.Type, e.To.ID))
	}
	return out
}

func entityStrings(entities []domain.GraphEntity) []string {
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		out = append(out, fmt.Sprintf("%s:%s@%d", e.Type, e.ID, e.Distance))
	}
	return out
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestNodeRelationshipsCoverEveryCanonicalField asserts the exact adjacency of the fixture
// node that touches every derivation path at once: authored node edges, session origin,
// topical sources, and the reverse reads of two different claim fields.
//
// The order is asserted too, because the documented ordering — relationship, then target
// type in model order, then target ID — is part of the contract rather than an accident.
func TestNodeRelationshipsCoverEveryCanonicalField(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))
	result := relationshipsOf(t, k, "node", "alpha")

	want := []string{
		"node:alpha -appearance_site_of-> claim:alpha-carries-energy",
		"node:alpha -basis_for-> claim:gamma-follows-from-alpha-and-beta",
		"node:alpha -characterized_by-> node:gamma",
		"node:alpha -originates_in-> session:session-01-fixture",
		"node:alpha -produces-> node:beta",
		"node:alpha -sourced_from-> source:fixture-reference-work",
		"node:alpha -sourced_from-> source:session-01-fixture",
	}
	if got := edgeStrings(result.Relationships); !equalStrings(got, want) {
		t.Fatalf("alpha relationships =\n%v\nwant\n%v", got, want)
	}

	// Neighbours are ordered by model layer, not alphabetically by type.
	wantNeighbors := []string{
		"session:session-01-fixture@1",
		"node:beta@1",
		"node:gamma@1",
		"claim:alpha-carries-energy@1",
		"claim:gamma-follows-from-alpha-and-beta@1",
		"source:fixture-reference-work@1",
		"source:session-01-fixture@1",
	}
	if got := entityStrings(result.Neighbors); !equalStrings(got, wantNeighbors) {
		t.Errorf("alpha neighbours =\n%v\nwant\n%v", got, wantNeighbors)
	}
	if result.Entity.Label != "Alpha" || result.Entity.Distance != 0 {
		t.Errorf("root entity = %#v", result.Entity)
	}
	if result.Partial {
		t.Error("fixture neighbourhood reported partial")
	}
}

// TestAuthoredAndDerivedEdgesAreDistinguishable asserts that every edge says which
// canonical field it came from and whether it is authored or a reverse read. A reverse edge
// that looked authored would misrepresent the corpus.
func TestAuthoredAndDerivedEdgesAreDistinguishable(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	want := map[string]struct {
		origin  string
		derived bool
	}{
		"node:alpha -produces-> node:beta":                                {domain.OriginNodeRelationships, false},
		"node:alpha -originates_in-> session:session-01-fixture":          {domain.OriginNodeSessionOrigin, false},
		"node:alpha -sourced_from-> source:fixture-reference-work":        {domain.OriginNodeSources, false},
		"node:alpha -appearance_site_of-> claim:alpha-carries-energy":     {domain.OriginClaimAppearsIn, true},
		"node:alpha -basis_for-> claim:gamma-follows-from-alpha-and-beta": {domain.OriginClaimDerivedFrom, true},
	}

	for _, edge := range relationshipsOf(t, k, "node", "alpha").Relationships {
		key := edgeStrings([]domain.GraphRelationship{edge})[0]
		expected, ok := want[key]
		if !ok {
			continue
		}
		if edge.Origin != expected.origin || edge.Derived != expected.derived {
			t.Errorf("%s: origin=%q derived=%v, want origin=%q derived=%v",
				key, edge.Origin, edge.Derived, expected.origin, expected.derived)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("edge never emitted: %s", key)
	}
}

// TestReverseNodeEdgeUsesCanonicalInverseLabel asserts that the reverse of an authored node
// edge carries the inverse declared in schemas/relationship-types.yaml rather than a label
// this package invented.
func TestReverseNodeEdgeUsesCanonicalInverseLabel(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	found := false
	for _, edge := range relationshipsOf(t, k, "node", "beta").Relationships {
		if edge.To.ID == "alpha" && edge.To.Type == domain.EntityNode {
			found = true
			if edge.Relationship != "produced_by" {
				t.Errorf("beta -> alpha relationship = %q, want the canonical inverse %q",
					edge.Relationship, "produced_by")
			}
			if !edge.Derived {
				t.Error("reverse node edge is not marked derived")
			}
		}
	}
	if !found {
		t.Fatal("no reverse edge from beta to alpha")
	}
}

// TestEvidenceRelationsKeepTheirEpistemicRole asserts the requirement the whole provenance
// layer exists for: a source that supports a claim, a source that contradicts one, and a
// source merely relevant to a node must not become the same edge.
func TestEvidenceRelationsKeepTheirEpistemicRole(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	want := []string{
		"source:fixture-reference-work -contradicts-> claim:beta-was-observed-in-1999",
		"source:fixture-reference-work -qualifies-> claim:alpha-may-extend-to-gamma",
		"source:fixture-reference-work -qualifies-> claim:gamma-follows-from-alpha-and-beta",
		"source:fixture-reference-work -source_for-> node:alpha",
		"source:fixture-reference-work -source_for-> node:gamma",
		"source:fixture-reference-work -supports-> claim:alpha-carries-energy",
	}
	got := edgeStrings(relationshipsOf(t, k, "source", "fixture-reference-work").Relationships)
	if !equalStrings(got, want) {
		t.Fatalf("fixture-reference-work relationships =\n%v\nwant\n%v", got, want)
	}
}

// TestAttributionIsNotEvidence asserts that a source cited only through claim attribution
// reaches the claim by the attribution relation and by nothing else.
func TestAttributionIsNotEvidence(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	want := []string{"source:fixture-attribution-source -attribution_for-> claim:beta-was-observed-in-1999"}
	if got := edgeStrings(relationshipsOf(t, k, "source", "fixture-attribution-source").Relationships); !equalStrings(got, want) {
		t.Fatalf("attribution source relationships =\n%v\nwant\n%v", got, want)
	}
}

// TestClaimRelationshipsSpanEvidenceAndAppearance asserts the claim side of the model.
func TestClaimRelationshipsSpanEvidenceAndAppearance(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	want := []string{
		"claim:beta-was-observed-in-1999 -appears_in-> session:session-01-fixture",
		"claim:beta-was-observed-in-1999 -appears_in-> node:beta",
		"claim:beta-was-observed-in-1999 -attributed_to-> source:fixture-attribution-source",
		"claim:beta-was-observed-in-1999 -contradicted_by-> source:fixture-reference-work",
		"claim:beta-was-observed-in-1999 -supported_by-> source:fixture-archive-record",
	}
	if got := edgeStrings(relationshipsOf(t, k, "claim", "beta-was-observed-in-1999").Relationships); !equalStrings(got, want) {
		t.Fatalf("claim relationships =\n%v\nwant\n%v", got, want)
	}
}

// TestUnparsedReferenceKindsProduceNoEdge asserts that appears_in kinds naming layers the
// backend does not load — vocabulary and document — produce no graph entity. The fixture
// claim carries one of each alongside its node appearance.
func TestUnparsedReferenceKindsProduceNoEdge(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	want := []string{
		"claim:alpha-carries-energy -appears_in-> node:alpha",
		"claim:alpha-carries-energy -basis_for-> claim:gamma-follows-from-alpha-and-beta",
		"claim:alpha-carries-energy -supported_by-> source:fixture-archive-record",
		"claim:alpha-carries-energy -supported_by-> source:fixture-reference-work",
	}
	if got := edgeStrings(relationshipsOf(t, k, "claim", "alpha-carries-energy").Relationships); !equalStrings(got, want) {
		t.Fatalf("claim relationships =\n%v\nwant\n%v", got, want)
	}
}

// TestSessionRelationships asserts the session side: the contribution map plus the claims
// that name the session as an appearance site.
func TestSessionRelationships(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	want := []string{
		"session:session-01-fixture -appearance_site_of-> claim:beta-was-observed-in-1999",
		"session:session-01-fixture -contributed_to-> node:alpha",
		"session:session-01-fixture -contributed_to-> node:beta",
	}
	if got := edgeStrings(relationshipsOf(t, k, "session", "session-01-fixture").Relationships); !equalStrings(got, want) {
		t.Fatalf("session relationships =\n%v\nwant\n%v", got, want)
	}
}

// TestEntityWithNoRelationships asserts that an entity the corpus contains but nothing
// references answers with an empty list rather than an error. It is the counterpart of
// TestUnknownEntityIsNotFound: existing-with-no-edges and not-existing are different facts.
func TestEntityWithNoRelationships(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	for _, tc := range []struct{ entityType, id string }{
		{"source", "fixture-uncited-source"},
		{"session", "session-02-unused"},
	} {
		result := relationshipsOf(t, k, tc.entityType, tc.id)
		if len(result.Relationships) != 0 || len(result.Neighbors) != 0 {
			t.Errorf("%s/%s = %v", tc.entityType, tc.id, edgeStrings(result.Relationships))
		}
		if result.Entity.ID != tc.id {
			t.Errorf("%s/%s root = %#v", tc.entityType, tc.id, result.Entity)
		}
	}
}

func TestUnknownEntityIsNotFound(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	for _, tc := range []struct{ entityType, id string }{
		{"node", "not-a-real-node"},
		{"claim", "not-a-real-claim"},
		{"source", "not-a-real-source"},
		{"session", "not-a-real-session"},
		// A node ID is not automatically a claim ID: identity is (type, id).
		{"claim", "alpha"},
	} {
		if _, err := k.EntityRelationshipsFor(tc.entityType, tc.id, service.TraversalQuery{}); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("%s/%s error = %v, want ErrNotFound", tc.entityType, tc.id, err)
		}
		if _, err := k.Traverse(tc.entityType, tc.id, service.TraversalQuery{}); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("traverse %s/%s error = %v, want ErrNotFound", tc.entityType, tc.id, err)
		}
	}
}

func TestUnsupportedEntityType(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	for _, entityType := range []string{"banana", "vocabulary", "experiment_run", "Node", "", "nodes"} {
		if _, err := k.Traverse(entityType, "alpha", service.TraversalQuery{}); !errors.Is(err, service.ErrUnsupportedEntityType) {
			t.Errorf("entity type %q error = %v, want ErrUnsupportedEntityType", entityType, err)
		}
	}
}

// TestTraversalDepth asserts that depth means shortest hop distance and that the default is
// the direct neighbourhood.
func TestTraversalDepth(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	// Unspecified depth is the direct neighbourhood, and identical to an explicit depth 1.
	def, err := k.Traverse("session", "session-01-fixture", service.TraversalQuery{})
	if err != nil {
		t.Fatalf("default traverse: %v", err)
	}
	if def.Depth != service.DefaultTraversalDepth {
		t.Errorf("default depth = %d, want %d", def.Depth, service.DefaultTraversalDepth)
	}
	one, err := k.Traverse("session", "session-01-fixture", service.TraversalQuery{Depth: 1})
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	if !equalStrings(entityStrings(def.Entities), entityStrings(one.Entities)) {
		t.Error("default depth differs from explicit depth 1")
	}

	wantByDepth := map[int][]string{
		1: {
			"session:session-01-fixture@0",
			"node:alpha@1",
			"node:beta@1",
			"claim:beta-was-observed-in-1999@1",
		},
		2: {
			"session:session-01-fixture@0",
			"node:alpha@1",
			"node:beta@1",
			"claim:beta-was-observed-in-1999@1",
			"node:gamma@2",
			"claim:alpha-carries-energy@2",
			"claim:gamma-follows-from-alpha-and-beta@2",
			"source:fixture-archive-record@2",
			"source:fixture-attribution-source@2",
			"source:fixture-reference-work@2",
			"source:session-01-fixture@2",
		},
		3: {
			"session:session-01-fixture@0",
			"node:alpha@1",
			"node:beta@1",
			"claim:beta-was-observed-in-1999@1",
			"node:gamma@2",
			"claim:alpha-carries-energy@2",
			"claim:gamma-follows-from-alpha-and-beta@2",
			"source:fixture-archive-record@2",
			"source:fixture-attribution-source@2",
			"source:fixture-reference-work@2",
			"source:session-01-fixture@2",
			"claim:alpha-may-extend-to-gamma@3",
		},
	}
	for depth := 1; depth <= service.MaxTraversalDepth; depth++ {
		result, err := k.Traverse("session", "session-01-fixture", service.TraversalQuery{Depth: depth})
		if err != nil {
			t.Fatalf("depth %d: %v", depth, err)
		}
		if got := entityStrings(result.Entities); !equalStrings(got, wantByDepth[depth]) {
			t.Errorf("depth %d entities =\n%v\nwant\n%v", depth, got, wantByDepth[depth])
		}
		if result.Partial {
			t.Errorf("depth %d reported partial over the fixture corpus", depth)
		}
		if result.Counts.Entities != len(result.Entities) || result.Counts.Relationships != len(result.Relationships) {
			t.Errorf("depth %d counts = %#v", depth, result.Counts)
		}
	}
}

// TestTraversalRejectsDepthOutsideBounds asserts that an out-of-range depth is refused
// rather than clamped. A caller who asked for depth 9 and silently received depth 3 would
// believe they had seen the whole neighbourhood.
func TestTraversalRejectsDepthOutsideBounds(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	for _, depth := range []int{-1, -999, service.MaxTraversalDepth + 1, 999999} {
		_, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: depth})
		var depthErr *service.InvalidDepthError
		if !errors.As(err, &depthErr) {
			t.Errorf("depth %d error = %v, want InvalidDepthError", depth, err)
			continue
		}
		if depthErr.Min != service.MinTraversalDepth || depthErr.Max != service.MaxTraversalDepth {
			t.Errorf("depth %d bounds = %#v", depth, depthErr)
		}
	}
}

// TestTraversalTerminatesOnACycle uses an explicit three-node cycle. Without a visited set
// this walk would not terminate; with one, each entity is reported exactly once at its
// shortest distance and no edge is repeated.
func TestTraversalTerminatesOnACycle(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "nodes/dsp/cyc-a.md",
		testsupport.ValidNode("cyc-a", "Cycle A", "dsp", "seed", `[{"target": "cyc-b", "type": "produces"}]`, "[]"))
	testsupport.Write(corpus, "nodes/dsp/cyc-b.md",
		testsupport.ValidNode("cyc-b", "Cycle B", "dsp", "seed", `[{"target": "cyc-c", "type": "produces"}]`, "[]"))
	testsupport.Write(corpus, "nodes/dsp/cyc-c.md",
		testsupport.ValidNode("cyc-c", "Cycle C", "dsp", "seed", `[{"target": "cyc-a", "type": "produces"}]`, "[]"))
	k := indexFrom(t, corpus)

	result, err := k.Traverse("node", "cyc-a", service.TraversalQuery{Depth: service.MaxTraversalDepth})
	if err != nil {
		t.Fatalf("traverse cycle: %v", err)
	}

	// Every node of the cycle appears exactly once. cyc-b and cyc-c are both one hop from
	// cyc-a because the reverse edge cyc-a -produced_by-> cyc-c is also a hop.
	want := []string{"node:cyc-a@0", "node:cyc-b@1", "node:cyc-c@1"}
	if got := entityStrings(result.Entities); !equalStrings(got, want) {
		t.Fatalf("cycle entities =\n%v\nwant\n%v", got, want)
	}

	wantEdges := []string{
		"node:cyc-a -produced_by-> node:cyc-c",
		"node:cyc-a -produces-> node:cyc-b",
		"node:cyc-b -produced_by-> node:cyc-a",
		"node:cyc-b -produces-> node:cyc-c",
		"node:cyc-c -produced_by-> node:cyc-b",
		"node:cyc-c -produces-> node:cyc-a",
	}
	if got := edgeStrings(result.Relationships); !equalStrings(got, wantEdges) {
		t.Fatalf("cycle edges =\n%v\nwant\n%v", got, wantEdges)
	}
}

// TestDuplicateEdgesAreDeduplicated covers the two ways the same normalised edge can be
// produced twice: a claim that names the same node in both derived_from and appears_in
// yields two distinct relations rather than one duplicated one, and a traversal that
// reaches one entity by several routes still reports each edge once.
func TestDuplicateEdgesAreDeduplicated(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	// gamma is both the appearance site and the derivation basis of the same claim.
	want := []string{
		"node:gamma -appearance_site_of-> claim:alpha-may-extend-to-gamma",
		"node:gamma -appearance_site_of-> claim:gamma-follows-from-alpha-and-beta",
		"node:gamma -basis_for-> claim:alpha-may-extend-to-gamma",
		"node:gamma -characterizes-> node:alpha",
		"node:gamma -characterizes-> node:beta",
		"node:gamma -sourced_from-> source:fixture-reference-work",
	}
	result := relationshipsOf(t, k, "node", "gamma")
	if got := edgeStrings(result.Relationships); !equalStrings(got, want) {
		t.Fatalf("gamma relationships =\n%v\nwant\n%v", got, want)
	}
	// Six edges, five distinct neighbours: the claim reached by two relations is one entity.
	if len(result.Neighbors) != 5 {
		t.Errorf("gamma neighbours = %d, want 5", len(result.Neighbors))
	}

	// Across a whole traversal, no normalised edge is emitted twice.
	traversal, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: service.MaxTraversalDepth})
	if err != nil {
		t.Fatalf("traverse: %v", err)
	}
	seen := map[string]bool{}
	for _, key := range edgeStrings(traversal.Relationships) {
		if seen[key] {
			t.Errorf("duplicate edge in traversal: %s", key)
		}
		seen[key] = true
	}
	entities := map[string]bool{}
	for _, e := range traversal.Entities {
		key := string(e.Type) + ":" + e.ID
		if entities[key] {
			t.Errorf("duplicate entity in traversal: %s", key)
		}
		entities[key] = true
	}
}

// TestTraversalIsByteStable runs the same request repeatedly over one index and over two
// separately built indexes. Go map iteration is randomised, so an unsorted projection would
// be reproducible only by accident.
func TestTraversalIsByteStable(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	first, err := json.Marshal(mustTraverse(t, k, "node", "alpha", service.TraversalQuery{Depth: 3}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 0; i < 25; i++ {
		again, err := json.Marshal(mustTraverse(t, k, "node", "alpha", service.TraversalQuery{Depth: 3}))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("traversal %d differed from the first run", i)
		}
	}

	other := indexFrom(t, testsupport.MutableCorpus(t))
	rebuilt, err := json.Marshal(mustTraverse(t, other, "node", "alpha", service.TraversalQuery{Depth: 3}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(rebuilt) != string(first) {
		t.Error("a separately built index produced a different traversal")
	}

	relFirst, err := json.Marshal(relationshipsOf(t, k, "source", "fixture-reference-work"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 0; i < 25; i++ {
		relAgain, err := json.Marshal(relationshipsOf(t, k, "source", "fixture-reference-work"))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(relAgain) != string(relFirst) {
			t.Fatalf("relationship listing %d differed from the first run", i)
		}
	}
}

func mustTraverse(t testing.TB, k *service.Knowledge, entityType, id string, q service.TraversalQuery) service.Traversal {
	t.Helper()
	result, err := k.Traverse(entityType, id, q)
	if err != nil {
		t.Fatalf("traverse %s/%s: %v", entityType, id, err)
	}
	return result
}

// TestFiltersRestrictExpansion asserts that a filter bounds which edges are followed, not
// merely which are printed, so depth keeps meaning hops along matching edges.
func TestFiltersRestrictExpansion(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	// Only topical source edges: one hop reaches the two sources alpha names and nothing
	// else, and a second hop reaches nothing because sourced_from does not leave a source.
	byRelationship, err := k.Traverse("node", "alpha", service.TraversalQuery{
		Depth: service.MaxTraversalDepth, Relationship: domain.RelSourcedFrom,
	})
	if err != nil {
		t.Fatalf("filtered traverse: %v", err)
	}
	want := []string{
		"node:alpha@0",
		"source:fixture-reference-work@1",
		"source:session-01-fixture@1",
	}
	if got := entityStrings(byRelationship.Entities); !equalStrings(got, want) {
		t.Errorf("relationship filter entities =\n%v\nwant\n%v", got, want)
	}
	if byRelationship.Filters.Relationship != domain.RelSourcedFrom {
		t.Errorf("filters not echoed: %#v", byRelationship.Filters)
	}

	// Only claim targets: alpha's two claim neighbours, and no further hop because the
	// claims' own claim-typed edges are the only ones that survive the filter.
	byType, err := k.Traverse("node", "alpha", service.TraversalQuery{
		Depth: 2, TargetType: string(domain.EntityClaim),
	})
	if err != nil {
		t.Fatalf("filtered traverse: %v", err)
	}
	wantByType := []string{
		"node:alpha@0",
		"claim:alpha-carries-energy@1",
		"claim:gamma-follows-from-alpha-and-beta@1",
	}
	if got := entityStrings(byType.Entities); !equalStrings(got, wantByType) {
		t.Errorf("target_type filter entities =\n%v\nwant\n%v", got, wantByType)
	}

	// The same filters apply to the direct-neighbour endpoint.
	direct, err := k.EntityRelationshipsFor("node", "alpha", service.TraversalQuery{
		Relationship: domain.RelSourcedFrom, TargetType: string(domain.EntitySource),
	})
	if err != nil {
		t.Fatalf("filtered relationships: %v", err)
	}
	if len(direct.Relationships) != 2 {
		t.Errorf("filtered relationships = %v", edgeStrings(direct.Relationships))
	}
}

// TestInvalidFiltersAreRefused asserts that an unrecognised filter value is an error rather
// than an empty result, and that a recognised-but-unused relationship is the opposite.
func TestInvalidFiltersAreRefused(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	for _, tc := range []struct {
		name  string
		query service.TraversalQuery
		param string
	}{
		{"relationship", service.TraversalQuery{Relationship: "whatever-i-want"}, "relationship"},
		{"relationship case", service.TraversalQuery{Relationship: "Produces"}, "relationship"},
		{"target type", service.TraversalQuery{TargetType: "banana"}, "target_type"},
		{"target type layer", service.TraversalQuery{TargetType: "vocabulary"}, "target_type"},
	} {
		_, err := k.Traverse("node", "alpha", tc.query)
		var invalid *service.InvalidFilterError
		if !errors.As(err, &invalid) {
			t.Errorf("%s: error = %v, want InvalidFilterError", tc.name, err)
			continue
		}
		if invalid.Param != tc.param {
			t.Errorf("%s: param = %q, want %q", tc.name, invalid.Param, tc.param)
		}
		if len(invalid.Allowed) == 0 {
			t.Errorf("%s: no allowed values offered", tc.name)
		}
	}

	// "processes" is in the fixture relationship vocabulary but no fixture node uses it.
	// That is an empty neighbourhood, not a rejected filter.
	result, err := k.Traverse("node", "alpha", service.TraversalQuery{Relationship: "processes"})
	if err != nil {
		t.Fatalf("unused but valid relationship: %v", err)
	}
	if len(result.Relationships) != 0 || len(result.Entities) != 1 {
		t.Errorf("unused relationship returned %v", edgeStrings(result.Relationships))
	}
}

// TestRelationshipVocabularyIsContractDerived asserts the filter vocabulary comes from the
// loaded contracts, not from the edges that happen to exist.
func TestRelationshipVocabularyIsContractDerived(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))
	vocab := k.RelationshipVocabulary()

	set := map[string]bool{}
	for _, name := range vocab {
		set[name] = true
	}
	for _, want := range []string{
		// Canonical node relationship types and their declared inverses.
		"produces", "produced_by", "characterized_by", "characterizes", "processes", "processed_by",
		// Canonical claim evidence relations and their active-voice reverses.
		"supported_by", "supports", "contradicted_by", "contradicts", "qualified_by", "qualifies",
		// The cross-layer relations.
		domain.RelOriginatesIn, domain.RelContributedTo, domain.RelSourcedFrom, domain.RelSourceFor,
		domain.RelAttributedTo, domain.RelAttributionFor, domain.RelAppearsIn,
		domain.RelAppearanceSiteOf, domain.RelDerivedFrom, domain.RelBasisFor,
	} {
		if !set[want] {
			t.Errorf("relationship vocabulary is missing %q", want)
		}
	}
	for i := 1; i < len(vocab); i++ {
		if vocab[i-1] >= vocab[i] {
			t.Fatalf("relationship vocabulary is not sorted: %v", vocab)
		}
	}
}

// TestTraversalBoundsAreEnforced synthesises a hub wider than MaxTraversalEntities and
// asserts that the request is truncated explicitly. A silent truncation would be a wrong
// answer rather than a small one.
func TestTraversalBoundsAreEnforced(t *testing.T) {
	const fanOut = service.MaxTraversalEntities + 20

	corpus := testsupport.MutableCorpus(t)
	targets := make([]string, 0, fanOut)
	for i := 0; i < fanOut; i++ {
		id := fmt.Sprintf("fan-%04d", i)
		targets = append(targets, fmt.Sprintf(`{"target": "%s", "type": "produces"}`, id))
		testsupport.Write(corpus, "nodes/dsp/"+id+".md",
			testsupport.ValidNode(id, "Fan "+id, "dsp", "seed", "[]", "[]"))
	}
	testsupport.Write(corpus, "nodes/dsp/hub.md",
		testsupport.ValidNode("hub", "Hub", "dsp", "seed", "["+joinAll(targets)+"]", "[]"))
	k := indexFrom(t, corpus)

	result, err := k.Traverse("node", "hub", service.TraversalQuery{Depth: 1})
	if err != nil {
		t.Fatalf("traverse hub: %v", err)
	}
	if !result.Partial {
		t.Fatal("oversized traversal did not report partial")
	}
	if result.TruncationReason != service.TruncationEntityLimit {
		t.Errorf("truncation reason = %q, want %q", result.TruncationReason, service.TruncationEntityLimit)
	}
	if len(result.Entities) > service.MaxTraversalEntities {
		t.Errorf("entities = %d, above the bound of %d", len(result.Entities), service.MaxTraversalEntities)
	}
	if len(result.Relationships) > service.MaxTraversalEdges {
		t.Errorf("relationships = %d, above the bound of %d", len(result.Relationships), service.MaxTraversalEdges)
	}

	// A truncated result is still internally consistent: every relationship it reports
	// points at an entity it also lists, so a client can render what it received without
	// having to defend against a dangling reference.
	listed := map[string]bool{}
	for _, e := range result.Entities {
		listed[string(e.Type)+":"+e.ID] = true
	}
	for _, edge := range result.Relationships {
		for _, ref := range []struct {
			side string
			val  string
		}{
			{"from", string(edge.From.Type) + ":" + edge.From.ID},
			{"to", string(edge.To.Type) + ":" + edge.To.ID},
		} {
			if !listed[ref.val] {
				t.Fatalf("truncated traversal reports an edge whose %s entity %s is not listed", ref.side, ref.val)
			}
		}
	}

	// Truncation is deterministic: the same oversized request drops the same tail.
	repeat, err := k.Traverse("node", "hub", service.TraversalQuery{Depth: 1})
	if err != nil {
		t.Fatalf("repeat traverse: %v", err)
	}
	if !equalStrings(entityStrings(result.Entities), entityStrings(repeat.Entities)) {
		t.Error("truncated traversal was not reproducible")
	}

	// The direct-neighbour endpoint carries the same bound.
	direct, err := k.EntityRelationshipsFor("node", "hub", service.TraversalQuery{})
	if err != nil {
		t.Fatalf("hub relationships: %v", err)
	}
	if !direct.Partial || direct.TruncationReason != service.TruncationEntityLimit {
		t.Errorf("direct listing = partial:%v reason:%q", direct.Partial, direct.TruncationReason)
	}
	if len(direct.Neighbors) > service.MaxTraversalEntities {
		t.Errorf("neighbours = %d, above the bound", len(direct.Neighbors))
	}
	directListed := map[string]bool{}
	for _, e := range direct.Neighbors {
		directListed[string(e.Type)+":"+e.ID] = true
	}
	for _, edge := range direct.Relationships {
		if !directListed[string(edge.To.Type)+":"+edge.To.ID] {
			t.Fatalf("truncated listing reports an edge to unlisted neighbour %s:%s", edge.To.Type, edge.To.ID)
		}
	}
}

func joinAll(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// TestTraversalDoesNotMutateTheIndex asserts the read-only guarantee holds at the type
// level: mutating a returned result must not reach the immutable startup snapshot.
func TestTraversalDoesNotMutateTheIndex(t *testing.T) {
	k := indexFrom(t, testsupport.MutableCorpus(t))

	first := mustTraverse(t, k, "node", "alpha", service.TraversalQuery{Depth: 2})
	before := len(first.Relationships)
	for i := range first.Relationships {
		first.Relationships[i].Relationship = "tampered"
		first.Relationships[i].Origin = "tampered"
	}
	for i := range first.Entities {
		first.Entities[i].Label = "tampered"
	}

	second := mustTraverse(t, k, "node", "alpha", service.TraversalQuery{Depth: 2})
	if len(second.Relationships) != before {
		t.Fatalf("relationship count changed: %d then %d", before, len(second.Relationships))
	}
	for _, edge := range second.Relationships {
		if edge.Relationship == "tampered" || edge.Origin == "tampered" {
			t.Fatal("mutating a traversal result reached the startup index")
		}
	}
	for _, entity := range second.Entities {
		if entity.Label == "tampered" {
			t.Fatal("mutating a traversal result reached the startup index")
		}
	}
}
