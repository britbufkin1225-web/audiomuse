package service

import (
	"errors"
	"sort"
	"strconv"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Phase 1C: the bounded relationship and traversal read model.
//
// It lives on Knowledge rather than in a package of its own for the reason Phase 1B gave
// for loading claims through the same adapter as nodes: there is one read model. The
// adjacency below is assembled from the records Phase 1A and 1B already parsed, inside the
// same New call, and reparsing the corpus into a second index would produce two answers to
// "does this ID resolve" that are free to disagree. It is built once and never written to
// again, so the immutability that makes Knowledge safe for concurrent readers still holds.

// Traversal bounds.
//
// These are service constants rather than configuration. They are API safety invariants,
// not deployment choices: a caller who could raise them could ask one request to serialise
// the entire corpus, and an operator who could lower them would change what the documented
// contract means. Configuration is for things that legitimately differ between deployments.
//
// The values are sized against the live corpus — 78 nodes, 220 edges, 3 sessions, 51
// registered sources — so a depth-3 traversal of today's repository completes without
// truncation while a corpus several times larger still cannot exhaust a request.
const (
	// DefaultTraversalDepth is what a caller who names no depth receives: the direct
	// neighbourhood, which is the answer to "what is related to this" and cannot fan out.
	DefaultTraversalDepth = 1

	// MaxTraversalDepth is a hard ceiling. It cannot be disabled by any parameter.
	//
	// Three hops is the length of the epistemic path the model is built around —
	// session -> node -> claim -> source — so it is exactly enough to walk from a
	// chronological record to the evidence standing behind a concept it introduced. A
	// fourth hop buys reach that is no longer explainable as one question.
	MaxTraversalDepth = 3

	// MinTraversalDepth is 1. Depth 0 is not accepted: a traversal that returns only its
	// own root answers nothing, and silently promoting it to 1 would return results the
	// caller did not ask for.
	MinTraversalDepth = 1

	// MaxTraversalEntities and MaxTraversalEdges bound the work and the payload of one
	// request independently of depth. Depth alone is not a bound: a single hub entity can
	// have hundreds of neighbours, so a depth-2 request over a large corpus could be far
	// more expensive than a depth-3 request over a sparse one.
	MaxTraversalEntities = 500
	MaxTraversalEdges    = 2000
)

// Truncation reasons. Stable strings, matched rather than parsed.
const (
	TruncationEntityLimit = "entity_limit_reached"
	TruncationEdgeLimit   = "edge_limit_reached"
)

// ErrUnsupportedEntityType reports a graph entity class outside domain.EntityTypes.
var ErrUnsupportedEntityType = errors.New("unsupported graph entity type")

// InvalidDepthError reports a traversal depth outside the accepted range.
//
// It is a distinct type rather than an InvalidFilterError because depth is a bounded
// integer rather than a value drawn from a canonical vocabulary, and rendering its bounds
// as an "allowed values" list would misdescribe it.
type InvalidDepthError struct {
	Min int
	Max int
}

func (e *InvalidDepthError) Error() string {
	return "traversal depth must be between " + strconv.Itoa(e.Min) + " and " + strconv.Itoa(e.Max)
}

// TraversalQuery is a bounded, deterministic traversal request.
//
// Depth zero means "unspecified" and receives DefaultTraversalDepth; any other
// out-of-range value is refused rather than clamped, because a caller who asked for depth
// 9 and silently received depth 3 would believe they had seen the whole neighbourhood.
//
// Relationship and TargetType are optional filters. Both are validated against the closed
// model vocabulary and both are applied during expansion rather than to the finished
// result, so a filtered traversal is the traversal of the filtered subgraph and depth keeps
// meaning "hops", not "hops before filtering".
type TraversalQuery struct {
	Depth        int
	Relationship string
	TargetType   string
}

// TraversalFilters echoes the controls a result was produced under, so a response is
// self-describing and a client caching two responses cannot confuse them.
type TraversalFilters struct {
	Relationship string `json:"relationship,omitempty"`
	TargetType   string `json:"target_type,omitempty"`
}

// TraversalCounts reports the size of a result.
type TraversalCounts struct {
	Entities      int `json:"entities"`
	Relationships int `json:"relationships"`
}

// EntityRelationships is the direct-neighbour projection of one entity.
//
// Neighbours are the distinct entities the relationships reach, carried with their labels
// so a client can render an adjacency view in one request. They are always at distance 1;
// the root itself is Entity and is not repeated in the list.
type EntityRelationships struct {
	Entity           domain.GraphEntity         `json:"entity"`
	Filters          TraversalFilters           `json:"filters"`
	Relationships    []domain.GraphRelationship `json:"relationships"`
	Neighbors        []domain.GraphEntity       `json:"neighbors"`
	Counts           TraversalCounts            `json:"counts"`
	Partial          bool                       `json:"partial"`
	TruncationReason string                     `json:"truncation_reason,omitempty"`
}

// Traversal is the bounded neighbourhood projection.
//
// Entities carries every entity reached, including the root at distance 0, where distance
// is the shortest number of hops from the root — a property breadth-first traversal gives
// for free. Partial is never false when anything was dropped: a truncated result that
// claimed to be complete would be a wrong answer rather than a small one.
type Traversal struct {
	Root             domain.EntityRef           `json:"root"`
	Depth            int                        `json:"depth"`
	Filters          TraversalFilters           `json:"filters"`
	Entities         []domain.GraphEntity       `json:"entities"`
	Relationships    []domain.GraphRelationship `json:"relationships"`
	Counts           TraversalCounts            `json:"counts"`
	Partial          bool                       `json:"partial"`
	TruncationReason string                     `json:"truncation_reason,omitempty"`
}

// edgeKey is the normalised identity of one relationship, used to deduplicate.
type edgeKey struct {
	from domain.EntityRef
	rel  string
	to   domain.EntityRef
}

// buildTraversal assembles the adjacency index and the relationship vocabulary.
//
// Every edge below is read from one canonical field. No edge is derived from shared
// keywords, similar titles, overlapping prose, co-occurrence or any similarity measure:
// docs/backend-architecture.md states that a projection manufacturing edges from proximity
// would insert unsourced claims into a corpus whose whole discipline is that claims carry
// provenance, and that rule does not weaken because the edge crosses a layer boundary.
//
// Each authored edge is emitted with its documented reverse. The reverse is marked
// Derived, following the Phase 1A rule that a derived view must never be mistakable for
// authored data. Reverse edges exist because the questions are genuinely different in each
// direction — "what does this claim rest on" and "what rests on this source" are not the
// same request — and because a traversal that could only follow authored direction would
// leave every leaf record unreachable from the evidence it supports.
func (k *Knowledge) buildTraversal() {
	k.adjacency = map[domain.EntityRef][]domain.GraphRelationship{}
	seen := map[edgeKey]bool{}

	add := func(from domain.EntityRef, rel string, to domain.EntityRef, origin string, derived bool) {
		if rel == "" || from.ID == "" || to.ID == "" {
			return
		}
		key := edgeKey{from: from, rel: rel, to: to}
		if seen[key] {
			return
		}
		seen[key] = true
		k.adjacency[from] = append(k.adjacency[from],
			domain.GraphRelationship{From: from, Relationship: rel, To: to, Origin: origin, Derived: derived})
	}

	// Pair emits an authored edge and its reverse together, so the two can never be added
	// in one place and forgotten in the other.
	pair := func(from domain.EntityRef, forward string, to domain.EntityRef, reverse, origin string) {
		add(from, forward, to, origin, false)
		add(to, reverse, from, origin, true)
	}

	inverse := map[string]string{}
	for _, rt := range k.relationshipTypes {
		inverse[rt.ID] = rt.Inverse
	}

	for _, node := range k.nodes {
		ref := domain.EntityRef{Type: domain.EntityNode, ID: node.ID}

		// node.relationships: the canonical typed graph. The forward label is the
		// authored relationship-type ID and the reverse label is that type's own declared
		// `inverse` from schemas/relationship-types.yaml, so neither name is invented
		// here. A type carrying no inverse yields no reverse edge rather than a guess.
		for _, rel := range node.Relationships {
			target := domain.EntityRef{Type: domain.EntityNode, ID: rel.Target}
			add(ref, rel.Type, target, domain.OriginNodeRelationships, false)
			if inv := inverse[rel.Type]; inv != "" {
				add(target, inv, ref, domain.OriginNodeRelationships, true)
			}
		}

		// node.session_origin: which sessions developed this concept. The reverse is the
		// many-to-many contribution map docs/knowledge-model.md describes, which Phase 1A
		// already serves as Session.NodeIDs.
		for _, sessionID := range node.SessionOrigin {
			pair(ref, domain.RelOriginatesIn,
				domain.EntityRef{Type: domain.EntitySession, ID: sessionID},
				domain.RelContributedTo, domain.OriginNodeSessionOrigin)
		}

		// node.sources: topical provenance. This is kept a separate relation from claim
		// evidence throughout. docs/claim-provenance-model.md distinguishes "this source is
		// relevant to this concept" from "this source materially supports this statement",
		// and a traversal that emitted both as one edge type would erase the distinction
		// the provenance layer exists to make.
		for _, sourceID := range node.Sources {
			pair(ref, domain.RelSourcedFrom,
				domain.EntityRef{Type: domain.EntitySource, ID: sourceID},
				domain.RelSourceFor, domain.OriginNodeSources)
		}
	}

	for _, claim := range k.claims {
		ref := domain.EntityRef{Type: domain.EntityClaim, ID: claim.ID}

		// claim.evidence: the evidential relation, named by the canonical relation the
		// record carries. An entry whose relation is outside the contract vocabulary would
		// have been fatal at load, so an unmapped inverse here means the vocabulary grew
		// without this map; the forward edge is still emitted and the reverse is omitted
		// rather than labelled with a guess.
		for _, e := range claim.Evidence {
			source := domain.EntityRef{Type: domain.EntitySource, ID: e.SourceID}
			add(ref, e.Relation, source, domain.OriginClaimEvidence, false)
			if inv := domain.EvidenceInverse[e.Relation]; inv != "" {
				add(source, inv, ref, domain.OriginClaimEvidence, true)
			}
		}

		// claim.attribution: who a statement is credited to, and the source recording the
		// credit. Separate from evidence because "who says so" is not "what stands behind
		// it", exactly as domain.ClaimAttribution states.
		for _, a := range claim.Attribution {
			pair(ref, domain.RelAttributedTo,
				domain.EntityRef{Type: domain.EntitySource, ID: a.SourceID},
				domain.RelAttributionFor, domain.OriginClaimAttribution)
		}

		// claim.appears_in and claim.derived_from: resolved by kind. Only the kinds naming
		// an addressable entity class produce an edge. vocabulary, document and
		// experiment_run references are carried through unresolved by Phase 1B because
		// those layers are not parsed, and inventing a graph entity for them would assert
		// a resolution that never happened.
		for _, r := range claim.AppearsIn {
			if target, ok := referenceEntity(r); ok {
				pair(ref, domain.RelAppearsIn, target,
					domain.RelAppearanceSiteOf, domain.OriginClaimAppearsIn)
			}
		}
		for _, r := range claim.DerivedFrom {
			if target, ok := referenceEntity(r); ok {
				pair(ref, domain.RelDerivedFrom, target,
					domain.RelBasisFor, domain.OriginClaimDerivedFrom)
			}
		}
	}

	// Deterministic adjacency order. Go map iteration is randomised and the corpus is
	// walked in canonical record order, so without this a traversal would be reproducible
	// only by accident. Ordering is relationship, then target type in model order, then
	// target ID.
	for ref, edges := range k.adjacency {
		sortRelationships(edges)
		k.adjacency[ref] = edges
	}

	k.relationshipNames = k.buildRelationshipVocabulary()
}

// referenceEntity resolves one kind-qualified claim reference to an addressable entity.
//
// A kind naming a layer the backend does not load resolves to nothing. There is
// deliberately no default case that guesses a type from the shape of the ref.
func referenceEntity(ref domain.ClaimReference) (domain.EntityRef, bool) {
	switch ref.Kind {
	case domain.ClaimKindNode:
		return domain.EntityRef{Type: domain.EntityNode, ID: ref.Ref}, true
	case domain.ClaimKindSession:
		return domain.EntityRef{Type: domain.EntitySession, ID: ref.Ref}, true
	case domain.ClaimKindClaim:
		return domain.EntityRef{Type: domain.EntityClaim, ID: ref.Ref}, true
	default:
		return domain.EntityRef{}, false
	}
}

// buildRelationshipVocabulary assembles every relationship name the model can emit.
//
// It is derived from the contracts loaded at startup — the relationship-type IDs and their
// declared inverses, and the claim evidence relations and their active-voice inverses —
// plus the fixed cross-layer names, rather than from the edges that happen to exist. A
// caller filtering on a recognised relationship the current corpus does not use therefore
// receives an empty result, which is what "no such edge here" means, instead of a 400 that
// would wrongly say the relationship is not part of the model.
func (k *Knowledge) buildRelationshipVocabulary() []string {
	names := map[string]bool{
		domain.RelOriginatesIn:     true,
		domain.RelContributedTo:    true,
		domain.RelSourcedFrom:      true,
		domain.RelSourceFor:        true,
		domain.RelAttributedTo:     true,
		domain.RelAttributionFor:   true,
		domain.RelAppearsIn:        true,
		domain.RelAppearanceSiteOf: true,
		domain.RelDerivedFrom:      true,
		domain.RelBasisFor:         true,
	}
	for _, rt := range k.relationshipTypes {
		names[rt.ID] = true
		if rt.Inverse != "" {
			names[rt.Inverse] = true
		}
	}
	for _, relation := range k.vocabularies.Claim.EvidenceRelations {
		names[relation] = true
		if inv := domain.EvidenceInverse[relation]; inv != "" {
			names[inv] = true
		}
	}
	return sortedSet(names)
}

// RelationshipVocabulary returns every relationship name the traversal model can emit.
func (k *Knowledge) RelationshipVocabulary() []string { return copyIDs(k.relationshipNames) }

// EntityRelationshipsFor returns the direct relationships of one entity.
//
// A type outside the model is ErrUnsupportedEntityType and an ID that resolves to no record
// is ErrNotFound. The two are kept apart deliberately, and neither is answered with an
// empty relationship list: "this entity exists and has no edges" and "this entity does not
// exist" are different facts, and returning the first for the second would let a caller
// build a view of a record the repository has never contained.
func (k *Knowledge) EntityRelationshipsFor(entityType, id string, q TraversalQuery) (EntityRelationships, error) {
	root, err := k.resolveEntity(entityType, id)
	if err != nil {
		return EntityRelationships{}, err
	}
	filters, err := k.normaliseFilters(q)
	if err != nil {
		return EntityRelationships{}, err
	}

	edges := make([]domain.GraphRelationship, 0, len(k.adjacency[root.Ref()]))
	neighbors := make([]domain.GraphEntity, 0)
	seen := map[domain.EntityRef]bool{}
	partial, reason := false, ""

	for _, edge := range k.adjacency[root.Ref()] {
		if !matchesFilters(edge, filters) {
			continue
		}
		// A new neighbour that would exceed the cap stops the listing before its edge is
		// recorded, so a truncated response never contains a relationship pointing at an
		// entity it does not list. A caller can render what it received without having to
		// defend against dangling references.
		if !seen[edge.To] && len(neighbors) >= MaxTraversalEntities {
			partial, reason = true, TruncationEntityLimit
			break
		}
		if len(edges) >= MaxTraversalEdges {
			partial, reason = true, TruncationEdgeLimit
			break
		}
		edges = append(edges, edge)
		if !seen[edge.To] {
			seen[edge.To] = true
			neighbors = append(neighbors, k.graphEntity(edge.To, 1))
		}
	}

	sortEntities(neighbors)
	return EntityRelationships{
		Entity:           root,
		Filters:          filters,
		Relationships:    edges,
		Neighbors:        neighbors,
		Counts:           TraversalCounts{Entities: len(neighbors), Relationships: len(edges)},
		Partial:          partial,
		TruncationReason: reason,
	}, nil
}

// Traverse walks the bounded neighbourhood of one entity.
//
// The algorithm is breadth-first. Depth then means shortest hop distance, which is the
// intuitive reading of the parameter and the one a caller exploring outward from a concept
// expects; a depth-first walk would report the same entity at whatever distance it happened
// to be found first, making the distance field a traversal artefact rather than a fact
// about the graph. Breadth-first also degrades honestly under the bounds below: what gets
// dropped is the far edge of the neighbourhood, not an arbitrary branch.
//
// Cycles are normal in this graph — every authored edge has a reverse, so alpha produces
// beta and beta produced_by alpha is already a two-cycle, and claim derivation can close
// longer loops. The visited set is what makes that safe: an entity is expanded exactly
// once, at its shortest distance, so a cyclic corpus terminates instead of looping.
func (k *Knowledge) Traverse(entityType, id string, q TraversalQuery) (Traversal, error) {
	root, err := k.resolveEntity(entityType, id)
	if err != nil {
		return Traversal{}, err
	}
	depth, err := normaliseDepth(q.Depth)
	if err != nil {
		return Traversal{}, err
	}
	filters, err := k.normaliseFilters(q)
	if err != nil {
		return Traversal{}, err
	}

	result := Traversal{
		Root:          root.Ref(),
		Depth:         depth,
		Filters:       filters,
		Entities:      []domain.GraphEntity{root},
		Relationships: []domain.GraphRelationship{},
	}

	visited := map[domain.EntityRef]bool{root.Ref(): true}
	emitted := map[edgeKey]bool{}
	frontier := []domain.EntityRef{root.Ref()}

	for hop := 1; hop <= depth && len(frontier) > 0 && !result.Partial; hop++ {
		next := make([]domain.EntityRef, 0, len(frontier))
		for _, current := range frontier {
			for _, edge := range k.adjacency[current] {
				if !matchesFilters(edge, filters) {
					continue
				}
				// Both caps are checked before the edge is recorded, and the entity cap is
				// checked only for an entity the traversal has not already reached. A
				// truncated result therefore never contains a relationship pointing at an
				// entity it does not list, so a client can render what it received without
				// defending against dangling references.
				if !visited[edge.To] && len(result.Entities) >= MaxTraversalEntities {
					result.Partial, result.TruncationReason = true, TruncationEntityLimit
					break
				}
				key := edgeKey{from: edge.From, rel: edge.Relationship, to: edge.To}
				if !emitted[key] {
					if len(result.Relationships) >= MaxTraversalEdges {
						result.Partial, result.TruncationReason = true, TruncationEdgeLimit
						break
					}
					emitted[key] = true
					result.Relationships = append(result.Relationships, edge)
				}
				if visited[edge.To] {
					continue
				}
				visited[edge.To] = true
				result.Entities = append(result.Entities, k.graphEntity(edge.To, hop))
				next = append(next, edge.To)
			}
			if result.Partial {
				break
			}
		}
		frontier = next
	}

	sortEntities(result.Entities)
	sortRelationshipsForOutput(result.Relationships)
	result.Counts = TraversalCounts{Entities: len(result.Entities), Relationships: len(result.Relationships)}
	return result, nil
}

// resolveEntity validates an entity class and resolves an ID to a labelled root entity.
//
// The ID is only ever a map key. It is never joined to a path, opened, or handed to the
// filesystem, so no traversal parameter can reach the operator's disk. That property is
// inherited from Phase 1A and is not weakened here.
func (k *Knowledge) resolveEntity(entityType, id string) (domain.GraphEntity, error) {
	if !domain.ValidEntityType(entityType) {
		return domain.GraphEntity{}, ErrUnsupportedEntityType
	}
	ref := domain.EntityRef{Type: domain.EntityType(entityType), ID: id}
	if !k.entityExists(ref) {
		return domain.GraphEntity{}, ErrNotFound
	}
	return k.graphEntity(ref, 0), nil
}

// entityExists resolves a ref against the records the index actually holds.
func (k *Knowledge) entityExists(ref domain.EntityRef) bool {
	switch ref.Type {
	case domain.EntityNode:
		_, ok := k.nodesByID[ref.ID]
		return ok
	case domain.EntitySession:
		_, ok := k.sessionsByID[ref.ID]
		return ok
	case domain.EntityClaim:
		_, ok := k.claimsByID[ref.ID]
		return ok
	case domain.EntitySource:
		_, ok := k.sourcesByID[ref.ID]
		return ok
	default:
		return false
	}
}

// graphEntity attaches the display label and the discovered distance to a ref.
//
// A label is canonical record content: a node or source title, a session title, a claim's
// statement. An entity the index cannot resolve keeps an empty label rather than acquiring
// a fabricated one; that can only happen for a reference the loader already reported, since
// an unresolved canonical reference is a fatal validation issue.
func (k *Knowledge) graphEntity(ref domain.EntityRef, distance int) domain.GraphEntity {
	entity := domain.GraphEntity{Type: ref.Type, ID: ref.ID, Distance: distance}
	switch ref.Type {
	case domain.EntityNode:
		if node, ok := k.nodesByID[ref.ID]; ok {
			entity.Label = node.Title
		}
	case domain.EntitySession:
		if session, ok := k.sessionsByID[ref.ID]; ok {
			entity.Label = session.Title
		}
	case domain.EntityClaim:
		if claim, ok := k.claimsByID[ref.ID]; ok {
			entity.Label = claim.Statement
		}
	case domain.EntitySource:
		if source, ok := k.sourcesByID[ref.ID]; ok {
			entity.Label = source.Title
		}
	}
	return entity
}

// normaliseDepth applies the depth contract: absent means default, out of range is refused.
func normaliseDepth(depth int) (int, error) {
	if depth == 0 {
		return DefaultTraversalDepth, nil
	}
	if depth < MinTraversalDepth || depth > MaxTraversalDepth {
		return 0, &InvalidDepthError{Min: MinTraversalDepth, Max: MaxTraversalDepth}
	}
	return depth, nil
}

// normaliseFilters validates the optional traversal filters against the closed vocabularies.
//
// Both are checked in a fixed order so two bad filters always produce the same error.
func (k *Knowledge) normaliseFilters(q TraversalQuery) (TraversalFilters, error) {
	if q.Relationship != "" && !contains(k.relationshipNames, q.Relationship) {
		return TraversalFilters{}, &InvalidFilterError{Param: "relationship", Allowed: copyIDs(k.relationshipNames)}
	}
	if q.TargetType != "" && !domain.ValidEntityType(q.TargetType) {
		return TraversalFilters{}, &InvalidFilterError{Param: "target_type", Allowed: domain.EntityTypeNames()}
	}
	return TraversalFilters{Relationship: q.Relationship, TargetType: q.TargetType}, nil
}

func matchesFilters(edge domain.GraphRelationship, filters TraversalFilters) bool {
	if filters.Relationship != "" && edge.Relationship != filters.Relationship {
		return false
	}
	if filters.TargetType != "" && string(edge.To.Type) != filters.TargetType {
		return false
	}
	return true
}

// entityRank orders entity types by the model's own layering rather than alphabetically, so
// a sorted result reads session, node, claim, source: chronology, concept, statement,
// provenance. An unknown type sorts last and deterministically.
func entityRank(t domain.EntityType) int {
	for i, known := range domain.EntityTypes {
		if known == t {
			return i
		}
	}
	return len(domain.EntityTypes)
}

// sortRelationships fixes adjacency order: relationship, target type, target ID.
func sortRelationships(edges []domain.GraphRelationship) {
	sort.SliceStable(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.Relationship != b.Relationship {
			return a.Relationship < b.Relationship
		}
		if a.To.Type != b.To.Type {
			return entityRank(a.To.Type) < entityRank(b.To.Type)
		}
		return a.To.ID < b.To.ID
	})
}

// sortRelationshipsForOutput fixes multi-source result order: source entity first, then the
// adjacency order above. A traversal collects edges from many entities, so the emitting
// entity has to lead or the result would depend on visit order.
func sortRelationshipsForOutput(edges []domain.GraphRelationship) {
	sort.SliceStable(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.From.Type != b.From.Type {
			return entityRank(a.From.Type) < entityRank(b.From.Type)
		}
		if a.From.ID != b.From.ID {
			return a.From.ID < b.From.ID
		}
		if a.Relationship != b.Relationship {
			return a.Relationship < b.Relationship
		}
		if a.To.Type != b.To.Type {
			return entityRank(a.To.Type) < entityRank(b.To.Type)
		}
		return a.To.ID < b.To.ID
	})
}

// sortEntities fixes result order: distance, then type in model order, then ID. Distance
// leads so a rendered result reads outward from the root.
func sortEntities(entities []domain.GraphEntity) {
	sort.SliceStable(entities, func(i, j int) bool {
		a, b := entities[i], entities[j]
		if a.Distance != b.Distance {
			return a.Distance < b.Distance
		}
		if a.Type != b.Type {
			return entityRank(a.Type) < entityRank(b.Type)
		}
		return a.ID < b.ID
	})
}
