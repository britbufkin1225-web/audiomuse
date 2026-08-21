package filesystem

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// nodesDir is the canonical node root. Domain subdirectories under it mirror the domain
// enum in schemas/node.schema.yaml but are navigation only: a node's domain comes from its
// front matter, never from its directory.
const nodesDir = "nodes"

// canonicalIDPattern is the shared ID contract from schemas/node.schema.yaml.
var canonicalIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// nodeRequiredFields is the required list from schemas/node.schema.yaml version 2. The
// schema also sets additional_properties: false and every property is required, so this is
// simultaneously the required set and the allowed set: a node's key set must equal it.
var nodeRequiredFields = []string{
	"id", "title", "domain", "status", "session_origin", "definition",
	"core_concepts", "relationships", "sources", "experiments",
	"practical_applications", "project_connections", "future_questions",
}

// loadNodes walks nodes/ deterministically and parses every canonical node record.
//
// fs.WalkDir visits directory entries in lexical order, so traversal does not depend on
// operating-system directory ordering. Results are sorted by ID afterwards regardless.
func (r *Repository) loadNodes(report *domain.ValidationReport) []domain.Node {
	var nodes []domain.Node
	seen := make(map[string]string)

	err := fs.WalkDir(r.fsys, nodesDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") || strings.HasSuffix(p, "/README.md") {
			return nil
		}
		raw, readErr := fs.ReadFile(r.fsys, p)
		if readErr != nil {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path: p, Message: readErr.Error(),
			})
			return nil
		}
		node, ok := parseNode(string(raw), p, report)
		if !ok {
			return nil
		}
		if prior, dup := seen[node.ID]; dup {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeDuplicateID,
				Ref: node.ID, Path: p,
				Message: fmt.Sprintf("node id is already defined by %s", prior),
			})
			return nil
		}
		seen[node.ID] = p
		nodes = append(nodes, node)
		return nil
	})
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: nodesDir, Message: err.Error(),
		})
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

// parseNode turns one canonical node file into a domain.Node.
//
// Structured front matter is parsed with a YAML parser rather than field regexes. Key-set
// checking happens before struct decoding so that a missing or unknown field is reported as
// such instead of surfacing as a zero value.
func parseNode(raw, relPath string, report *domain.ValidationReport) (domain.Node, bool) {
	fatal := func(code, msg string, ref string) (domain.Node, bool) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: code, Ref: ref, Path: relPath, Message: msg,
		})
		return domain.Node{}, false
	}

	frontMatter, body, err := splitFrontMatter(raw)
	if err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(frontMatter), &doc); err != nil {
		return fatal(domain.CodeMalformedRecord, "front matter is not valid YAML: "+err.Error(), "")
	}
	mapping, err := documentMapping(&doc)
	if err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}

	keys, err := mappingKeys(mapping)
	if err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}
	present := make(map[string]bool, len(keys))
	for _, k := range keys {
		present[k] = true
	}
	allowed := make(map[string]bool, len(nodeRequiredFields))
	for _, f := range nodeRequiredFields {
		allowed[f] = true
		if !present[f] {
			return fatal(domain.CodeMissingField, "missing required field "+f, "")
		}
	}
	for _, k := range keys {
		if !allowed[k] {
			return fatal(domain.CodeUnknownField, "unknown top-level field "+k, "")
		}
	}

	var node domain.Node
	if err := mapping.Decode(&node); err != nil {
		return fatal(domain.CodeMalformedRecord, "front matter does not match the node contract: "+err.Error(), "")
	}

	if !canonicalIDPattern.MatchString(node.ID) {
		return fatal(domain.CodeInvalidID, fmt.Sprintf("node id %q is not canonical kebab-case", node.ID), node.ID)
	}

	// Relationship items carry additional_properties: false with required [target, type].
	// Struct decoding silently tolerates extra keys, so check the shape explicitly.
	if err := checkRelationshipShape(mapping); err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), node.ID)
	}

	node.Path = relPath
	node.Body = strings.TrimSpace(body)
	normaliseNodeSlices(&node)
	return node, true
}

// normaliseNodeSlices replaces nil slices with empty ones so the JSON projection renders
// [] rather than null for a canonically empty list. This changes representation only; no
// canonical value is altered.
func normaliseNodeSlices(n *domain.Node) {
	ensure := func(s *[]string) {
		if *s == nil {
			*s = []string{}
		}
	}
	ensure(&n.SessionOrigin)
	ensure(&n.CoreConcepts)
	ensure(&n.Sources)
	ensure(&n.Experiments)
	ensure(&n.PracticalApplications)
	ensure(&n.ProjectConnections)
	ensure(&n.FutureQuestions)
	if n.Relationships == nil {
		n.Relationships = []domain.Relationship{}
	}
}

// documentMapping unwraps a parsed YAML document to its top-level mapping node.
func documentMapping(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil, fmt.Errorf("empty YAML document")
		}
		doc = doc.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected a YAML mapping at the top level")
	}
	return doc, nil
}

// mappingKeys returns the mapping's keys in document order and rejects duplicates.
//
// Comparison is exact and case-sensitive, matching the canonical identity semantics in
// docs/knowledge-model.md.
func mappingKeys(mapping *yaml.Node) ([]string, error) {
	keys := make([]string, 0, len(mapping.Content)/2)
	seen := make(map[string]bool, len(mapping.Content)/2)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		if seen[key] {
			return nil, fmt.Errorf("duplicate top-level field %s", key)
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys, nil
}

// checkRelationshipShape enforces the relationships[] item contract: a mapping whose key
// set is exactly {target, type}.
func checkRelationshipShape(mapping *yaml.Node) error {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != "relationships" {
			continue
		}
		seq := mapping.Content[i+1]
		if seq.Kind == yaml.ScalarNode && seq.Tag == "!!null" {
			return nil
		}
		if seq.Kind != yaml.SequenceNode {
			return fmt.Errorf("relationships must be a list")
		}
		for _, item := range seq.Content {
			if item.Kind != yaml.MappingNode {
				return fmt.Errorf("each relationship must be a mapping with target and type")
			}
			keys, err := mappingKeys(item)
			if err != nil {
				return fmt.Errorf("relationship entry: %w", err)
			}
			if len(keys) != 2 {
				return fmt.Errorf("relationship entry must declare exactly target and type")
			}
			sorted := append([]string(nil), keys...)
			sort.Strings(sorted)
			if sorted[0] != "target" || sorted[1] != "type" {
				return fmt.Errorf("relationship entry must declare exactly target and type")
			}
		}
		return nil
	}
	return nil
}
