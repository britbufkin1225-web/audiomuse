// Package filesystem is the only package that reads the AudioMuse canonical repository.
//
// It opens the corpus root as an io/fs.FS and performs read calls exclusively. There is no
// write, create, rename or remove call anywhere in this package, which is what makes the
// read-only guarantee in docs/backend-architecture.md structural rather than incidental.
package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository"
)

// requiredMarkers are the canonical paths a directory must contain to be an AudioMuse
// repository root. They are checked before any traversal so that a misconfigured root
// fails immediately with a clear message instead of producing an empty corpus.
var requiredMarkers = []string{
	nodesDir,
	"schemas/node.schema.yaml",
	relationshipTypesPath,
	sourceRegistryPath,
}

// Repository is the read-only filesystem adapter for a canonical AudioMuse corpus.
type Repository struct {
	fsys fs.FS
	name string
}

var _ repository.KnowledgeRepository = (*Repository)(nil)

// New opens the canonical repository rooted at path root.
//
// os.DirFS confines every subsequent read to that subtree: fs paths are slash-separated
// and unrooted, so no traversal outside the corpus is expressible through this adapter.
func New(root string) (*Repository, error) {
	if root == "" {
		return nil, fmt.Errorf("repository root is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("repository root %s is not readable: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository root %s is not a directory", abs)
	}
	fsys := os.DirFS(abs)
	if err := checkMarkers(fsys, abs); err != nil {
		return nil, err
	}
	return &Repository{fsys: fsys, name: filepath.Base(abs)}, nil
}

// NewFromFS builds an adapter over an arbitrary read-only filesystem. Tests use it to run
// the real parsing and validation code against a synthetic fixture corpus.
func NewFromFS(fsys fs.FS, name string) (*Repository, error) {
	if fsys == nil {
		return nil, fmt.Errorf("filesystem is nil")
	}
	if err := checkMarkers(fsys, name); err != nil {
		return nil, err
	}
	return &Repository{fsys: fsys, name: name}, nil
}

func checkMarkers(fsys fs.FS, label string) error {
	for _, marker := range requiredMarkers {
		if _, err := fs.Stat(fsys, marker); err != nil {
			return fmt.Errorf("%s does not look like an AudioMuse repository: missing %s", label, marker)
		}
	}
	return nil
}

// Describe identifies the corpus without exposing an absolute filesystem path.
func (r *Repository) Describe() repository.Descriptor {
	return repository.Descriptor{Name: r.name, Kind: "filesystem"}
}

// Load reads the whole canonical corpus once and reports what it found.
//
// Parsing and reference resolution are separate passes, so a reference check resolves
// against the complete ID set rather than against whatever happened to be read so far.
func (r *Repository) Load(ctx context.Context) (*repository.Corpus, *domain.ValidationReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	report := &domain.ValidationReport{}

	sources := r.loadSources(report)
	relationshipTypes := r.loadRelationshipTypes(report)
	sessions := r.buildSessions(sources, report)
	nodes := r.loadNodes(report)

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	resolveReferences(nodes, sources, sessions, relationshipTypes, report)
	sessions = attachSessionNodes(nodes, sessions)
	reportUncited(nodes, sources, sessions, report)

	report.Sort()
	return &repository.Corpus{
		Nodes:             nodes,
		Sources:           sources,
		Sessions:          sessions,
		RelationshipTypes: relationshipTypes,
	}, report, nil
}

// resolveReferences checks every cross-record reference a node declares.
//
// Comparison is exact and case-sensitive throughout, matching the canonical identity
// semantics in docs/knowledge-model.md: silently normalising case would hide an authoring
// defect rather than surface it.
func resolveReferences(
	nodes []domain.Node,
	sources []domain.Source,
	sessions []domain.Session,
	relationshipTypes []domain.RelationshipType,
	report *domain.ValidationReport,
) {
	nodeIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	sourceIDs := make(map[string]bool, len(sources))
	for _, s := range sources {
		sourceIDs[s.ID] = true
	}
	sessionIDs := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		sessionIDs[s.ID] = true
	}
	typeIDs := make(map[string]bool, len(relationshipTypes))
	for _, t := range relationshipTypes {
		typeIDs[t.ID] = true
	}

	for _, node := range nodes {
		add := func(code, msg string) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: code, Ref: node.ID, Path: node.Path, Message: msg,
			})
		}

		seenEdges := make(map[string]bool, len(node.Relationships))
		for _, rel := range node.Relationships {
			if !nodeIDs[rel.Target] {
				add(domain.CodeUnresolvedTarget,
					fmt.Sprintf("relationship target %q does not resolve to a canonical node", rel.Target))
			}
			if !typeIDs[rel.Type] {
				add(domain.CodeInvalidRelationType,
					fmt.Sprintf("relationship type %q is not in %s", rel.Type, relationshipTypesPath))
			}
			if rel.Target == node.ID {
				add(domain.CodeSelfLink, fmt.Sprintf("node declares a self-link of type %q", rel.Type))
			}
			key := rel.Type + "|" + rel.Target
			if seenEdges[key] {
				add(domain.CodeDuplicateEdge,
					fmt.Sprintf("duplicate relationship %s --%s--> %s", node.ID, rel.Type, rel.Target))
			}
			seenEdges[key] = true
		}

		for _, sessionID := range node.SessionOrigin {
			if !sessionIDs[sessionID] {
				add(domain.CodeUnresolvedSession,
					fmt.Sprintf("session_origin %q is not a source registered as type: session", sessionID))
			}
		}
		for _, sourceID := range node.Sources {
			if !sourceIDs[sourceID] {
				add(domain.CodeUnresolvedSource,
					fmt.Sprintf("source %q is not registered in %s", sourceID, sourceRegistryPath))
			}
		}
	}
}

// attachSessionNodes fills each session's derived contribution list.
//
// This is the reverse read of node session_origin, which docs/knowledge-model.md describes
// as a many-to-many contribution map. It derives nothing that a node did not already state.
func attachSessionNodes(nodes []domain.Node, sessions []domain.Session) []domain.Session {
	bySession := make(map[string][]string, len(sessions))
	for _, node := range nodes {
		for _, sessionID := range node.SessionOrigin {
			bySession[sessionID] = append(bySession[sessionID], node.ID)
		}
	}
	out := make([]domain.Session, 0, len(sessions))
	for _, session := range sessions {
		ids := bySession[session.ID]
		if ids == nil {
			ids = []string{}
		} else {
			ids = append([]string(nil), ids...)
			sort.Strings(ids)
		}
		session.NodeIDs = ids
		out = append(out, session)
	}
	return out
}

// reportUncited records corpus gaps: registered records that nothing cites. These are
// warnings. A source may legitimately be registered ahead of the node that will use it,
// and the backend has no authority to decide otherwise.
func reportUncited(
	nodes []domain.Node,
	sources []domain.Source,
	sessions []domain.Session,
	report *domain.ValidationReport,
) {
	citedSources := make(map[string]bool)
	for _, node := range nodes {
		for _, id := range node.Sources {
			citedSources[id] = true
		}
	}
	for _, source := range sources {
		if !citedSources[source.ID] {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityWarning, Code: domain.CodeUncitedSource, Ref: source.ID,
				Path: sourceRegistryPath, Message: "registered source is not cited by any node",
			})
		}
	}
	for _, session := range sessions {
		if len(session.NodeIDs) == 0 {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityWarning, Code: domain.CodeUncitedSession, Ref: session.ID,
				Path: sourceRegistryPath, Message: "registered session is not named by any node session_origin",
			})
		}
	}
}
