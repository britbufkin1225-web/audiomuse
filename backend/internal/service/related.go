package service

import (
	"errors"
	"sort"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Phase 2A: deterministic related-knowledge discovery.
//
// Every earlier phase answers a question the caller already knows how to ask. Phase 1A-1D list a
// class the caller named, Phase 1C walks a graph from a vertex the caller named, and Phase 1E-1J
// find text the caller typed. This file answers the one question a reader has while holding a
// record and no query: given this, what else in AudioMuse should I read, and why that.
//
// It derives nothing new from the corpus. The canonical references of every searchable record
// are already resolved once at startup by the Phase 1F context layer - authored references and
// their documented reverse reads, each carrying the canonical field it came from - and this file
// reads that projection. Re-deriving the connections here would create a second definition of
// one repository fact that is free to disagree with the first, which is exactly the failure the
// traversal and context layers were each careful to avoid.
//
// What the phase adds is precedence and bounds. The context layer returns everything a record
// references in a reading order; a discovery answer has to be short, which means something has
// to decide what is cut, and "whatever the corpus listed last" is not a decision. domain.related
// holds that decision as a closed table over canonical field names, and this file applies it,
// deduplicates the destinations, orders the result totally, and cuts it at a documented bound.
//
// Nothing here measures similarity. There is no embedding, no vector, no keyword overlap, no
// co-occurrence count and no model call, and no request can introduce one: the operation takes
// no query text at all. Two records are related here if and only if some canonical record wrote
// down a reference between them.

// Related-knowledge bounds.
//
// These are service constants rather than configuration, for the reason the traversal and
// context bounds give: they are API safety invariants, not deployment choices. A caller able to
// raise them could turn one navigation request into a corpus dump, and an operator able to lower
// them would change what the documented contract means.
const (
	// DefaultRelatedLimit is what a caller who names no limit receives.
	//
	// It is the same number as MaxContextRelationsPerResult, and deliberately so: that bound was
	// sized to be a navigable list rather than a record dump, and this list is read for exactly
	// that purpose. Past roughly this many suggestions a reader is no longer choosing where to
	// go next, they are reading an index.
	DefaultRelatedLimit = 25

	// MaxRelatedLimit is the hard ceiling. It cannot be disabled by any parameter.
	//
	// It is well below the 200 that pages a search result set, because the two are different
	// requests. A search page is a result set a caller works through; a discovery list is a
	// choice a reader makes, and a hundred options is already past the point where ranking them
	// helps. It also keeps the whole response bounded by a number that can be stated: at most
	// MaxRelatedLimit items each carrying at most MaxRelatedEvidencePerItem reasons, which is
	// 500 canonical relations, a quarter of the 2,000 the API already says no single request
	// exceeds.
	MaxRelatedLimit = 100

	// MaxRelatedRelationsScanned bounds how many canonical relations one request may examine.
	//
	// The other two bounds cap what a response carries; this one caps the work behind it. Without
	// it the cost of a request would be bounded by a property of the corpus - the widest context
	// any one record happens to have - rather than by anything this package declares. That is a
	// bound in practice and not in contract, and the difference shows up exactly when the corpus
	// grows past the size the bounds were reasoned about at.
	//
	// It is the same number as MaxContextRelationsPerResponse and MaxTraversalEdges, so one
	// statement still covers the whole API: no single request examines more than 2,000 canonical
	// relationships, whichever route asked for them. Against the corpus today the widest single
	// context is 75 relations, so the ceiling has more than an order of magnitude of headroom and
	// no request reaches it; it is a safety invariant rather than part of the answer.
	//
	// A scan that stops early is reported rather than hidden. It is the one bound that can make
	// RelatedCounts.Eligible a floor instead of a total, and a caller cannot be left to discover
	// that by comparing counts across requests.
	MaxRelatedRelationsScanned = 2000

	// MaxRelatedEvidencePerItem bounds how many canonical connections one item may report,
	// counting the primary reason.
	//
	// A bound is needed because evidence is a per-item list inside a per-response list, so
	// without one the payload would be the product of two corpus properties rather than of two
	// constants. It is small because the list is an explanation, not a record: the strongest
	// connection is the reason the item is here, and a handful of others is enough to show that
	// the pair is connected in more than one way. An item whose evidence is cut short says so
	// and reports its full count, so nothing is dropped silently.
	MaxRelatedEvidencePerItem = 5
)

// ErrUnsupportedRelatedEntityType reports a starting class outside domain.SearchEntityTypes.
//
// It is a distinct error from ErrUnsupportedEntityType, which names the four graph classes.
// Related-knowledge discovery starts from any of the six searchable classes, so an error listing
// the graph's four would tell a caller asking about a vocabulary entry that vocabulary entries
// are not supported - which is false, and is precisely the confusion between the graph model and
// the discovery model that the earlier phases spell out at length.
var ErrUnsupportedRelatedEntityType = errors.New("unsupported related-knowledge entity type")

// Relationship-scope errors, added in Phase 2B.
//
// They mirror the Phase 1H class-list errors one for one, and the parallel is deliberate: a
// caller who has learned what a malformed entity_types list is refused for should not have to
// learn a second set of rules for the second list on the same route. A malformed filter is
// refused rather than repaired, for the reason every other malformed filter on this API is - a
// discovery whose scope silently differed from the one the caller wrote would return a list, an
// eligible count and a set of explanations that do not mean what they think they mean, and unlike
// a mistyped query string that is invisible in the response.
//
// They are distinct error values rather than reuses of the entity_types ones because each renders
// into a message naming its own parameter. A caller who wrote one comma too many in
// relationship_types must be told which of the two lists on this route was wrong.
var (
	// ErrEmptyRelatedPriority reports a relationship-class list carrying a blank member, which is
	// what a leading, trailing or doubled separator produces, and what a whitespace-only value
	// trims to.
	ErrEmptyRelatedPriority = errors.New("relationship type filter must not contain an empty value")

	// ErrDuplicateRelatedPriority reports a precedence class named twice. A repeated class cannot
	// change which relations are admitted, so accepting it would be harmless and refusing it is
	// still right: the two things a caller might mean - a set and a multiset - differ, and a
	// filter that quietly collapses one into the other is a filter whose contract is guessed at.
	//
	// Rejecting rather than normalising is the existing convention on this API rather than a new
	// decision. entity_types has refused a repeated class since Phase 1H, and the two lists sit
	// on one route; normalising one while refusing the other would be the drift both spellings
	// were written to avoid.
	ErrDuplicateRelatedPriority = errors.New("relationship type filter must not repeat a value")
)

// RelatedQuery is a bounded, deterministic related-knowledge request.
//
// It carries no query text and no depth, and both omissions are the contract rather than a gap.
// Text would make this search, which exists; depth would make it traversal, which also exists.
// What is left is the two things a navigation request legitimately says: how much to return, and
// which kinds of record the reader is interested in.
//
// Limit zero means unspecified and receives DefaultRelatedLimit. EntityTypes is the destination
// scope and reuses the Phase 1H class-list semantics exactly - trimmed, exact, closed set, no
// blank member, no repetition - because it is the same restriction over the same six classes and
// a second spelling of one filter would be a second contract to keep in step. An empty slice
// means every searchable class.
//
// There is deliberately no single-class Type alongside it. Phase 1H carries both spellings
// because Type predates the list and an existing contract had to keep working; this route has no
// such history, so it offers the set spelling only rather than being born with two ways to say
// one thing.
//
// RelationshipTypes is the Phase 2B relationship scope and is the third thing a navigation
// request legitimately says: which kinds of connection the reader is interested in. It is a
// filter and never a ranking parameter - it decides which canonical relations are eligible, and
// the precedence among whatever remains is the same closed table it has always been. A caller
// cannot promote a class by naming it, cannot reorder the classes by the order they are written
// in, and cannot introduce a weight; a scope of one class returns exactly the items an unfiltered
// discovery would have ranked in that class, in the same relative order.
//
// Its members are the precedence classes of domain.RelatedPriorities, and the semantics follow
// EntityTypes exactly - trimmed, exact, closed set, no blank member, no repetition, an empty
// slice meaning every class - so the two lists on one route are one set of rules rather than two.
type RelatedQuery struct {
	Limit             int
	EntityTypes       []string
	RelationshipTypes []string
}

// relatedScope is the resolved set of precedence classes one discovery may be explained by.
//
// An empty scope is every class, which is what an unfiltered request has always meant, and it is
// the same shape searchScope has for the same reason: one type carries the admission test, so
// the grouping loop asks one question per axis rather than branching on whether a filter is set.
type relatedScope []domain.RelatedPriority

// has is exact membership, and is separate from admits for the reason searchScope.has is: an
// empty scope admits everything and contains nothing, and the duplicate check needs the second
// question rather than the first.
func (s relatedScope) has(p domain.RelatedPriority) bool {
	for _, allowed := range s {
		if allowed == p {
			return true
		}
	}
	return false
}

// admits reports whether a connection of this precedence class may explain an item.
func (s relatedScope) admits(p domain.RelatedPriority) bool {
	return len(s) == 0 || s.has(p)
}

// names renders the scope for the response echo, in precedence order. The slice is fresh on every
// call, so an echoed scope can never alias anything the index holds.
func (s relatedScope) names() []string {
	out := make([]string, 0, len(s))
	for _, p := range s {
		out = append(out, string(p))
	}
	return out
}

// resolveRelatedScope validates the relationship filter and returns the classes a discovery may be
// explained by, or nil for every class.
//
// It is a pure function of the request and runs before any grouping, so a malformed filter costs
// one pass over at most a handful of short strings rather than a pass over the projection. That
// ordering is also the bound: a request whose filter is refused never reaches the relation scan
// at all, so no spelling of a filter can buy work.
//
// Members are trimmed and then compared exactly, which is the comparison every canonical filter on
// this API already uses: "Conceptual" and "CONCEPTUAL" are refused rather than folded, because a
// filter that guesses at a caller's spelling is a filter that can guess wrong and answer a
// different question. Nothing else is normalised - there is no alias table, no plural form and no
// grouping of classes into families, because the API has no such convention to follow and
// inventing one here would make this list the only place a model term is not written as the model
// spells it.
//
// The returned scope is in precedence order rather than the caller's, so two spellings of one
// scope produce one response. That is what makes the filter order-independent rather than merely
// order-tolerant: the echo, the admission test and the result are all functions of the set.
//
// unclassified is refused with every other unknown value, and deliberately: it is the fallback for
// a canonical field the model does not name, no corpus can produce one, and a scope that accepted
// it would answer with an empty set a caller could read as "nothing connects these records in an
// unrecognised way" rather than as "that is not a class you may ask for".
func resolveRelatedScope(requested []string) (relatedScope, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	scope := make(relatedScope, 0, len(requested))
	for _, raw := range requested {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, ErrEmptyRelatedPriority
		}
		if !domain.ValidRelatedPriority(value) {
			return nil, &InvalidFilterError{Param: "relationship_types", Allowed: domain.RelatedPriorityNames()}
		}
		class := domain.RelatedPriority(value)
		if scope.has(class) {
			return nil, ErrDuplicateRelatedPriority
		}
		scope = append(scope, class)
	}
	sort.SliceStable(scope, func(i, j int) bool {
		return domain.RelatedPriorityRank(scope[i]) < domain.RelatedPriorityRank(scope[j])
	})
	return scope, nil
}

// relatedGroup collects every canonical connection between the start and one destination record.
//
// It is the unit deduplication works over: a destination appears in a discovery result once, no
// matter how many canonical fields reach it, because the reader is being offered a record to
// open rather than a list of edges to read. The relations are kept rather than counted, so the
// strongest can become the reason and the rest can be reported as further evidence.
type relatedGroup struct {
	ref       searchRef
	relations []domain.ContextRelation
}

// buildRelated indexes the display summary of every searchable record.
//
// Titles are not indexed again: they come from searchLabels, which the context layer already
// built from the discovery documents, so a related item's title is by construction the same
// string as that record's search title and its context label. Summaries need their own map
// because Phase 1F had no use for them, and widening searchLabels from a string map into a
// struct map would reshape a Phase 1F index to serve a Phase 2A need - the kind of change that
// makes a later reader unable to tell which phase a field belongs to.
//
// It runs once, in New, after every projection it reads, so the map is never written to again
// and the immutability that makes Knowledge safe for concurrent readers still holds.
func (k *Knowledge) buildRelated() {
	k.relatedSummary = make(map[searchRef]string, len(k.searchDocs))
	for _, doc := range k.searchDocs {
		if doc.summary == "" {
			// A class with no natural summary field contributes none rather than an empty
			// string, following the discovery layer's own rule that a record does not acquire a
			// field its contract does not give it.
			continue
		}
		k.relatedSummary[searchRef{entityType: doc.entityType, id: doc.id}] = doc.summary
	}
}

// RelatedKnowledgeFor returns the bounded, ranked, explained set of records related to one
// canonical record.
//
// The order of operations is fixed and each step is separable: validate the class, validate the
// destination scope, validate the relationship scope, resolve the start, group the start's
// eligible canonical context by destination, rank each group, order the groups, count, then cut
// at the limit. Counting before cutting is what lets the response report an exact eligible total;
// cutting before counting would make the total the size of the page, which the caller can already
// see.
//
// Both filters are applied before the limit and neither is applied after it, which is the
// difference between a filter and a post-filter over a page. A caller asking for one relationship
// class and twenty-five items receives up to twenty-five items of that class, not whatever
// survives of the first twenty-five items of the unfiltered ranking.
//
// The whole request is validated before the start is looked up, so a malformed request is never
// reported as a missing record. It matters because the two failures ask different things of the
// caller: a rejected scope means "fix the query string you wrote", a missing start means "that
// record is not in this corpus", and a caller who sent both mistakes at once and was told only
// about the ID would fix it, resend, and be refused a second time for a mistake that was already
// visible in the request they sent.
//
// The Phase 1C routes already do this for the one bound they check at the edge - an out-of-range
// depth is refused on a root that does not resolve - but resolve the root before validating the
// relationship and target_type vocabularies, so a mistyped filter there is still reported as a
// missing entity. That is a Phase 1C contract and this phase does not reach into it. What is
// decided here is only what this route does, and doing it in the other order would have made a
// second surface behave that way rather than one fewer.
//
// A class outside the searchable six is ErrUnsupportedRelatedEntityType and an ID that resolves
// to no record is ErrNotFound. The two are kept apart deliberately, and neither is answered with
// an empty list: a record that exists and references nothing, and a record the corpus has never
// contained, are different facts, and answering the second with the first would let a caller
// build a view of a record that does not exist.
func (k *Knowledge) RelatedKnowledgeFor(entityType, id string, q RelatedQuery) (domain.RelatedKnowledge, error) {
	if !domain.ValidSearchEntityType(entityType) {
		return domain.RelatedKnowledge{}, ErrUnsupportedRelatedEntityType
	}
	// Scope is resolved before the start is, and before any grouping. It is the cheapest
	// validation on the request and the one most likely to be wrong in a hand-written query
	// string, and refusing it first means a malformed class list neither reaches the projection
	// nor is masked by a lookup that missed. The single-class spelling is passed as empty
	// because this route does not offer one; resolveSearchScope is otherwise the Phase 1H
	// function unchanged, so the two surfaces cannot drift about what a class list means.
	scope, err := resolveSearchScope("", q.EntityTypes)
	if err != nil {
		return domain.RelatedKnowledge{}, err
	}
	// The relationship scope is resolved next and for the same reasons: it is cheap, it is a
	// filter rather than an address, and refusing it here means a malformed class list neither
	// reaches the projection nor is masked by a lookup that missed. The two filters are validated
	// in the order they appear in the accepted query string, so a request carrying two malformed
	// lists is refused with the same error on every run.
	relationScope, err := resolveRelatedScope(q.RelationshipTypes)
	if err != nil {
		return domain.RelatedKnowledge{}, err
	}
	start := searchRef{entityType: domain.SearchEntityType(entityType), id: id}
	title, ok := k.searchLabels[start]
	if !ok {
		return domain.RelatedKnowledge{}, ErrNotFound
	}
	limit := normaliseRelatedLimit(q.Limit)

	groups, scanned, scanCut := k.groupRelated(start, scope, relationScope, MaxRelatedRelationsScanned)
	items := make([]domain.RelatedItem, 0, len(groups))
	for _, group := range groups {
		items = append(items, k.relatedItem(group))
	}
	sort.SliceStable(items, func(i, j int) bool { return lessRelatedItem(items[i], items[j]) })

	eligible := len(items)
	if len(items) > limit {
		items = items[:limit]
	}

	// Each echo is present exactly when the caller supplied that filter, and carries the
	// normalised scope rather than their spelling of it. A caller can therefore tell an
	// unrestricted axis from a restricted one, and can see what their list normalised to, without
	// the response having to carry a separate "was this filtered" flag beside each list.
	var echoTypes []string
	if len(q.EntityTypes) > 0 {
		echoTypes = scope.names()
	}
	var echoRelationships []string
	if len(q.RelationshipTypes) > 0 {
		echoRelationships = relationScope.names()
	}
	return domain.RelatedKnowledge{
		Start:             domain.RelatedStart{EntityType: start.entityType, ID: start.id, Title: title},
		EntityTypes:       echoTypes,
		RelationshipTypes: echoRelationships,
		Limit:             limit,
		Bounds: domain.RelatedBounds{
			MaxEvidencePerItem:  MaxRelatedEvidencePerItem,
			MaxRelationsScanned: MaxRelatedRelationsScanned,
			RelationsScanned:    scanned,
			RelationsTruncated:  scanCut,
		},
		Counts:    domain.RelatedCounts{Eligible: eligible, Returned: len(items)},
		Truncated: len(items) < eligible,
		Items:     items,
	}, nil
}

// groupRelated collects the start's canonical context into one group per destination record.
//
// Iteration is over the context slice, which the Phase 1F builder already ordered, and the group
// list is appended to in first-encounter order with a map used only to find an existing group.
// The map is never ranged over, so the grouping is a function of the corpus rather than of Go
// map iteration; the explicit sort afterwards would fix the order anyway, and this keeps the
// input to that sort reproducible as well.
//
// Three exclusions run here rather than later, and the order they are written in is the order
// they are cheapest in rather than a precedence among them.
//
// A destination outside the requested class scope is not an answer to the question that was
// asked. The start itself is not an answer either: a discovery result telling a reader to go and
// read the record they are already holding is not a place to go next. And a relation whose
// precedence class the caller excluded is not eligible to explain anything, which is the Phase 2B
// filter.
//
// The relationship filter is applied to the relation rather than to the finished item, and that
// is the phase's central filtering decision. Filtering items after grouping would have two
// consequences the contract cannot carry: an item admitted by its strongest connection would keep
// reporting excluded connections as its evidence and its evidence count, so a filtered response
// would explain an item by a class the caller had removed; and an item whose strongest connection
// was excluded would vanish entirely even where a weaker admitted connection also reaches it, so
// a filter would silently drop records that satisfy it. Filtering the relations makes the
// eligible set exactly "the connections the caller asked about", and everything downstream -
// the winning reason, the evidence, the count, the ranking - is then computed over that set
// without needing to know a filter was applied at all.
//
// Excluding the start is defence in depth rather than a filter the corpus needs - every canonical
// layer with a reference list treats a record referencing itself as a fatal validation issue, so
// an index that started cannot hold one - and it is kept because the cost is one comparison and
// the alternative is a contract that depends on a rule enforced three packages away. That
// exclusion is on identity, class and ID together, so the registry entry and the session
// projected from it, which share an ID and are two different records, are never confused for one
// another.
//
// maxScanned bounds the relations examined and is returned alongside the count actually examined
// and whether the scan stopped early. It is a parameter rather than a constant read from inside
// the loop, following contextFor, which takes its response budget the same way: a function that
// reaches for its own ceiling is not a function of its inputs, and the bound that shaped a result
// should be visible at the call site that assembles the response rather than buried at the point
// it happens to be enforced. Every caller in this package passes MaxRelatedRelationsScanned.
//
// The scan is counted before every exclusion rather than after, and that is the whole point of
// the bound. Counting only what survives would let a request with a narrow scope walk an
// unbounded number of relations while admitting almost none of them, which is precisely the work
// a ceiling on examined relations exists to cap. A caller therefore cannot use either filter to
// buy a larger scan, and the scan count a filtered response reports is the same number the
// unfiltered one reports: it describes the work done, not the answer produced.
func (k *Knowledge) groupRelated(
	start searchRef, scope searchScope, relationScope relatedScope, maxScanned int,
) ([]relatedGroup, int, bool) {
	relations := k.searchContext[start]
	scanned := len(relations)
	truncated := false
	// A non-positive ceiling is read as unbounded rather than as a request to examine nothing,
	// following normaliseRelatedLimit, where zero means unspecified rather than empty. No caller
	// passes one; the guard is here so that an internal misuse degrades into the behaviour that
	// preceded this bound rather than into a panic on a negative slice bound.
	if maxScanned > 0 && scanned > maxScanned {
		scanned, truncated = maxScanned, true
	}

	groups := make([]relatedGroup, 0, scanned)
	index := make(map[searchRef]int, scanned)

	for _, relation := range relations[:scanned] {
		ref := searchRef{entityType: relation.Entity.EntityType, id: relation.Entity.ID}
		if ref == start {
			continue
		}
		if !scope.admits(ref.entityType) {
			continue
		}
		// The precedence class is read from the same closed table the reason will be built from,
		// so a relation the filter admits and the reason it produces cannot disagree about which
		// class this connection belongs to. An unclassified origin is admitted only by an
		// unfiltered request, because unclassified is not a value the filter accepts.
		if !relationScope.admits(domain.RelatedPriorityFor(relation.Origin)) {
			continue
		}
		at, seen := index[ref]
		if !seen {
			at = len(groups)
			index[ref] = at
			groups = append(groups, relatedGroup{ref: ref})
		}
		groups[at].relations = append(groups[at].relations, relation)
	}
	return groups, scanned, truncated
}

// relatedItem turns one destination's connections into one explained, bounded result.
//
// The relations are ordered by the same precedence the items themselves are ordered by, so the
// strongest connection becomes the reason the item is here and the ordering of an item and the
// ordering inside it are one policy rather than two. That is the deduplication rule in full: one
// item per destination, the strongest canonical connection as its reason, the rest reported as
// further evidence in the same order, cut at MaxRelatedEvidencePerItem with the full count kept.
//
// Title and Summary are this record's own display fields and nothing else. A related item never
// borrows text from the start or from the connection, so an item cannot present another record's
// words as its own.
func (k *Knowledge) relatedItem(group relatedGroup) domain.RelatedItem {
	reasons := make([]domain.RelatedReason, 0, len(group.relations))
	for _, relation := range group.relations {
		reasons = append(reasons, domain.NewRelatedReason(relation))
	}
	sort.SliceStable(reasons, func(i, j int) bool { return lessRelatedReason(reasons[i], reasons[j]) })

	kept := len(reasons)
	if kept > MaxRelatedEvidencePerItem {
		kept = MaxRelatedEvidencePerItem
	}
	// The primary reason is reasons[0] and is not repeated inside the additional list, so no
	// connection is reported twice and a client counting what it received cannot double-count.
	// The slice is fresh, so a response never aliases the startup index.
	var additional []domain.RelatedReason
	if kept > 1 {
		additional = append([]domain.RelatedReason(nil), reasons[1:kept]...)
	}

	return domain.RelatedItem{
		EntityType:         group.ref.entityType,
		ID:                 group.ref.id,
		Title:              k.searchLabels[group.ref],
		Summary:            k.relatedSummary[group.ref],
		Reason:             reasons[0],
		AdditionalEvidence: additional,
		EvidenceCount:      len(reasons),
		EvidenceTruncated:  kept < len(reasons),
	}
}

// lessRelatedReason is the total order canonical connections are ranked by.
//
// Precedence first, which is the phase's ranking policy. Then an authored reference before a
// derived one: an authored reference is something the starting record itself wrote down, and a
// derived one is another record's reference read backwards, so presenting the record's own
// statements first matches how a reader reads the record. Direction is the second key rather
// than the first because the canonical field is the stronger statement about what kind of
// connection this is, and which way it was written is secondary to what it says.
//
// The last two keys are the relation name and the canonical field. They are tie-breakers rather
// than a reading order, and they make the order total: the context layer deduplicates on
// relation, destination, origin and direction together, so no two connections between the same
// pair of records agree on all four keys.
func lessRelatedReason(a, b domain.RelatedReason) bool {
	if a.PriorityRank != b.PriorityRank {
		return a.PriorityRank < b.PriorityRank
	}
	if a.Derived != b.Derived {
		return !a.Derived
	}
	if a.Relation != b.Relation {
		return a.Relation < b.Relation
	}
	return a.Origin < b.Origin
}

// lessRelatedItem is the total order a discovery result is returned in.
//
// The first two keys are the primary reason's precedence and direction, so the list reads
// strongest connection first. The last two are the destination's canonical class in model order
// and then its canonical ID, which is the tie-break every ordered projection in this service
// already uses; the pair is unique because a destination appears exactly once after grouping, so
// no two items ever compare equal and the order is total. That is what makes repeated runs and
// separately built indexes produce the same bytes rather than merely the same set.
//
// The obvious alternative was to sort by relation name after the precedence class, the way
// sortContextRelations does, which would group an item's neighbours by the words used to connect
// them. It was rejected because the two lists are read differently: a context list is read as
// the record's reference structure, where grouping by relation is the structure, while a
// discovery list is read as a set of records to choose between, where the class of record is
// what a reader is choosing among and the relation name within one precedence class says almost
// nothing to separate them.
func lessRelatedItem(a, b domain.RelatedItem) bool {
	if a.Reason.PriorityRank != b.Reason.PriorityRank {
		return a.Reason.PriorityRank < b.Reason.PriorityRank
	}
	if a.Reason.Derived != b.Reason.Derived {
		return !a.Reason.Derived
	}
	if a.EntityType != b.EntityType {
		return domain.SearchEntityRank(a.EntityType) < domain.SearchEntityRank(b.EntityType)
	}
	return a.ID < b.ID
}

// normaliseRelatedLimit applies the limit contract.
//
// Absent or zero is the default and an over-large value is clamped to the ceiling, which is what
// limit means on every other route of this API. It is deliberately not the traversal depth
// contract, where an out-of-range value is refused: depth changes the meaning of a request -
// a caller who asked for three hops and silently received one would believe they had seen the
// whole neighbourhood - whereas a clamped limit returns the front of the same ordering the
// caller asked for, and the response echoes the limit that was applied along with the eligible
// total, so a clamp is visible rather than silent.
//
// A negative limit does not reach here from HTTP, where the shared integer parser refuses it, and
// is treated as unspecified for a direct caller rather than being read as a request to return
// nothing.
func normaliseRelatedLimit(limit int) int {
	if limit <= 0 {
		return DefaultRelatedLimit
	}
	if limit > MaxRelatedLimit {
		return MaxRelatedLimit
	}
	return limit
}
