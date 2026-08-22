package service

import (
	"sort"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// This file holds the practice layer of the immutable startup index: vocabulary entries,
// experiment definitions and experiment runs.
//
// Nothing here touches the graph. buildGraph reads node relationships and nothing else, and no
// function in this file adds a vertex, an edge, or a traversal step. A vocabulary
// cross-reference, an experiment's node_refs and a run's experiment_id are all resolved
// references between records, which is a different thing from a typed canonical edge.

// VocabularyQuery is a bounded, deterministic vocabulary list request.
//
// Domain, NodeID, SessionID and Tag are exact case-sensitive canonical matches, following the
// identity semantics in docs/knowledge-model.md. Q is human-facing lexical search and is
// case-insensitive substring matching, exactly as on the node and claim lists.
//
// None of the identifier filters is checked against existence: an unknown ID yields an empty
// result set, which is what "no entry stands in that relation" means, and matches how the
// Phase 1B evidence filters already behave.
type VocabularyQuery struct {
	Q         string
	Domain    string
	NodeID    string
	SessionID string
	Tag       string
	Limit     int
	Offset    int
}

// VocabularyList is the vocabulary list projection.
type VocabularyList struct {
	Page       Page                       `json:"page"`
	Vocabulary []domain.VocabularySummary `json:"vocabulary"`
}

// ExperimentQuery is a bounded, deterministic experiment list request.
//
// Status, Type and Difficulty are bounded by schemas/experiment.schema.yaml and are rejected
// when outside it, so a caller cannot filter by a value the contract does not define and read
// the empty result as "none exist".
type ExperimentQuery struct {
	Q            string
	Status       string
	Type         string
	Difficulty   string
	NodeID       string
	VocabularyID string
	SessionID    string
	SourceID     string
	Limit        int
	Offset       int
}

// ExperimentList is the experiment list projection.
type ExperimentList struct {
	Page        Page                       `json:"page"`
	Experiments []domain.ExperimentSummary `json:"experiments"`
}

// ExperimentRunQuery is a bounded, deterministic experiment-run list request.
//
// Status is bounded by schemas/experiment-run.schema.yaml. Performed is the derived
// planned/executed split: it is offered as a filter because "which runs actually happened" is
// not expressible through a single status value, and expressing it as three separate status
// requests would invite a client to hard-code the enum.
//
// There is no lexical search here. A run has no authored prose identity — its identifying
// fields are its ID, the definition it executes and its lifecycle state — and searching its
// observation statements would make a text hit mean "this run observed that", which is an
// evidence assertion the list projection has no business making.
type ExperimentRunQuery struct {
	ExperimentID string
	Status       string
	SourceID     string
	Performed    *bool
	Limit        int
	Offset       int
}

// ExperimentRunList is the experiment-run list projection.
type ExperimentRunList struct {
	Page Page                          `json:"page"`
	Runs []domain.ExperimentRunSummary `json:"experiment_runs"`
}

// buildPractice constructs every practice-layer index.
//
// All of it happens once, inside New, so the maps are never written to again and the
// immutability that makes Knowledge safe for concurrent readers still holds. Only reverse views
// that an endpoint would otherwise have to recompute per request are indexed:
//
//	runIDsByExperiment    /api/v1/experiments/{id}.run_ids and the list run tally
//	runCountsByExperiment /api/v1/experiments.runs
//	experimentIDsByVocab  /api/v1/vocabulary/{id}.experiment_ids
//	claimIDsByVocab       /api/v1/vocabulary/{id}.claim_ids
//
// The forward filters — vocabulary by node, session or tag, experiments by node, vocabulary,
// session or source — are answered by testing the record's own reference list. Those lists are
// short and authored, and an index over them would add a second representation of a fact the
// record already states.
func (k *Knowledge) buildPractice() {
	k.runIDsByExperiment = map[string][]string{}
	k.runCountsByExperiment = map[string]domain.ExperimentRunCounts{}
	k.experimentIDsByVocab = map[string][]string{}
	k.claimIDsByVocab = map[string][]string{}

	// Experiments and runs are already in canonical ID order, so every list appended here is
	// built in that order and needs no later sort.
	for _, experiment := range k.experiments {
		for _, ref := range experiment.VocabularyRefs {
			k.experimentIDsByVocab[ref] = appendDistinct(k.experimentIDsByVocab[ref], experiment.ID)
		}
	}
	for _, run := range k.runs {
		k.runIDsByExperiment[run.ExperimentID] = append(k.runIDsByExperiment[run.ExperimentID], run.ID)
		counts := k.runCountsByExperiment[run.ExperimentID]
		counts.Total++
		switch run.Status {
		case domain.RunStatusPlanned:
			counts.Planned++
		case domain.RunStatusCompleted:
			counts.Completed++
		case domain.RunStatusIncomplete:
			counts.Incomplete++
		case domain.RunStatusInvalid:
			counts.Invalid++
		}
		k.runCountsByExperiment[run.ExperimentID] = counts
	}
	for _, claim := range k.claims {
		for _, ref := range claim.AppearsIn {
			if ref.Kind == domain.ClaimKindVocabulary {
				k.claimIDsByVocab[ref.Ref] = appendDistinct(k.claimIDsByVocab[ref.Ref], claim.ID)
			}
		}
	}
}

// ListVocabulary filters, searches and pages the canonical vocabulary entries.
//
// Multiple filters compose with AND. Results keep canonical ID order, matching every other
// AudioMuse list projection; search never reorders by relevance, so two requests against an
// unchanged corpus return byte-identical bodies.
func (k *Knowledge) ListVocabulary(q VocabularyQuery) (VocabularyList, error) {
	if q.Domain != "" && !contains(k.vocabularies.VocabularyDomains, q.Domain) {
		return VocabularyList{}, &InvalidFilterError{Param: "domain", Allowed: k.vocabularies.VocabularyDomains}
	}
	limit, offset := normalisePaging(q.Limit, q.Offset)
	needle := boundedNeedle(q.Q)

	matched := make([]domain.VocabularySummary, 0, len(k.vocabulary))
	for _, entry := range k.vocabulary {
		if q.Domain != "" && entry.Domain != q.Domain {
			continue
		}
		if q.NodeID != "" && !containsExact(entry.NodeRefs, q.NodeID) {
			continue
		}
		if q.SessionID != "" && !containsExact(entry.SessionRefs, q.SessionID) {
			continue
		}
		if q.Tag != "" && !containsExact(entry.Tags, q.Tag) {
			continue
		}
		if needle != "" && !searchFieldsMatch(k.vocabularySearchText[entry.ID], needle) {
			continue
		}
		matched = append(matched, domain.VocabularySummary{
			ID:               entry.ID,
			Term:             entry.Term,
			Domain:           entry.Domain,
			Definition:       entry.Definition,
			NodeRefs:         copyIDs(entry.NodeRefs),
			SessionRefs:      copyIDs(entry.SessionRefs),
			Tags:             copyIDs(entry.Tags),
			RelatedTermCount: len(entry.RelatedTerms),
		})
	}

	page, meta := paginate(matched, limit, offset)
	return VocabularyList{Page: meta, Vocabulary: page}, nil
}

// VocabularyByID returns one entry with the derived reverse views of what refers to it.
func (k *Knowledge) VocabularyByID(id string) (domain.VocabularyDetail, error) {
	entry, ok := k.vocabularyByID[id]
	if !ok {
		return domain.VocabularyDetail{}, ErrNotFound
	}
	return domain.VocabularyDetail{
		VocabularyEntry: cloneVocabularyEntry(entry),
		ExperimentIDs:   copyIDs(k.experimentIDsByVocab[id]),
		ClaimIDs:        copyIDs(k.claimIDsByVocab[id]),
	}, nil
}

// ListExperiments filters, searches and pages the canonical experiment definitions.
func (k *Knowledge) ListExperiments(q ExperimentQuery) (ExperimentList, error) {
	vocab := k.vocabularies.Experiment
	for _, check := range []struct {
		param, value string
		allowed      []string
	}{
		{"status", q.Status, vocab.Statuses},
		{"type", q.Type, vocab.Types},
		{"difficulty", q.Difficulty, vocab.Difficulties},
	} {
		if check.value != "" && !contains(check.allowed, check.value) {
			return ExperimentList{}, &InvalidFilterError{Param: check.param, Allowed: check.allowed}
		}
	}

	limit, offset := normalisePaging(q.Limit, q.Offset)
	needle := boundedNeedle(q.Q)

	matched := make([]domain.ExperimentSummary, 0, len(k.experiments))
	for _, experiment := range k.experiments {
		if q.Status != "" && experiment.Status != q.Status {
			continue
		}
		if q.Type != "" && experiment.Type != q.Type {
			continue
		}
		if q.Difficulty != "" && experiment.Difficulty != q.Difficulty {
			continue
		}
		if q.NodeID != "" && !containsExact(experiment.NodeRefs, q.NodeID) {
			continue
		}
		if q.VocabularyID != "" && !containsExact(experiment.VocabularyRefs, q.VocabularyID) {
			continue
		}
		if q.SessionID != "" && !containsExact(experiment.SessionRefs, q.SessionID) {
			continue
		}
		if q.SourceID != "" && !containsExact(experiment.SourceRefs, q.SourceID) {
			continue
		}
		if needle != "" && !searchFieldsMatch(k.experimentSearchText[experiment.ID], needle) {
			continue
		}
		matched = append(matched, domain.ExperimentSummary{
			ID:             experiment.ID,
			Title:          experiment.Title,
			Status:         experiment.Status,
			Type:           experiment.Type,
			Difficulty:     experiment.Difficulty,
			Purpose:        experiment.Purpose,
			NodeRefs:       copyIDs(experiment.NodeRefs),
			VocabularyRefs: copyIDs(experiment.VocabularyRefs),
			SessionRefs:    copyIDs(experiment.SessionRefs),
			Runs:           k.runCountsByExperiment[experiment.ID],
		})
	}

	page, meta := paginate(matched, limit, offset)
	return ExperimentList{Page: meta, Experiments: page}, nil
}

// ExperimentByID returns one definition with the derived tally and IDs of its runs.
//
// The runs are named, not embedded: a definition that carried its results inline would read as
// though the results were part of the specification.
func (k *Knowledge) ExperimentByID(id string) (domain.ExperimentDetail, error) {
	experiment, ok := k.experimentsByID[id]
	if !ok {
		return domain.ExperimentDetail{}, ErrNotFound
	}
	return domain.ExperimentDetail{
		Experiment: cloneExperiment(experiment),
		Runs:       k.runCountsByExperiment[id],
		RunIDs:     copyIDs(k.runIDsByExperiment[id]),
	}, nil
}

// ListExperimentRuns filters and pages the canonical experiment-run records.
func (k *Knowledge) ListExperimentRuns(q ExperimentRunQuery) (ExperimentRunList, error) {
	allowed := k.vocabularies.ExperimentRun.Statuses
	if q.Status != "" && !contains(allowed, q.Status) {
		return ExperimentRunList{}, &InvalidFilterError{Param: "status", Allowed: allowed}
	}

	limit, offset := normalisePaging(q.Limit, q.Offset)

	matched := make([]domain.ExperimentRunSummary, 0, len(k.runs))
	for _, run := range k.runs {
		if q.ExperimentID != "" && run.ExperimentID != q.ExperimentID {
			continue
		}
		if q.Status != "" && run.Status != q.Status {
			continue
		}
		if q.SourceID != "" && !containsExact(run.SourceRefs, q.SourceID) {
			continue
		}
		if q.Performed != nil && run.Performed() != *q.Performed {
			continue
		}
		matched = append(matched, summariseRun(run))
	}

	page, meta := paginate(matched, limit, offset)
	return ExperimentRunList{Page: meta, Runs: page}, nil
}

// ExperimentRunByID returns one run with its derived performed flag and the title of the
// definition it executes.
func (k *Knowledge) ExperimentRunByID(id string) (domain.ExperimentRunDetail, error) {
	run, ok := k.runsByID[id]
	if !ok {
		return domain.ExperimentRunDetail{}, ErrNotFound
	}
	// The experiment always resolves: an unresolved experiment_id is fatal at load. The lookup
	// is still guarded so a missing definition yields an empty label rather than a panic.
	title := ""
	if experiment, ok := k.experimentsByID[run.ExperimentID]; ok {
		title = experiment.Title
	}
	return domain.ExperimentRunDetail{
		ExperimentRun:   cloneRun(run),
		Performed:       run.Performed(),
		ExperimentTitle: title,
	}, nil
}

// summariseRun projects one run into its list form.
//
// RunDate is copied rather than shared: it is a pointer on the canonical record and handing out
// the index's own pointer would let a caller write through it into the startup snapshot.
func summariseRun(run domain.ExperimentRun) domain.ExperimentRunSummary {
	return domain.ExperimentRunSummary{
		ID:               run.ID,
		ExperimentID:     run.ExperimentID,
		Status:           run.Status,
		Performed:        run.Performed(),
		RunDate:          copyStringPtr(run.RunDate),
		ObservationCount: len(run.Observations),
		MeasurementCount: len(run.Measurements),
		SourceRefs:       copyIDs(run.SourceRefs),
	}
}

// cloneVocabularyEntry, cloneExperiment and cloneRun prevent a caller from reaching the index
// through a canonical record's nested slices and pointers. The HTTP handlers only encode these
// values, but Knowledge is also a package API and its immutability guarantee must hold for
// every caller.
func cloneVocabularyEntry(entry domain.VocabularyEntry) domain.VocabularyEntry {
	entry.Technologies = copyIDs(entry.Technologies)
	entry.NodeRefs = copyIDs(entry.NodeRefs)
	entry.SessionRefs = copyIDs(entry.SessionRefs)
	entry.RelatedTerms = copyIDs(entry.RelatedTerms)
	entry.Tags = copyIDs(entry.Tags)
	return entry
}

func cloneExperiment(experiment domain.Experiment) domain.Experiment {
	for _, list := range []*[]string{
		&experiment.NodeRefs, &experiment.VocabularyRefs, &experiment.SessionRefs,
		&experiment.SourceRefs, &experiment.RequiredEquipment, &experiment.OptionalEquipment,
		&experiment.Safety, &experiment.Setup, &experiment.Procedure,
		&experiment.Observations, &experiment.Measurements, &experiment.ExpectedBehavior,
		&experiment.Interpretation, &experiment.Limitations, &experiment.Repeatability,
		&experiment.RelatedExperiments, &experiment.ProjectConnections,
	} {
		*list = copyIDs(*list)
	}
	return experiment
}

func cloneRun(run domain.ExperimentRun) domain.ExperimentRun {
	run.RunDate = copyStringPtr(run.RunDate)
	for _, list := range []*[]string{
		&run.EnvironmentNotes, &run.Equipment, &run.Software, &run.ProcedureDeviations,
		&run.Limitations, &run.SafetyNotes, &run.Interpretation, &run.FollowUpQuestions,
		&run.SourceRefs,
	} {
		*list = copyIDs(*list)
	}

	settings := make([]domain.RunControlSetting, 0, len(run.ControlSettings))
	for _, setting := range run.ControlSettings {
		setting.Unit = copyStringPtr(setting.Unit)
		settings = append(settings, setting)
	}
	run.ControlSettings = settings

	run.Observations = append(make([]domain.RunObservation, 0, len(run.Observations)), run.Observations...)

	measurements := make([]domain.RunMeasurement, 0, len(run.Measurements))
	for _, measurement := range run.Measurements {
		measurement.Uncertainty = copyStringPtr(measurement.Uncertainty)
		measurement.Limitations = copyIDs(measurement.Limitations)
		measurements = append(measurements, measurement)
	}
	run.Measurements = measurements
	return run
}

func copyStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

// VocabularyDomains returns the canonical domains present in the vocabulary layer, sorted.
//
// It is served alongside the node domains rather than merged with them: the two layers reuse
// the same domain enum but a domain may legitimately appear in one and not the other, and a
// single merged list would make a caller's domain filter return nothing without explanation.
func (k *Knowledge) VocabularyDomains() []string {
	seen := make(map[string]bool, len(k.vocabulary))
	out := make([]string, 0)
	for _, entry := range k.vocabulary {
		if !seen[entry.Domain] {
			seen[entry.Domain] = true
			out = append(out, entry.Domain)
		}
	}
	sort.Strings(out)
	return out
}

// ExperimentRunTotals is the corpus-wide run tally, derived the same way the per-experiment
// tally is and reported on the project summary.
func (k *Knowledge) ExperimentRunTotals() domain.ExperimentRunCounts {
	var totals domain.ExperimentRunCounts
	for _, run := range k.runs {
		totals.Total++
		switch run.Status {
		case domain.RunStatusPlanned:
			totals.Planned++
		case domain.RunStatusCompleted:
			totals.Completed++
		case domain.RunStatusIncomplete:
			totals.Incomplete++
		case domain.RunStatusInvalid:
			totals.Invalid++
		}
	}
	return totals
}
