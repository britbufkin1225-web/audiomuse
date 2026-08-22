package service

import (
	"sort"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// InvalidFilterError reports a filter value that is outside a canonical vocabulary.
//
// Allowed is the vocabulary read from the canonical contract at startup, so echoing it back
// to a caller discloses only repository contract data. The caller's own value is not echoed
// by the service; the HTTP layer decides what is safe to reflect.
type InvalidFilterError struct {
	Param   string
	Allowed []string
}

func (e *InvalidFilterError) Error() string {
	return "unsupported value for filter " + e.Param
}

// SourceQuery is a bounded, deterministic source list request.
//
// Type, Relationship, EvidenceClass and Retrieval are bounded by schemas/source.schema.yaml
// and are rejected when outside it. ClaimID, NodeID and SessionID are canonical identifiers
// and are not validated against existence: an unknown ID yields an empty result set, which
// is what "no source stands in this relation to that record" means, and matches how the
// Phase 1A session filter already behaves.
type SourceQuery struct {
	Q             string
	Type          string
	Relationship  string
	EvidenceClass string
	Retrieval     string
	ClaimID       string
	NodeID        string
	SessionID     string
	Limit         int
	Offset        int
}

// SourceList is the source list projection.
type SourceList struct {
	Page    Page                   `json:"page"`
	Sources []domain.SourceSummary `json:"sources"`
}

// ClaimQuery is a bounded, deterministic claim list request.
//
// ClaimType, Confidence, DisputeStatus, TemporalPrecision and Relation are bounded by
// schemas/claim.schema.yaml and are rejected when outside it. Relation selects claims
// carrying at least one evidence entry with that relation, which is what makes
// "?source_id=x&relation=contradicted_by" answerable.
type ClaimQuery struct {
	Q                 string
	ClaimType         string
	Confidence        string
	DisputeStatus     string
	TemporalPrecision string
	Relation          string
	SourceID          string
	NodeID            string
	SessionID         string
	Limit             int
	Offset            int
}

// ClaimList is the claim list projection.
type ClaimList struct {
	Page   Page                  `json:"page"`
	Claims []domain.ClaimSummary `json:"claims"`
}

// buildEvidence constructs every evidence-layer index.
//
// All of it happens once, inside New, so the maps are never written to again and the
// immutability that makes Knowledge safe for concurrent readers still holds. Each reverse
// index below exists because one endpoint or filter would otherwise have to scan the whole
// claim set per request:
//
//	claimIDsBySourceID      /api/v1/claims?source_id=
//	claimIDsByNodeID        /api/v1/claims?node_id=
//	claimIDsBySessionID     /api/v1/claims?session_id=
//	sourceClaims            /api/v1/sources/{id}.claims
//	attributedClaimIDs      /api/v1/sources/{id}.attributed_claim_ids
//	nodeIDsBySourceID       /api/v1/sources/{id}.node_ids and the source list node count
//	sessionIDsBySourceID    /api/v1/sources/{id}.session_ids
//	sourceIDsBySessionID    /api/v1/sources?session_id=
//
// No index is built for claim -> source, claim -> node or source -> claim-cited-nodes:
// the first two are already fields on the claim record and the third has no endpoint.
func (k *Knowledge) buildEvidence() {
	k.claimIDsBySourceID = map[string][]string{}
	k.claimIDsByNodeID = map[string][]string{}
	k.claimIDsBySessionID = map[string][]string{}
	k.sourceClaims = map[string][]domain.SourceClaimRef{}
	k.attributedClaimIDs = map[string][]string{}
	k.nodeIDsBySourceID = map[string][]string{}
	k.sessionIDsBySourceID = map[string][]string{}
	k.sourceIDsBySessionID = map[string][]string{}

	// Topical: the nodes whose canonical sources: list names a source. This is Phase 1A
	// data and is kept apart from claim evidence throughout; see domain.SourceDetail.
	for _, node := range k.nodes {
		for _, sourceID := range node.Sources {
			k.nodeIDsBySourceID[sourceID] = append(k.nodeIDsBySourceID[sourceID], node.ID)
		}
	}

	// Claims are already in canonical ID order, so every list appended here is built in
	// that order and needs deduplication rather than sorting.
	for _, claim := range k.claims {
		k.claimSearchText[claim.ID] = strings.ToLower(claim.ID + "\n" + claim.Statement)

		citedSources := map[string]bool{}
		for _, e := range claim.Evidence {
			k.sourceClaims[e.SourceID] = append(k.sourceClaims[e.SourceID],
				domain.SourceClaimRef{ClaimID: claim.ID, Relation: e.Relation})
			citedSources[e.SourceID] = true
		}
		for _, a := range claim.Attribution {
			k.attributedClaimIDs[a.SourceID] = appendDistinct(k.attributedClaimIDs[a.SourceID], claim.ID)
			citedSources[a.SourceID] = true
		}
		for _, sourceID := range sortedSet(citedSources) {
			k.claimIDsBySourceID[sourceID] = append(k.claimIDsBySourceID[sourceID], claim.ID)
		}

		for _, ref := range claim.AppearsIn {
			switch ref.Kind {
			case domain.ClaimKindNode:
				k.claimIDsByNodeID[ref.Ref] = appendDistinct(k.claimIDsByNodeID[ref.Ref], claim.ID)
			case domain.ClaimKindSession:
				k.claimIDsBySessionID[ref.Ref] = appendDistinct(k.claimIDsBySessionID[ref.Ref], claim.ID)
				// Source <-> session is claim-mediated: a source is connected to a session
				// when a claim citing it appears there. There is no direct canonical edge
				// between a registry entry and a session, and inventing one from node
				// session_origin would silently merge the topical and evidential relations.
				for _, sourceID := range sortedSet(citedSources) {
					k.sourceIDsBySessionID[ref.Ref] = appendDistinct(k.sourceIDsBySessionID[ref.Ref], sourceID)
					k.sessionIDsBySourceID[sourceID] = appendDistinct(k.sessionIDsBySourceID[sourceID], ref.Ref)
				}
			}
		}
	}

	for id, refs := range k.sourceClaims {
		sort.SliceStable(refs, func(i, j int) bool {
			if refs[i].ClaimID != refs[j].ClaimID {
				return refs[i].ClaimID < refs[j].ClaimID
			}
			return refs[i].Relation < refs[j].Relation
		})
		k.sourceClaims[id] = refs
	}
	for id, ids := range k.nodeIDsBySourceID {
		sort.Strings(ids)
		k.nodeIDsBySourceID[id] = ids
	}
	for id, ids := range k.sessionIDsBySourceID {
		sort.Strings(ids)
		k.sessionIDsBySourceID[id] = ids
	}
	for id, ids := range k.sourceIDsBySessionID {
		sort.Strings(ids)
		k.sourceIDsBySessionID[id] = ids
	}
}

// ListSources filters, searches and pages the canonical source registry.
//
// Multiple filters compose with AND. Results keep canonical ID order, matching every other
// AudioMuse list projection, and search does not reorder by relevance for the reason
// ListNodes gives: ranking would require a weighting judgement the repository has no basis
// for, and an ID-ordered list is reproducible byte for byte.
func (k *Knowledge) ListSources(q SourceQuery) (SourceList, error) {
	vocab := k.vocabularies.Source
	for _, check := range []struct {
		param, value string
		allowed      []string
	}{
		{"type", q.Type, vocab.Types},
		{"relationship", q.Relationship, vocab.Relationships},
		{"evidence_class", q.EvidenceClass, vocab.EvidenceClasses},
		{"retrieval", q.Retrieval, vocab.Retrievals},
	} {
		if check.value != "" && !contains(check.allowed, check.value) {
			return SourceList{}, &InvalidFilterError{Param: check.param, Allowed: check.allowed}
		}
	}

	limit, offset := normalisePaging(q.Limit, q.Offset)
	needle := boundedNeedle(q.Q)

	var byClaim, byNode, bySession map[string]bool
	if q.ClaimID != "" {
		byClaim = newIDSet(k.sourceIDsForClaim(q.ClaimID))
	}
	if q.NodeID != "" {
		if node, ok := k.nodesByID[q.NodeID]; ok {
			byNode = newIDSet(node.Sources)
		} else {
			byNode = map[string]bool{}
		}
	}
	if q.SessionID != "" {
		bySession = newIDSet(k.sourceIDsBySessionID[q.SessionID])
	}

	matched := make([]domain.SourceSummary, 0, len(k.sources))
	for _, source := range k.sources {
		if q.Type != "" && source.Type != q.Type {
			continue
		}
		if q.Relationship != "" && source.Relationship != q.Relationship {
			continue
		}
		if q.EvidenceClass != "" && (source.EvidenceClass == nil || *source.EvidenceClass != q.EvidenceClass) {
			continue
		}
		if q.Retrieval != "" && (source.Retrieval == nil || *source.Retrieval != q.Retrieval) {
			continue
		}
		if byClaim != nil && !byClaim[source.ID] {
			continue
		}
		if byNode != nil && !byNode[source.ID] {
			continue
		}
		if bySession != nil && !bySession[source.ID] {
			continue
		}
		if needle != "" && !strings.Contains(k.sourceSearchText[source.ID], needle) {
			continue
		}
		matched = append(matched, domain.SourceSummary{
			ID:            source.ID,
			Type:          source.Type,
			Title:         source.Title,
			Author:        source.Author,
			Year:          source.Year,
			Relationship:  source.Relationship,
			EvidenceClass: source.EvidenceClass,
			Retrieval:     source.Retrieval,
			ClaimCount:    len(k.claimIDsBySourceID[source.ID]),
			NodeCount:     len(k.nodeIDsBySourceID[source.ID]),
		})
	}

	page, meta := paginate(matched, limit, offset)
	return SourceList{Page: meta, Sources: page}, nil
}

// SourceByID returns one registry entry with the derived reverse views of what cites it.
func (k *Knowledge) SourceByID(id string) (domain.SourceDetail, error) {
	source, ok := k.sourcesByID[id]
	if !ok {
		return domain.SourceDetail{}, ErrNotFound
	}
	claims := append([]domain.SourceClaimRef(nil), k.sourceClaims[id]...)
	if claims == nil {
		claims = []domain.SourceClaimRef{}
	}
	return domain.SourceDetail{
		Source:             source,
		Claims:             claims,
		AttributedClaimIDs: copyIDs(k.attributedClaimIDs[id]),
		NodeIDs:            copyIDs(k.nodeIDsBySourceID[id]),
		SessionIDs:         copyIDs(k.sessionIDsBySourceID[id]),
	}, nil
}

// ListClaims filters, searches and pages the canonical claim records.
//
// Multiple filters compose with AND, and results keep canonical ID order.
func (k *Knowledge) ListClaims(q ClaimQuery) (ClaimList, error) {
	vocab := k.vocabularies.Claim
	for _, check := range []struct {
		param, value string
		allowed      []string
	}{
		{"claim_type", q.ClaimType, vocab.ClaimTypes},
		{"confidence", q.Confidence, vocab.ConfidenceLevels},
		{"dispute_status", q.DisputeStatus, vocab.DisputeStatuses},
		{"temporal_precision", q.TemporalPrecision, vocab.TemporalPrecisions},
		{"relation", q.Relation, vocab.EvidenceRelations},
	} {
		if check.value != "" && !contains(check.allowed, check.value) {
			return ClaimList{}, &InvalidFilterError{Param: check.param, Allowed: check.allowed}
		}
	}

	limit, offset := normalisePaging(q.Limit, q.Offset)
	needle := boundedNeedle(q.Q)

	var bySource, byNode, bySession map[string]bool
	if q.SourceID != "" {
		bySource = newIDSet(k.claimIDsBySourceID[q.SourceID])
	}
	if q.NodeID != "" {
		byNode = newIDSet(k.claimIDsByNodeID[q.NodeID])
	}
	if q.SessionID != "" {
		bySession = newIDSet(k.claimIDsBySessionID[q.SessionID])
	}

	matched := make([]domain.ClaimSummary, 0, len(k.claims))
	for _, claim := range k.claims {
		if q.ClaimType != "" && claim.ClaimType != q.ClaimType {
			continue
		}
		if q.Confidence != "" && claim.Confidence != q.Confidence {
			continue
		}
		if q.DisputeStatus != "" && claim.DisputeStatus != q.DisputeStatus {
			continue
		}
		if q.TemporalPrecision != "" && claim.TemporalPrecision != q.TemporalPrecision {
			continue
		}
		if q.Relation != "" && !hasRelation(claim, q.Relation) {
			continue
		}
		if bySource != nil && !bySource[claim.ID] {
			continue
		}
		if byNode != nil && !byNode[claim.ID] {
			continue
		}
		if bySession != nil && !bySession[claim.ID] {
			continue
		}
		if needle != "" && !strings.Contains(k.claimSearchText[claim.ID], needle) {
			continue
		}
		matched = append(matched, domain.ClaimSummary{
			ID:                claim.ID,
			Statement:         claim.Statement,
			ClaimType:         claim.ClaimType,
			Confidence:        claim.Confidence,
			DisputeStatus:     claim.DisputeStatus,
			TemporalPrecision: claim.TemporalPrecision,
			EvidenceCount:     len(claim.Evidence),
			AttributionCount:  len(claim.Attribution),
			AppearsInCount:    len(claim.AppearsIn),
		})
	}

	page, meta := paginate(matched, limit, offset)
	return ClaimList{Page: meta, Claims: page}, nil
}

// ClaimByID returns one claim with its evidence context flattened into sorted ID lists.
func (k *Knowledge) ClaimByID(id string) (domain.ClaimDetail, error) {
	claim, ok := k.claimsByID[id]
	if !ok {
		return domain.ClaimDetail{}, ErrNotFound
	}
	return domain.ClaimDetail{
		Claim:      claim,
		SourceIDs:  k.sourceIDsForClaim(id),
		NodeIDs:    appearanceRefs(claim, domain.ClaimKindNode),
		SessionIDs: appearanceRefs(claim, domain.ClaimKindSession),
	}, nil
}

// Vocabularies returns the canonical contract vocabularies the index was built against.
func (k *Knowledge) Vocabularies() domain.Vocabularies {
	v := k.vocabularies
	return domain.Vocabularies{
		Claim: domain.ClaimVocabulary{
			ClaimTypes:         copyIDs(v.Claim.ClaimTypes),
			ConfidenceLevels:   copyIDs(v.Claim.ConfidenceLevels),
			DisputeStatuses:    copyIDs(v.Claim.DisputeStatuses),
			TemporalPrecisions: copyIDs(v.Claim.TemporalPrecisions),
			EvidenceRelations:  copyIDs(v.Claim.EvidenceRelations),
			DerivedFromKinds:   copyIDs(v.Claim.DerivedFromKinds),
			AppearsInKinds:     copyIDs(v.Claim.AppearsInKinds),
		},
		Source: domain.SourceVocabulary{
			Types:           copyIDs(v.Source.Types),
			Relationships:   copyIDs(v.Source.Relationships),
			EvidenceClasses: copyIDs(v.Source.EvidenceClasses),
			Retrievals:      copyIDs(v.Source.Retrievals),
		},
	}
}

// sourceIDsForClaim returns the distinct sorted sources one claim cites, counting both
// evidence and attribution. Attribution counts because the source that records who credits
// a statement is still a source the claim depends on.
func (k *Knowledge) sourceIDsForClaim(claimID string) []string {
	claim, ok := k.claimsByID[claimID]
	if !ok {
		return []string{}
	}
	seen := map[string]bool{}
	for _, e := range claim.Evidence {
		seen[e.SourceID] = true
	}
	for _, a := range claim.Attribution {
		seen[a.SourceID] = true
	}
	return sortedSet(seen)
}

// appearanceRefs returns the distinct sorted appearance sites of one kind.
func appearanceRefs(claim domain.Claim, kind string) []string {
	seen := map[string]bool{}
	for _, ref := range claim.AppearsIn {
		if ref.Kind == kind {
			seen[ref.Ref] = true
		}
	}
	return sortedSet(seen)
}

func hasRelation(claim domain.Claim, relation string) bool {
	for _, e := range claim.Evidence {
		if e.Relation == relation {
			return true
		}
	}
	return false
}

// paginate applies the page window and reports what was returned. It is generic over the
// summary type so the source and claim lists cannot drift apart in paging behaviour.
func paginate[T any](matched []T, limit, offset int) ([]T, Page) {
	total := len(matched)
	var page []T
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = matched[offset:end]
	}
	if page == nil {
		page = []T{}
	}
	return page, Page{Total: total, Count: len(page), Limit: limit, Offset: offset}
}

// boundedNeedle normalises a search term and applies the service query bound.
func boundedNeedle(q string) string {
	needle := strings.ToLower(strings.TrimSpace(q))
	if len(needle) > MaxQueryChars {
		needle = needle[:MaxQueryChars]
	}
	return needle
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func newIDSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func appendDistinct(ids []string, id string) []string {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

// sortedSet renders a membership set as a sorted slice, which is how every derived ID list
// leaves the index. Go map iteration order is randomised, so sorting is what makes the
// projection identical on every run rather than merely usually identical.
func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func copyIDs(ids []string) []string {
	out := append([]string(nil), ids...)
	if out == nil {
		out = []string{}
	}
	return out
}
