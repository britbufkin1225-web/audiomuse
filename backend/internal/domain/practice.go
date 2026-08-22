package domain

// This file holds the AudioMuse practice layer: vocabulary entries, experiment definitions,
// and the individual runs that execute them.
//
// Contracts: schemas/vocabulary.schema.yaml, schemas/experiment.schema.yaml and
// schemas/experiment-run.schema.yaml. The prose models are vocabulary/README.md,
// experiments/README.md and experiment-runs/README.md, which state the four distinctions
// this file is built to keep visible in the type system rather than only in documentation:
//
//	vocabulary cross-reference  !=  typed graph edge
//	experiment definition       !=  evidence that it was run
//	planned run                 !=  completed run
//	observation                 !=  measurement
//
// Nothing here becomes a graph vertex or edge. The canonical graph is nodes and their
// authored relationships; see knowledge.go.

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// jsonNumberPattern is the JSON number grammar.
//
// Every canonical record in these three layers is authored as JSON-compatible YAML — the
// repository validators parse each value with ConvertFrom-Json — so a numeric scalar that is
// not a JSON number is an authoring defect, not a value to coerce. Accepting only this
// grammar is also what makes CanonicalNumber safe to emit verbatim.
var jsonNumberPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

// errNotJSONNumber marks a numeric scalar outside the JSON number grammar.
var errNotJSONNumber = errors.New("value is not a JSON-compatible number")

// CanonicalNumber is a numeric scalar carried through the projection exactly as authored.
//
// A measurement value is evidence. Decoding "2.50" into a float64 and re-encoding it would
// serve "2.5", silently discarding the significant figure the author recorded — precision the
// experiment-run contract exists to protect. The raw token is therefore kept and re-emitted,
// and is validated against the JSON number grammar on the way in so that what is emitted is
// always a well-formed JSON number.
type CanonicalNumber struct {
	raw string
}

// UnmarshalYAML decodes a numeric scalar, rejecting anything that is not a JSON number.
//
// YAML admits numeric spellings JSON does not — 0x1F, .5, +3, inf, nan — and quoted strings
// that merely look numeric. All are refused rather than converted: the canonical validators
// refuse them too, and a converted value would no longer be the authored one.
func (n *CanonicalNumber) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errNotJSONNumber
	}
	switch value.Tag {
	case "!!int", "!!float":
	default:
		return errNotJSONNumber
	}
	if !jsonNumberPattern.MatchString(value.Value) {
		return errNotJSONNumber
	}
	n.raw = value.Value
	return nil
}

// MarshalJSON emits the authored token, which is a valid JSON number by construction.
func (n CanonicalNumber) MarshalJSON() ([]byte, error) {
	if n.raw == "" {
		return nil, errNotJSONNumber
	}
	return []byte(n.raw), nil
}

// UnmarshalJSON reads a value back from the projection's own output.
//
// It exists so a Go client — including this repository's own tests — can decode an API response
// into the same types it was encoded from. A published type that cannot be read back by its own
// package is a trap for the first consumer who tries. The same grammar check applies, so a
// hand-written body cannot smuggle in a token the encoder would never produce.
func (n *CanonicalNumber) UnmarshalJSON(data []byte) error {
	raw := string(data)
	if !jsonNumberPattern.MatchString(raw) {
		return errNotJSONNumber
	}
	n.raw = raw
	return nil
}

// String returns the authored token.
func (n CanonicalNumber) String() string { return n.raw }

// Float returns the value as a float64 for a caller that needs to compute with it. The
// projection itself never does; it serves the authored token.
func (n CanonicalNumber) Float() (float64, error) { return strconv.ParseFloat(n.raw, 64) }

// ControlValue is a control setting's configured value.
//
// schemas/experiment-run.schema.yaml allows a number or a string here, because a configured
// input may be 440 or "position B". The two are kept distinguishable rather than collapsed
// into a string: a control setting is not a measurement, but it is still a record of what was
// configured, and a number arriving as a quoted string in the API would misreport it.
type ControlValue struct {
	number CanonicalNumber
	text   string
	isText bool
}

// UnmarshalYAML decodes either permitted scalar form.
func (v *ControlValue) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("control setting value must be a number or a string")
	}
	if value.Tag == "!!str" {
		v.isText = true
		v.text = value.Value
		return nil
	}
	if err := v.number.UnmarshalYAML(value); err != nil {
		return fmt.Errorf("control setting value must be a number or a string")
	}
	v.isText = false
	return nil
}

// MarshalJSON emits the value in the form it was authored in.
func (v ControlValue) MarshalJSON() ([]byte, error) {
	if v.isText {
		return json.Marshal(v.text)
	}
	return v.number.MarshalJSON()
}

// UnmarshalJSON reads a control value back from the projection's own output, for the reason
// CanonicalNumber.UnmarshalJSON exists.
func (v *ControlValue) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		v.isText = true
		v.text = text
		return nil
	}
	if err := v.number.UnmarshalJSON(data); err != nil {
		return fmt.Errorf("control setting value must be a number or a string")
	}
	v.isText = false
	return nil
}

// IsText reports whether the value was authored as a string rather than a number.
func (v ControlValue) IsText() bool { return v.isText }

// Text returns the string form, which is empty unless IsText reports true.
func (v ControlValue) Text() string { return v.text }

// Number returns the numeric form, which is meaningful only when IsText reports false.
func (v ControlValue) Number() CanonicalNumber { return v.number }

// VocabularyEntry is one canonical AudioMuse terminology record.
//
// Contract: schemas/vocabulary.schema.yaml version 1. Every field below is required by that
// contract, which also sets additional_properties: false, so a record's key set must equal
// this set exactly.
//
// RelatedTerms is the field this type exists to keep honest. vocabulary/README.md states it
// plainly: related terms are for human navigation only, are not typed graph relationships, do
// not imply equivalence, never create graph edges, and must not affect node degree. It is
// therefore modelled as a plain ID list on the vocabulary record, is never projected into
// domain.Graph, is never given a relationship type, and is never merged with NodeRefs.
//
// Path is not canonical content: it is the repository-relative file the record was read from.
type VocabularyEntry struct {
	ID                  string   `json:"id"                   yaml:"id"`
	Term                string   `json:"term"                 yaml:"term"`
	Domain              string   `json:"domain"               yaml:"domain"`
	Definition          string   `json:"definition"           yaml:"definition"`
	DigitalRelationship string   `json:"digital_relationship" yaml:"digital_relationship"`
	BestUse             string   `json:"best_use"             yaml:"best_use"`
	Technologies        []string `json:"technologies"         yaml:"technologies"`
	NodeRefs            []string `json:"node_refs"            yaml:"node_refs"`
	SessionRefs         []string `json:"session_refs"         yaml:"session_refs"`
	RelatedTerms        []string `json:"related_terms"        yaml:"related_terms"`
	Tags                []string `json:"tags"                 yaml:"tags"`

	Path string `json:"path" yaml:"-"`
}

// VocabularySummary is the list projection of a VocabularyEntry.
//
// It carries the definition because a terminology list whose entries did not say what they
// mean would force one detail request per row to answer the question the list was asked.
type VocabularySummary struct {
	ID               string   `json:"id"`
	Term             string   `json:"term"`
	Domain           string   `json:"domain"`
	Definition       string   `json:"definition"`
	NodeRefs         []string `json:"node_refs"`
	SessionRefs      []string `json:"session_refs"`
	Tags             []string `json:"tags"`
	RelatedTermCount int      `json:"related_term_count"`
}

// VocabularyDetail is the single-record projection: the canonical entry plus the derived
// reverse views of what refers to it.
//
// ExperimentIDs is the reverse read of experiment vocabulary_refs and ClaimIDs the reverse
// read of claim appears_in: vocabulary. Both are derived views of references authored
// elsewhere, and neither is a graph edge. They are kept under names that say which record did
// the referring, exactly as RelatedTerms is kept separate from NodeRefs.
type VocabularyDetail struct {
	VocabularyEntry
	ExperimentIDs []string `json:"experiment_ids"`
	ClaimIDs      []string `json:"claim_ids"`
}

// Experiment is one canonical AudioMuse experiment definition.
//
// Contract: schemas/experiment.schema.yaml version 1. A definition is a reusable
// specification, never evidence that anybody performed it; experiments/README.md is explicit
// that the layer holds a bounded repeatable exercise, not a result store.
//
// Observations and Measurements on a definition are instructions — what a performer should
// record, and which of those recordings would be qualitative rather than quantitative. They
// are prose string lists here, while an ExperimentRun's observations and measurements are
// typed objects carrying context, method, tool, calibration and uncertainty. The two layers
// therefore cannot be confused in Go or in JSON, which is the point.
type Experiment struct {
	ID                 string   `json:"id"                  yaml:"id"`
	Title              string   `json:"title"               yaml:"title"`
	Status             string   `json:"status"              yaml:"status"`
	Type               string   `json:"type"                yaml:"type"`
	Difficulty         string   `json:"difficulty"          yaml:"difficulty"`
	Purpose            string   `json:"purpose"             yaml:"purpose"`
	NodeRefs           []string `json:"node_refs"           yaml:"node_refs"`
	VocabularyRefs     []string `json:"vocabulary_refs"     yaml:"vocabulary_refs"`
	SessionRefs        []string `json:"session_refs"        yaml:"session_refs"`
	SourceRefs         []string `json:"source_refs"         yaml:"source_refs"`
	RequiredEquipment  []string `json:"required_equipment"  yaml:"required_equipment"`
	OptionalEquipment  []string `json:"optional_equipment"  yaml:"optional_equipment"`
	Safety             []string `json:"safety"              yaml:"safety"`
	Setup              []string `json:"setup"               yaml:"setup"`
	Procedure          []string `json:"procedure"           yaml:"procedure"`
	Observations       []string `json:"observations"        yaml:"observations"`
	Measurements       []string `json:"measurements"        yaml:"measurements"`
	ExpectedBehavior   []string `json:"expected_behavior"   yaml:"expected_behavior"`
	Interpretation     []string `json:"interpretation"      yaml:"interpretation"`
	Limitations        []string `json:"limitations"         yaml:"limitations"`
	Repeatability      []string `json:"repeatability"       yaml:"repeatability"`
	RelatedExperiments []string `json:"related_experiments" yaml:"related_experiments"`
	ProjectConnections []string `json:"project_connections" yaml:"project_connections"`

	Path string `json:"path" yaml:"-"`
}

// ExperimentRunCounts is the derived execution tally for one experiment definition.
//
// Every canonical run status is reported separately rather than folded into a "done" count.
// Collapsing incomplete or invalid runs into completed ones would assert evidence the records
// explicitly withhold, and reporting only a total would hide that a definition with three runs
// may have produced no evidence at all.
//
// These counts are derived at startup from canonical experiment-run records. They are never
// written back to the repository, and experiment-runs/index.md is not their source.
type ExperimentRunCounts struct {
	Total      int `json:"total"`
	Planned    int `json:"planned"`
	Completed  int `json:"completed"`
	Incomplete int `json:"incomplete"`
	Invalid    int `json:"invalid"`
}

// ExperimentSummary is the list projection of an Experiment.
//
// The run tally travels with the summary because "which of these has actually been performed"
// is the first question an experiment list is asked, and answering it per row would otherwise
// cost one detail request each.
type ExperimentSummary struct {
	ID             string              `json:"id"`
	Title          string              `json:"title"`
	Status         string              `json:"status"`
	Type           string              `json:"type"`
	Difficulty     string              `json:"difficulty"`
	Purpose        string              `json:"purpose"`
	NodeRefs       []string            `json:"node_refs"`
	VocabularyRefs []string            `json:"vocabulary_refs"`
	SessionRefs    []string            `json:"session_refs"`
	Runs           ExperimentRunCounts `json:"runs"`
}

// ExperimentDetail is the single-record projection: the canonical definition plus the derived
// view of the runs that reference it.
//
// RunIDs names the runs; it does not embed them. A definition that carried its own results
// inline would read as though those results were part of the definition, which is the exact
// merge experiment-runs/ was separated out to prevent.
type ExperimentDetail struct {
	Experiment
	Runs   ExperimentRunCounts `json:"runs"`
	RunIDs []string            `json:"run_ids"`
}

// RunControlSetting is one configured input of an experiment run.
//
// Contract: schemas/experiment-run.schema.yaml (control_settings[]), key set exactly
// {quantity, value, unit, context}. experiment-runs/README.md is explicit that these are not
// measurements of physical acoustic output: a nominal oscillator frequency is what was dialled
// in, not what came out. The type is therefore separate from RunMeasurement and carries no
// method, tool, calibration or uncertainty, because a configured input has none.
type RunControlSetting struct {
	Quantity string       `json:"quantity" yaml:"quantity"`
	Value    ControlValue `json:"value"    yaml:"value"`
	Unit     *string      `json:"unit"     yaml:"unit"`
	Context  string       `json:"context"  yaml:"context"`
}

// RunObservation is one qualitative finding from an experiment run.
//
// Contract: schemas/experiment-run.schema.yaml (observations[]), key set exactly
// {statement, context}. An observation is perceptual or descriptive and carries no value,
// unit, method or calibration — it cannot, because nothing was measured. Context is required
// and served verbatim: a statement without the circumstances it was noticed in is exactly the
// unattributable listening impression the run contract exists to prevent.
type RunObservation struct {
	Statement string `json:"statement" yaml:"statement"`
	Context   string `json:"context"   yaml:"context"`
}

// RunMeasurement is one quantitative finding from an experiment run.
//
// Contract: schemas/experiment-run.schema.yaml (measurements[]), key set exactly
// {quantity, value, unit, method, tool, calibration, uncertainty, limitations}. Every one of
// those fields is served: experiment-runs/README.md requires them precisely so that a number
// cannot masquerade as measured evidence, and a projection that dropped method, tool or
// calibration would restore exactly that failure. Uncertainty is a pointer because the
// contract allows an explicit null, which means "not stated" and differs from "none".
type RunMeasurement struct {
	Quantity    string          `json:"quantity"    yaml:"quantity"`
	Value       CanonicalNumber `json:"value"       yaml:"value"`
	Unit        string          `json:"unit"        yaml:"unit"`
	Method      string          `json:"method"      yaml:"method"`
	Tool        string          `json:"tool"        yaml:"tool"`
	Calibration string          `json:"calibration" yaml:"calibration"`
	Uncertainty *string         `json:"uncertainty" yaml:"uncertainty"`
	Limitations []string        `json:"limitations" yaml:"limitations"`
}

// Experiment run statuses from schemas/experiment-run.schema.yaml.
//
// These constants are used only to decide which lifecycle rule applies to a record. The
// permitted set itself is still read from the contract at startup; naming values here does not
// widen or narrow it.
const (
	RunStatusPlanned    = "planned"
	RunStatusCompleted  = "completed"
	RunStatusIncomplete = "incomplete"
	RunStatusInvalid    = "invalid"
)

// ExperimentRun is one canonical planned or performed execution of an experiment.
//
// Contract: schemas/experiment-run.schema.yaml version 1. Every field below is required by
// that contract, which also sets additional_properties: false.
//
// RunDate is a pointer because the contract allows an explicit null and gives that null a
// meaning: a planned run has no date because it has not happened. Serving a zero-value empty
// string instead would turn "not performed" into "performed on an unknown date".
type ExperimentRun struct {
	ID                  string              `json:"id"                   yaml:"id"`
	ExperimentID        string              `json:"experiment_id"        yaml:"experiment_id"`
	RunDate             *string             `json:"run_date"             yaml:"run_date"`
	Status              string              `json:"status"               yaml:"status"`
	EnvironmentNotes    []string            `json:"environment_notes"    yaml:"environment_notes"`
	Equipment           []string            `json:"equipment"            yaml:"equipment"`
	Software            []string            `json:"software"             yaml:"software"`
	ProcedureDeviations []string            `json:"procedure_deviations" yaml:"procedure_deviations"`
	ControlSettings     []RunControlSetting `json:"control_settings"     yaml:"control_settings"`
	Observations        []RunObservation    `json:"observations"         yaml:"observations"`
	Measurements        []RunMeasurement    `json:"measurements"         yaml:"measurements"`
	Limitations         []string            `json:"limitations"          yaml:"limitations"`
	SafetyNotes         []string            `json:"safety_notes"         yaml:"safety_notes"`
	Interpretation      []string            `json:"interpretation"       yaml:"interpretation"`
	FollowUpQuestions   []string            `json:"follow_up_questions"  yaml:"follow_up_questions"`
	SourceRefs          []string            `json:"source_refs"          yaml:"source_refs"`

	Path string `json:"path" yaml:"-"`
}

// Performed reports whether the run was executed at all.
//
// It is derived from Status and is not a second canonical field: planned is the only status
// meaning nothing was run, and completed, incomplete and invalid all mean execution happened.
// It is served so that "planned or actually performed" is answerable without a client
// hard-coding the status enum, and it deliberately does not imply the run produced usable
// evidence — that is what Status and the evidence arrays are for.
func (r ExperimentRun) Performed() bool { return r.Status != RunStatusPlanned }

// ExperimentRunSummary is the list projection of an ExperimentRun.
//
// Observation and measurement counts are reported separately, never summed into one evidence
// count. A run with three observations and no measurements is a different kind of record from
// one with three measurements, and a single number would erase the distinction the run
// contract is built on.
type ExperimentRunSummary struct {
	ID               string   `json:"id"`
	ExperimentID     string   `json:"experiment_id"`
	Status           string   `json:"status"`
	Performed        bool     `json:"performed"`
	RunDate          *string  `json:"run_date"`
	ObservationCount int      `json:"observation_count"`
	MeasurementCount int      `json:"measurement_count"`
	SourceRefs       []string `json:"source_refs"`
}

// ExperimentRunDetail is the single-record projection: the canonical run plus the derived
// performed flag and the title of the definition it executes.
//
// ExperimentTitle is a convenience label only. The authoritative link is ExperimentID, which
// is canonical; the title is carried so a run can be displayed without a second request and is
// never a substitute for reading the definition.
type ExperimentRunDetail struct {
	ExperimentRun
	Performed       bool   `json:"performed"`
	ExperimentTitle string `json:"experiment_title"`
}

// ExperimentVocabulary is the bounded set of values the experiment contract declares.
//
// Read from schemas/experiment.schema.yaml at startup for the reason ClaimVocabulary is read
// from the claim contract: every one of these is an API filter, and a vocabulary change must
// be a schema change rather than something a validator can be edited to allow silently.
type ExperimentVocabulary struct {
	Statuses     []string `json:"statuses"`
	Types        []string `json:"types"`
	Difficulties []string `json:"difficulties"`
}

// ExperimentRunVocabulary is the bounded set of values the experiment-run contract declares.
//
// Calibrations is not an API filter but is validated at load: a calibration value outside the
// contract would let an uncalibrated reading be served as though its calibration were known,
// which is the most consequential misreading available in this layer.
type ExperimentRunVocabulary struct {
	Statuses     []string `json:"statuses"`
	Calibrations []string `json:"calibrations"`
}
