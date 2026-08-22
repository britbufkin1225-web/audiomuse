// Package service builds and serves the in-memory read-only projection of an AudioMuse
// corpus.
//
// The index is constructed once during New and is never written to afterwards. Because no
// exported method mutates any field, a Knowledge value is safe for concurrent use by many
// HTTP handlers without locking; the immutability is the synchronisation. Every accessor
// copies slices it hands out so a caller cannot reach back into the index either.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository"
)

// ErrNotFound reports that a canonical record does not exist.
var ErrNotFound = errors.New("record not found")

// Knowledge is the immutable startup index over one corpus snapshot.
type Knowledge struct {
	descriptor repository.Descriptor
	report     *domain.ValidationReport

	nodes       []domain.Node
	nodesByID   map[string]domain.Node
	inboundByID map[string][]domain.InboundRelationship
	searchText  map[string]string

	sessions          []domain.Session
	sessionsByID      map[string]domain.Session
	relationshipTypes []domain.RelationshipType

	// Evidence layer (Phase 1B). Sources were counted but not indexed in Phase 1A; they
	// are now a served projection in their own right.
	sources          []domain.Source
	sourcesByID      map[string]domain.Source
	sourceSearchText map[string]string
	claims           []domain.Claim
	claimsByID       map[string]domain.Claim
	claimSearchText  map[string]string
	vocabularies     domain.Vocabularies

	// Derived reverse views over the evidence layer. Each is documented at buildEvidence.
	claimIDsBySourceID   map[string][]string
	claimIDsByNodeID     map[string][]string
	claimIDsBySessionID  map[string][]string
	sourceClaims         map[string][]domain.SourceClaimRef
	attributedClaimIDs   map[string][]string
	nodeIDsBySourceID    map[string][]string
	sessionIDsBySourceID map[string][]string
	sourceIDsBySessionID map[string][]string

	graph domain.Graph
}

// New loads the corpus through the repository interface and builds the startup index.
//
// A corpus with fatal validation issues is refused: serving a projection the backend knows
// to be wrong would be worse than failing to start, and the caller receives the full list.
func New(ctx context.Context, repo repository.KnowledgeRepository) (*Knowledge, error) {
	corpus, report, err := repo.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load canonical repository: %w", err)
	}
	if err := report.FatalError(); err != nil {
		return nil, err
	}

	k := &Knowledge{
		descriptor:        repo.Describe(),
		report:            report,
		nodes:             corpus.Nodes,
		nodesByID:         make(map[string]domain.Node, len(corpus.Nodes)),
		inboundByID:       make(map[string][]domain.InboundRelationship, len(corpus.Nodes)),
		searchText:        make(map[string]string, len(corpus.Nodes)),
		sessions:          corpus.Sessions,
		sessionsByID:      make(map[string]domain.Session, len(corpus.Sessions)),
		relationshipTypes: corpus.RelationshipTypes,
		sources:           corpus.Sources,
		sourcesByID:       make(map[string]domain.Source, len(corpus.Sources)),
		sourceSearchText:  make(map[string]string, len(corpus.Sources)),
		claims:            corpus.Claims,
		claimsByID:        make(map[string]domain.Claim, len(corpus.Claims)),
		claimSearchText:   make(map[string]string, len(corpus.Claims)),
		vocabularies:      corpus.Vocabularies,
	}

	for _, node := range corpus.Nodes {
		k.nodesByID[node.ID] = node
		k.searchText[node.ID] = searchCorpusFor(node)
	}
	for _, session := range corpus.Sessions {
		k.sessionsByID[session.ID] = session
	}
	for _, source := range corpus.Sources {
		k.sourcesByID[source.ID] = source
		k.sourceSearchText[source.ID] = searchCorpusForSource(source)
	}
	for _, claim := range corpus.Claims {
		k.claimsByID[claim.ID] = claim
	}

	k.buildInbound()
	k.buildGraph()
	k.buildEvidence()
	return k, nil
}

// searchCorpusForSource assembles the lexical search corpus for one registry entry.
//
// Only the entry's own identifying fields contribute — id, title and author — so a match is
// always explainable by pointing at the registry line. Notes are excluded deliberately: they
// are free prose about retrieval and external locators, and searching them would make a hit
// mean something different from a hit on any other AudioMuse list.
func searchCorpusForSource(source domain.Source) string {
	parts := []string{source.ID, source.Title}
	if source.Author != nil {
		parts = append(parts, *source.Author)
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// searchCorpusFor assembles the lexical search corpus for one node.
//
// The searchable fields are the node's canonical identity and description fields: id,
// title, domain, status, definition and core_concepts. Nothing outside the node's own front
// matter contributes, so a match is always explainable by pointing at the record. The node
// schema has no aliases or tags field, so neither is searched.
func searchCorpusFor(node domain.Node) string {
	parts := make([]string, 0, 5+len(node.CoreConcepts))
	parts = append(parts, node.ID, node.Title, node.Domain, node.Status, node.Definition)
	parts = append(parts, node.CoreConcepts...)
	return strings.ToLower(strings.Join(parts, "\n"))
}

// buildInbound derives the reverse adjacency of the authored edges.
//
// Inbound edges are a display convenience, exactly as docs/knowledge-model.md permits, and
// are stored separately from a node's authored relationships so the two can never be
// confused. No reverse canonical edge is synthesised.
func (k *Knowledge) buildInbound() {
	for _, node := range k.nodes {
		for _, rel := range node.Relationships {
			k.inboundByID[rel.Target] = append(k.inboundByID[rel.Target],
				domain.InboundRelationship{Source: node.ID, Type: rel.Type})
		}
	}
	for id, inbound := range k.inboundByID {
		sort.Slice(inbound, func(i, j int) bool {
			if inbound[i].Source != inbound[j].Source {
				return inbound[i].Source < inbound[j].Source
			}
			return inbound[i].Type < inbound[j].Type
		})
		k.inboundByID[id] = inbound
	}
}

// buildGraph projects the corpus into the read-only graph view.
//
// Every edge originates from a node's explicit relationships array. Nothing is inferred
// from text, keywords, co-occurrence, or proximity. Ordering is fixed so that two runs over
// an unchanged corpus emit byte-identical graphs.
func (k *Knowledge) buildGraph() {
	nodes := make([]domain.GraphNode, 0, len(k.nodes))
	edges := make([]domain.GraphEdge, 0)
	for _, node := range k.nodes {
		nodes = append(nodes, domain.GraphNode{
			ID: node.ID, Label: node.Title, Domain: node.Domain, Status: node.Status,
		})
		for _, rel := range node.Relationships {
			edges = append(edges, domain.GraphEdge{Source: node.ID, Target: rel.Target, Type: rel.Type})
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Type != edges[j].Type {
			return edges[i].Type < edges[j].Type
		}
		return edges[i].Target < edges[j].Target
	})
	k.graph = domain.Graph{
		Nodes:    nodes,
		Edges:    edges,
		Metadata: domain.GraphMetadata{NodeCount: len(nodes), EdgeCount: len(edges)},
	}
}

// Graph returns a defensive copy of the full graph projection.
func (k *Knowledge) Graph() domain.Graph {
	return domain.Graph{
		Nodes:    append([]domain.GraphNode(nil), k.graph.Nodes...),
		Edges:    append([]domain.GraphEdge(nil), k.graph.Edges...),
		Metadata: k.graph.Metadata,
	}
}

// NodeByID returns one full node projection with its derived inbound adjacency.
func (k *Knowledge) NodeByID(id string) (domain.NodeDetail, error) {
	node, ok := k.nodesByID[id]
	if !ok {
		return domain.NodeDetail{}, ErrNotFound
	}
	inbound := append([]domain.InboundRelationship(nil), k.inboundByID[id]...)
	if inbound == nil {
		inbound = []domain.InboundRelationship{}
	}
	return domain.NodeDetail{Node: node, InboundRelationships: inbound}, nil
}

// SessionByID returns one session projection.
func (k *Knowledge) SessionByID(id string) (domain.Session, error) {
	session, ok := k.sessionsByID[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return session, nil
}

// Domains returns the canonical domains present in the corpus, sorted.
func (k *Knowledge) Domains() []string {
	seen := make(map[string]bool, len(k.nodes))
	out := make([]string, 0)
	for _, node := range k.nodes {
		if !seen[node.Domain] {
			seen[node.Domain] = true
			out = append(out, node.Domain)
		}
	}
	sort.Strings(out)
	return out
}

// Statuses returns the canonical node statuses present in the corpus, sorted.
func (k *Knowledge) Statuses() []string {
	seen := make(map[string]bool, len(k.nodes))
	out := make([]string, 0)
	for _, node := range k.nodes {
		if !seen[node.Status] {
			seen[node.Status] = true
			out = append(out, node.Status)
		}
	}
	sort.Strings(out)
	return out
}

// Report returns the validation report from the load that built this index.
func (k *Knowledge) Report() *domain.ValidationReport { return k.report }

// Descriptor identifies the corpus this index was built from.
func (k *Knowledge) Descriptor() repository.Descriptor { return k.descriptor }
