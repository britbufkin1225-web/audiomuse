package domain

// This file holds the AudioMuse traversal layer: the bounded graph view that lets a caller
// move between the four addressable canonical record classes.
//
// Nothing here is a new canonical concept. Every entity class below is a record kind the
// backend already loads, and every relationship below is either an edge a canonical record
// authored or the documented reverse read of one. The layer exists because sessions, nodes,
// claims and sources reference each other in the corpus and no single Phase 1A or 1B
// projection lets a caller follow those references more than one field at a time.

// EntityType is the class of an addressable graph entity.
//
// The set is closed and matches the four record kinds the backend loads and can resolve by
// ID. Vocabulary entries, experiments and experiment runs are canonical repository layers
// that the backend still does not parse (see docs/backend-architecture.md), so they are not
// addressable here and no edge is emitted towards them: claiming to traverse into a layer
// that was never loaded would be worse than saying it is absent.
type EntityType string

const (
	EntitySession EntityType = "session"
	EntityNode    EntityType = "node"
	EntityClaim   EntityType = "claim"
	EntitySource  EntityType = "source"
)

// EntityTypes is the closed set, in the canonical epistemic order the corpus is layered in:
// chronology, concept, statement, provenance.
//
// Order is fixed rather than sorted so the list a caller is offered in a validation error
// reads as the model rather than as an alphabetisation of it.
var EntityTypes = []EntityType{EntitySession, EntityNode, EntityClaim, EntitySource}

// ValidEntityType reports whether a caller-supplied string names an addressable class.
func ValidEntityType(value string) bool {
	for _, t := range EntityTypes {
		if string(t) == value {
			return true
		}
	}
	return false
}

// EntityTypeNames renders the closed set for an error message.
func EntityTypeNames() []string {
	out := make([]string, 0, len(EntityTypes))
	for _, t := range EntityTypes {
		out = append(out, string(t))
	}
	return out
}

// EntityRef addresses one graph entity.
//
// Type and ID together are the identity: a registry entry of type session and the session
// projected from it share an ID but are two different projections, and the pair keeps them
// apart. The ref carries no record content, so a relationship never embeds a Node inside a
// Claim inside a Source and traversal payloads cannot grow recursively.
type EntityRef struct {
	Type EntityType `json:"type"`
	ID   string     `json:"id"`
}

// GraphEntity is an entity as it appears in a traversal result: a ref, a display label, and
// the shortest number of hops at which the traversal reached it.
//
// Label is deterministic record content — a node or source title, a session title, a claim
// statement — so a client can render a neighbourhood without a second request per entity.
// It is display metadata only and is never the identity.
type GraphEntity struct {
	Type     EntityType `json:"type"`
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Distance int        `json:"distance"`
}

// Ref reduces an entity to its identity.
func (e GraphEntity) Ref() EntityRef { return EntityRef{Type: e.Type, ID: e.ID} }

// GraphRelationship is one directed, typed, explained edge of the traversal graph.
//
// Origin names the canonical field the edge was read from, so "why does this edge exist"
// is answerable from the edge itself rather than by re-deriving it. Derived separates an
// edge a record authored from the reverse read of one: docs/knowledge-model.md states that
// AudioMuse stores each supported claim once in its clearest direction and that inverse
// labels are descriptive metadata rather than storable edges, so a reverse edge that
// presented itself as authored data would misrepresent the corpus.
type GraphRelationship struct {
	From         EntityRef `json:"from"`
	Relationship string    `json:"relationship"`
	To           EntityRef `json:"to"`
	Origin       string    `json:"origin"`
	Derived      bool      `json:"derived"`
}

// Canonical fields that traversal edges are derived from. Every emitted edge names exactly
// one of these, and each names a real field of a real canonical contract.
const (
	OriginNodeRelationships = "node.relationships"
	OriginNodeSessionOrigin = "node.session_origin"
	OriginNodeSources       = "node.sources"
	OriginClaimEvidence     = "claim.evidence"
	OriginClaimAttribution  = "claim.attribution"
	OriginClaimAppearsIn    = "claim.appears_in"
	OriginClaimDerivedFrom  = "claim.derived_from"
)

// Cross-layer relationship names.
//
// Node-to-node edges do not appear here: they use the canonical relationship-type IDs from
// schemas/relationship-types.yaml directly, and their reverse edges use that file's own
// declared `inverse` label. Neither is invented by this package.
//
// The names below are the cross-layer relations, one pair per canonical field. Each forward
// name restates the field it comes from and each reverse name is its grammatical inverse,
// so no name asserts a relation the field does not already state:
//
//	node.session_origin   node --originates_in--> session   session --contributed_to--> node
//	node.sources          node --sourced_from--> source     source --source_for--> node
//	claim.attribution     claim --attributed_to--> source   source --attribution_for--> claim
//	claim.appears_in      claim --appears_in--> node|session  node|session --appearance_site_of--> claim
//	claim.derived_from    claim --derived_from--> claim|node  claim|node --basis_for--> claim
//
// claim.evidence is deliberately absent: its relationship name is the canonical evidence
// relation the record itself carries (supported_by, contradicted_by, qualified_by), and its
// reverse is that relation in the active voice. See EvidenceInverse.
const (
	RelOriginatesIn     = "originates_in"
	RelContributedTo    = "contributed_to"
	RelSourcedFrom      = "sourced_from"
	RelSourceFor        = "source_for"
	RelAttributedTo     = "attributed_to"
	RelAttributionFor   = "attribution_for"
	RelAppearsIn        = "appears_in"
	RelAppearanceSiteOf = "appearance_site_of"
	RelDerivedFrom      = "derived_from"
	RelBasisFor         = "basis_for"
)

// EvidenceInverse maps a canonical evidence relation to its reverse traversal label.
//
// schemas/claim.schema.yaml states the relation from the claim's side, in the passive
// voice: a claim is supported_by a source. The reverse edge is the same statement read from
// the source's side, so it is the same verb in the active voice. Nothing is added: a source
// that supports a claim is precisely what "this claim is supported by that source" says.
//
// The relation is preserved rather than flattened to a generic "evidence" edge because
// docs/claim-provenance-model.md treats support, contradiction and qualification as
// different facts, and a traversal that merged them would let a contradicting source look
// like a supporting one.
var EvidenceInverse = map[string]string{
	"supported_by":    "supports",
	"contradicted_by": "contradicts",
	"qualified_by":    "qualifies",
}
