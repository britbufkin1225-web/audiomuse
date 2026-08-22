package service_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

func vocabularyListIDs(list service.VocabularyList) []string {
	out := make([]string, 0, len(list.Vocabulary))
	for _, entry := range list.Vocabulary {
		out = append(out, entry.ID)
	}
	return out
}

func listVocabulary(t testing.TB, k *service.Knowledge, query service.VocabularyQuery) service.VocabularyList {
	t.Helper()
	list, err := k.ListVocabulary(query)
	if err != nil {
		t.Fatalf("list vocabulary: %v", err)
	}
	return list
}

func experimentListIDs(list service.ExperimentList) []string {
	out := make([]string, 0, len(list.Experiments))
	for _, experiment := range list.Experiments {
		out = append(out, experiment.ID)
	}
	return out
}

func runListIDs(list service.ExperimentRunList) []string {
	out := make([]string, 0, len(list.Runs))
	for _, run := range list.Runs {
		out = append(out, run.ID)
	}
	return out
}

func mustListExperiments(t testing.TB, k *service.Knowledge, q service.ExperimentQuery) service.ExperimentList {
	t.Helper()
	list, err := k.ListExperiments(q)
	if err != nil {
		t.Fatalf("list experiments %+v: %v", q, err)
	}
	return list
}

func mustListRuns(t testing.TB, k *service.Knowledge, q service.ExperimentRunQuery) service.ExperimentRunList {
	t.Helper()
	list, err := k.ListExperimentRuns(q)
	if err != nil {
		t.Fatalf("list experiment runs %+v: %v", q, err)
	}
	return list
}

func TestVocabularyListIsCanonicallyOrdered(t *testing.T) {
	k := evidenceIndex(t)
	list := listVocabulary(t, k, service.VocabularyQuery{})

	want := []string{"fixture-companion", "fixture-orphan-term", "fixture-term"}
	if got := vocabularyListIDs(list); !reflect.DeepEqual(got, want) {
		t.Errorf("vocabulary = %v, want %v", got, want)
	}
	if list.Page.Total != 3 || list.Page.Count != 3 {
		t.Errorf("page = %+v, want total 3 count 3", list.Page)
	}
}

func TestVocabularyFilters(t *testing.T) {
	k := evidenceIndex(t)
	cases := []struct {
		name  string
		query service.VocabularyQuery
		want  []string
	}{
		{"domain", service.VocabularyQuery{Domain: "dsp"}, []string{"fixture-companion"}},
		{"node", service.VocabularyQuery{NodeID: "alpha"}, []string{"fixture-term"}},
		{"session", service.VocabularyQuery{SessionID: "session-01-fixture"}, []string{"fixture-term"}},
		{"tag", service.VocabularyQuery{Tag: "orphan"}, []string{"fixture-orphan-term"}},
		{"composed", service.VocabularyQuery{Domain: "acoustics", Tag: "fixture"},
			[]string{"fixture-orphan-term", "fixture-term"}},
		{"search matches the term", service.VocabularyQuery{Q: "companion"}, []string{"fixture-companion"}},
		{"search is case-insensitive", service.VocabularyQuery{Q: "COMPANION"}, []string{"fixture-companion"}},
		{"search matches a technology", service.VocabularyQuery{Q: "go test"}, []string{"fixture-term"}},
		// An identifier filter is not checked against existence: an unknown ID means "no entry
		// stands in that relation", which is an empty result rather than an error.
		{"unknown node id", service.VocabularyQuery{NodeID: "no-such-node"}, []string{}},
		// Canonical identity is case-sensitive; only lexical search is tolerant.
		{"node filter case drift", service.VocabularyQuery{NodeID: "Alpha"}, []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := vocabularyListIDs(listVocabulary(t, k, tc.query))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestVocabularyBoundedFilterRejectsCaseDrift separates the two kinds of vocabulary filter.
//
// Domain is bounded by the canonical node-domain enum, so a value outside it — including one
// that differs only in casing — is refused rather than answered with an empty list. An empty
// list would mean "no entry is in that domain", which asserts something about the corpus that
// was never asked; the caller named a domain the corpus has no concept of. NodeID is a
// canonical identifier and is deliberately not validated this way, because an unknown ID does
// mean "no entry stands in that relation". TestVocabularyFilters covers that half.
func TestVocabularyBoundedFilterRejectsCaseDrift(t *testing.T) {
	k := evidenceIndex(t)

	if _, err := k.ListVocabulary(service.VocabularyQuery{Domain: "DSP"}); err == nil {
		t.Fatal("case-drifted domain was accepted, want rejection")
	} else {
		var invalid *service.InvalidFilterError
		if !errors.As(err, &invalid) {
			t.Fatalf("err = %v, want *service.InvalidFilterError", err)
		}
		if invalid.Param != "domain" {
			t.Errorf("param = %q, want %q", invalid.Param, "domain")
		}
	}

	// The rejection is the enum contract, not a rejection of every unmatched value: a
	// canonically cased domain the fixture corpus happens not to use still answers empty.
	if _, err := k.ListVocabulary(service.VocabularyQuery{Domain: "invented-domain"}); err == nil {
		t.Error("unknown domain was accepted, want rejection")
	}
}

// TestVocabularyDetailDerivesReverseViews covers the two derived lists on a vocabulary detail,
// including the case where both are empty.
func TestVocabularyDetailDerivesReverseViews(t *testing.T) {
	k := evidenceIndex(t)

	entry, err := k.VocabularyByID("fixture-term")
	if err != nil {
		t.Fatalf("vocabulary lookup: %v", err)
	}
	if got, want := entry.ExperimentIDs, []string{"fixture-listening-exercise"}; !reflect.DeepEqual(got, want) {
		t.Errorf("experiment_ids = %v, want %v", got, want)
	}
	if got, want := entry.ClaimIDs, []string{"alpha-carries-energy"}; !reflect.DeepEqual(got, want) {
		t.Errorf("claim_ids = %v, want %v", got, want)
	}

	orphan, err := k.VocabularyByID("fixture-orphan-term")
	if err != nil {
		t.Fatalf("vocabulary lookup: %v", err)
	}
	if orphan.ExperimentIDs == nil || orphan.ClaimIDs == nil {
		t.Fatal("an unreferenced entry must derive empty lists, not nil ones")
	}
	if len(orphan.ExperimentIDs) != 0 || len(orphan.ClaimIDs) != 0 {
		t.Errorf("orphan entry derived references: %+v", orphan)
	}
}

// TestVocabularyReferencesAreNotGraphEdges is the contract check that Phase 1D exists under.
// Resolving a vocabulary cross-reference must not add a vertex or an edge to the graph.
func TestVocabularyReferencesAreNotGraphEdges(t *testing.T) {
	k := evidenceIndex(t)
	graph := k.Graph()

	practiceIDs := map[string]bool{
		"fixture-term": true, "fixture-companion": true, "fixture-orphan-term": true,
		"fixture-listening-exercise": true, "fixture-visualization-exercise": true,
		"fixture-listening-exercise-planned-a": true, "fixture-listening-exercise-completed-a": true,
	}
	for _, node := range graph.Nodes {
		if practiceIDs[node.ID] {
			t.Errorf("practice record %q became a graph vertex", node.ID)
		}
	}
	for _, edge := range graph.Edges {
		if practiceIDs[edge.Source] || practiceIDs[edge.Target] {
			t.Errorf("practice record produced a graph edge %+v", edge)
		}
	}

	// The fixture graph is three nodes and the edges their relationships declare. Phase 1D must
	// not have changed either count.
	if got, want := graph.Metadata.NodeCount, 3; got != want {
		t.Errorf("graph node count = %d, want %d", got, want)
	}
}

func TestVocabularyDetailUnknownID(t *testing.T) {
	k := evidenceIndex(t)
	if _, err := k.VocabularyByID("no-such-term"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestExperimentListCarriesRunTally(t *testing.T) {
	k := evidenceIndex(t)
	list := mustListExperiments(t, k, service.ExperimentQuery{})

	if got, want := experimentListIDs(list), []string{
		"fixture-listening-exercise", "fixture-visualization-exercise",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("experiments = %v, want %v", got, want)
	}

	withRuns := list.Experiments[0]
	want := domain.ExperimentRunCounts{Total: 2, Planned: 1, Completed: 1}
	if withRuns.Runs != want {
		t.Errorf("run tally = %+v, want %+v", withRuns.Runs, want)
	}

	// An experiment nobody has run must report zeroes rather than omitting the tally, so a
	// client cannot read a missing field as "unknown".
	withoutRuns := list.Experiments[1]
	if withoutRuns.Runs != (domain.ExperimentRunCounts{}) {
		t.Errorf("run tally = %+v, want an all-zero tally", withoutRuns.Runs)
	}
}

func TestExperimentFilters(t *testing.T) {
	k := evidenceIndex(t)
	cases := []struct {
		name  string
		query service.ExperimentQuery
		want  []string
	}{
		{"status", service.ExperimentQuery{Status: "established"}, []string{"fixture-visualization-exercise"}},
		{"type", service.ExperimentQuery{Type: "listening"}, []string{"fixture-listening-exercise"}},
		{"difficulty", service.ExperimentQuery{Difficulty: "intermediate"}, []string{"fixture-visualization-exercise"}},
		{"node", service.ExperimentQuery{NodeID: "beta"}, []string{"fixture-listening-exercise"}},
		{"vocabulary", service.ExperimentQuery{VocabularyID: "fixture-companion"}, []string{"fixture-visualization-exercise"}},
		{"session", service.ExperimentQuery{SessionID: "session-01-fixture"}, []string{"fixture-listening-exercise"}},
		{"source", service.ExperimentQuery{SourceID: "fixture-reference-work"}, []string{"fixture-listening-exercise"}},
		{"search", service.ExperimentQuery{Q: "visualization"}, []string{"fixture-visualization-exercise"}},
		{"composed to nothing", service.ExperimentQuery{Type: "listening", Difficulty: "intermediate"}, []string{}},
		{"unknown node id", service.ExperimentQuery{NodeID: "no-such-node"}, []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := experimentListIDs(mustListExperiments(t, k, tc.query))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestExperimentBoundedFiltersAreRejected covers the three filters bounded by
// schemas/experiment.schema.yaml. A value outside the contract is refused rather than returning
// an empty set that a caller would read as "none exist".
func TestExperimentBoundedFiltersAreRejected(t *testing.T) {
	k := evidenceIndex(t)
	cases := []struct {
		param string
		query service.ExperimentQuery
	}{
		{"status", service.ExperimentQuery{Status: "provisional"}},
		{"type", service.ExperimentQuery{Type: "measurement"}},
		{"difficulty", service.ExperimentQuery{Difficulty: "expert"}},
		{"status", service.ExperimentQuery{Status: "Proof"}},
	}
	for _, tc := range cases {
		t.Run(tc.param+"="+tc.query.Status+tc.query.Type+tc.query.Difficulty, func(t *testing.T) {
			_, err := k.ListExperiments(tc.query)
			var invalid *service.InvalidFilterError
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %v, want InvalidFilterError", err)
			}
			if invalid.Param != tc.param {
				t.Errorf("param = %q, want %q", invalid.Param, tc.param)
			}
			if len(invalid.Allowed) == 0 {
				t.Error("rejected filter did not report the accepted values")
			}
		})
	}
}

func TestExperimentDetailNamesItsRuns(t *testing.T) {
	k := evidenceIndex(t)

	experiment, err := k.ExperimentByID("fixture-listening-exercise")
	if err != nil {
		t.Fatalf("experiment lookup: %v", err)
	}
	want := []string{
		"fixture-listening-exercise-completed-a",
		"fixture-listening-exercise-planned-a",
	}
	if !reflect.DeepEqual(experiment.RunIDs, want) {
		t.Errorf("run_ids = %v, want %v", experiment.RunIDs, want)
	}
	if experiment.Runs.Total != 2 || experiment.Runs.Planned != 1 || experiment.Runs.Completed != 1 {
		t.Errorf("run tally = %+v", experiment.Runs)
	}

	empty, err := k.ExperimentByID("fixture-visualization-exercise")
	if err != nil {
		t.Fatalf("experiment lookup: %v", err)
	}
	if empty.RunIDs == nil {
		t.Fatal("an experiment with no runs must report an empty list, not nil")
	}
	if len(empty.RunIDs) != 0 {
		t.Errorf("run_ids = %v, want empty", empty.RunIDs)
	}
}

func TestExperimentDetailUnknownID(t *testing.T) {
	k := evidenceIndex(t)
	if _, err := k.ExperimentByID("no-such-experiment"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestExperimentRunFilters(t *testing.T) {
	k := evidenceIndex(t)
	yes, no := true, false
	cases := []struct {
		name  string
		query service.ExperimentRunQuery
		want  []string
	}{
		{"all", service.ExperimentRunQuery{}, []string{
			"fixture-listening-exercise-completed-a", "fixture-listening-exercise-planned-a",
		}},
		{"experiment", service.ExperimentRunQuery{ExperimentID: "fixture-listening-exercise"}, []string{
			"fixture-listening-exercise-completed-a", "fixture-listening-exercise-planned-a",
		}},
		{"experiment with no runs", service.ExperimentRunQuery{ExperimentID: "fixture-visualization-exercise"}, []string{}},
		{"status planned", service.ExperimentRunQuery{Status: "planned"}, []string{"fixture-listening-exercise-planned-a"}},
		{"performed true", service.ExperimentRunQuery{Performed: &yes}, []string{"fixture-listening-exercise-completed-a"}},
		{"performed false", service.ExperimentRunQuery{Performed: &no}, []string{"fixture-listening-exercise-planned-a"}},
		{"source", service.ExperimentRunQuery{SourceID: "fixture-archive-record"}, []string{"fixture-listening-exercise-completed-a"}},
		{"unknown experiment id", service.ExperimentRunQuery{ExperimentID: "no-such-experiment"}, []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runListIDs(mustListRuns(t, k, tc.query))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ids = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExperimentRunStatusFilterIsBounded(t *testing.T) {
	k := evidenceIndex(t)
	_, err := k.ListExperimentRuns(service.ExperimentRunQuery{Status: "finished"})
	var invalid *service.InvalidFilterError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want InvalidFilterError", err)
	}
	if invalid.Param != "status" {
		t.Errorf("param = %q, want status", invalid.Param)
	}
}

// TestRunSummaryKeepsEvidenceKindsApart asserts the list projection reports observation and
// measurement counts separately and never as one evidence total.
func TestRunSummaryKeepsEvidenceKindsApart(t *testing.T) {
	k := evidenceIndex(t)
	list := mustListRuns(t, k, service.ExperimentRunQuery{})

	completed := list.Runs[0]
	if completed.ID != "fixture-listening-exercise-completed-a" {
		t.Fatalf("unexpected first run %q", completed.ID)
	}
	if !completed.Performed {
		t.Error("completed run reported as not performed")
	}
	if completed.ObservationCount != 1 || completed.MeasurementCount != 1 {
		t.Errorf("observation/measurement counts = %d/%d, want 1/1",
			completed.ObservationCount, completed.MeasurementCount)
	}
	if completed.RunDate == nil || *completed.RunDate != "1999-04-01" {
		t.Errorf("run_date = %v, want 1999-04-01", completed.RunDate)
	}

	planned := list.Runs[1]
	if planned.Performed {
		t.Error("planned run reported as performed")
	}
	if planned.RunDate != nil {
		t.Errorf("planned run carries a date %q", *planned.RunDate)
	}
	if planned.ObservationCount != 0 || planned.MeasurementCount != 0 {
		t.Error("planned run reported evidence")
	}
}

func TestExperimentRunDetail(t *testing.T) {
	k := evidenceIndex(t)

	run, err := k.ExperimentRunByID("fixture-listening-exercise-completed-a")
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	if !run.Performed {
		t.Error("completed run reported as not performed")
	}
	if got, want := run.ExperimentTitle, "Fixture Listening Exercise"; got != want {
		t.Errorf("experiment_title = %q, want %q", got, want)
	}
	if got, want := run.ExperimentID, "fixture-listening-exercise"; got != want {
		t.Errorf("experiment_id = %q, want %q", got, want)
	}
	if len(run.ControlSettings) != 2 {
		t.Errorf("control settings = %d, want 2", len(run.ControlSettings))
	}

	if _, err := k.ExperimentRunByID("no-such-run"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

// TestPracticeDetailsAreDefensiveCopies asserts a caller cannot mutate the immutable startup
// index through a detail response, including through the pointer fields the run layer uses.
func TestPracticeDetailsAreDefensiveCopies(t *testing.T) {
	k := evidenceIndex(t)

	entry, err := k.VocabularyByID("fixture-term")
	if err != nil {
		t.Fatalf("vocabulary lookup: %v", err)
	}
	entry.RelatedTerms[0] = "tampered"
	entry.NodeRefs[0] = "tampered"

	experiment, err := k.ExperimentByID("fixture-listening-exercise")
	if err != nil {
		t.Fatalf("experiment lookup: %v", err)
	}
	experiment.NodeRefs[0] = "tampered"
	experiment.Procedure[0] = "tampered"

	run, err := k.ExperimentRunByID("fixture-listening-exercise-completed-a")
	if err != nil {
		t.Fatalf("run lookup: %v", err)
	}
	*run.RunDate = "2999-01-01"
	run.Observations[0].Statement = "tampered"
	run.Measurements[0].Limitations[0] = "tampered"

	pristineEntry, _ := k.VocabularyByID("fixture-term")
	if pristineEntry.RelatedTerms[0] != "fixture-companion" || pristineEntry.NodeRefs[0] != "alpha" {
		t.Errorf("vocabulary index was mutated through a detail response: %+v", pristineEntry)
	}
	pristineExperiment, _ := k.ExperimentByID("fixture-listening-exercise")
	if pristineExperiment.NodeRefs[0] != "alpha" || pristineExperiment.Procedure[0] == "tampered" {
		t.Errorf("experiment index was mutated through a detail response: %+v", pristineExperiment)
	}
	pristineRun, _ := k.ExperimentRunByID("fixture-listening-exercise-completed-a")
	if *pristineRun.RunDate != "1999-04-01" {
		t.Errorf("run date was mutated through a detail response: %q", *pristineRun.RunDate)
	}
	if pristineRun.Observations[0].Statement == "tampered" {
		t.Error("run observations were mutated through a detail response")
	}
	if pristineRun.Measurements[0].Limitations[0] == "tampered" {
		t.Error("measurement limitations were mutated through a detail response")
	}
}

// TestRunSummaryDateIsACopy covers the same rule on the list projection, which hands out the
// canonical run_date pointer if it does not copy it.
func TestRunSummaryDateIsACopy(t *testing.T) {
	k := evidenceIndex(t)
	list := mustListRuns(t, k, service.ExperimentRunQuery{Status: "completed"})
	*list.Runs[0].RunDate = "2999-01-01"

	again := mustListRuns(t, k, service.ExperimentRunQuery{Status: "completed"})
	if got := *again.Runs[0].RunDate; got != "1999-04-01" {
		t.Errorf("run_date = %q after a caller wrote through the summary pointer", got)
	}
}

func TestPracticePaging(t *testing.T) {
	k := evidenceIndex(t)

	first := listVocabulary(t, k, service.VocabularyQuery{Limit: 2})
	if got, want := vocabularyListIDs(first), []string{"fixture-companion", "fixture-orphan-term"}; !reflect.DeepEqual(got, want) {
		t.Errorf("page one = %v, want %v", got, want)
	}
	if first.Page.Total != 3 || first.Page.Count != 2 || first.Page.Limit != 2 {
		t.Errorf("page = %+v", first.Page)
	}

	second := listVocabulary(t, k, service.VocabularyQuery{Limit: 2, Offset: 2})
	if got, want := vocabularyListIDs(second), []string{"fixture-term"}; !reflect.DeepEqual(got, want) {
		t.Errorf("page two = %v, want %v", got, want)
	}

	past := listVocabulary(t, k, service.VocabularyQuery{Offset: 99})
	if len(past.Vocabulary) != 0 || past.Page.Total != 3 {
		t.Errorf("past-the-end page = %+v", past)
	}

	clamped := listVocabulary(t, k, service.VocabularyQuery{Limit: 10000})
	if clamped.Page.Limit != service.MaxLimit {
		t.Errorf("limit = %d, want it clamped to %d", clamped.Page.Limit, service.MaxLimit)
	}
}

// TestPracticeProjectionIsDeterministic builds the index twice and compares every practice
// projection, so no result can depend on Go map iteration order.
func TestPracticeProjectionIsDeterministic(t *testing.T) {
	first, second := evidenceIndex(t), evidenceIndex(t)

	if !reflect.DeepEqual(listVocabulary(t, first, service.VocabularyQuery{}), listVocabulary(t, second, service.VocabularyQuery{})) {
		t.Error("two indexes produced different vocabulary lists")
	}
	if !reflect.DeepEqual(mustListExperiments(t, first, service.ExperimentQuery{}),
		mustListExperiments(t, second, service.ExperimentQuery{})) {
		t.Error("two indexes produced different experiment lists")
	}
	if !reflect.DeepEqual(mustListRuns(t, first, service.ExperimentRunQuery{}),
		mustListRuns(t, second, service.ExperimentRunQuery{})) {
		t.Error("two indexes produced different experiment-run lists")
	}

	firstDetail, err := first.VocabularyByID("fixture-term")
	if err != nil {
		t.Fatalf("vocabulary lookup: %v", err)
	}
	secondDetail, err := second.VocabularyByID("fixture-term")
	if err != nil {
		t.Fatalf("vocabulary lookup: %v", err)
	}
	if !reflect.DeepEqual(firstDetail, secondDetail) {
		t.Error("two indexes produced different vocabulary details")
	}
}

func TestProjectSummaryReportsThePracticeLayer(t *testing.T) {
	k := evidenceIndex(t)
	project := k.Project()

	if project.Counts.Vocabulary != 3 {
		t.Errorf("vocabulary count = %d, want 3", project.Counts.Vocabulary)
	}
	if project.Counts.Experiments != 2 {
		t.Errorf("experiment count = %d, want 2", project.Counts.Experiments)
	}
	if project.Counts.ExperimentRuns != 2 {
		t.Errorf("experiment run count = %d, want 2", project.Counts.ExperimentRuns)
	}
	want := domain.ExperimentRunCounts{Total: 2, Planned: 1, Completed: 1}
	if project.ExperimentRuns != want {
		t.Errorf("run totals = %+v, want %+v", project.ExperimentRuns, want)
	}
	if got, wantDomains := project.VocabularyDomains, []string{"acoustics", "dsp"}; !reflect.DeepEqual(got, wantDomains) {
		t.Errorf("vocabulary_domains = %v, want %v", got, wantDomains)
	}
	for _, layer := range []string{"vocabulary", "experiments", "experiment-runs"} {
		if !contains(project.CanonicalLayer, layer) {
			t.Errorf("canonical_layers_served = %v, want it to include %s", project.CanonicalLayer, layer)
		}
	}

	vocab := k.Vocabularies()
	if len(vocab.Experiment.Statuses) == 0 || len(vocab.Experiment.Types) == 0 ||
		len(vocab.Experiment.Difficulties) == 0 {
		t.Errorf("experiment contract vocabulary is empty: %+v", vocab.Experiment)
	}
	if len(vocab.ExperimentRun.Statuses) == 0 || len(vocab.ExperimentRun.Calibrations) == 0 {
		t.Errorf("experiment-run contract vocabulary is empty: %+v", vocab.ExperimentRun)
	}
}

func TestDiagnosticsReportsLoadedCorpusSize(t *testing.T) {
	k := evidenceIndex(t)
	corpus := k.Diagnostics().Corpus

	if corpus.Vocabulary != 3 || corpus.Experiments != 2 || corpus.ExperimentRuns != 2 {
		t.Errorf("diagnostics corpus = %+v", corpus)
	}
	if corpus.Nodes != 3 {
		t.Errorf("diagnostics node count = %d, want 3", corpus.Nodes)
	}
}
