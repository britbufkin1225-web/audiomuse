package domain

// This file holds the AudioMuse related-knowledge layer: the shape one canonical record takes
// when it is returned as an answer to "given this, what should I read next".
//
// Nothing here is a new canonical concept, and nothing here is a graph concept. Every related
// item is a record the backend already loads, reached over a reference some canonical record
// actually authored or the documented reverse read of one, and every explanation names the
// canonical field it was read from. The layer exists because Phase 1E answers "where does this
// text appear" and Phase 1C answers "what is the neighbourhood of this graph vertex", and
// neither answers the question a reader has while holding one record: which of the things this
// record is connected to is worth opening first.
//
// Related-knowledge discovery is not search and it is not traversal. It takes no query text, so
// it cannot be steered by wording; it takes no depth, so it cannot expand; and it walks nothing,
// because the canonical references of every searchable record are already resolved once at
// startup by the Phase 1F context layer. What this layer adds is precedence: a documented,
// closed ranking of the canonical fields a connection can come from, so that a bounded list can
// be cut somewhere principled rather than wherever the corpus happened to stop.
//
// There is no similarity anywhere in it. No embedding, no vector, no keyword overlap, no
// co-occurrence and no model output contributes to whether two records are related or to how a
// related item ranks. docs/backend-architecture.md states that a projection manufacturing edges
// from proximity would insert unsourced claims into a corpus whose whole discipline is that
// claims carry provenance, and a projection that ranked authored references by their apparent
// similarity would do the same thing one step later: it would present a machine's guess about
// meaning as though the corpus had said it.

// RelatedPriority is the precedence class of one canonical connection.
//
// It is a property of the canonical field the connection was read from - never of the records at
// either end, never of the text they contain, and never of how many connections a record has.
// That is what makes it explainable: a caller who receives a ranked list can ask why one item is
// above another, and the answer is always that AudioMuse wrote the connection down in this field
// rather than that one, which is a fact about the corpus rather than a judgement about the
// reader.
type RelatedPriority string

// The precedence classes, strongest first.
//
// Each is one sentence about what its fields assert, and the order is the model's own reading of
// how directly a connection bears on understanding the record in hand:
//
//   - a typed edge between two concepts is the only connection AudioMuse authors specifically as
//     a knowledge relation, so it leads;
//   - what materially stands behind a statement comes next, and is kept ahead of who the
//     statement is credited to, because docs/claim-provenance-model.md treats what supports a
//     statement and who says it as different facts, and the first is the one a reader checks;
//   - which records a statement is about or rests on follows, since that is a statement's own
//     subject matter rather than its provenance;
//   - a node's topical sources and its chronological origin come after that: both are true and
//     both are about where a concept came from rather than what it means;
//   - the practice layer's reference lists follow, because a vocabulary entry naming a node is a
//     pointer into the concept layer rather than a claim about it;
//   - and curated navigation lists come last, because vocabulary/README.md states that related
//     terms are human navigation only and imply neither equivalence nor a graph edge, which is
//     the weakest thing any canonical field here says.
const (
	PriorityConceptual   RelatedPriority = "conceptual"
	PriorityEvidential   RelatedPriority = "evidential"
	PriorityAttributive  RelatedPriority = "attributive"
	PriorityAssertional  RelatedPriority = "assertional"
	PriorityContextual   RelatedPriority = "contextual"
	PriorityReferential  RelatedPriority = "referential"
	PriorityNavigational RelatedPriority = "navigational"

	// PriorityUnclassified is the fallback for a canonical field this table does not name. It
	// exists so that a field added to the context layer without being classified here degrades
	// into a visibly unranked item at the end of the list rather than silently acquiring the
	// precedence of whichever class a default happened to pick. No corpus can produce it today:
	// RelatedPriorityOrigins is the closed set of fields the context layer emits, and a test
	// requires every member of it to classify.
	PriorityUnclassified RelatedPriority = "unclassified"
)

// RelatedPriorities is the closed set, in descending precedence.
//
// The order is the ranking policy written down, exactly as SearchMatchSignals is for search
// ranking. It is a fixed list rather than a sorted one so that the sequence a caller is shown in
// a document or a validation message reads as the model rather than as an alphabetisation of it.
var RelatedPriorities = []RelatedPriority{
	PriorityConceptual,
	PriorityEvidential,
	PriorityAttributive,
	PriorityAssertional,
	PriorityContextual,
	PriorityReferential,
	PriorityNavigational,
}

// RelatedPriorityNames renders the closed set for a document or an error message.
func RelatedPriorityNames() []string {
	out := make([]string, 0, len(RelatedPriorities))
	for _, p := range RelatedPriorities {
		out = append(out, string(p))
	}
	return out
}

// RelatedPriorityRank is the integer precedence of one class: lower is stronger.
//
// It is a small ordinal rather than a weight, and the difference matters. Search ranking sums
// weights because a hit can fire several independent signals at once and the score has to
// combine them; a related item's precedence comes from exactly one canonical field, so there is
// nothing to add up. Introducing a weight here would invite exactly the arithmetic this layer
// must not perform - three weak references outranking one strong one - which is how a precedence
// order quietly becomes a popularity score.
//
// An unclassified class sorts last and deterministically, following SearchEntityRank.
func RelatedPriorityRank(p RelatedPriority) int {
	for i, known := range RelatedPriorities {
		if known == p {
			return i
		}
	}
	return len(RelatedPriorities)
}

// RelatedPriorityOrigins is the closed set of canonical fields a related connection can be read
// from: every origin the Phase 1C adjacency and the Phase 1F context layer between them emit.
//
// It is written down so the classification below is checkable rather than merely asserted. One
// test walks this list and requires every member to classify; a second walks the relations the
// fixture corpus actually produces and requires the same, so a future canonical field cannot
// reach the discovery contract without a deliberate decision about where it ranks.
var RelatedPriorityOrigins = []string{
	OriginNodeRelationships,
	OriginNodeSessionOrigin,
	OriginNodeSources,
	OriginClaimEvidence,
	OriginClaimAttribution,
	OriginClaimAppearsIn,
	OriginClaimDerivedFrom,
	OriginVocabularyNodeRefs,
	OriginVocabularySessionRefs,
	OriginVocabularyRelatedTerms,
	OriginExperimentNodeRefs,
	OriginExperimentVocabularyRefs,
	OriginExperimentSessionRefs,
	OriginExperimentSourceRefs,
	OriginExperimentRelated,
}

// RelatedPriorityFor classifies one canonical field.
//
// It switches on the origin rather than on the relation name, and that is the phase's central
// ranking decision. Relation names are not a closed set: node-to-node edges carry the
// relationship-type IDs from schemas/relationship-types.yaml and their declared inverses, so the
// vocabulary grows whenever the corpus adds a type, and a table keyed by relation name would
// either have to be edited in lockstep with a canonical contract or would silently drop new
// types into a fallback. Origins are closed, are already carried on every relation the context
// layer emits, and name exactly the fact the precedence is about - which canonical field said
// so. It is a switch rather than a map for the reason SearchSignalWeight is: a map invites
// ranging over it, and a ranking policy must never be read in Go map order.
func RelatedPriorityFor(origin string) RelatedPriority {
	switch origin {
	case OriginNodeRelationships:
		return PriorityConceptual
	case OriginClaimEvidence:
		return PriorityEvidential
	case OriginClaimAttribution:
		return PriorityAttributive
	case OriginClaimAppearsIn, OriginClaimDerivedFrom:
		return PriorityAssertional
	case OriginNodeSources, OriginNodeSessionOrigin:
		return PriorityContextual
	case OriginVocabularyNodeRefs, OriginVocabularySessionRefs,
		OriginExperimentNodeRefs, OriginExperimentVocabularyRefs,
		OriginExperimentSessionRefs, OriginExperimentSourceRefs:
		return PriorityReferential
	case OriginVocabularyRelatedTerms, OriginExperimentRelated:
		return PriorityNavigational
	}
	return PriorityUnclassified
}

// RelatedReason is the structured explanation of one canonical connection.
//
// It is structured rather than prose because prose would have to be generated, and generated
// prose is the one kind of explanation this backend cannot check. Every field below is either
// copied from a canonical record or derived by the table above from a field name that was:
// Relation is the canonical relation the corpus uses, Origin is the canonical field it was read
// from, Derived says whether the starting record authored the reference or the backend read
// another record's backwards, and the two priority fields are the precedence class and its rank.
//
// There is no confidence, no score, no similarity and no probability here, and the omission is
// the point. A number in this object would be read as a measurement, and nothing in the corpus
// measures how related two records are.
//
// The reason names neither endpoint. The originating record is the start of the whole response
// and is the origin of every reason in it by construction, and the destination is the item the
// reason belongs to; repeating both on every reason would make a bounded payload grow with
// nothing but restatement. Following either end is done through its own route, addressed by the
// (entity_type, id) pair each already carries.
type RelatedReason struct {
	Relation     string          `json:"relation"`
	Origin       string          `json:"origin"`
	Derived      bool            `json:"derived"`
	Priority     RelatedPriority `json:"priority"`
	PriorityRank int             `json:"priority_rank"`
}

// NewRelatedReason classifies one resolved context relation.
//
// It is the only way a reason is produced, so a reason's priority and the origin it claims to
// have been derived from cannot disagree: there is no path by which an item acquires a
// precedence its own origin does not account for.
func NewRelatedReason(relation ContextRelation) RelatedReason {
	priority := RelatedPriorityFor(relation.Origin)
	return RelatedReason{
		Relation:     relation.Relation,
		Origin:       relation.Origin,
		Derived:      relation.Derived,
		Priority:     priority,
		PriorityRank: RelatedPriorityRank(priority),
	}
}

// RelatedItem is one record the reader could go to next.
//
// EntityType and ID together are the identity, for the reason ContextRef gives: two classes may
// share an ID, so a bare ID is not an address. Title and Summary are the same display fields the
// discovery layer already chose for that class, copied verbatim and never borrowed from another
// record, so an item's own fields describe its own record and nothing else. Neither is the whole
// record: a discovery item says what a record is called and why it is here, and reading it is a
// request to that record's own route.
//
// Reason is the strongest canonical connection between the start and this item, and it is what
// the item's position in the list was decided by. AdditionalEvidence carries the other canonical
// connections between the same two records, bounded, in the same precedence order; it is absent
// when there is only one, so the common case is not padded with an empty list. EvidenceCount is
// how many connections there are in total, so an item whose evidence was cut short says so
// rather than looking like an item with fewer connections than it has.
type RelatedItem struct {
	EntityType SearchEntityType `json:"entity_type"`
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Summary    string           `json:"summary,omitempty"`

	Reason             RelatedReason   `json:"reason"`
	AdditionalEvidence []RelatedReason `json:"additional_evidence,omitempty"`
	EvidenceCount      int             `json:"evidence_count"`
	EvidenceTruncated  bool            `json:"evidence_truncated"`
}

// RelatedStart is the resolved starting record of a discovery request.
//
// It echoes the record the backend actually resolved rather than the caller's raw path, so a
// response describes the discovery that ran. Title is present because a caller who navigated by
// ID alone should be able to confirm they landed on the record they meant before reading a list
// of places to go from it.
type RelatedStart struct {
	EntityType SearchEntityType `json:"entity_type"`
	ID         string           `json:"id"`
	Title      string           `json:"title"`
}

// RelatedCounts reports the size of a discovery result.
//
// Eligible is how many distinct records are related to the start once the requested destination
// scope has been applied, counted before the limit; Returned is how many the response carries.
// The pair is what makes truncation honest: a caller can tell a complete short list from the
// front of a long one without a second request. Eligible is an exact count rather than an
// estimate, because the whole canonical context of every record is resolved once at startup and
// is already bounded, so counting it costs a slice length.
type RelatedCounts struct {
	Eligible int `json:"eligible"`
	Returned int `json:"returned"`
}

// RelatedKnowledge is the related-knowledge discovery projection.
//
// Items is ordered and the order is the answer, so there is no rank field on an item: a rank
// would be the array index restated, and two ways of reading one ordering can disagree after an
// edit. The ordering is documented at the service, and it is total, so two identical requests
// against an unchanged corpus produce the same bytes.
//
// There is no offset and no page object. Paging a discovery list would make it a cursor over a
// derived ordering rather than a bounded answer, and a reader deciding where to go next is not
// working through a result set. A caller who needs everything a record is connected to has that
// already: the context layer serves it under search, and the traversal routes serve the graph
// classes' neighbourhoods.
type RelatedKnowledge struct {
	Start RelatedStart `json:"start"`

	// EntityTypes echoes the destination scope that was applied, in canonical class order rather
	// than the caller's, so the response describes the request that ran. It is absent when the
	// caller supplied none, which is what an unrestricted discovery has always meant.
	EntityTypes []string `json:"entity_types,omitempty"`

	Limit     int           `json:"limit"`
	Counts    RelatedCounts `json:"counts"`
	Truncated bool          `json:"truncated"`
	Items     []RelatedItem `json:"items"`
}
