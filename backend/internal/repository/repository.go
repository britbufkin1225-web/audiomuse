// Package repository declares the read-only contract between AudioMuse services and
// whatever holds the canonical corpus.
//
// The interface deliberately has no write, create, update or delete method. The read-only
// guarantee is therefore a property of the type system rather than of the current
// implementation, and a mutation path cannot appear without a visible contract change.
package repository

import (
	"context"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// Descriptor identifies the corpus a repository is serving, without exposing an absolute
// filesystem path to callers that may serialise it.
type Descriptor struct {
	// Name is a non-sensitive label for the corpus, such as the repository directory name.
	Name string
	// Kind describes the adapter, for example "filesystem".
	Kind string
}

// KnowledgeRepository provides deterministic read access to canonical AudioMuse records.
//
// Implementations must return records in a stable order for an unchanged corpus, must
// never write, and must report parse and reference problems through the returned
// domain.ValidationReport rather than by repairing records.
//
// A future SQLite, embedded-index or Postgres adapter can satisfy this interface without
// any change above the repository layer. None is implemented in Phase 1A.
type KnowledgeRepository interface {
	// Load reads the entire canonical corpus once.
	//
	// It returns a Corpus and a validation report. A report containing fatal issues means
	// the corpus cannot be safely projected; callers decide whether to abort. A non-nil
	// error means the corpus could not be read at all.
	Load(ctx context.Context) (*Corpus, *domain.ValidationReport, error)

	// Describe identifies the corpus being served.
	Describe() Descriptor
}

// Corpus is one consistent snapshot of the canonical records the backend reads.
//
// Phase 1B added Claims and Vocabularies, the AudioMuse evidence layer. Experiments,
// experiment runs and vocabulary entries remain canonical repository layers the backend
// deliberately does not parse; see docs/backend-architecture.md.
//
// Vocabularies carries the bounded value sets read from schemas/claim.schema.yaml and
// schemas/source.schema.yaml. They travel with the corpus rather than being compiled into
// the service because they are canonical contract data, and the service validates every
// evidence filter against them.
type Corpus struct {
	Nodes             []domain.Node
	Sources           []domain.Source
	Sessions          []domain.Session
	Claims            []domain.Claim
	RelationshipTypes []domain.RelationshipType
	Vocabularies      domain.Vocabularies
}
