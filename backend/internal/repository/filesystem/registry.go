package filesystem

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

const (
	sourceRegistryPath    = "sources/source-registry.yaml"
	relationshipTypesPath = "schemas/relationship-types.yaml"
	sessionsDir           = "sessions"
)

type sourceRegistryFile struct {
	Schema  string          `yaml:"schema"`
	Version int             `yaml:"version"`
	Sources []domain.Source `yaml:"sources"`
}

type relationshipTypesFile struct {
	Schema            string                    `yaml:"schema"`
	Version           int                       `yaml:"version"`
	RelationshipTypes []domain.RelationshipType `yaml:"relationship_types"`
}

var relationshipLabelPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

// loadSources reads the canonical provenance registry.
//
// A registry that cannot be read or parsed is fatal: session identity and every node
// provenance reference resolve against it, so without it the projection cannot be trusted.
func (r *Repository) loadSources(report *domain.ValidationReport) []domain.Source {
	raw, err := fs.ReadFile(r.fsys, sourceRegistryPath)
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: sourceRegistryPath, Message: err.Error(),
		})
		return nil
	}
	var file sourceRegistryFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path:    sourceRegistryPath,
			Message: "source registry is not valid YAML: " + err.Error(),
		})
		return nil
	}

	seen := make(map[string]bool, len(file.Sources))
	sources := make([]domain.Source, 0, len(file.Sources))
	for _, source := range file.Sources {
		if !canonicalIDPattern.MatchString(source.ID) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeInvalidID, Ref: source.ID,
				Path:    sourceRegistryPath,
				Message: fmt.Sprintf("source id %q is not canonical kebab-case", source.ID),
			})
			continue
		}
		if seen[source.ID] {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeDuplicateID, Ref: source.ID,
				Path: sourceRegistryPath, Message: "source id is declared more than once",
			})
			continue
		}
		seen[source.ID] = true
		r.checkLocator(source, report)
		sources = append(sources, source)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].ID < sources[j].ID })
	return sources
}

// checkLocator reports a registered locator that does not resolve inside the repository.
//
// A dangling locator is a warning rather than a fatal issue: it is a corpus gap for a human
// to decide about and it does not make the node or graph projection wrong. An unsafe
// locator is fatal, because joining it to the corpus root is exactly what must never happen.
func (r *Repository) checkLocator(source domain.Source, report *domain.ValidationReport) {
	if source.Locator == "" || isExternalLocator(source.Locator) {
		return
	}
	rel, err := safeRelPath(source.Locator)
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeUnsafePath, Ref: source.ID,
			Path:    sourceRegistryPath,
			Message: fmt.Sprintf("locator %q is not a safe repository-relative path", source.Locator),
		})
		return
	}
	if _, err := fs.Stat(r.fsys, rel); err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityWarning, Code: domain.CodeMissingLocator, Ref: source.ID,
			Path: rel, Message: "registered locator does not exist in the repository",
		})
	}
}

// loadRelationshipTypes reads the bounded canonical edge vocabulary.
//
// It is fatal if unreadable: without it every edge type would have to be accepted, which
// would let an unauthorised relationship type into the graph projection.
func (r *Repository) loadRelationshipTypes(report *domain.ValidationReport) []domain.RelationshipType {
	raw, err := fs.ReadFile(r.fsys, relationshipTypesPath)
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: relationshipTypesPath, Message: err.Error(),
		})
		return nil
	}
	var file relationshipTypesFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path:    relationshipTypesPath,
			Message: "relationship type vocabulary is not valid YAML: " + err.Error(),
		})
		return nil
	}
	if len(file.RelationshipTypes) == 0 {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path:    relationshipTypesPath,
			Message: "relationship type vocabulary declares no types",
		})
		return nil
	}
	// Phase 1C reads inverse as an executable traversal label, so it needs stronger
	// guarantees than the Phase 1A forward-only graph did. Keep the forward and inverse
	// namespaces disjoint and unique: otherwise one response label could mean two
	// different predicates, or an authored edge could collide with a generated reverse
	// edge and lose its derived provenance marker during graph deduplication.
	seenNames := make(map[string]bool, len(file.RelationshipTypes)*2)
	types := make([]domain.RelationshipType, 0, len(file.RelationshipTypes))
	for _, relationshipType := range file.RelationshipTypes {
		invalid := false
		for _, value := range []struct{ field, label string }{
			{"id", relationshipType.ID},
			{"inverse", relationshipType.Inverse},
		} {
			if !relationshipLabelPattern.MatchString(value.label) {
				report.Add(domain.ValidationIssue{
					Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
					Ref: relationshipType.ID, Path: relationshipTypesPath,
					Message: fmt.Sprintf("relationship type %s %q is not canonical snake_case", value.field, value.label),
				})
				invalid = true
			}
		}
		if relationshipType.ID == relationshipType.Inverse && relationshipType.ID != "" {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Ref: relationshipType.ID, Path: relationshipTypesPath,
				Message: "directed relationship type must not declare itself as its inverse",
			})
			invalid = true
		}
		for _, name := range []string{relationshipType.ID, relationshipType.Inverse} {
			if name == "" {
				continue
			}
			if seenNames[name] {
				report.Add(domain.ValidationIssue{
					Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
					Ref: relationshipType.ID, Path: relationshipTypesPath,
					Message: fmt.Sprintf("relationship label %q is declared more than once", name),
				})
				invalid = true
			} else {
				seenNames[name] = true
			}
		}
		if !invalid {
			types = append(types, relationshipType)
		}
	}
	sort.Slice(types, func(i, j int) bool { return types[i].ID < types[j].ID })
	return types
}

// buildSessions derives canonical sessions from the registry.
//
// docs/knowledge-model.md constrains node session_origin to sources registered as
// type: session, which makes the registry the canonical identity for a session. The
// sessions/ directory carries the transcript; its absence is recorded as a warning rather
// than treated as a missing session.
func (r *Repository) buildSessions(sources []domain.Source, report *domain.ValidationReport) []domain.Session {
	sessions := make([]domain.Session, 0)
	for _, source := range sources {
		if source.Type != domain.SourceTypeSession {
			continue
		}
		present := true
		if _, err := fs.Stat(r.fsys, sessionsDir+"/"+source.ID); err != nil {
			present = false
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityWarning, Code: domain.CodeMissingSessionDir, Ref: source.ID,
				Path:    sessionsDir + "/" + source.ID,
				Message: "registered session has no directory under sessions/",
			})
		}
		sessions = append(sessions, domain.Session{
			ID:               source.ID,
			Title:            source.Title,
			Locator:          source.Locator,
			Relationship:     source.Relationship,
			DirectoryPresent: present,
			NodeIDs:          []string{},
		})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })
	return sessions
}
