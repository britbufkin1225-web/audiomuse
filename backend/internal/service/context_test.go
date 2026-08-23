package service_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// contextStrings renders one result's context as "-relation-> type/id (origin, derived)".
//
// Origin and the derived flag are part of the rendering rather than checked separately,
// because they are the two facts that keep a context item honest: which canonical field the
// relation was read from, and whether the record authored it or the backend read it backwards.
// An assertion that checked only relation and target would pass on a projection that had
// quietly started presenting derived views as authored data.
func contextStrings(resolved *domain.SearchResultContext) []string {
	if resolved == nil {
		return nil
	}
	out := make([]string, 0, len(resolved.Related))
	for _, relation := range resolved.Related {
		out = append(out, fmt.Sprintf("-%s-> %s/%s (%s, derived=%v)",
			relation.Relation, relation.Entity.EntityType, relation.Entity.ID,
			relation.Origin, relation.Derived))
	}
	return out
}

// contextOf runs one search restricted to a single record and returns its resolved context.
//
// The query is the record's own canonical ID, which is an id_exact match, so the record is
// always the first result and the helper never depends on ordering it is not testing.
func contextOf(t testing.TB, k *service.Knowledge, entityType domain.SearchEntityType, id string) *domain.SearchResultContext {
	t.Helper()
	results := mustSearch(t, k, service.SearchQuery{
		Q: id, Type: string(entityType), Limit: 200, IncludeContext: true,
	})
	for _, result := range results.Results {
		if result.EntityType == entityType && result.ID == id {
			if result.Context == nil {
				t.Fatalf("%s/%s was returned with no context object", entityType, id)
			}
			return result.Context
		}
	}
	t.Fatalf("%s/%s was not returned by a search for its own ID", entityType, id)
	return nil
}

// TestSearchWithoutContextIsUnchanged is the backwards-compatibility assertion.
//
// Phase 1F must not enlarge the response every existing caller already parses. A search that
// did not ask for context carries none at all — not an empty object, which a client could read
// as "this record references nothing" — and the echoed control is absent too, so a plain Phase
// 1E request produces the Phase 1E body byte for byte.
func TestSearchWithoutContextIsUnchanged(t *testing.T) {
	k := evidenceIndex(t)

	results := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200})
	if results.IncludeContext {
		t.Error("a search that did not ask for context echoed include_context")
	}
	if len(results.Results) == 0 {
		t.Fatal("the universal needle matched nothing, so this test would pass vacuously")
	}
	for _, result := range results.Results {
		if result.Context != nil {
			t.Errorf("result %s/%s carries context that was never requested", result.EntityType, result.ID)
		}
	}
}

// TestContextPreservesResultIdentityAndOrder is the load-bearing assertion of the phase.
//
// Context is metadata around a match, never a signal about one. Asking for it must not change
// which records matched, which order they arrived in, why they matched, or what they display.
// If a future change ever let a relationship count reach the ordering — "more provenance,
// higher rank" — this is what fails.
func TestContextPreservesResultIdentityAndOrder(t *testing.T) {
	k := evidenceIndex(t)

	for _, needle := range []string{universalNeedle, "fixture", "alpha", "synthetic"} {
		t.Run(needle, func(t *testing.T) {
			plain := mustSearch(t, k, service.SearchQuery{Q: needle, Limit: 200})
			enriched := mustSearch(t, k, service.SearchQuery{Q: needle, Limit: 200, IncludeContext: true})

			if !enriched.IncludeContext {
				t.Error("a search that asked for context did not echo include_context")
			}
			if plain.Page != enriched.Page {
				t.Errorf("page = %+v with context, %+v without", enriched.Page, plain.Page)
			}
			if got, want := searchRefs(enriched), searchRefs(plain); !reflect.DeepEqual(got, want) {
				t.Fatalf("context changed the result set or its order:\n%v\nwant\n%v", got, want)
			}
			// Strip the context and the two result sets must be identical values, which covers
			// title, summary, match kind and matched fields in one comparison.
			stripped := make([]domain.SearchResult, 0, len(enriched.Results))
			for _, result := range enriched.Results {
				result.Context = nil
				stripped = append(stripped, result)
			}
			if !reflect.DeepEqual(stripped, plain.Results) {
				t.Error("context resolution altered a field of the results themselves")
			}
		})
	}
}

// TestClaimContextResolvesItsProvenance is the provenance case the phase exists for.
//
// A claim hit is the result a reader can act on least without knowing what stands behind it.
// The exact set is asserted, including the two facts a weaker assertion would lose: a
// contradicting source is presented exactly like a supporting one under its own canonical
// relation, and attribution stays a separate relation from evidence, because who credits a
// statement is not what stands behind it.
func TestClaimContextResolvesItsProvenance(t *testing.T) {
	k := evidenceIndex(t)

	// The disputed fixture claim: supported by one source, contradicted by another, credited
	// through a third, and appearing in both a node and a session.
	want := []string{
		"-appears_in-> session/session-01-fixture (claim.appears_in, derived=false)",
		"-appears_in-> node/beta (claim.appears_in, derived=false)",
		"-attributed_to-> source/fixture-attribution-source (claim.attribution, derived=false)",
		"-contradicted_by-> source/fixture-reference-work (claim.evidence, derived=false)",
		"-supported_by-> source/fixture-archive-record (claim.evidence, derived=false)",
	}
	got := contextStrings(contextOf(t, k, domain.SearchClaim, "beta-was-observed-in-1999"))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("claim context =\n%v\nwant\n%v", got, want)
	}

	// Nothing in the context ranks or scores the provenance it resolved. The contradicting and
	// the supporting source are the same kind of object, distinguished only by the canonical
	// relation each record authored.
	resolved := contextOf(t, k, domain.SearchClaim, "beta-was-observed-in-1999")
	if resolved.Count != len(want) || resolved.Returned != len(want) || resolved.Truncated {
		t.Errorf("counts = %+v, want a complete context of %d", resolved, len(want))
	}
}

// TestSourceContextResolvesWhatCitesIt is the reverse provenance read.
//
// A source hit answers "what rests on this", which is a different question from "what does
// this rest on" and is why the reverse relations exist at all. Every item here is derived: a
// registry entry does not author a list of the claims that cite it.
func TestSourceContextResolvesWhatCitesIt(t *testing.T) {
	k := evidenceIndex(t)

	want := []string{
		"-contradicts-> claim/beta-was-observed-in-1999 (claim.evidence, derived=true)",
		"-qualifies-> claim/alpha-may-extend-to-gamma (claim.evidence, derived=true)",
		"-qualifies-> claim/gamma-follows-from-alpha-and-beta (claim.evidence, derived=true)",
		"-referenced_by-> experiment/fixture-listening-exercise (experiment.source_refs, derived=true)",
		"-source_for-> node/alpha (node.sources, derived=true)",
		"-source_for-> node/gamma (node.sources, derived=true)",
		"-supports-> claim/alpha-carries-energy (claim.evidence, derived=true)",
	}
	if got := contextStrings(contextOf(t, k, domain.SearchSource, "fixture-reference-work")); !reflect.DeepEqual(got, want) {
		t.Errorf("source context =\n%v\nwant\n%v", got, want)
	}
}

// TestNodeContextCoversEveryDerivationPath asserts the node that touches all of them at once:
// authored node edges and their reverses, session origin, topical sources, the reverse reads of
// two claim fields, and the practice-layer references the traversal graph does not model.
//
// Topical provenance and claim evidence stay separate relations throughout. node.sources says
// "this source is relevant to this concept" and claim.evidence says "this source materially
// supports this statement"; merging them would erase the distinction the provenance layer
// exists to make.
func TestNodeContextCoversEveryDerivationPath(t *testing.T) {
	k := evidenceIndex(t)

	want := []string{
		"-appearance_site_of-> claim/alpha-carries-energy (claim.appears_in, derived=true)",
		"-basis_for-> claim/gamma-follows-from-alpha-and-beta (claim.derived_from, derived=true)",
		"-characterized_by-> node/gamma (node.relationships, derived=false)",
		"-originates_in-> session/session-01-fixture (node.session_origin, derived=false)",
		"-produces-> node/beta (node.relationships, derived=false)",
		"-referenced_by-> vocabulary/fixture-term (vocabulary.node_refs, derived=true)",
		"-referenced_by-> experiment/fixture-listening-exercise (experiment.node_refs, derived=true)",
		"-sourced_from-> source/fixture-reference-work (node.sources, derived=false)",
		"-sourced_from-> source/session-01-fixture (node.sources, derived=false)",
	}
	if got := contextStrings(contextOf(t, k, domain.SearchNode, "alpha")); !reflect.DeepEqual(got, want) {
		t.Errorf("node context =\n%v\nwant\n%v", got, want)
	}
}

// TestVocabularyContextResolvesWhereTheTermIsUsed covers the class that motivated the phase.
//
// A vocabulary hit is the result least useful on its own: a term and a definition say nothing
// about where the corpus uses the word. Its context names the node and session the entry
// itself points at, the experiment that cites it, the claim that appears in it, and its
// authored related term — every one of them a reference some record wrote down.
func TestVocabularyContextResolvesWhereTheTermIsUsed(t *testing.T) {
	k := evidenceIndex(t)

	want := []string{
		"-appearance_site_of-> claim/alpha-carries-energy (claim.appears_in, derived=true)",
		"-referenced_by-> experiment/fixture-listening-exercise (experiment.vocabulary_refs, derived=true)",
		"-references-> session/session-01-fixture (vocabulary.session_refs, derived=false)",
		"-references-> node/alpha (vocabulary.node_refs, derived=false)",
		"-related_term-> vocabulary/fixture-companion (vocabulary.related_terms, derived=false)",
	}
	if got := contextStrings(contextOf(t, k, domain.SearchVocabulary, "fixture-term")); !reflect.DeepEqual(got, want) {
		t.Errorf("vocabulary context =\n%v\nwant\n%v", got, want)
	}

	// No definition is generated. The label of a resolved entity is canonical record content,
	// never prose the backend composed about it.
	entry, err := k.VocabularyByID("fixture-term")
	if err != nil {
		t.Fatalf("vocabulary lookup: %v", err)
	}
	for _, relation := range contextOf(t, k, domain.SearchVocabulary, "fixture-term").Related {
		if relation.Entity.Label == entry.Definition {
			t.Error("a context label carried the searching record's own definition")
		}
	}
}

// TestSessionContextResolvesItsContributions covers the chronological class.
//
// A session authors nothing about the nodes that came out of it: docs/knowledge-model.md
// describes node session_origin as a many-to-many contribution map, so every node here arrives
// as a derived reverse read rather than as something the session declared.
func TestSessionContextResolvesItsContributions(t *testing.T) {
	k := evidenceIndex(t)

	want := []string{
		"-appearance_site_of-> claim/beta-was-observed-in-1999 (claim.appears_in, derived=true)",
		"-contributed_to-> node/alpha (node.session_origin, derived=true)",
		"-contributed_to-> node/beta (node.session_origin, derived=true)",
		"-referenced_by-> vocabulary/fixture-term (vocabulary.session_refs, derived=true)",
		"-referenced_by-> experiment/fixture-listening-exercise (experiment.session_refs, derived=true)",
	}
	if got := contextStrings(contextOf(t, k, domain.SearchSession, "session-01-fixture")); !reflect.DeepEqual(got, want) {
		t.Errorf("session context =\n%v\nwant\n%v", got, want)
	}

	// A registered session and its registry entry share an ID and are two projections of one
	// record. Their contexts are addressed separately and are genuinely different, which is why
	// a context ref is never resolved by ID alone.
	entry := contextStrings(contextOf(t, k, domain.SearchSource, "session-01-fixture"))
	wantEntry := []string{
		"-source_for-> node/alpha (node.sources, derived=true)",
		"-source_for-> node/beta (node.sources, derived=true)",
	}
	if !reflect.DeepEqual(entry, wantEntry) {
		t.Errorf("registry-entry context =\n%v\nwant\n%v", entry, wantEntry)
	}
}

// TestExperimentContextStopsAtTheDefinition is the practice-layer boundary.
//
// An experiment definition resolves the records it references and nothing downstream of it. No
// run appears, in any relation, under any origin: a run is not a searchable class, its prose is
// observation and measurement, and letting a definition's context reach it would be the route
// by which evidence-bearing text re-entered generic search.
func TestExperimentContextStopsAtTheDefinition(t *testing.T) {
	k := evidenceIndex(t)

	want := []string{
		"-references-> session/session-01-fixture (experiment.session_refs, derived=false)",
		"-references-> node/alpha (experiment.node_refs, derived=false)",
		"-references-> node/beta (experiment.node_refs, derived=false)",
		"-references-> source/fixture-reference-work (experiment.source_refs, derived=false)",
		"-references-> vocabulary/fixture-term (experiment.vocabulary_refs, derived=false)",
		"-related_experiment-> experiment/fixture-visualization-exercise (experiment.related_experiments, derived=false)",
	}
	resolved := contextOf(t, k, domain.SearchExperiment, "fixture-listening-exercise")
	if got := contextStrings(resolved); !reflect.DeepEqual(got, want) {
		t.Errorf("experiment context =\n%v\nwant\n%v", got, want)
	}

	detail, err := k.ExperimentByID("fixture-listening-exercise")
	if err != nil {
		t.Fatalf("experiment lookup: %v", err)
	}
	if len(detail.RunIDs) == 0 {
		t.Fatal("the fixture definition has no runs, so this assertion is vacuous")
	}
	for _, relation := range resolved.Related {
		for _, runID := range detail.RunIDs {
			if relation.Entity.ID == runID {
				t.Errorf("experiment context resolved its run %q", runID)
			}
		}
	}
}

// TestContextNamesOnlySearchableClasses is the structural half of the same boundary.
//
// Every entity any context can name is one of the six searchable classes, so an experiment run
// cannot appear anywhere in the projection whatever field pointed at it. A claim's
// appears_in: document reference is checked too: it names no addressable layer, so it resolves
// to nothing rather than to a ref a client could not follow.
func TestContextNamesOnlySearchableClasses(t *testing.T) {
	k := evidenceIndex(t)

	runIDs := map[string]bool{}
	for _, run := range mustListRuns(t, k, service.ExperimentRunQuery{Limit: service.MaxLimit}).Runs {
		runIDs[run.ID] = true
	}
	if len(runIDs) == 0 {
		t.Fatal("the fixture corpus has no runs, so this test would pass vacuously")
	}

	documentRefs := map[string]bool{}
	for _, claim := range mustListClaims(t, k, service.ClaimQuery{Limit: service.MaxLimit}).Claims {
		detail, err := k.ClaimByID(claim.ID)
		if err != nil {
			t.Fatalf("claim lookup: %v", err)
		}
		for _, ref := range detail.AppearsIn {
			if ref.Kind == domain.ClaimKindDocument {
				documentRefs[ref.Ref] = true
			}
		}
	}
	if len(documentRefs) == 0 {
		t.Fatal("no fixture claim carries a document reference, so this test would pass vacuously")
	}

	results := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200, IncludeContext: true})
	seen := 0
	for _, result := range results.Results {
		for _, relation := range result.Context.Related {
			seen++
			if !domain.ValidSearchEntityType(string(relation.Entity.EntityType)) {
				t.Errorf("context named unsupported class %q", relation.Entity.EntityType)
			}
			if runIDs[relation.Entity.ID] {
				t.Errorf("context named experiment run %q as a %s", relation.Entity.ID, relation.Entity.EntityType)
			}
			if documentRefs[relation.Entity.ID] {
				t.Errorf("context named unresolvable document %q", relation.Entity.ID)
			}
			if relation.Entity.ID == "" || relation.Relation == "" || relation.Origin == "" {
				t.Errorf("context item %+v is missing its identity, relation or origin", relation)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no context was resolved at all, so this test would pass vacuously")
	}
}

// TestContextAgreesWithTraversalAdjacency is the anti-drift assertion between the two layers.
//
// One repository connection must have one meaning. Every context item whose target is a graph
// class must be an edge the Phase 1C adjacency already reports for the same record, with the
// same relation name, the same canonical origin and the same authored-or-derived flag. A future
// change that re-derived those relations here rather than reusing the adjacency would produce a
// second, quietly different answer to "how is this connected", and would fail here.
//
// The agreement is asserted in both directions. Containment alone would catch an invented
// relation but not a dropped one, and the resolver does have a place where a relation can
// disappear silently: it skips any target with no discovery document. That guard is meant to be
// unreachable — every graph class is searchable, so every adjacency target resolves — and an
// untested "cannot happen" is exactly how a projection starts quietly serving less than the
// corpus states. Records whose context was truncated are compared for containment only, since a
// bounded list is a deliberate subset rather than a drop.
func TestContextAgreesWithTraversalAdjacency(t *testing.T) {
	k := evidenceIndex(t)

	results := mustSearch(t, k, service.SearchQuery{Q: universalNeedle, Limit: 200, IncludeContext: true})
	compared := 0
	for _, result := range results.Results {
		graphType, ok := map[domain.SearchEntityType]string{
			domain.SearchSession: "session",
			domain.SearchNode:    "node",
			domain.SearchClaim:   "claim",
			domain.SearchSource:  "source",
		}[result.EntityType]
		if !ok {
			continue
		}
		edges := map[string]bool{}
		for _, edge := range relationshipsOf(t, k, graphType, result.ID).Relationships {
			edges[fmt.Sprintf("%s|%s|%s|%s|%v",
				edge.Relationship, edge.To.Type, edge.To.ID, edge.Origin, edge.Derived)] = true
		}
		resolved := map[string]bool{}
		for _, relation := range result.Context.Related {
			if relation.Entity.EntityType == domain.SearchVocabulary ||
				relation.Entity.EntityType == domain.SearchExperiment {
				// Deliberately outside the graph; see TestContextDoesNotEnrolRecordsInTheGraph.
				continue
			}
			key := fmt.Sprintf("%s|%s|%s|%s|%v",
				relation.Relation, relation.Entity.EntityType, relation.Entity.ID,
				relation.Origin, relation.Derived)
			if !edges[key] {
				t.Errorf("%s/%s context item %q is not a canonical traversal edge",
					result.EntityType, result.ID, key)
			}
			resolved[key] = true
			compared++
		}

		// The reverse direction: nothing the adjacency reports may go missing from an untruncated
		// context.
		if result.Context.Truncated {
			continue
		}
		for key := range edges {
			if !resolved[key] {
				t.Errorf("%s/%s context dropped the canonical traversal edge %q",
					result.EntityType, result.ID, key)
			}
		}
	}
	if compared == 0 {
		t.Fatal("no graph-class context was compared, so this test would pass vacuously")
	}
}

// TestContextDoesNotEnrolRecordsInTheGraph is the Phase 1F half of the practice-layer rule.
//
// Phase 1E asserted that a record being findable does not make it traversable. Phase 1F makes
// the same records reachable as context refs, which is the more tempting version of the same
// mistake: an entity named in a relationship looks exactly like a graph vertex. It is not. A
// vocabulary entry or experiment definition that appears in a context is still refused as a
// traversal root, still absent from the graph, and still not an edge endpoint.
func TestContextDoesNotEnrolRecordsInTheGraph(t *testing.T) {
	k := evidenceIndex(t)
	before := k.Graph()

	named := map[domain.SearchEntityType][]string{}
	for _, result := range mustSearch(t, k, service.SearchQuery{
		Q: universalNeedle, Limit: 200, IncludeContext: true,
	}).Results {
		for _, relation := range result.Context.Related {
			switch relation.Entity.EntityType {
			case domain.SearchVocabulary, domain.SearchExperiment:
				named[relation.Entity.EntityType] = append(
					named[relation.Entity.EntityType], relation.Entity.ID)
			}
		}
	}

	for _, entityType := range []domain.SearchEntityType{domain.SearchVocabulary, domain.SearchExperiment} {
		if len(named[entityType]) == 0 {
			t.Fatalf("no %s was named as context, so this test would pass vacuously", entityType)
		}
		for _, id := range named[entityType] {
			if _, err := k.Traverse(string(entityType), id, service.TraversalQuery{Depth: 1}); err == nil {
				t.Errorf("context-named %s %q was accepted as a traversal root", entityType, id)
			}
			for _, vertex := range before.Nodes {
				if vertex.ID == id {
					t.Errorf("context-named %s %q appeared as a graph vertex", entityType, id)
				}
			}
			for _, edge := range before.Edges {
				if edge.Source == id || edge.Target == id {
					t.Errorf("context-named %s %q appeared as a graph edge endpoint", entityType, id)
				}
			}
		}
	}

	if after := k.Graph(); !reflect.DeepEqual(before, after) {
		t.Error("the graph projection changed across context requests")
	}
}

// TestContextIsEmptyRatherThanAbsentForAnUnreferencedRecord covers the deterministic empty case.
//
// The fixture corpus carries records nothing refers to on purpose. Each must resolve to an
// empty context rather than to a missing object or an error: "this record references nothing
// and nothing references it" is an answer, and it has to stay distinguishable from "context was
// not requested".
func TestContextIsEmptyRatherThanAbsentForAnUnreferencedRecord(t *testing.T) {
	k := evidenceIndex(t)

	for _, unreferenced := range []struct {
		entityType domain.SearchEntityType
		id         string
	}{
		{domain.SearchVocabulary, "fixture-orphan-term"},
		{domain.SearchSource, "fixture-uncited-source"},
		{domain.SearchSession, "session-02-unused"},
	} {
		t.Run(string(unreferenced.entityType)+"/"+unreferenced.id, func(t *testing.T) {
			resolved := contextOf(t, k, unreferenced.entityType, unreferenced.id)
			if resolved.Related == nil {
				t.Error("an empty context serialised as a null list rather than an empty one")
			}
			if len(resolved.Related) != 0 || resolved.Count != 0 || resolved.Returned != 0 {
				t.Errorf("context = %+v, want an empty one", resolved)
			}
			if resolved.Truncated {
				t.Error("an empty context reported itself as truncated")
			}
		})
	}
}

// TestContextOrderIsStableAcrossRuns pins reproducibility.
//
// The projection is assembled from several loops and from an adjacency ordered by a different
// class ranking, and Go map iteration is randomised, so a missing sort would show up here as an
// occasional difference rather than as a consistent one. Rebuilding the index from scratch
// covers the build order too, not only the per-request read.
func TestContextOrderIsStableAcrossRuns(t *testing.T) {
	first := mustSearch(t, evidenceIndex(t), service.SearchQuery{
		Q: universalNeedle, Limit: 200, IncludeContext: true,
	})
	for i := 0; i < 8; i++ {
		again := mustSearch(t, evidenceIndex(t), service.SearchQuery{
			Q: universalNeedle, Limit: 200, IncludeContext: true,
		})
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("context differed on rebuild %d", i)
		}
	}
}

// hubCorpus writes a ring of densely connected nodes into a copy of the fixture.
//
// Each node points at the next degree nodes around the ring, so it ends up with degree authored
// relations and degree reverse ones: enough canonical context to exceed what one result is
// allowed to carry. The fixture corpus deliberately has no record like this and the live corpus
// only just reaches the per-result cap, so a synthetic cluster is the only honest way to test
// bounds against the shapes they exist for rather than against the shapes that exist today.
func hubCorpus(t testing.TB, count, degree int) fstest.MapFS {
	t.Helper()
	corpus := testsupport.MutableCorpus(t)
	for i := 0; i < count; i++ {
		edges := make([]string, 0, degree)
		for step := 1; step <= degree; step++ {
			edges = append(edges, fmt.Sprintf(`{"target": "hub-%03d", "type": "produces"}`, (i+step)%count))
		}
		id := fmt.Sprintf("hub-%03d", i)
		testsupport.Write(corpus, "nodes/dsp/"+id+".md", testsupport.ValidNode(
			id, "Hub "+id, "dsp", "seed", "["+strings.Join(edges, ", ")+"]", "[]"))
	}
	return corpus
}

// TestContextIsBoundedPerResult asserts one heavily referenced record cannot fill a page.
//
// The record still reports its full Count, so a caller can tell that it was cut short and can
// read the rest through the record's own route or through traversal. A truncated context that
// claimed to be complete would be a wrong answer rather than a small one.
func TestContextIsBoundedPerResult(t *testing.T) {
	k := indexFrom(t, hubCorpus(t, 20, 13))

	resolved := contextOf(t, k, domain.SearchNode, "hub-000")
	if resolved.Count <= service.MaxContextRelationsPerResult {
		t.Fatalf("the hub node has only %d relations, so the bound is not exercised", resolved.Count)
	}
	if resolved.Returned != service.MaxContextRelationsPerResult {
		t.Errorf("returned = %d, want the per-result cap of %d",
			resolved.Returned, service.MaxContextRelationsPerResult)
	}
	if len(resolved.Related) != resolved.Returned {
		t.Errorf("returned = %d but %d items were carried", resolved.Returned, len(resolved.Related))
	}
	if !resolved.Truncated {
		t.Error("a truncated context did not report itself as truncated")
	}

	// Truncation keeps the head of the ordered list, so it is deterministic rather than an
	// arbitrary subset: the same request twice returns the same items in the same order.
	if again := contextOf(t, k, domain.SearchNode, "hub-000"); !reflect.DeepEqual(resolved, again) {
		t.Error("a truncated context was not reproducible")
	}
}

// TestContextIsBoundedPerResponse asserts the whole-response bound.
//
// A per-result cap alone multiplies by the page size, so a full page of well-connected records
// would still be an unbounded payload. The budget is spent in result order, which keeps the
// context of the highest-precedence hits intact rather than thinning every result equally.
func TestContextIsBoundedPerResponse(t *testing.T) {
	// Enough hub nodes that the per-result cap alone would still exceed the response budget.
	count := service.MaxContextRelationsPerResponse/service.MaxContextRelationsPerResult + 5
	k := indexFrom(t, hubCorpus(t, count, 13))

	results := mustSearch(t, k, service.SearchQuery{
		Q: "hub-", Type: string(domain.SearchNode), Limit: service.MaxLimit, IncludeContext: true,
	})
	if len(results.Results) != count {
		t.Fatalf("%d hub nodes matched, want all %d", len(results.Results), count)
	}

	total, exhausted := 0, false
	for _, result := range results.Results {
		total += result.Context.Returned
		if exhausted && result.Context.Returned != 0 {
			t.Errorf("%s carried context after the response budget was spent", result.ID)
		}
		if result.Context.Returned < result.Context.Count {
			if !result.Context.Truncated {
				t.Errorf("%s dropped context without reporting truncation", result.ID)
			}
			exhausted = exhausted || result.Context.Returned == 0
		}
	}
	if total != service.MaxContextRelationsPerResponse {
		t.Errorf("response carried %d context items, want the cap of %d",
			total, service.MaxContextRelationsPerResponse)
	}
	if !exhausted {
		t.Error("the response budget was never exhausted, so the bound is not exercised")
	}
}

// TestContextIsNotAMatchSurface asserts context text can never make a record match.
//
// Context resolves records the hit references; it does not make their text searchable. A query
// that appears only inside a related record's fields must not return the referring record,
// because a hit that could not be explained by one of the referring record's own fields would
// contradict matched_fields, which is the evidence the discovery layer serves with every result.
func TestContextIsNotAMatchSurface(t *testing.T) {
	k := evidenceIndex(t)

	// This phrase lives only in one claim's statement. The node that claim appears in, and the
	// sources it cites, carry it nowhere in their own fields.
	const needle = "without transporting the medium itself"
	results := mustSearch(t, k, service.SearchQuery{Q: needle, Limit: 200, IncludeContext: true})
	if len(results.Results) != 1 {
		t.Fatalf("needle matched %v, want exactly the one claim that states it", searchRefs(results))
	}
	if results.Results[0].EntityType != domain.SearchClaim {
		t.Errorf("needle matched a %s, want a claim", results.Results[0].EntityType)
	}

	// The same query with context off returns the same single hit, so asking for context did
	// not widen the corpus that was searched.
	plain := mustSearch(t, k, service.SearchQuery{Q: needle, Limit: 200})
	if !reflect.DeepEqual(searchRefs(plain), searchRefs(results)) {
		t.Error("context resolution changed which records matched")
	}
}

// TestContextHandsOutDefensiveCopies mutates a response and re-reads the index.
//
// Knowledge is immutable and is safe for concurrent readers because of it. A context list that
// shared the index's own backing array would let one request's caller corrupt the next
// request's answer, so the guarantee is asserted rather than assumed.
func TestContextHandsOutDefensiveCopies(t *testing.T) {
	k := evidenceIndex(t)

	before := contextStrings(contextOf(t, k, domain.SearchNode, "alpha"))
	resolved := contextOf(t, k, domain.SearchNode, "alpha")
	if len(resolved.Related) == 0 {
		t.Fatal("alpha resolved no context, so this test would pass vacuously")
	}
	for i := range resolved.Related {
		resolved.Related[i].Relation = "mutated"
		resolved.Related[i].Entity.ID = "mutated"
		resolved.Related[i].Entity.Label = "mutated"
		resolved.Related[i].Origin = "mutated"
		resolved.Related[i].Derived = !resolved.Related[i].Derived
	}
	resolved.Related = append(resolved.Related, domain.ContextRelation{Relation: "appended"})

	if after := contextStrings(contextOf(t, k, domain.SearchNode, "alpha")); !reflect.DeepEqual(after, before) {
		t.Errorf("the index changed after a caller mutated a response:\n%v\nwant\n%v", after, before)
	}
}
