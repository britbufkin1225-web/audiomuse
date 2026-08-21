package service

import (
	"sort"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Query bounds. Paging and query length are capped in the service rather than only at the
// HTTP edge, so a future non-HTTP caller inherits the same limits.
const (
	DefaultLimit  = 50
	MaxLimit      = 200
	MaxQueryChars = 128
)

// NodeQuery is a bounded, deterministic node list request.
//
// Domain, Status and Session are exact case-sensitive canonical matches, following the
// identity semantics in docs/knowledge-model.md. Q is human-facing lexical search and is
// deliberately tolerant: it is case-insensitive substring matching, which that document
// explicitly separates from canonical reference validation.
type NodeQuery struct {
	Q       string
	Domain  string
	Status  string
	Session string
	Limit   int
	Offset  int
}

// Page describes the slice of a result set that was returned.
type Page struct {
	Total  int `json:"total"`
	Count  int `json:"count"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// NodeList is the node list projection.
type NodeList struct {
	Page  Page                 `json:"page"`
	Nodes []domain.NodeSummary `json:"nodes"`
}

// SessionQuery is a bounded session list request.
type SessionQuery struct {
	Q      string
	Limit  int
	Offset int
}

// SessionList is the session list projection.
type SessionList struct {
	Page     Page             `json:"page"`
	Sessions []domain.Session `json:"sessions"`
}

// normalise applies the paging bounds. A caller that supplies nothing gets DefaultLimit;
// a caller that asks for more than MaxLimit is clamped rather than refused, and a negative
// offset is treated as zero.
func normalisePaging(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ListNodes filters, searches and pages the canonical nodes.
//
// Results keep canonical ID order throughout. Search does not reorder by relevance:
// scoring would require a weighting judgement that Phase 1A has no basis to make, and an
// unranked ID-ordered list is reproducible byte for byte across runs.
func (k *Knowledge) ListNodes(q NodeQuery) NodeList {
	limit, offset := normalisePaging(q.Limit, q.Offset)
	needle := strings.ToLower(strings.TrimSpace(q.Q))
	if len(needle) > MaxQueryChars {
		needle = needle[:MaxQueryChars]
	}

	matched := make([]domain.NodeSummary, 0, len(k.nodes))
	for _, node := range k.nodes {
		if q.Domain != "" && node.Domain != q.Domain {
			continue
		}
		if q.Status != "" && node.Status != q.Status {
			continue
		}
		if q.Session != "" && !containsExact(node.SessionOrigin, q.Session) {
			continue
		}
		if needle != "" && !strings.Contains(k.searchText[node.ID], needle) {
			continue
		}
		matched = append(matched, summarise(node, len(k.inboundByID[node.ID])))
	}

	total := len(matched)
	page := matched
	if offset >= total {
		page = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		page = matched[offset:end]
	}
	if page == nil {
		page = []domain.NodeSummary{}
	}

	return NodeList{
		Page:  Page{Total: total, Count: len(page), Limit: limit, Offset: offset},
		Nodes: page,
	}
}

// ListSessions filters and pages the canonical sessions.
//
// Sessions carry no domain or status, so the only filter is lexical search over the id and
// title, matching how the registry identifies them.
func (k *Knowledge) ListSessions(q SessionQuery) SessionList {
	limit, offset := normalisePaging(q.Limit, q.Offset)
	needle := strings.ToLower(strings.TrimSpace(q.Q))
	if len(needle) > MaxQueryChars {
		needle = needle[:MaxQueryChars]
	}

	matched := make([]domain.Session, 0, len(k.sessions))
	for _, session := range k.sessions {
		if needle != "" {
			haystack := strings.ToLower(session.ID + "\n" + session.Title)
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		matched = append(matched, session)
	}

	total := len(matched)
	var page []domain.Session
	if offset < total {
		end := offset + limit
		if end > total {
			end = total
		}
		page = matched[offset:end]
	}
	if page == nil {
		page = []domain.Session{}
	}

	return SessionList{
		Page:     Page{Total: total, Count: len(page), Limit: limit, Offset: offset},
		Sessions: page,
	}
}

func summarise(node domain.Node, inbound int) domain.NodeSummary {
	origin := append([]string(nil), node.SessionOrigin...)
	if origin == nil {
		origin = []string{}
	}
	sort.Strings(origin)
	return domain.NodeSummary{
		ID:                node.ID,
		Title:             node.Title,
		Domain:            node.Domain,
		Status:            node.Status,
		Definition:        node.Definition,
		SessionOrigin:     origin,
		RelationshipCount: len(node.Relationships),
		InboundCount:      inbound,
	}
}

// containsExact performs an exact, case-sensitive canonical ID membership test.
func containsExact(haystack []string, needle string) bool {
	for _, candidate := range haystack {
		if candidate == needle {
			return true
		}
	}
	return false
}
