package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

const (
	// experimentRecordsDir and experimentRunRecordsDir are the canonical practice roots.
	//
	// Unlike claims/ and vocabulary/, each file here holds exactly one record rather than a
	// stream: tools/validate-experiments.ps1 and tools/validate-experiment-runs.ps1 both read a
	// whole file as a single record, and the record's ID is what identifies it, not the filename.
	experimentRecordsDir    = "experiments/records"
	experimentRunRecordsDir = "experiment-runs/records"
)

// experimentRequiredFields is the required list from schemas/experiment.schema.yaml version 1.
// The contract also sets additional_properties: false and marks every property required, so
// this is simultaneously the required set and the allowed set.
var experimentRequiredFields = []string{
	"id", "title", "status", "type", "difficulty", "purpose",
	"node_refs", "vocabulary_refs", "session_refs", "source_refs",
	"required_equipment", "optional_equipment", "safety", "setup", "procedure",
	"observations", "measurements", "expected_behavior", "interpretation",
	"limitations", "repeatability", "related_experiments", "project_connections",
}

// experimentRunRequiredFields is the required list from schemas/experiment-run.schema.yaml
// version 1, under the same contract rule.
var experimentRunRequiredFields = []string{
	"id", "experiment_id", "run_date", "status",
	"environment_notes", "equipment", "software", "procedure_deviations",
	"control_settings", "observations", "measurements",
	"limitations", "safety_notes", "interpretation", "follow_up_questions", "source_refs",
}

// Nested item key sets from schemas/experiment-run.schema.yaml. Each nested object declares
// additional_properties: false, and struct decoding tolerates extra keys silently, so the
// shapes are checked against the YAML tree before decoding.
var (
	controlSettingItemFields = []string{"quantity", "value", "unit", "context"}
	observationItemFields    = []string{"statement", "context"}
	measurementItemFields    = []string{
		"quantity", "value", "unit", "method", "tool", "calibration", "uncertainty", "limitations",
	}
)

// isoDatePattern is the run_date format from schemas/experiment-run.schema.yaml.
//
// Only the shape is checked here, not whether the date is real or in the past. Calendar
// validity and the "a run cannot be recorded before it is performed" rule belong to
// tools/validate-experiment-runs.ps1, which is their canonical authority; the future-date rule
// in particular depends on the wall clock, and a projection whose validity changed with the
// time of day would not be the deterministic one this backend promises.
var isoDatePattern = regexp.MustCompile(`^\d{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])$`)

// loadExperiments reads every canonical experiment definition.
//
// The directory is walked in lexical order and results are sorted by ID afterwards. A corpus
// with no experiments/records/ directory is not an error, matching the claim and vocabulary
// layers.
func (r *Repository) loadExperiments(vocab domain.ExperimentVocabulary, report *domain.ValidationReport) []domain.Experiment {
	experiments := make([]domain.Experiment, 0)
	seen := make(map[string]string)

	r.walkRecords(experimentRecordsDir, report, func(p string, raw []byte) {
		experiment, ok := parseExperiment(raw, p, vocab, report)
		if !ok {
			return
		}
		if prior, dup := seen[experiment.ID]; dup {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeDuplicateID,
				Ref: experiment.ID, Path: p,
				Message: fmt.Sprintf("experiment id is already defined by %s", prior),
			})
			return
		}
		seen[experiment.ID] = p
		experiments = append(experiments, experiment)
	})

	sort.Slice(experiments, func(i, j int) bool { return experiments[i].ID < experiments[j].ID })
	return experiments
}

// loadExperimentRuns reads every canonical experiment-run record.
func (r *Repository) loadExperimentRuns(vocab domain.ExperimentRunVocabulary, report *domain.ValidationReport) []domain.ExperimentRun {
	runs := make([]domain.ExperimentRun, 0)
	seen := make(map[string]string)

	r.walkRecords(experimentRunRecordsDir, report, func(p string, raw []byte) {
		run, ok := parseExperimentRun(raw, p, vocab, report)
		if !ok {
			return
		}
		if prior, dup := seen[run.ID]; dup {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeDuplicateID,
				Ref: run.ID, Path: p,
				Message: fmt.Sprintf("experiment run id is already defined by %s", prior),
			})
			return
		}
		seen[run.ID] = p
		runs = append(runs, run)
	})

	sort.Slice(runs, func(i, j int) bool { return runs[i].ID < runs[j].ID })
	return runs
}

// walkRecords visits every .yaml file under one canonical record root in lexical order.
//
// An absent root yields no records rather than an error: each practice layer is additive and a
// corpus may legitimately predate it, exactly as claims/records/ may be absent.
func (r *Repository) walkRecords(dir string, report *domain.ValidationReport, visit func(path string, raw []byte)) {
	if _, err := fs.Stat(r.fsys, dir); err != nil {
		return
	}
	err := fs.WalkDir(r.fsys, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") {
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
		visit(p, raw)
		return nil
	})
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: dir, Message: err.Error(),
		})
	}
}

// singleRecordMapping decodes a one-record file down to its checked top-level mapping.
//
// A second document in the file is refused rather than ignored: these layers are one record per
// file, and silently dropping the rest would hide a record from every projection at once.
func singleRecordMapping(raw []byte, fields []string) (*yaml.Node, *recordError) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, malformed("file contains no canonical record")
		}
		return nil, malformed("record is not valid YAML: %s", err.Error())
	}
	// A second document is refused rather than ignored. yaml.Unmarshal would silently return
	// only the first, hiding every later record from every projection at once.
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, malformed("file must contain exactly one canonical record")
	}
	return checkedMapping(&doc, fields)
}

// parseExperiment turns one canonical experiment file into a domain.Experiment.
func parseExperiment(raw []byte, relPath string, vocab domain.ExperimentVocabulary, report *domain.ValidationReport) (domain.Experiment, bool) {
	fatal := func(code, msg, ref string) (domain.Experiment, bool) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: code, Ref: ref, Path: relPath, Message: msg,
		})
		return domain.Experiment{}, false
	}

	mapping, shapeErr := singleRecordMapping(raw, experimentRequiredFields)
	if shapeErr != nil {
		return fatal(shapeErr.code, shapeErr.message, "")
	}

	var experiment domain.Experiment
	if err := mapping.Decode(&experiment); err != nil {
		return fatal(domain.CodeMalformedRecord,
			"record does not match the experiment contract: "+err.Error(), "")
	}
	if !canonicalIDPattern.MatchString(experiment.ID) {
		return fatal(domain.CodeInvalidID,
			fmt.Sprintf("experiment id %q is not canonical kebab-case", experiment.ID), experiment.ID)
	}
	for _, field := range []struct{ name, value string }{
		{"title", experiment.Title},
		{"status", experiment.Status},
		{"type", experiment.Type},
		{"difficulty", experiment.Difficulty},
		{"purpose", experiment.Purpose},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fatal(domain.CodeMissingField, "required field "+field.name+" is empty", experiment.ID)
		}
	}

	experiment.Path = relPath
	normaliseExperimentSlices(&experiment)
	checkExperimentVocabulary(experiment, vocab, report)
	checkExperimentLists(experiment, report)
	return experiment, true
}

// normaliseExperimentSlices replaces nil slices with empty ones so the JSON projection renders
// [] rather than null. Representation only.
func normaliseExperimentSlices(e *domain.Experiment) {
	for _, list := range experimentStringLists(e) {
		ensureStrings(list)
	}
}

// experimentStringLists returns every string list on an experiment in contract order, so
// normalisation and list checking cannot drift apart by forgetting one field.
func experimentStringLists(e *domain.Experiment) []*[]string {
	return []*[]string{
		&e.NodeRefs, &e.VocabularyRefs, &e.SessionRefs, &e.SourceRefs,
		&e.RequiredEquipment, &e.OptionalEquipment, &e.Safety, &e.Setup, &e.Procedure,
		&e.Observations, &e.Measurements, &e.ExpectedBehavior, &e.Interpretation,
		&e.Limitations, &e.Repeatability, &e.RelatedExperiments, &e.ProjectConnections,
	}
}

// checkExperimentVocabulary validates the three bounded fields against the contract.
//
// They are checked at startup rather than at query time because the API exposes all three as
// filters. A value outside the vocabulary would produce a record that no valid filter can reach
// and that no invalid filter is allowed to name — invisible in every projection.
func checkExperimentVocabulary(e domain.Experiment, vocab domain.ExperimentVocabulary, report *domain.ValidationReport) {
	add := func(field, value string) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeInvalidVocabulary, Ref: e.ID, Path: e.Path,
			Message: fmt.Sprintf("%s %q is not declared in %s", field, value, experimentSchemaPath),
		})
	}
	if !newSet(vocab.Statuses)[e.Status] {
		add("status", e.Status)
	}
	if !newSet(vocab.Types)[e.Type] {
		add("type", e.Type)
	}
	if !newSet(vocab.Difficulties)[e.Difficulty] {
		add("difficulty", e.Difficulty)
	}
}

// checkExperimentLists enforces the per-list rules: no blank value and no repeat within a list,
// plus the self-reference rule the canonical validator applies to related_experiments.
func checkExperimentLists(e domain.Experiment, report *domain.ValidationReport) {
	names := []string{
		"node_refs", "vocabulary_refs", "session_refs", "source_refs",
		"required_equipment", "optional_equipment", "safety", "setup", "procedure",
		"observations", "measurements", "expected_behavior", "interpretation",
		"limitations", "repeatability", "related_experiments", "project_connections",
	}
	lists := experimentStringLists(&e)
	for i, name := range names {
		checkDistinctNonEmpty(name, *lists[i], e.ID, e.Path, report)
	}
	for _, ref := range e.RelatedExperiments {
		if ref == e.ID {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeSelfReference, Ref: e.ID, Path: e.Path,
				Message: "experiment lists itself as a related experiment",
			})
		}
	}
}

// parseExperimentRun turns one canonical run file into a domain.ExperimentRun.
func parseExperimentRun(raw []byte, relPath string, vocab domain.ExperimentRunVocabulary, report *domain.ValidationReport) (domain.ExperimentRun, bool) {
	fatal := func(code, msg, ref string) (domain.ExperimentRun, bool) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: code, Ref: ref, Path: relPath, Message: msg,
		})
		return domain.ExperimentRun{}, false
	}

	mapping, shapeErr := singleRecordMapping(raw, experimentRunRequiredFields)
	if shapeErr != nil {
		return fatal(shapeErr.code, shapeErr.message, "")
	}
	for _, shape := range []struct {
		field  string
		fields []string
	}{
		{"control_settings", controlSettingItemFields},
		{"observations", observationItemFields},
		{"measurements", measurementItemFields},
	} {
		if err := checkObjectListShape(mapping, shape.field, shape.fields); err != nil {
			return fatal(domain.CodeMalformedRecord, err.Error(), "")
		}
	}

	var run domain.ExperimentRun
	if err := mapping.Decode(&run); err != nil {
		return fatal(domain.CodeMalformedRecord,
			"record does not match the experiment-run contract: "+err.Error(), "")
	}
	if !canonicalIDPattern.MatchString(run.ID) {
		return fatal(domain.CodeInvalidID,
			fmt.Sprintf("experiment run id %q is not canonical kebab-case", run.ID), run.ID)
	}
	if !canonicalIDPattern.MatchString(run.ExperimentID) {
		return fatal(domain.CodeInvalidID,
			fmt.Sprintf("experiment_id %q is not a canonical identifier", run.ExperimentID), run.ID)
	}

	run.Path = relPath
	normaliseRunSlices(&run)
	checkRunVocabulary(run, vocab, report)
	checkRunDate(run, report)
	checkRunLists(run, report)
	checkRunLifecycle(run, report)
	return run, true
}

// normaliseRunSlices replaces nil slices with empty ones so the JSON projection renders []
// rather than null. Representation only.
func normaliseRunSlices(r *domain.ExperimentRun) {
	for _, list := range runStringLists(r) {
		ensureStrings(list)
	}
	if r.ControlSettings == nil {
		r.ControlSettings = []domain.RunControlSetting{}
	}
	if r.Observations == nil {
		r.Observations = []domain.RunObservation{}
	}
	if r.Measurements == nil {
		r.Measurements = []domain.RunMeasurement{}
	}
	for i := range r.Measurements {
		ensureStrings(&r.Measurements[i].Limitations)
	}
}

// runStringLists returns every plain string list on a run in contract order.
func runStringLists(r *domain.ExperimentRun) []*[]string {
	return []*[]string{
		&r.EnvironmentNotes, &r.Equipment, &r.Software, &r.ProcedureDeviations,
		&r.Limitations, &r.SafetyNotes, &r.Interpretation, &r.FollowUpQuestions, &r.SourceRefs,
	}
}

// checkRunVocabulary validates the run status and every measurement calibration.
//
// Status is an API filter, so an out-of-vocabulary value would make the record unreachable.
// Calibration is not a filter but is checked at the same severity, because a calibration value
// outside the contract would let an uncalibrated reading be served as though its calibration
// were known — the single most consequential misreading this layer allows.
func checkRunVocabulary(run domain.ExperimentRun, vocab domain.ExperimentRunVocabulary, report *domain.ValidationReport) {
	add := func(field, value string) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeInvalidVocabulary, Ref: run.ID, Path: run.Path,
			Message: fmt.Sprintf("%s %q is not declared in %s", field, value, experimentRunSchemaPath),
		})
	}
	if !newSet(vocab.Statuses)[run.Status] {
		add("status", run.Status)
	}
	calibrations := newSet(vocab.Calibrations)
	for _, m := range run.Measurements {
		if !calibrations[m.Calibration] {
			add("measurement calibration", m.Calibration)
		}
	}
}

// checkRunDate validates the run_date shape only. See isoDatePattern for what is deliberately
// left to the canonical PowerShell validator.
func checkRunDate(run domain.ExperimentRun, report *domain.ValidationReport) {
	if run.RunDate == nil {
		return
	}
	if !isoDatePattern.MatchString(*run.RunDate) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeInvalidRunDate, Ref: run.ID, Path: run.Path,
			Message: fmt.Sprintf("run_date %q is not an ISO YYYY-MM-DD date", *run.RunDate),
		})
	}
}

// checkRunLists enforces the per-list rules and the required non-empty fields inside every
// nested evidence object.
//
// A measurement missing its unit, method, tool or calibration is fatal rather than served with
// a blank: experiment-runs/README.md requires that context precisely so a number cannot be read
// as measured evidence without it, and serving the number anyway would defeat the requirement.
func checkRunLists(run domain.ExperimentRun, report *domain.ValidationReport) {
	names := []string{
		"environment_notes", "equipment", "software", "procedure_deviations",
		"limitations", "safety_notes", "interpretation", "follow_up_questions", "source_refs",
	}
	lists := runStringLists(&run)
	for i, name := range names {
		checkDistinctNonEmpty(name, *lists[i], run.ID, run.Path, report)
	}

	missing := func(what string) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMissingField, Ref: run.ID, Path: run.Path,
			Message: what,
		})
	}
	for _, setting := range run.ControlSettings {
		if strings.TrimSpace(setting.Quantity) == "" {
			missing("control setting has an empty quantity")
		}
		if strings.TrimSpace(setting.Context) == "" {
			missing("control setting has an empty context")
		}
		if setting.Value.IsText() && strings.TrimSpace(setting.Value.Text()) == "" {
			missing("control setting has an empty value")
		}
		if setting.Unit != nil && strings.TrimSpace(*setting.Unit) == "" {
			missing("control setting has an empty unit; omit it as null when there is none")
		}
	}
	for _, observation := range run.Observations {
		if strings.TrimSpace(observation.Statement) == "" {
			missing("observation has an empty statement")
		}
		if strings.TrimSpace(observation.Context) == "" {
			missing("observation has an empty context")
		}
	}
	for _, measurement := range run.Measurements {
		for _, field := range []struct{ name, value string }{
			{"quantity", measurement.Quantity},
			{"unit", measurement.Unit},
			{"method", measurement.Method},
			{"tool", measurement.Tool},
			{"calibration", measurement.Calibration},
		} {
			if strings.TrimSpace(field.value) == "" {
				missing("measurement has an empty " + field.name)
			}
		}
		if measurement.Uncertainty != nil && strings.TrimSpace(*measurement.Uncertainty) == "" {
			missing("measurement has an empty uncertainty; state it as null when it is not known")
		}
		checkDistinctNonEmpty("measurement limitations", measurement.Limitations, run.ID, run.Path, report)
	}
}

// checkRunLifecycle enforces the planned/performed contract from
// schemas/experiment-run.schema.yaml and experiment-runs/README.md.
//
// This is the one place where the backend deliberately re-implements a canonical semantic rule
// rather than deferring to the PowerShell validator, and the reason is the same one that
// governs every other startup check: the projection depends on it. The API serves a derived
// performed flag and derived per-status run counts on every experiment. A record claiming
// planned while carrying measurements would make those derived values assert that evidence
// exists where the repository says none does — the exact confusion this layer is separated out
// to prevent. Rules the projection does not depend on, such as calendar validity and the
// future-date bound, stay with tools/validate-experiment-runs.ps1.
func checkRunLifecycle(run domain.ExperimentRun, report *domain.ValidationReport) {
	conflict := func(msg string) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeRunLifecycleConflict, Ref: run.ID,
			Path: run.Path, Message: msg,
		})
	}
	hasEvidence := len(run.Observations) > 0 || len(run.Measurements) > 0

	if run.Status == domain.RunStatusPlanned {
		if run.RunDate != nil {
			conflict("planned run carries a run_date; a run that has not happened has no date")
		}
		if hasEvidence {
			conflict("planned run carries observations or measurements; a planned run is not evidence")
		}
		if len(run.Interpretation) > 0 {
			conflict("planned run carries interpretation; there is nothing yet to interpret")
		}
		if len(run.ProcedureDeviations) > 0 {
			conflict("planned run carries procedure deviations; the procedure has not been executed")
		}
		return
	}

	if run.RunDate == nil {
		conflict("performed run has no run_date; a performed run must say when it happened")
	}
	if run.Status == domain.RunStatusCompleted && !hasEvidence {
		conflict("completed run carries no observation or measurement evidence")
	}
	if run.Status == domain.RunStatusInvalid && len(run.Interpretation) > 0 {
		conflict("invalid run carries interpretation; its evidence must not support conclusions")
	}
}

// resolveExperimentReferences checks every cross-record reference the practice layer declares.
//
// Only references the canonical contracts permit are checked, and each is resolved against the
// layer that actually owns the identity: nodes for node_refs, vocabulary entries for
// vocabulary_refs, sources registered as type: session for session_refs, the registry for
// source_refs, and experiment definitions for a run's experiment_id.
//
// Resolving these references does not make them graph edges. No vertex and no edge is added to
// domain.Graph anywhere in this file; see service.buildGraph, which reads node relationships and
// nothing else.
func resolveExperimentReferences(
	experiments []domain.Experiment,
	runs []domain.ExperimentRun,
	vocabulary []domain.VocabularyEntry,
	nodes []domain.Node,
	sources []domain.Source,
	sessions []domain.Session,
	report *domain.ValidationReport,
) {
	experimentIDs := idSet(experiments, func(e domain.Experiment) string { return e.ID })
	vocabularyIDs := idSet(vocabulary, func(v domain.VocabularyEntry) string { return v.ID })
	nodeIDs := idSet(nodes, func(n domain.Node) string { return n.ID })
	sourceIDs := idSet(sources, func(s domain.Source) string { return s.ID })
	sessionIDs := idSet(sessions, func(s domain.Session) string { return s.ID })

	for _, experiment := range experiments {
		fatal := func(code, msg string) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: code, Ref: experiment.ID, Path: experiment.Path,
				Message: msg,
			})
		}
		for _, ref := range experiment.NodeRefs {
			if !nodeIDs[ref] {
				fatal(domain.CodeUnresolvedTarget,
					fmt.Sprintf("node_refs %q does not resolve to a canonical node", ref))
			}
		}
		for _, ref := range experiment.VocabularyRefs {
			if !vocabularyIDs[ref] {
				fatal(domain.CodeUnresolvedVocabulary,
					fmt.Sprintf("vocabulary_refs %q does not resolve to a canonical vocabulary entry", ref))
			}
		}
		for _, ref := range experiment.SessionRefs {
			if !sessionIDs[ref] {
				fatal(domain.CodeUnresolvedSession,
					fmt.Sprintf("session_refs %q is not a source registered as type: session", ref))
			}
		}
		for _, ref := range experiment.SourceRefs {
			if !sourceIDs[ref] {
				fatal(domain.CodeUnresolvedSource,
					fmt.Sprintf("source_refs %q is not registered in %s", ref, sourceRegistryPath))
			}
		}
		for _, ref := range experiment.RelatedExperiments {
			// A self-reference is already reported by checkExperimentLists and always resolves,
			// so it does not reach this branch twice.
			if !experimentIDs[ref] {
				fatal(domain.CodeUnresolvedExperiment,
					fmt.Sprintf("related_experiments %q does not resolve to a canonical experiment", ref))
			}
		}
	}

	for _, run := range runs {
		fatal := func(code, msg string) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: code, Ref: run.ID, Path: run.Path, Message: msg,
			})
		}
		if !experimentIDs[run.ExperimentID] {
			fatal(domain.CodeUnresolvedExperiment,
				fmt.Sprintf("experiment_id %q does not resolve to a canonical experiment definition", run.ExperimentID))
		}
		for _, ref := range run.SourceRefs {
			if !sourceIDs[ref] {
				fatal(domain.CodeUnresolvedSource,
					fmt.Sprintf("source_refs %q is not registered in %s", ref, sourceRegistryPath))
			}
		}
	}
}
