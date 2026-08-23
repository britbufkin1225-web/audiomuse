package service

import (
	"sort"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Phase 1F: the search-result context resolver.
//
// Phase 1E answers "where in the corpus does this text appear". This file answers the question
// a reader asks immediately afterwards — "and what is that record connected to" — without
// making them guess which of eight routes to try next.
//
// It resolves rather than infers. Every relation below is read from one canonical field of one
// canonical record, and the overwhelming majority are not derived here at all: they are the
// Phase 1C adjacency, reused unchanged. Nothing in this file measures similarity, compares
// keywords, scores provenance or decides that two records are "related" because they look
// alike. A context item a caller receives is a reference some record actually authored, or the
// documented reverse read of one, and it says which field it came from.
//
// It is also not a second traversal system. The whole projection is built once, in New, from
// records the earlier phases already parsed, and a request is a map lookup and a slice bound.
// There is no depth parameter, no frontier, no visited set and no expansion: context stops at
// the hit's own direct references, and a caller who wants the neighbourhood keeps using
// /api/v1/graph/entities/{type}/{id}/traverse, which is the route shaped for that question.

// Context bounds.
//
// These are service constants rather than configuration, for the reason the traversal bounds
// give: they are API safety invariants, not deployment choices. A caller able to raise them
// could turn one search into a corpus dump, and an operator able to lower them would change
// what the documented contract means.
//
// Both are needed, and neither implies the other. The per-result cap keeps one heavily
// referenced record — a foundational node, a source half the corpus cites — from filling a page
// on its own. The per-response cap bounds the request as a whole, since a page may carry up to
// MaxLimit results and a per-result cap alone would multiply by it.
//
// They are sized against the live corpus, the same way the traversal bounds are. Its widest
// single context is 75 references and a full 200-result page demands roughly 1,800 before either
// cap applies, so today only the handful of genuine hub records is ever shortened. A bound that
// truncated a routine request would stop being a safety invariant and start being part of the
// contract.
const (
	// MaxContextRelationsPerResult is the most context a single hit may carry. It is sized to
	// be a navigable list rather than a record dump: past roughly this many references a reader
	// is no longer orienting themselves, they are reading the record, which is what that
	// record's own route is for.
	MaxContextRelationsPerResult = 25

	// MaxContextRelationsPerResponse bounds one response across every result on the page. It is
	// spent in result order, so a truncated response keeps the context of the highest-precedence
	// hits intact rather than thinning every result equally.
	//
	// It is deliberately the same number as MaxTraversalEdges, so one statement covers the whole
	// API: no single request serialises more than 2,000 canonical relationships, whichever route
	// asked for them. Without it a full page could carry MaxLimit * MaxContextRelationsPerResult,
	// which is two and a half times this.
	MaxContextRelationsPerResponse = 2000
)

// searchRef is the identity of one searchable canonical record.
//
// It is the context layer's map key and mirrors domain.ContextRef without the display label, so
// the label is stored once, in searchLabels, rather than once per relation that names the
// record. Two classes may share an ID, so the class is part of the key.
type searchRef struct {
	entityType domain.SearchEntityType
	id         string
}

// contextKey is the normalised identity of one resolved relation, used to deduplicate.
//
// Origin and derived are part of the key rather than only of the payload. Two canonical fields
// may legitimately connect the same pair of records — a node names a session in session_origin
// and a vocabulary entry names the same session in session_refs — and collapsing those into one
// item would discard the answer to "which field said so". Keying on all four also makes
// deduplication independent of the order relations were added in, so the built projection does
// not depend on which loop ran first.
type contextKey struct {
	relation string
	to       searchRef
	origin   string
	derived  bool
}

// buildSearchContext assembles the context projection for every searchable record.
//
// It runs once, in New, after every earlier phase's index, so the maps are never written to
// again and the immutability that makes Knowledge safe for concurrent readers still holds. It
// is built eagerly rather than per request for the same reason the adjacency is: the ordering
// then has to be established once, and no request can be made expensive by asking for the
// context of a well-connected record.
//
// There are two sources, and the split is the phase's central design decision:
//
//	the Phase 1C adjacency     every relation between session, node, claim and source
//	canonical reference lists  the practice-layer references the graph deliberately excludes
//
// The first is reused verbatim — same relation name, same origin, same derived flag — so a
// connection means the same thing whether a caller reads it through traversal or through
// search. Re-deriving those edges here would create two definitions of one repository fact that
// are free to disagree, which is precisely the failure the traversal layer was careful to
// avoid.
//
// The second exists because the graph addresses four record classes and search addresses six. A
// vocabulary entry naming a node, or an experiment definition naming a session, is a resolved
// canonical reference that traversal.go states explicitly is not a graph edge. That rule is
// untouched here: those references produce a ContextRef, which is a navigation identity, and
// never an EntityRef, a graph vertex or an edge endpoint.
func (k *Knowledge) buildSearchContext() {
	// Labels come from the discovery documents, so a record's context label is by construction
	// the same string as its search title. Deciding a second time which canonical field is a
	// class's display field would let the two answers drift.
	k.searchLabels = make(map[searchRef]string, len(k.searchDocs))
	for _, doc := range k.searchDocs {
		k.searchLabels[searchRef{entityType: doc.entityType, id: doc.id}] = doc.title
	}

	k.searchContext = make(map[searchRef][]domain.ContextRelation, len(k.searchDocs))
	seen := make(map[searchRef]map[contextKey]bool, len(k.searchDocs))

	add := func(from searchRef, relation string, to searchRef, origin string, derived bool) {
		if relation == "" || from.id == "" || to.id == "" {
			return
		}
		// A target with no discovery document is not addressable by this contract and is
		// dropped rather than emitted as a bare identifier a client could not follow. Every
		// reference reaching here does resolve — an unresolved canonical reference is fatal at
		// load — so this is a guard on the contract rather than a filter on the corpus.
		label, ok := k.searchLabels[to]
		if !ok {
			return
		}
		key := contextKey{relation: relation, to: to, origin: origin, derived: derived}
		if seen[from] == nil {
			seen[from] = map[contextKey]bool{}
		}
		if seen[from][key] {
			return
		}
		seen[from][key] = true
		k.searchContext[from] = append(k.searchContext[from], domain.ContextRelation{
			Relation: relation,
			Entity:   domain.ContextRef{EntityType: to.entityType, ID: to.id, Label: label},
			Origin:   origin,
			Derived:  derived,
		})
	}

	// pair emits an authored reference and its reverse read together, so the two can never be
	// added in one place and forgotten in the other. It is the same helper buildTraversal uses,
	// applied to the reference fields the graph does not model.
	pair := func(from searchRef, forward string, to searchRef, reverse, origin string) {
		add(from, forward, to, origin, false)
		add(to, reverse, from, origin, true)
	}

	// The graph relations, projected. Iteration is over the discovery documents rather than over
	// the adjacency map, because Go map iteration is randomised and the deduplication above must
	// see the same input order on every run.
	for _, doc := range k.searchDocs {
		root, ok := graphRefForSearchType(doc.entityType, doc.id)
		if !ok {
			continue
		}
		self := searchRef{entityType: doc.entityType, id: doc.id}
		for _, edge := range k.adjacency[root] {
			target, ok := searchRefForEntity(edge.To)
			if !ok {
				continue
			}
			add(self, edge.Relationship, target, edge.Origin, edge.Derived)
		}
	}

	// claim appears_in: vocabulary. This is the one canonical claim reference that names a
	// searchable record the graph does not address, and it is the reason a vocabulary hit can
	// show the claims made about the term. The relation keeps its traversal name: it is the same
	// canonical field and the same fact, read into a layer that can address the target.
	//
	// appears_in: document and derived_from: experiment_run resolve to nothing here. A document
	// reference names no addressable layer at all, and an experiment run is deliberately not a
	// searchable class, so neither has a ContextRef to point at. The run exclusion is the Phase
	// 1E boundary holding: context resolves identities, and it must not become the route by
	// which observation and measurement prose re-enters generic search.
	for _, claim := range k.claims {
		ref := searchRef{entityType: domain.SearchClaim, id: claim.ID}
		for _, r := range claim.AppearsIn {
			if r.Kind != domain.ClaimKindVocabulary {
				continue
			}
			pair(ref, domain.RelAppearsIn, searchRef{entityType: domain.SearchVocabulary, id: r.Ref},
				domain.RelAppearanceSiteOf, domain.OriginClaimAppearsIn)
		}
	}

	// Vocabulary references. A vocabulary hit is the result least useful without context — a
	// bare term and its definition say nothing about where the corpus uses it — so the entry's
	// own reference lists are resolved in both directions.
	for _, entry := range k.vocabulary {
		ref := searchRef{entityType: domain.SearchVocabulary, id: entry.ID}
		for _, id := range entry.NodeRefs {
			pair(ref, domain.ContextRelReferences, searchRef{entityType: domain.SearchNode, id: id},
				domain.ContextRelReferencedBy, domain.OriginVocabularyNodeRefs)
		}
		for _, id := range entry.SessionRefs {
			pair(ref, domain.ContextRelReferences, searchRef{entityType: domain.SearchSession, id: id},
				domain.ContextRelReferencedBy, domain.OriginVocabularySessionRefs)
		}
		// Authored direction only; see the related-term note in domain/context.go.
		for _, id := range entry.RelatedTerms {
			add(ref, domain.ContextRelRelatedTerm, searchRef{entityType: domain.SearchVocabulary, id: id},
				domain.OriginVocabularyRelatedTerms, false)
		}
	}

	// Experiment definition references: the definition's own reference lists and nothing else. A
	// definition's runs are not resolved. ExperimentByID names its runs rather than embedding
	// them, so a definition never reads as though its results were part of its specification,
	// and a run is not a searchable class, so there is nothing here for a context ref to
	// address. Following a definition to its results stays an explicit request to
	// /api/v1/experiment-runs?experiment_id=.
	for _, experiment := range k.experiments {
		ref := searchRef{entityType: domain.SearchExperiment, id: experiment.ID}
		for _, id := range experiment.NodeRefs {
			pair(ref, domain.ContextRelReferences, searchRef{entityType: domain.SearchNode, id: id},
				domain.ContextRelReferencedBy, domain.OriginExperimentNodeRefs)
		}
		for _, id := range experiment.VocabularyRefs {
			pair(ref, domain.ContextRelReferences, searchRef{entityType: domain.SearchVocabulary, id: id},
				domain.ContextRelReferencedBy, domain.OriginExperimentVocabularyRefs)
		}
		for _, id := range experiment.SessionRefs {
			pair(ref, domain.ContextRelReferences, searchRef{entityType: domain.SearchSession, id: id},
				domain.ContextRelReferencedBy, domain.OriginExperimentSessionRefs)
		}
		for _, id := range experiment.SourceRefs {
			pair(ref, domain.ContextRelReferences, searchRef{entityType: domain.SearchSource, id: id},
				domain.ContextRelReferencedBy, domain.OriginExperimentSourceRefs)
		}
		// Authored direction only, for the reason related terms are: see domain/context.go.
		for _, id := range experiment.RelatedExperiments {
			add(ref, domain.ContextRelRelatedExperiment, searchRef{entityType: domain.SearchExperiment, id: id},
				domain.OriginExperimentRelated, false)
		}
	}

	// Deterministic order, applied once. Without it the projection would be reproducible only by
	// accident: relations arrive from several loops, and the adjacency contributed them in the
	// graph's own four-class ranking, which is not the six-class ranking a context list is read
	// in.
	for ref, relations := range k.searchContext {
		sortContextRelations(relations)
		k.searchContext[ref] = relations
	}
}

// graphRefForSearchType maps a searchable class onto its traversal class.
//
// The mapping is one-to-one over the four graph classes and returns nothing for the two that
// are not graph entities. There is deliberately no default case that guesses: vocabulary and
// experiment must resolve to no EntityRef here, because giving them one would make them graph
// vertices by side effect of resolving search context, which is exactly the boundary
// traversal.go draws.
func graphRefForSearchType(entityType domain.SearchEntityType, id string) (domain.EntityRef, bool) {
	switch entityType {
	case domain.SearchSession:
		return domain.EntityRef{Type: domain.EntitySession, ID: id}, true
	case domain.SearchNode:
		return domain.EntityRef{Type: domain.EntityNode, ID: id}, true
	case domain.SearchClaim:
		return domain.EntityRef{Type: domain.EntityClaim, ID: id}, true
	case domain.SearchSource:
		return domain.EntityRef{Type: domain.EntitySource, ID: id}, true
	default:
		return domain.EntityRef{}, false
	}
}

// searchRefForEntity is the reverse mapping, applied to a traversal edge's target.
func searchRefForEntity(ref domain.EntityRef) (searchRef, bool) {
	switch ref.Type {
	case domain.EntitySession:
		return searchRef{entityType: domain.SearchSession, id: ref.ID}, true
	case domain.EntityNode:
		return searchRef{entityType: domain.SearchNode, id: ref.ID}, true
	case domain.EntityClaim:
		return searchRef{entityType: domain.SearchClaim, id: ref.ID}, true
	case domain.EntitySource:
		return searchRef{entityType: domain.SearchSource, id: ref.ID}, true
	default:
		return searchRef{}, false
	}
}

// sortContextRelations fixes context order: relation, target class in model order, target ID,
// then the canonical field and whether the item was authored or derived.
//
// The last two are tie-breakers rather than a reading order. They matter only where two
// canonical fields connect the same pair of records under the same relation name, and without
// them such a pair would sort by whichever loop happened to add it first, which is not a
// property of the corpus.
func sortContextRelations(relations []domain.ContextRelation) {
	sort.SliceStable(relations, func(i, j int) bool {
		a, b := relations[i], relations[j]
		if a.Relation != b.Relation {
			return a.Relation < b.Relation
		}
		if a.Entity.EntityType != b.Entity.EntityType {
			return domain.SearchEntityRank(a.Entity.EntityType) < domain.SearchEntityRank(b.Entity.EntityType)
		}
		if a.Entity.ID != b.Entity.ID {
			return a.Entity.ID < b.Entity.ID
		}
		if a.Origin != b.Origin {
			return a.Origin < b.Origin
		}
		return !a.Derived && b.Derived
	})
}

// contextFor returns the bounded context of one record and reports how much of the response
// budget it spent.
//
// budget is what remains of MaxContextRelationsPerResponse. A record whose context is cut short
// still reports its full Count, so a truncated context is visibly truncated rather than looking
// like a complete short one. A record with no canonical context returns an empty list rather
// than nothing at all: "this record references nothing" is an answer, and omitting the object
// would make it indistinguishable from a context that was never requested.
//
// The returned relations are a fresh slice of values, so a caller cannot reach the startup
// index through a response. ContextRelation holds no slice or pointer of its own, so copying
// the slice copies the whole payload.
func (k *Knowledge) contextFor(ref searchRef, budget int) (*domain.SearchResultContext, int) {
	all := k.searchContext[ref]

	limit := MaxContextRelationsPerResult
	if budget < limit {
		limit = budget
	}
	if limit < 0 {
		limit = 0
	}
	returned := len(all)
	if returned > limit {
		returned = limit
	}

	related := make([]domain.ContextRelation, 0, returned)
	related = append(related, all[:returned]...)

	return &domain.SearchResultContext{
		Related:   related,
		Count:     len(all),
		Returned:  returned,
		Truncated: returned < len(all),
	}, returned
}

// resolveContext attaches context to the results of one page, in page order.
//
// It runs after matching, ordering and paging, and it changes none of them. Context is metadata
// about a hit, never a signal about it: a record does not sort higher for being well connected,
// and it does not appear at all for being connected to something that matched. The page is a
// slice of a result set built fresh for this request, so writing to it cannot touch the index.
func (k *Knowledge) resolveContext(page []domain.SearchResult) {
	budget := MaxContextRelationsPerResponse
	for i := range page {
		resolved, spent := k.contextFor(
			searchRef{entityType: page[i].EntityType, id: page[i].ID}, budget)
		budget -= spent
		page[i].Context = resolved
	}
}
