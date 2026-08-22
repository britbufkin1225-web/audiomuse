package filesystem

import (
	"fmt"
	"io/fs"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

const (
	claimSchemaPath         = "schemas/claim.schema.yaml"
	sourceSchemaPath        = "schemas/source.schema.yaml"
	experimentSchemaPath    = "schemas/experiment.schema.yaml"
	experimentRunSchemaPath = "schemas/experiment-run.schema.yaml"
	nodeSchemaPath          = "schemas/node.schema.yaml"
)

// claimSchemaFile is the subset of schemas/claim.schema.yaml the backend reads.
//
// Only the bounded vocabularies are read. The semantic rules block in that file is the
// contract for tools/validate-claims.ps1 and is deliberately not reimplemented here; see
// resolveClaimReferences for why.
type claimSchemaFile struct {
	Schema             string   `yaml:"schema"`
	Version            int      `yaml:"version"`
	ClaimTypes         []string `yaml:"claim_types"`
	ConfidenceLevels   []string `yaml:"confidence_levels"`
	DisputeStatuses    []string `yaml:"dispute_statuses"`
	TemporalPrecisions []string `yaml:"temporal_precisions"`
	EvidenceRelations  []string `yaml:"evidence_relations"`
	DerivedFromKinds   []string `yaml:"derived_from_kinds"`
	AppearsInKinds     []string `yaml:"appears_in_kinds"`
}

// sourceSchemaFile is the subset of schemas/source.schema.yaml the backend reads: the
// enum list on each bounded property.
type sourceSchemaFile struct {
	Schema     string                    `yaml:"schema"`
	Version    int                       `yaml:"version"`
	Properties map[string]schemaProperty `yaml:"properties"`
}

type schemaProperty struct {
	Enum  []string    `yaml:"enum"`
	Items schemaItems `yaml:"items"`
}

// schemaItems is the nested item contract of an array property. Only the bounded enums on item
// properties are read; the rest of the item shape is enforced by checkObjectListShape against
// the record itself.
type schemaItems struct {
	Properties map[string]schemaProperty `yaml:"properties"`
}

// experimentSchemaFile and experimentRunSchemaFile are the subsets of the two practice
// contracts the backend reads: the enum list on each bounded property.
type experimentSchemaFile struct {
	Schema     string                    `yaml:"schema"`
	Version    int                       `yaml:"version"`
	Properties map[string]schemaProperty `yaml:"properties"`
}

type experimentRunSchemaFile struct {
	Schema     string                    `yaml:"schema"`
	Version    int                       `yaml:"version"`
	Properties map[string]schemaProperty `yaml:"properties"`
}

// loadVocabularies reads the two canonical contract files that bound the evidence layer.
//
// They are read rather than compiled in for the reason schemas/claim.schema.yaml states
// about its own validator: a vocabulary change must be a schema change and must not be
// possible to make silently inside code. Every evidence filter the API accepts is checked
// against these lists, so an unreadable contract is fatal — serving unbounded filters over
// a provenance layer would let a caller believe they had filtered by a confidence level
// that does not exist.
func (r *Repository) loadVocabularies(report *domain.ValidationReport) domain.Vocabularies {
	return domain.Vocabularies{
		Claim:             r.loadClaimVocabulary(report),
		Source:            r.loadSourceVocabulary(report),
		VocabularyDomains: r.loadVocabularyDomains(report),
		Experiment:        r.loadExperimentVocabulary(report),
		ExperimentRun:     r.loadExperimentRunVocabulary(report),
	}
}

// loadVocabularyDomains reads the domain vocabulary that schemas/vocabulary.schema.yaml
// explicitly reuses from schemas/node.schema.yaml. The PowerShell vocabulary validator uses
// this same enum, so reading it here prevents the projection from accepting a domain the
// canonical repository rejects.
func (r *Repository) loadVocabularyDomains(report *domain.ValidationReport) []string {
	var file sourceSchemaFile
	if !r.decodeContract(nodeSchemaPath, &file, report) {
		return nil
	}
	domains := file.Properties["domain"].Enum
	requireEnums(nodeSchemaPath, map[string][]string{"domain": domains}, report)
	return domains
}

// loadExperimentVocabulary reads the bounded status, type and difficulty enums from
// schemas/experiment.schema.yaml.
//
// All three are API filters, so an unreadable or enum-less contract is fatal for the reason the
// claim contract is: serving an unbounded filter over a bounded field would let a caller believe
// they had filtered by a value the repository does not define.
func (r *Repository) loadExperimentVocabulary(report *domain.ValidationReport) domain.ExperimentVocabulary {
	var file experimentSchemaFile
	if !r.decodeContract(experimentSchemaPath, &file, report) {
		return domain.ExperimentVocabulary{}
	}
	vocab := domain.ExperimentVocabulary{
		Statuses:     file.Properties["status"].Enum,
		Types:        file.Properties["type"].Enum,
		Difficulties: file.Properties["difficulty"].Enum,
	}
	requireEnums(experimentSchemaPath, map[string][]string{
		"status":     vocab.Statuses,
		"type":       vocab.Types,
		"difficulty": vocab.Difficulties,
	}, report)
	return vocab
}

// loadExperimentRunVocabulary reads the bounded status enum and the nested measurement
// calibration enum from schemas/experiment-run.schema.yaml.
func (r *Repository) loadExperimentRunVocabulary(report *domain.ValidationReport) domain.ExperimentRunVocabulary {
	var file experimentRunSchemaFile
	if !r.decodeContract(experimentRunSchemaPath, &file, report) {
		return domain.ExperimentRunVocabulary{}
	}
	vocab := domain.ExperimentRunVocabulary{
		Statuses:     file.Properties["status"].Enum,
		Calibrations: file.Properties["measurements"].Items.Properties["calibration"].Enum,
	}
	requireEnums(experimentRunSchemaPath, map[string][]string{
		"status":                   vocab.Statuses,
		"measurements.calibration": vocab.Calibrations,
	}, report)
	return vocab
}

// requireEnums reports a contract that declares no values for a property the backend depends on.
// Names are visited in sorted order so a report built by iterating a map is identical across runs.
func requireEnums(path string, enums map[string][]string, report *domain.ValidationReport) {
	for _, name := range sortedKeys(enums) {
		if len(enums[name]) == 0 {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path:    path,
				Message: "contract declares no enum for property " + name,
			})
		}
	}
}

func (r *Repository) loadClaimVocabulary(report *domain.ValidationReport) domain.ClaimVocabulary {
	var file claimSchemaFile
	if !r.decodeContract(claimSchemaPath, &file, report) {
		return domain.ClaimVocabulary{}
	}
	vocab := domain.ClaimVocabulary{
		ClaimTypes:         file.ClaimTypes,
		ConfidenceLevels:   file.ConfidenceLevels,
		DisputeStatuses:    file.DisputeStatuses,
		TemporalPrecisions: file.TemporalPrecisions,
		EvidenceRelations:  file.EvidenceRelations,
		DerivedFromKinds:   file.DerivedFromKinds,
		AppearsInKinds:     file.AppearsInKinds,
	}
	required := map[string][]string{
		"claim_types":         vocab.ClaimTypes,
		"confidence_levels":   vocab.ConfidenceLevels,
		"dispute_statuses":    vocab.DisputeStatuses,
		"temporal_precisions": vocab.TemporalPrecisions,
		"evidence_relations":  vocab.EvidenceRelations,
		"derived_from_kinds":  vocab.DerivedFromKinds,
		"appears_in_kinds":    vocab.AppearsInKinds,
	}
	for _, name := range sortedKeys(required) {
		if len(required[name]) == 0 {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path:    claimSchemaPath,
				Message: "claim contract declares no " + name,
			})
		}
	}
	return vocab
}

func (r *Repository) loadSourceVocabulary(report *domain.ValidationReport) domain.SourceVocabulary {
	var file sourceSchemaFile
	if !r.decodeContract(sourceSchemaPath, &file, report) {
		return domain.SourceVocabulary{}
	}
	vocab := domain.SourceVocabulary{
		Types:           file.Properties["type"].Enum,
		Relationships:   file.Properties["relationship"].Enum,
		EvidenceClasses: file.Properties["evidence_class"].Enum,
		Retrievals:      file.Properties["retrieval"].Enum,
	}
	required := map[string][]string{
		"type":           vocab.Types,
		"relationship":   vocab.Relationships,
		"evidence_class": vocab.EvidenceClasses,
		"retrieval":      vocab.Retrievals,
	}
	for _, name := range sortedKeys(required) {
		if len(required[name]) == 0 {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path:    sourceSchemaPath,
				Message: "source contract declares no enum for property " + name,
			})
		}
	}
	return vocab
}

// decodeContract reads and unmarshals one canonical contract file, reporting a fatal issue
// if it is unreadable or not valid YAML.
func (r *Repository) decodeContract(path string, into any, report *domain.ValidationReport) bool {
	raw, err := fs.ReadFile(r.fsys, path)
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: path, Message: "canonical contract could not be read: " + err.Error(),
		})
		return false
	}
	if err := yaml.Unmarshal(raw, into); err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: path, Message: "canonical contract is not valid YAML: " + err.Error(),
		})
		return false
	}
	return true
}

// checkSourceVocabulary validates each registry entry against the source contract.
//
// Phase 1A resolved source references but did not check the registry's own bounded fields,
// because nothing depended on them. Phase 1B serves them as API filters, so an
// out-of-vocabulary value would produce a filter whose result set silently means something
// other than what the caller asked for. Optional fields are checked only when present:
// docs/claim-provenance-model.md states that evidence_class and retrieval are annotated
// when a claim comes to depend on the source, not in a repository-wide sweep.
func checkSourceVocabulary(sources []domain.Source, vocab domain.SourceVocabulary, report *domain.ValidationReport) {
	types := newSet(vocab.Types)
	relationships := newSet(vocab.Relationships)
	classes := newSet(vocab.EvidenceClasses)
	retrievals := newSet(vocab.Retrievals)

	for _, source := range sources {
		add := func(field, value string) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeInvalidVocabulary, Ref: source.ID,
				Path:    sourceRegistryPath,
				Message: fmt.Sprintf("source %s %q is not declared in %s", field, value, sourceSchemaPath),
			})
		}
		if !types[source.Type] {
			add("type", source.Type)
		}
		if !relationships[source.Relationship] {
			add("relationship", source.Relationship)
		}
		if source.EvidenceClass != nil && !classes[*source.EvidenceClass] {
			add("evidence_class", *source.EvidenceClass)
		}
		if source.Retrieval != nil && !retrievals[*source.Retrieval] {
			add("retrieval", *source.Retrieval)
		}
	}
}

// newSet builds an exact, case-sensitive membership set, matching the canonical identity
// rule in docs/knowledge-model.md. Case drift is an authoring defect to report, not a
// difference to normalise away.
func newSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, v := range values {
		set[v] = true
	}
	return set
}

// sortedKeys returns map keys in a fixed order so a report built by iterating a map is
// identical across runs.
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
