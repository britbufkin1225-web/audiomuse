package filesystem_test

import (
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// Fixture paths for the practice layer. Each is replaced wholesale by a defect test so the
// on-disk fixture stays valid and readable.
const (
	fixtureVocabularyPath = "vocabulary/entries/fixture-terms.yaml"
	fixtureExperimentPath = "experiments/records/fixture-listening-exercise.yaml"
	fixtureRunPath        = "experiment-runs/records/fixture-listening-exercise-planned-a.yaml"
)

func vocabularyIDs(corpus *repository.Corpus) []string {
	out := make([]string, 0, len(corpus.Vocabulary))
	for _, entry := range corpus.Vocabulary {
		out = append(out, entry.ID)
	}
	return out
}

func experimentIDs(corpus *repository.Corpus) []string {
	out := make([]string, 0, len(corpus.Experiments))
	for _, experiment := range corpus.Experiments {
		out = append(out, experiment.ID)
	}
	return out
}

func runIDs(corpus *repository.Corpus) []string {
	out := make([]string, 0, len(corpus.ExperimentRuns))
	for _, run := range corpus.ExperimentRuns {
		out = append(out, run.ID)
	}
	return out
}

func experimentByID(t testing.TB, corpus *repository.Corpus, id string) domain.Experiment {
	t.Helper()
	for _, experiment := range corpus.Experiments {
		if experiment.ID == id {
			return experiment
		}
	}
	t.Fatalf("experiment %s was not loaded", id)
	return domain.Experiment{}
}

func runByID(t testing.TB, corpus *repository.Corpus, id string) domain.ExperimentRun {
	t.Helper()
	for _, run := range corpus.ExperimentRuns {
		if run.ID == id {
			return run
		}
	}
	t.Fatalf("experiment run %s was not loaded", id)
	return domain.ExperimentRun{}
}

func vocabularyByID(t testing.TB, corpus *repository.Corpus, id string) domain.VocabularyEntry {
	t.Helper()
	for _, entry := range corpus.Vocabulary {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("vocabulary entry %s was not loaded", id)
	return domain.VocabularyEntry{}
}

func TestVocabularyEntriesParse(t *testing.T) {
	corpus, report := loadCorpus(t, testsupport.MutableCorpus(t))
	if report.HasFatal() {
		t.Fatalf("valid fixture corpus reported fatal issues: %v", report.Fatal())
	}

	entry := vocabularyByID(t, corpus, "fixture-term")
	if got, want := entry.Term, "Fixture Term"; got != want {
		t.Errorf("term = %q, want %q", got, want)
	}
	if got, want := entry.Domain, "acoustics"; got != want {
		t.Errorf("domain = %q, want %q", got, want)
	}
	if got, want := entry.NodeRefs, []string{"alpha"}; !reflect.DeepEqual(got, want) {
		t.Errorf("node_refs = %v, want %v", got, want)
	}
	if got, want := entry.RelatedTerms, []string{"fixture-companion"}; !reflect.DeepEqual(got, want) {
		t.Errorf("related_terms = %v, want %v", got, want)
	}
	if entry.Path != fixtureVocabularyPath {
		t.Errorf("path = %q, want %q", entry.Path, fixtureVocabularyPath)
	}
}

// TestVocabularyIsOrderedByID asserts the projection does not depend on the order documents
// appear in a stream: fixture-term is authored first but sorts after fixture-companion.
func TestVocabularyIsOrderedByID(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	want := []string{"fixture-companion", "fixture-orphan-term", "fixture-term"}
	if got := vocabularyIDs(corpus); !reflect.DeepEqual(got, want) {
		t.Errorf("vocabulary order = %v, want %v", got, want)
	}
}

// TestVocabularyEmptyListsAreEmptyNotNil covers the JSON representation rule: a canonically
// empty list must render as [] rather than null.
func TestVocabularyEmptyListsAreEmptyNotNil(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	entry := vocabularyByID(t, corpus, "fixture-orphan-term")
	if entry.Technologies == nil || entry.NodeRefs == nil ||
		entry.SessionRefs == nil || entry.RelatedTerms == nil {
		t.Fatalf("empty vocabulary lists decoded as nil: %#v", entry)
	}
	if len(entry.NodeRefs) != 0 || len(entry.RelatedTerms) != 0 {
		t.Errorf("orphan entry unexpectedly carries references: %#v", entry)
	}
}

func TestExperimentRecordsParse(t *testing.T) {
	corpus, report := loadCorpus(t, testsupport.MutableCorpus(t))
	if report.HasFatal() {
		t.Fatalf("valid fixture corpus reported fatal issues: %v", report.Fatal())
	}

	experiment := experimentByID(t, corpus, "fixture-listening-exercise")
	if got, want := experiment.Status, "proof"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	if got, want := experiment.Type, "listening"; got != want {
		t.Errorf("type = %q, want %q", got, want)
	}
	if got, want := experiment.VocabularyRefs, []string{"fixture-term"}; !reflect.DeepEqual(got, want) {
		t.Errorf("vocabulary_refs = %v, want %v", got, want)
	}
	if got, want := experiment.RelatedExperiments, []string{"fixture-visualization-exercise"}; !reflect.DeepEqual(got, want) {
		t.Errorf("related_experiments = %v, want %v", got, want)
	}
	if len(experiment.Procedure) == 0 {
		t.Error("procedure was dropped; a definition without its procedure is not reproducible")
	}
}

// TestExperimentDefinitionEvidenceFieldsStayProse is the definition-versus-evidence check at the
// type level: a definition's observations and measurements are instructions, so they must stay
// string lists and must never decode into the typed evidence objects a run carries.
func TestExperimentDefinitionEvidenceFieldsStayProse(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	experiment := experimentByID(t, corpus, "fixture-listening-exercise")

	if got := reflect.TypeOf(experiment.Observations).String(); got != "[]string" {
		t.Errorf("experiment observations are %s, want []string", got)
	}
	if got := reflect.TypeOf(experiment.Measurements).String(); got != "[]string" {
		t.Errorf("experiment measurements are %s, want []string", got)
	}

	run := runByID(t, corpus, "fixture-listening-exercise-completed-a")
	if got := reflect.TypeOf(run.Observations).String(); got != "[]domain.RunObservation" {
		t.Errorf("run observations are %s, want []domain.RunObservation", got)
	}
	if got := reflect.TypeOf(run.Measurements).String(); got != "[]domain.RunMeasurement" {
		t.Errorf("run measurements are %s, want []domain.RunMeasurement", got)
	}
}

func TestExperimentRunsParse(t *testing.T) {
	corpus, report := loadCorpus(t, testsupport.MutableCorpus(t))
	if report.HasFatal() {
		t.Fatalf("valid fixture corpus reported fatal issues: %v", report.Fatal())
	}
	if got, want := runIDs(corpus), []string{
		"fixture-listening-exercise-completed-a",
		"fixture-listening-exercise-planned-a",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("run order = %v, want %v", got, want)
	}
	if got, want := experimentIDs(corpus), []string{
		"fixture-listening-exercise", "fixture-visualization-exercise",
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("experiment order = %v, want %v", got, want)
	}
}

// TestPlannedRunIsDistinguishableFromPerformed is the load-bearing distinction of the run layer.
func TestPlannedRunIsDistinguishableFromPerformed(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))

	planned := runByID(t, corpus, "fixture-listening-exercise-planned-a")
	if planned.Performed() {
		t.Error("a planned run reports itself as performed")
	}
	if planned.RunDate != nil {
		t.Errorf("planned run carries a date %q", *planned.RunDate)
	}
	if len(planned.Observations) != 0 || len(planned.Measurements) != 0 {
		t.Error("planned run carries evidence")
	}

	completed := runByID(t, corpus, "fixture-listening-exercise-completed-a")
	if !completed.Performed() {
		t.Error("a completed run reports itself as not performed")
	}
	if completed.RunDate == nil || *completed.RunDate != "1999-04-01" {
		t.Errorf("completed run date = %v, want 1999-04-01", completed.RunDate)
	}
}

// TestObservationAndMeasurementStayDistinct asserts a run's qualitative and quantitative
// evidence keep the fields that make them different kinds of record.
func TestObservationAndMeasurementStayDistinct(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	run := runByID(t, corpus, "fixture-listening-exercise-completed-a")

	if len(run.Observations) != 1 {
		t.Fatalf("observations = %d, want 1", len(run.Observations))
	}
	if run.Observations[0].Context == "" {
		t.Error("observation lost its context; a statement without circumstances is not an observation")
	}

	if len(run.Measurements) != 1 {
		t.Fatalf("measurements = %d, want 1", len(run.Measurements))
	}
	measurement := run.Measurements[0]
	for name, value := range map[string]string{
		"unit":        measurement.Unit,
		"method":      measurement.Method,
		"tool":        measurement.Tool,
		"calibration": measurement.Calibration,
	} {
		if value == "" {
			t.Errorf("measurement lost its %s; a bare number is not measured evidence", name)
		}
	}
	if measurement.Uncertainty != nil {
		t.Errorf("uncertainty = %q, want an explicit null carried as nil", *measurement.Uncertainty)
	}
	if len(measurement.Limitations) == 0 {
		t.Error("a measurement with unknown calibration must keep its limitation")
	}
}

// TestMeasurementValueKeepsAuthoredPrecision covers the reason measurement values are carried as
// the authored token: 72.50 is a significant-figure claim, and re-encoding it as 72.5 would
// silently weaken recorded evidence.
func TestMeasurementValueKeepsAuthoredPrecision(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	run := runByID(t, corpus, "fixture-listening-exercise-completed-a")

	if got, want := run.Measurements[0].Value.String(), "72.50"; got != want {
		t.Errorf("measurement value = %q, want %q", got, want)
	}
	if got, err := run.Measurements[0].Value.Float(); err != nil || got != 72.5 {
		t.Errorf("measurement value as float = %v, %v; want 72.5, nil", got, err)
	}
}

// TestControlSettingKeepsNumberAndStringApart covers the number-or-string contract on a control
// setting, which is a configured input rather than a measurement of anything.
func TestControlSettingKeepsNumberAndStringApart(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	run := runByID(t, corpus, "fixture-listening-exercise-completed-a")

	if len(run.ControlSettings) != 2 {
		t.Fatalf("control settings = %d, want 2", len(run.ControlSettings))
	}
	numeric := run.ControlSettings[0]
	if numeric.Value.IsText() {
		t.Error("a numeric control value decoded as text")
	}
	if got, want := numeric.Value.Number().String(), "440"; got != want {
		t.Errorf("numeric control value = %q, want %q", got, want)
	}
	if numeric.Unit == nil || *numeric.Unit != "Hz" {
		t.Errorf("numeric control unit = %v, want Hz", numeric.Unit)
	}

	textual := run.ControlSettings[1]
	if !textual.Value.IsText() {
		t.Error("a string control value decoded as a number")
	}
	if textual.Unit != nil {
		t.Errorf("unit = %q, want an explicit null carried as nil", *textual.Unit)
	}
}

// TestPracticeLayerIsOptional asserts a corpus that predates these layers still loads. The
// evidence layer already had this property and the practice layer must not remove it.
func TestPracticeLayerIsOptional(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	for path := range corpus {
		switch {
		case path == fixtureVocabularyPath,
			path == fixtureExperimentPath,
			path == "experiments/records/fixture-visualization-exercise.yaml",
			path == fixtureRunPath,
			path == "experiment-runs/records/fixture-listening-exercise-completed-a.yaml":
			delete(corpus, path)
		}
	}
	// The fixture claim set names a vocabulary appearance site, which cannot resolve once the
	// vocabulary layer is gone. Replacing the claims with one that appears only in a node keeps
	// the test about layer absence rather than about a dangling reference.
	testsupport.Write(corpus, fixtureClaimsPath, testsupport.ValidClaim(
		"alpha-carries-energy", "technical_fact", "moderate", "undisputed",
		testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
		testsupport.AppearsInNode("alpha")))

	loaded, report := loadCorpus(t, corpus)
	if report.HasFatal() {
		t.Fatalf("a corpus without the practice layer reported fatal issues: %v", report.Fatal())
	}
	if len(loaded.Vocabulary) != 0 || len(loaded.Experiments) != 0 || len(loaded.ExperimentRuns) != 0 {
		t.Errorf("expected empty practice layers, got %d/%d/%d",
			len(loaded.Vocabulary), len(loaded.Experiments), len(loaded.ExperimentRuns))
	}
}

// TestClaimExperimentRunReferenceResolves covers the Phase 1D upgrade of
// derived_from: experiment_run from a shape check to a real resolution.
func TestClaimExperimentRunReferenceResolves(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, fixtureClaimsPath, testsupport.ValidClaim(
		"run-derived-claim", "technical_fact", "moderate", "undisputed",
		testsupport.SupportedBy("fixture-reference-work"), "[]",
		`[{"kind": "experiment_run", "ref": "fixture-listening-exercise-completed-a"}]`,
		testsupport.AppearsInNode("alpha")))

	_, report := loadCorpus(t, corpus)
	if report.HasFatal() {
		t.Fatalf("a claim derived from a real run reported fatal issues: %v", report.Fatal())
	}
}

func TestPracticeFatalDefects(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(fstest.MapFS)
		wantErr string
	}{
		{
			name: "vocabulary entry declares an unknown field",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						"[]", "[]", "[]", "[]")+"nickname: \"extra\"\n")
			},
			wantErr: domain.CodeUnknownField,
		},
		{
			name: "vocabulary entry omits a required field",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					"---\nid: \"solo-term\"\nterm: \"Solo Term\"\ndomain: \"acoustics\"\n")
			},
			wantErr: domain.CodeMissingField,
		},
		{
			name: "vocabulary id is not canonical kebab-case",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("Solo_Term", "Solo Term", "acoustics",
						"[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidID,
		},
		{
			name: "vocabulary domain is outside the node domain vocabulary",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "invented-domain",
						"[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "two vocabulary entries share an id",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "First Term", "acoustics", "[]", "[]", "[]", "[]")+
						testsupport.ValidVocabularyEntry("solo-term", "Second Term", "acoustics", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeDuplicateID,
		},
		{
			name: "two vocabulary entries share a term in different case",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("first-term", "Shared Term", "acoustics", "[]", "[]", "[]", "[]")+
						testsupport.ValidVocabularyEntry("second-term", "SHARED TERM", "acoustics", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeDuplicateTerm,
		},
		{
			name: "vocabulary node reference does not resolve",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						`["no-such-node"]`, "[]", "[]", "[]"))
			},
			wantErr: domain.CodeUnresolvedTarget,
		},
		{
			name: "vocabulary node reference drifts in case",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						`["Alpha"]`, "[]", "[]", "[]"))
			},
			wantErr: domain.CodeUnresolvedTarget,
		},
		{
			name: "vocabulary session reference names a non-session source",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						"[]", `["fixture-reference-work"]`, "[]", "[]"))
			},
			wantErr: domain.CodeUnresolvedSession,
		},
		{
			name: "vocabulary related term does not resolve",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						"[]", "[]", `["no-such-term"]`, "[]"))
			},
			wantErr: domain.CodeUnresolvedVocabulary,
		},
		{
			name: "vocabulary entry relates to itself",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						"[]", "[]", `["solo-term"]`, "[]"))
			},
			wantErr: domain.CodeSelfReference,
		},
		{
			name: "vocabulary list repeats a reference",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureVocabularyPath,
					testsupport.ValidVocabularyEntry("solo-term", "Solo Term", "acoustics",
						`["alpha", "alpha"]`, "[]", "[]", "[]"))
			},
			wantErr: domain.CodeDuplicateReference,
		},
		{
			name: "experiment status is outside the contract",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureExperimentPath,
					testsupport.ValidExperiment("solo-experiment", "provisional", "listening",
						"introductory", "[]", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "experiment type drifts in case",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureExperimentPath,
					testsupport.ValidExperiment("solo-experiment", "proof", "Listening",
						"introductory", "[]", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "experiment vocabulary reference does not resolve",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureExperimentPath,
					testsupport.ValidExperiment("solo-experiment", "proof", "listening",
						"introductory", "[]", `["no-such-term"]`, "[]", "[]", "[]"))
			},
			wantErr: domain.CodeUnresolvedVocabulary,
		},
		{
			name: "experiment source reference is not registered",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureExperimentPath,
					testsupport.ValidExperiment("solo-experiment", "proof", "listening",
						"introductory", "[]", "[]", "[]", `["no-such-source"]`, "[]"))
			},
			wantErr: domain.CodeUnresolvedSource,
		},
		{
			name: "experiment relates to itself",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureExperimentPath,
					testsupport.ValidExperiment("solo-experiment", "proof", "listening",
						"introductory", "[]", "[]", "[]", "[]", `["solo-experiment"]`))
			},
			wantErr: domain.CodeSelfReference,
		},
		{
			name: "experiment file holds two records",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureExperimentPath,
					testsupport.ValidExperiment("solo-experiment", "proof", "listening",
						"introductory", "[]", "[]", "[]", "[]", "[]")+"---\n"+
						testsupport.ValidExperiment("second-experiment", "proof", "listening",
							"introductory", "[]", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "run names an experiment that does not exist",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "no-such-experiment", "null", "planned", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeUnresolvedExperiment,
		},
		{
			name: "run experiment reference drifts in case",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "Fixture-Listening-Exercise", "null", "planned", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidID,
		},
		{
			name: "run status is outside the contract",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "finished", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "planned run carries observations",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", "null", "planned",
					testsupport.FixtureObservation("It sounded steady."), "[]", "[]", "[]"))
			},
			wantErr: domain.CodeRunLifecycleConflict,
		},
		{
			name: "planned run carries a run date",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "planned", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeRunLifecycleConflict,
		},
		{
			name: "planned run carries interpretation",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", "null", "planned",
					"[]", "[]", `["The effect was clear."]`, "[]"))
			},
			wantErr: domain.CodeRunLifecycleConflict,
		},
		{
			name: "completed run carries no evidence",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "completed", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeRunLifecycleConflict,
		},
		{
			name: "performed run has no run date",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", "null", "completed",
					testsupport.FixtureObservation("It sounded steady."), "[]", "[]", "[]"))
			},
			wantErr: domain.CodeRunLifecycleConflict,
		},
		{
			name: "invalid run carries interpretation",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "invalid",
					testsupport.FixtureObservation("The chain was wrong."), "[]",
					`["The effect was clear."]`, "[]"))
			},
			wantErr: domain.CodeRunLifecycleConflict,
		},
		{
			name: "run date is not an ISO date",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"01/04/1999"`, "incomplete", "[]", "[]", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidRunDate,
		},
		{
			name: "measurement calibration is outside the contract",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "completed", "[]",
					testsupport.FixtureMeasurement("level", "1", "dB", "calibrated", "[]"), "[]", "[]"))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "measurement value is not a JSON number",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "completed", "[]",
					testsupport.FixtureMeasurement("level", `"72.5"`, "dB", "known", "[]"), "[]", "[]"))
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "measurement omits a contract field",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "completed", "[]",
					`[{"quantity": "level", "value": 1, "unit": "dB"}]`, "[]", "[]"))
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "observation omits its context",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", `"1999-04-01"`, "completed",
					`[{"statement": "It sounded steady."}]`, "[]", "[]", "[]"))
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "run source reference is not registered",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureRunPath, testsupport.ValidExperimentRun(
					"solo-run", "fixture-listening-exercise", "null", "planned",
					"[]", "[]", "[]", `["no-such-source"]`))
			},
			wantErr: domain.CodeUnresolvedSource,
		},
		{
			name: "claim derives from an experiment run that does not exist",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, fixtureClaimsPath, testsupport.ValidClaim(
					"run-derived-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]",
					`[{"kind": "experiment_run", "ref": "no-such-run"}]`,
					testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeUnresolvedRun,
		},
		{
			name: "experiment contract declares no enum for a served filter",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "schemas/experiment.schema.yaml",
					"schema: audiomuse-experiment\nversion: 1\nproperties:\n  id: { type: string }\n")
			},
			wantErr: domain.CodeMalformedRecord,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corpus := testsupport.MutableCorpus(t)
			tc.mutate(corpus)
			_, report := loadCorpus(t, corpus)

			for _, issue := range report.Fatal() {
				if issue.Code == tc.wantErr {
					return
				}
			}
			t.Fatalf("want a fatal %s; got %v", tc.wantErr, report.Fatal())
		})
	}
}

// TestPracticeLoadIsDeterministic loads the same corpus twice and compares the practice layer
// in full, so no ordering can depend on directory traversal or Go map iteration.
func TestPracticeLoadIsDeterministic(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	first, firstReport := loadCorpus(t, corpus)
	second, secondReport := loadCorpus(t, corpus)

	if !reflect.DeepEqual(first.Vocabulary, second.Vocabulary) {
		t.Error("two loads produced different vocabulary projections")
	}
	if !reflect.DeepEqual(first.Experiments, second.Experiments) {
		t.Error("two loads produced different experiment projections")
	}
	if !reflect.DeepEqual(first.ExperimentRuns, second.ExperimentRuns) {
		t.Error("two loads produced different experiment-run projections")
	}
	if !reflect.DeepEqual(firstReport.Issues, secondReport.Issues) {
		t.Error("two loads produced different validation reports")
	}
}
