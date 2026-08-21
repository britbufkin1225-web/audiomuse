// Package domain holds the typed representation of AudioMuse canonical records.
//
// Types here are derived from the contracts in schemas/ and use the repository's own
// terminology. The package performs no I/O and imports nothing outside the standard
// library so that it stays a pure description of what the corpus contains.
package domain

// Relationship is one directed, typed edge authored on a node.
//
// Contract: schemas/node.schema.yaml (relationships[]) and schemas/relationship-types.yaml.
// Direction is part of the claim; the inverse is never stored.
type Relationship struct {
	Target string `json:"target" yaml:"target"`
	Type   string `json:"type"   yaml:"type"`
}

// InboundRelationship is a derived view of an edge pointing at a node.
//
// It is not canonical data. It is the reverse read of another node's authored
// Relationship, kept in a separate field so a derived view can never be mistaken
// for something the node declared about itself.
type InboundRelationship struct {
	Source string `json:"source"`
	Type   string `json:"type"`
}

// Node is a durable AudioMuse conceptual unit.
//
// Contract: schemas/node.schema.yaml (version 2). Every field below is required by that
// schema; none is invented here. Path is not canonical content but the repository-relative
// location the record was read from, retained for diagnostics.
type Node struct {
	ID                    string         `json:"id"                     yaml:"id"`
	Title                 string         `json:"title"                  yaml:"title"`
	Domain                string         `json:"domain"                 yaml:"domain"`
	Status                string         `json:"status"                 yaml:"status"`
	SessionOrigin         []string       `json:"session_origin"         yaml:"session_origin"`
	Definition            string         `json:"definition"             yaml:"definition"`
	CoreConcepts          []string       `json:"core_concepts"          yaml:"core_concepts"`
	Relationships         []Relationship `json:"relationships"          yaml:"relationships"`
	Sources               []string       `json:"sources"                yaml:"sources"`
	Experiments           []string       `json:"experiments"            yaml:"experiments"`
	PracticalApplications []string       `json:"practical_applications" yaml:"practical_applications"`
	ProjectConnections    []string       `json:"project_connections"    yaml:"project_connections"`
	FutureQuestions       []string       `json:"future_questions"       yaml:"future_questions"`

	// Path is the repository-relative file the record was read from and Body is the
	// markdown prose following the front matter. Neither is front-matter content; both are
	// carried so a projection can cite its origin and render the authored explanation.
	Path string `json:"path"           yaml:"-"`
	Body string `json:"body,omitempty" yaml:"-"`
}

// NodeSummary is the list projection of a Node. It carries identity and the fields the
// list filters and lexical search operate on, without the full record body.
type NodeSummary struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Domain            string   `json:"domain"`
	Status            string   `json:"status"`
	Definition        string   `json:"definition"`
	SessionOrigin     []string `json:"session_origin"`
	RelationshipCount int      `json:"relationship_count"`
	InboundCount      int      `json:"inbound_count"`
}

// NodeDetail is the single-record projection: the canonical node plus the derived
// inbound adjacency the graph already contains.
type NodeDetail struct {
	Node
	InboundRelationships []InboundRelationship `json:"inbound_relationships"`
}

// Source is one entry in the canonical provenance registry.
//
// Contract: schemas/source.schema.yaml, data: sources/source-registry.yaml.
// Author, Year, EvidenceClass, Retrieval and Notes are optional in that contract and are
// modelled as pointers so an absent field stays distinguishable from an empty one.
type Source struct {
	ID            string  `json:"id"                       yaml:"id"`
	Type          string  `json:"type"                     yaml:"type"`
	Title         string  `json:"title"                    yaml:"title"`
	Author        *string `json:"author,omitempty"         yaml:"author"`
	Year          *int    `json:"year,omitempty"           yaml:"year"`
	Locator       string  `json:"locator"                  yaml:"locator"`
	Relationship  string  `json:"relationship"             yaml:"relationship"`
	EvidenceClass *string `json:"evidence_class,omitempty" yaml:"evidence_class"`
	Retrieval     *string `json:"retrieval,omitempty"      yaml:"retrieval"`
	Notes         *string `json:"notes,omitempty"          yaml:"notes"`
}

// SourceTypeSession is the registry type that marks a source as a session record.
//
// AudioMuse has no separate session contract: docs/knowledge-model.md states that node
// session_origin may name only sources registered as type session, which makes the
// registry the canonical identity for a session.
const SourceTypeSession = "session"

// Session is a chronological AudioMuse exploration record.
//
// Identity, title and locator come from the registry entry; DirectoryPresent records
// whether sessions/<id>/ exists on disk; NodeIDs is the derived reverse read of node
// session_origin, which docs/knowledge-model.md describes as a many-to-many contribution
// map rather than as ownership.
type Session struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Locator          string   `json:"locator"`
	Relationship     string   `json:"relationship"`
	DirectoryPresent bool     `json:"directory_present"`
	NodeIDs          []string `json:"node_ids"`
}

// RelationshipType is one entry of the bounded canonical edge vocabulary.
//
// Contract and data: schemas/relationship-types.yaml. Inverse is descriptive metadata; it
// is never a storable edge type.
type RelationshipType struct {
	ID             string `json:"id"             yaml:"id"`
	Name           string `json:"name"           yaml:"name"`
	Directionality string `json:"directionality" yaml:"directionality"`
	Meaning        string `json:"meaning"        yaml:"meaning"`
	Inverse        string `json:"inverse"        yaml:"inverse"`
}

// GraphNode is a node projected into the read-only graph view.
type GraphNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Domain string `json:"domain,omitempty"`
	Status string `json:"status,omitempty"`
}

// GraphEdge is one authored relationship projected into the read-only graph view.
// Every edge originates from a node's explicit relationships array. No edge is inferred.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// GraphMetadata reports the size of a graph projection.
type GraphMetadata struct {
	NodeCount int `json:"node_count"`
	EdgeCount int `json:"edge_count"`
}

// Graph is the full read-only graph projection.
type Graph struct {
	Nodes    []GraphNode   `json:"nodes"`
	Edges    []GraphEdge   `json:"edges"`
	Metadata GraphMetadata `json:"metadata"`
}
