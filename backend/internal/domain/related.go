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
//
// Phase 2B also makes it the allowlist of the relationship-scope filter, and that reuse is the
// point rather than a convenience: the values a caller may filter by, the classes the ranking is
// defined over and the names an error message lists are one list read three ways. A separate
// filter vocabulary would be a second spelling of a closed seven-value set, free to drift from
// the table that actually decides precedence.
//
// PriorityUnclassified is absent, as it has always been. It is the fallback for a canonical field
// this model does not name and no corpus can produce one today, so accepting it as a filter value
// would offer a caller a scope whose only honest answer is empty.
func RelatedPriorityNames() []string {
	out := make([]string, 0, len(RelatedPriorities))
	for _, p := range RelatedPriorities {
		out = append(out, string(p))
	}
	return out
}

// ValidRelatedPriority reports whether a caller-supplied string names a precedence class.
//
// The comparison is exact and case-sensitive, which is what every canonical filter on this API
// already does: "Conceptual" is refused rather than folded, because a filter that guesses at a
// caller's spelling is a filter that can guess wrong and answer a different question.
func ValidRelatedPriority(value string) bool {
	for _, known := range RelatedPriorities {
		if string(known) == value {
			return true
		}
	}
	return false
}

// RelatedPriorityExplanation renders one precedence class as a fixed sentence.
//
// It is a closed switch over a closed vocabulary and it reads no argument other than the class,
// so the same class always produces the same bytes. Nothing here is generated, interpolated from
// a record, or derived from the text of either endpoint: the sentence says which kind of
// canonical field connected two records and what that kind of field asserts, which is a fact
// about the corpus vocabulary rather than a statement about this particular pair.
//
// The sentences are deliberately about the field and never about the reader's task. None of them
// says a destination is important, relevant, similar or worth reading, because none of those is
// something the corpus wrote down; the caller is told what AudioMuse recorded and decides for
// themselves. They also avoid asserting causality the field does not carry - a source that
// supports a claim is cited by it, which is not the same as having caused it.
//
// A class the table does not name renders the unclassified sentence rather than an empty string,
// so an item explained by an unrecognised field is visibly unranked rather than silently
// unexplained. That is the same degradation RelatedPriorityFor already chooses, for the same
// reason.
func RelatedPriorityExplanation(p RelatedPriority) string {
	switch p {
	case PriorityConceptual:
		return "A typed concept relationship connects the two records."
	case PriorityEvidential:
		return "A claim's evidence list cites one record in support of the other."
	case PriorityAttributive:
		return "A claim's attribution list credits one record to the other."
	case PriorityAssertional:
		return "A claim states that it appears in, or derives from, the other record."
	case PriorityContextual:
		return "A node names the other record as one of its sources or as its session origin."
	case PriorityReferential:
		return "A practice-layer reference list names the other record."
	case PriorityNavigational:
		return "A curated navigation list names the other record; it implies no equivalence and no graph edge."
	}
	return "The canonical field connecting the two records is outside the declared precedence model."
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
// Explanation, added in Phase 2B, is the one human-readable field, and it is a lookup rather than
// a sentence built here: RelatedPriorityExplanation maps the precedence class to a fixed string.
// It carries no new fact - a caller reading Priority already has everything the sentence says -
// and exists because the machine-readable class is a vocabulary term a person reading one
// response has no way to expand. It is on every reason rather than only the winning one because
// NewRelatedReason is the single constructor, so a uniform field cannot drift into a contract
// where some reasons explain themselves and others do not.
//
// There is no confidence, no score, no similarity and no probability here, and the omission is
// the point. A number in this object would be read as a measurement, and nothing in the corpus
// measures how related two records are. Explanation does not reintroduce one in prose: it is one
// of eight fixed strings, it never names either endpoint, and it never says a destination is
// relevant, important or similar.
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
	Explanation  string          `json:"explanation"`
}

// NewRelatedReason classifies one resolved context relation.
//
// It is the only way a reason is produced, so a reason's priority, the origin it claims to have
// been derived from, and the sentence it is explained by cannot disagree: there is no path by
// which an item acquires a precedence its own origin does not account for, and none by which it
// acquires an explanation its own precedence does not account for.
func NewRelatedReason(relation ContextRelation) RelatedReason {
	priority := RelatedPriorityFor(relation.Origin)
	return RelatedReason{
		Relation:     relation.Relation,
		Origin:       relation.Origin,
		Derived:      relation.Derived,
		Priority:     priority,
		PriorityRank: RelatedPriorityRank(priority),
		Explanation:  RelatedPriorityExplanation(priority),
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
// Reason is the strongest eligible canonical connection between the start and this item, and it
// is what the item's position in the list was decided by. AdditionalEvidence carries the other
// eligible canonical connections between the same two records, bounded, in the same precedence
// order; it is absent when there is only one, so the common case is not padded with an empty
// list. EvidenceCount is how many eligible connections there are in total, so an item whose
// evidence was cut short says so rather than looking like an item with fewer connections than it
// has.
//
// "Eligible" is the whole request's relationship scope, and on an unfiltered request - which is
// every Phase 2A request - it is every connection, so nothing above changes meaning. Under a
// Phase 2B relationship filter it is the connections whose precedence class the caller admitted:
// an item's reason and its evidence describe the discovery that ran, so a filtered response never
// explains an item by a class the caller excluded, and never counts one towards the evidence of
// an item it did not rank.
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

// RelatedBounds reports the bounds that shaped one discovery result.
//
// The response already echoes the applied item limit at the top level, because a caller who
// asked for more items than the ceiling allows needs to see which number was actually used.
// These are the other two caps, reported for the same reason - a bounded answer that does not say
// what bounded it cannot be told apart from a complete one - together with what the relation scan
// actually did, since a ceiling nothing reached and a ceiling that cut the work short are
// different facts about the answer.
//
// MaxEvidencePerItem is the cap an item's connection list was cut at, and it is the number that
// makes a per-item EvidenceTruncated flag interpretable - a caller reading "this explanation was
// shortened" can otherwise only guess by how much. MaxRelationsScanned is the ceiling on how many
// canonical relations one request may examine at all, RelationsScanned is how many it did, and
// RelationsTruncated says whether the scan stopped early.
//
// RelationsTruncated is the one flag here that changes how another field must be read. While it
// is false, RelatedCounts.Eligible is the exact number of distinct related records, which is what
// the counts contract promises. While it is true, the scan stopped before the record's canonical
// context ran out, so Eligible is a floor rather than a total. Saying so is the whole point of
// carrying the flag: a silent scan ceiling would turn an exact count into an approximate one with
// nothing in the response to mark the change.
type RelatedBounds struct {
	MaxEvidencePerItem  int  `json:"max_evidence_per_item"`
	MaxRelationsScanned int  `json:"max_relations_scanned"`
	RelationsScanned    int  `json:"relations_scanned"`
	RelationsTruncated  bool `json:"relations_truncated"`
}

// RelatedCounts reports the size of a discovery result.
//
// Eligible is how many distinct records are related to the start once the requested destination
// and relationship scopes have both been applied, counted before the limit; Returned is how many
// the response carries.
// The pair is what makes truncation honest: a caller can tell a complete short list from the
// front of a long one without a second request. Eligible is an exact count rather than an
// estimate, because the whole canonical context of every record is resolved once at startup and
// is already bounded, so counting it costs a slice length.
//
// The one case where Eligible is a floor rather than a total is a request whose relation scan hit
// its ceiling, and RelatedBounds.RelationsTruncated is how a caller knows that happened.
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

	// RelationshipTypes echoes the relationship scope that was applied, in precedence order
	// rather than the caller's, added in Phase 2B. It is the same convention EntityTypes uses and
	// deliberately so: one applied filter is echoed under the name of the parameter that applied
	// it, normalised, and is absent when the caller supplied none.
	//
	// Absent therefore means every precedence class, which is what an unfiltered discovery has
	// always returned. The alternative - echoing all seven classes on an unfiltered request - was
	// rejected because it would make the two requests indistinguishable in the response while
	// they differ in the contract: a caller who names all seven has pinned the scope against a
	// model that may later grow a class, and a caller who names none has not.
	RelationshipTypes []string `json:"relationship_types,omitempty"`

	Limit     int           `json:"limit"`
	Bounds    RelatedBounds `json:"bounds"`
	Counts    RelatedCounts `json:"counts"`
	Truncated bool          `json:"truncated"`
	Items     []RelatedItem `json:"items"`
}
