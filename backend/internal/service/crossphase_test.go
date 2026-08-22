package service_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// practiceRecordIDs is every vocabulary entry, experiment and experiment run in the fixture
// corpus. It is written out rather than derived from the index so that a bug which dropped a
// practice record from the projection could not also silently empty this set.
var practiceRecordIDs = []string{
	"fixture-term",
	"fixture-companion",
	"fixture-orphan-term",
	"fixture-listening-exercise",
	"fixture-visualization-exercise",
	"fixture-listening-exercise-planned-a",
	"fixture-listening-exercise-completed-a",
}

// graphRoots returns every addressable traversal root in the fixture corpus, as (type, id).
//
// The list is built from the projections rather than hard-coded because its job is coverage:
// a practice record that leaked into the graph would have to appear from one of these roots,
// so the set must track the corpus rather than a snapshot of it.
func graphRoots(t testing.TB, k *service.Knowledge) [][2]string {
	t.Helper()
	roots := make([][2]string, 0)
	for _, n := range k.ListNodes(service.NodeQuery{Limit: 500}).Nodes {
		roots = append(roots, [2]string{"node", n.ID})
	}
	for _, s := range k.ListSessions(service.SessionQuery{Limit: 500}).Sessions {
		roots = append(roots, [2]string{"session", s.ID})
	}
	claims, err := k.ListClaims(service.ClaimQuery{Limit: 500})
	if err != nil {
		t.Fatalf("list claims: %v", err)
	}
	for _, c := range claims.Claims {
		roots = append(roots, [2]string{"claim", c.ID})
	}
	sources, err := k.ListSources(service.SourceQuery{Limit: 500})
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	for _, s := range sources.Sources {
		roots = append(roots, [2]string{"source", s.ID})
	}
	return roots
}

// TestPracticeRecordsAreNotTraversalEntities is the load-bearing cross-phase assertion.
//
// Phase 1D made the backend parse vocabulary entries, experiments and experiment runs, and
// resolve the claim references that name them. Phase 1C walks a bounded adjacency over the
// four addressable record classes. The rule joining them is that parsing a layer does not
// enrol it in the graph: no practice record may surface as a traversal entity or as either
// endpoint of a traversal relationship, from any root, at any depth.
//
// This is asserted by exhaustion rather than by sampling one root, because the failure mode
// it guards against — some future edge builder resolving a vocabulary or experiment_run ref
// into an EntityRef — would show up at whichever root happens to reach that claim.
func TestPracticeRecordsAreNotTraversalEntities(t *testing.T) {
	k := evidenceIndex(t)

	practice := make(map[string]bool, len(practiceRecordIDs))
	for _, id := range practiceRecordIDs {
		practice[id] = true
	}

	roots := graphRoots(t, k)
	if len(roots) == 0 {
		t.Fatal("no traversal roots found; the fixture corpus is not exercising this test")
	}

	check := func(where string, entities []domain.GraphEntity, edges []domain.GraphRelationship) {
		for _, e := range entities {
			if practice[e.ID] {
				t.Errorf("%s: practice record %q surfaced as a traversal entity (type %s)", where, e.ID, e.Type)
			}
			if !domain.ValidEntityType(string(e.Type)) {
				t.Errorf("%s: entity %q has non-canonical type %q", where, e.ID, e.Type)
			}
		}
		for _, edge := range edges {
			if practice[edge.From.ID] || practice[edge.To.ID] {
				t.Errorf("%s: practice record produced a traversal edge %s:%s -%s-> %s:%s",
					where, edge.From.Type, edge.From.ID, edge.Relationship, edge.To.Type, edge.To.ID)
			}
		}
	}

	for _, root := range roots {
		entityType, id := root[0], root[1]

		rels, err := k.EntityRelationshipsFor(entityType, id, service.TraversalQuery{})
		if err != nil {
			t.Fatalf("relationships for %s/%s: %v", entityType, id, err)
		}
		check("relationships "+entityType+"/"+id, rels.Neighbors, rels.Relationships)

		// Depth 3 is the documented maximum, so this reaches every entity the contract can
		// ever return from this root.
		for depth := 1; depth <= 3; depth++ {
			result, err := k.Traverse(entityType, id, service.TraversalQuery{Depth: depth})
			if err != nil {
				t.Fatalf("traverse %s/%s depth %d: %v", entityType, id, depth, err)
			}
			check("traverse "+entityType+"/"+id, result.Entities, result.Relationships)
		}
	}
}

// TestPracticeRecordsAreNotAddressableAsTraversalRoots is the other direction of the same
// rule. A practice ID must not be reachable as a root either, and the closed entity-type set
// must not have grown a practice class.
func TestPracticeRecordsAreNotAddressableAsTraversalRoots(t *testing.T) {
	for _, entityType := range []string{"vocabulary", "experiment", "experiment_run", "experiment-run"} {
		if domain.ValidEntityType(entityType) {
			t.Errorf("%q is addressable as a graph entity type, want it outside the closed set", entityType)
		}
	}

	k := evidenceIndex(t)
	if _, err := k.Traverse("vocabulary", "fixture-term", service.TraversalQuery{Depth: 1}); err == nil {
		t.Error("traversal accepted a vocabulary root, want rejection")
	}
	if _, err := k.Traverse("experiment", "fixture-listening-exercise", service.TraversalQuery{Depth: 1}); err == nil {
		t.Error("traversal accepted an experiment root, want rejection")
	}
}

// TestVocabularyReferenceResolvesWithoutBecomingAnEdge states the distinction the two phases
// have to hold together, on the fixture records where they meet.
//
// A claim naming a vocabulary entry in appears_in is resolved at load — Phase 1D made an
// unresolvable one fatal — and the resolution is served, as vocabulary/{id}.claim_ids. The
// same reference contributes nothing to that claim's traversal adjacency. Resolution and
// graph membership are different facts, and this test fails if either half stops being true:
// if the reference stopped resolving, or if it started producing an edge.
func TestVocabularyReferenceResolvesWithoutBecomingAnEdge(t *testing.T) {
	k := evidenceIndex(t)

	resolved := 0
	for _, id := range []string{"fixture-term", "fixture-companion", "fixture-orphan-term"} {
		detail, err := k.VocabularyByID(id)
		if err != nil {
			t.Fatalf("vocabulary detail %s: %v", id, err)
		}
		resolved += len(detail.ClaimIDs)

		for _, claimID := range detail.ClaimIDs {
			rels := relationshipsOf(t, k, "claim", claimID)
			for _, edge := range rels.Relationships {
				if edge.To.ID == id || edge.From.ID == id {
					t.Errorf("claim %q resolved vocabulary reference became traversal edge %+v", claimID, edge)
				}
			}
		}
	}

	if resolved == 0 {
		t.Fatal("no claim resolved to any fixture vocabulary entry; this test no longer proves anything")
	}
}

// TestRelationshipVocabularyIsUnchangedByThePracticeLayer guards the traversal filter
// vocabulary. It is derived from schemas/relationship-types.yaml plus the fixed cross-layer
// relations, and loading the practice layer must not have widened it: a relationship name a
// caller can filter on is part of the traversal contract.
func TestRelationshipVocabularyIsUnchangedByThePracticeLayer(t *testing.T) {
	k := evidenceIndex(t)

	for _, name := range k.RelationshipVocabulary() {
		switch name {
		case "references_vocabulary", "vocabulary_for", "related_term", "has_run",
			"run_of", "experiment_for", "references_experiment":
			t.Errorf("practice relation %q entered the traversal relationship vocabulary", name)
		}
	}

	// Filtering on a practice-shaped relation name must be refused as outside the closed
	// vocabulary, not silently answered with an empty traversal.
	if _, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: 1, Relationship: "related_term"}); err == nil {
		t.Error("traversal accepted a practice relationship filter, want rejection")
	}
}

// combinedIndex builds a second, independent index over the same fixture corpus.
func combinedIndex(t testing.TB) *service.Knowledge {
	t.Helper()
	repo, err := filesystem.NewFromFS(testsupport.CorpusFS(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open fixture corpus: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	return knowledge
}

// TestCombinedIndexIsDeterministic builds the whole thing twice and compares every layer.
//
// Both phases already test their own determinism. This one exists because they now share a
// constructor: buildTraversal and buildPractice both run in New, both walk k.claims, and Go
// map iteration is randomised. If either builder ever developed an order dependence on the
// other, the per-phase tests could still pass while the combined projection drifted.
func TestCombinedIndexIsDeterministic(t *testing.T) {
	a, b := combinedIndex(t), combinedIndex(t)

	if !reflect.DeepEqual(a.Graph(), b.Graph()) {
		t.Error("graph projection differs between two builds")
	}
	if !reflect.DeepEqual(a.RelationshipVocabulary(), b.RelationshipVocabulary()) {
		t.Error("relationship vocabulary differs between two builds")
	}
	if !reflect.DeepEqual(a.Project(), b.Project()) {
		t.Error("project summary differs between two builds")
	}
	if !reflect.DeepEqual(a.VocabularyDomains(), b.VocabularyDomains()) {
		t.Error("vocabulary domains differ between two builds")
	}
	if !reflect.DeepEqual(a.ExperimentRunTotals(), b.ExperimentRunTotals()) {
		t.Error("experiment run totals differ between two builds")
	}

	for _, root := range graphRoots(t, a) {
		entityType, id := root[0], root[1]
		ra, err := a.Traverse(entityType, id, service.TraversalQuery{Depth: 3})
		if err != nil {
			t.Fatalf("traverse %s/%s: %v", entityType, id, err)
		}
		rb, err := b.Traverse(entityType, id, service.TraversalQuery{Depth: 3})
		if err != nil {
			t.Fatalf("traverse %s/%s: %v", entityType, id, err)
		}
		if !reflect.DeepEqual(ra, rb) {
			t.Errorf("traversal from %s/%s differs between two builds", entityType, id)
		}
	}

	if !reflect.DeepEqual(listVocabulary(t, a, service.VocabularyQuery{}),
		listVocabulary(t, b, service.VocabularyQuery{})) {
		t.Error("vocabulary list differs between two builds")
	}
	if !reflect.DeepEqual(mustListExperiments(t, a, service.ExperimentQuery{}),
		mustListExperiments(t, b, service.ExperimentQuery{})) {
		t.Error("experiment list differs between two builds")
	}
	if !reflect.DeepEqual(mustListRuns(t, a, service.ExperimentRunQuery{}),
		mustListRuns(t, b, service.ExperimentRunQuery{})) {
		t.Error("experiment run list differs between two builds")
	}
}

// TestCombinedIndexHandsOutDefensiveCopies mutates what each layer returns and then re-reads
// through a second caller.
//
// Immutability is what makes the index safe to share across handlers without a lock, so it
// has to hold for every layer on the shared service, not only the layer whose phase added
// the test. A returned slice that aliased index state would let one request corrupt the next.
func TestCombinedIndexHandsOutDefensiveCopies(t *testing.T) {
	k := evidenceIndex(t)

	t.Run("relationship vocabulary", func(t *testing.T) {
		got := k.RelationshipVocabulary()
		if len(got) == 0 {
			t.Skip("no relationship names to mutate")
		}
		want := append([]string(nil), got...)
		got[0] = "mutated"
		if after := k.RelationshipVocabulary(); !reflect.DeepEqual(after, want) {
			t.Errorf("relationship vocabulary was mutated through a caller copy: %v", after)
		}
	})

	t.Run("traversal result", func(t *testing.T) {
		before, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: 3})
		if err != nil {
			t.Fatalf("traverse: %v", err)
		}
		want := append([]domain.GraphEntity(nil), before.Entities...)
		if len(before.Entities) > 0 {
			before.Entities[0] = domain.GraphEntity{ID: "mutated"}
		}
		if len(before.Relationships) > 0 {
			before.Relationships[0] = domain.GraphRelationship{Relationship: "mutated"}
		}
		after, err := k.Traverse("node", "alpha", service.TraversalQuery{Depth: 3})
		if err != nil {
			t.Fatalf("traverse: %v", err)
		}
		if !reflect.DeepEqual(after.Entities, want) {
			t.Error("traversal entities were mutated through a caller copy")
		}
	})

	t.Run("vocabulary reverse views", func(t *testing.T) {
		detail, err := k.VocabularyByID("fixture-term")
		if err != nil {
			t.Fatalf("vocabulary detail: %v", err)
		}
		wantClaims := append([]string(nil), detail.ClaimIDs...)
		wantExperiments := append([]string(nil), detail.ExperimentIDs...)
		for i := range detail.ClaimIDs {
			detail.ClaimIDs[i] = "mutated"
		}
		for i := range detail.ExperimentIDs {
			detail.ExperimentIDs[i] = "mutated"
		}
		again, err := k.VocabularyByID("fixture-term")
		if err != nil {
			t.Fatalf("vocabulary detail: %v", err)
		}
		if !reflect.DeepEqual(again.ClaimIDs, wantClaims) {
			t.Errorf("vocabulary claim_ids were mutated through a caller copy: %v", again.ClaimIDs)
		}
		if !reflect.DeepEqual(again.ExperimentIDs, wantExperiments) {
			t.Errorf("vocabulary experiment_ids were mutated through a caller copy: %v", again.ExperimentIDs)
		}
	})

	t.Run("experiment run ids and counts", func(t *testing.T) {
		detail, err := k.ExperimentByID("fixture-listening-exercise")
		if err != nil {
			t.Fatalf("experiment detail: %v", err)
		}
		want := append([]string(nil), detail.RunIDs...)
		wantCounts := detail.Runs
		for i := range detail.RunIDs {
			detail.RunIDs[i] = "mutated"
		}
		detail.Runs.Total = 9999
		again, err := k.ExperimentByID("fixture-listening-exercise")
		if err != nil {
			t.Fatalf("experiment detail: %v", err)
		}
		if !reflect.DeepEqual(again.RunIDs, want) {
			t.Errorf("experiment run_ids were mutated through a caller copy: %v", again.RunIDs)
		}
		if again.Runs != wantCounts {
			t.Errorf("experiment run counts were mutated through a caller copy: %+v", again.Runs)
		}
	})

	t.Run("bounded enum collections", func(t *testing.T) {
		domains := k.VocabularyDomains()
		if len(domains) == 0 {
			t.Skip("no vocabulary domains to mutate")
		}
		want := append([]string(nil), domains...)
		domains[0] = "mutated"
		if after := k.VocabularyDomains(); !reflect.DeepEqual(after, want) {
			t.Errorf("vocabulary domains were mutated through a caller copy: %v", after)
		}
	})
}

// TestDiscoveryDoesNotEnrolRecordsInTheGraph is the Phase 1E half of the practice-layer rule.
//
// Phase 1E makes vocabulary entries and experiment definitions findable through one
// cross-layer search surface. Discoverability and graph membership are different contracts,
// and conflating them is the specific mistake this phase was most able to make: it would be
// easy to reason that a record good enough to return from a search is a record good enough to
// traverse. This asserts the separation on the records a caller has just found — each is still
// refused as a traversal root, still absent from the graph, and still not an edge endpoint.
func TestDiscoveryDoesNotEnrolRecordsInTheGraph(t *testing.T) {
	k := evidenceIndex(t)
	before := k.Graph()

	found := map[domain.SearchEntityType][]string{}
	for _, result := range mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200}).Results {
		found[result.EntityType] = append(found[result.EntityType], result.ID)
	}

	for _, entityType := range []domain.SearchEntityType{domain.SearchVocabulary, domain.SearchExperiment} {
		if len(found[entityType]) == 0 {
			t.Fatalf("no %s results, so this test would pass vacuously", entityType)
		}
		for _, id := range found[entityType] {
			if _, err := k.Traverse(string(entityType), id, service.TraversalQuery{Depth: 1}); err == nil {
				t.Errorf("discoverable %s %q was accepted as a traversal root", entityType, id)
			}
			if _, err := k.EntityRelationshipsFor(string(entityType), id, service.TraversalQuery{}); err == nil {
				t.Errorf("discoverable %s %q was accepted as a relationship root", entityType, id)
			}
			for _, vertex := range before.Nodes {
				if vertex.ID == id {
					t.Errorf("discoverable %s %q appeared as a graph vertex", entityType, id)
				}
			}
			for _, edge := range before.Edges {
				if edge.Source == id || edge.Target == id {
					t.Errorf("discoverable %s %q appeared as a graph edge endpoint", entityType, id)
				}
			}
		}
	}

	// The graph is unchanged by searching. The index is immutable, so this can only fail if
	// the discovery projection acquired a side effect on a shared structure.
	if after := k.Graph(); !reflect.DeepEqual(before, after) {
		t.Error("the graph projection changed across search requests")
	}
}
