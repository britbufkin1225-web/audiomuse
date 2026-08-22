package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

func TestVocabularyRoutes(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/vocabulary")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var list service.VocabularyList
	decode(t, rec, &list)
	if got, want := list.Page.Total, 3; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := list.Vocabulary[0].ID, "fixture-companion"; got != want {
		t.Errorf("first entry = %q, want %q (canonical ID order)", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/vocabulary?domain=dsp&q=companion")
	decode(t, rec, &list)
	if got, want := list.Page.Total, 1; got != want {
		t.Errorf("AND-composed total = %d, want %d", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/vocabulary/fixture-term")
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", rec.Code)
	}
	var detail domain.VocabularyDetail
	decode(t, rec, &detail)
	if detail.Term != "Fixture Term" {
		t.Errorf("term = %q", detail.Term)
	}
	if len(detail.ExperimentIDs) != 1 || len(detail.ClaimIDs) != 1 {
		t.Errorf("derived reverse views = %+v", detail)
	}
}

func TestExperimentRoutes(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/experiments")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var list service.ExperimentList
	decode(t, rec, &list)
	if got, want := list.Page.Total, 2; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/experiments?type=listening&node_id=beta")
	decode(t, rec, &list)
	if got, want := list.Page.Total, 1; got != want {
		t.Errorf("AND-composed total = %d, want %d", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/experiments/fixture-listening-exercise")
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", rec.Code)
	}
	var detail domain.ExperimentDetail
	decode(t, rec, &detail)
	if detail.Runs.Total != 2 || detail.Runs.Planned != 1 || detail.Runs.Completed != 1 {
		t.Errorf("run tally = %+v", detail.Runs)
	}
	if len(detail.RunIDs) != 2 {
		t.Errorf("run_ids = %v", detail.RunIDs)
	}
}

func TestExperimentRunRoutes(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/experiment-runs")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var list service.ExperimentRunList
	decode(t, rec, &list)
	if got, want := list.Page.Total, 2; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/experiment-runs?experiment_id=fixture-listening-exercise&performed=false")
	decode(t, rec, &list)
	if got, want := list.Page.Total, 1; got != want {
		t.Fatalf("planned-only total = %d, want %d", got, want)
	}
	if got, want := list.Runs[0].ID, "fixture-listening-exercise-planned-a"; got != want {
		t.Errorf("planned run = %q, want %q", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/experiment-runs/fixture-listening-exercise-completed-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", rec.Code)
	}
	var detail domain.ExperimentRunDetail
	decode(t, rec, &detail)
	if !detail.Performed {
		t.Error("completed run reported as not performed")
	}
	if detail.ExperimentTitle == "" {
		t.Error("run detail did not name the definition it executes")
	}
}

// TestRunEvidenceSerialisesWithItsContext asserts the JSON a client actually receives keeps
// observations and measurements apart and keeps every field that makes a number evidence.
func TestRunEvidenceSerialisesWithItsContext(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet,
		"/api/v1/experiment-runs/fixture-listening-exercise-completed-a")

	var body map[string]any
	decode(t, rec, &body)

	observations, ok := body["observations"].([]any)
	if !ok || len(observations) != 1 {
		t.Fatalf("observations = %#v", body["observations"])
	}
	observation := observations[0].(map[string]any)
	if _, present := observation["context"]; !present {
		t.Error("serialised observation lost its context")
	}
	for _, absent := range []string{"value", "unit", "method", "tool", "calibration"} {
		if _, present := observation[absent]; present {
			t.Errorf("serialised observation carries %q; an observation measures nothing", absent)
		}
	}

	measurements, ok := body["measurements"].([]any)
	if !ok || len(measurements) != 1 {
		t.Fatalf("measurements = %#v", body["measurements"])
	}
	measurement := measurements[0].(map[string]any)
	for _, required := range []string{"quantity", "value", "unit", "method", "tool", "calibration", "uncertainty", "limitations"} {
		if _, present := measurement[required]; !present {
			t.Errorf("serialised measurement lost %q", required)
		}
	}
	if measurement["uncertainty"] != nil {
		t.Errorf("uncertainty = %v, want an explicit null", measurement["uncertainty"])
	}
}

// TestMeasurementValueIsServedAsTheAuthoredNumber checks the raw response bytes rather than a
// decoded value, because Go's JSON decoder would collapse 72.50 to 72.5 and hide the very thing
// the CanonicalNumber type exists to preserve.
func TestMeasurementValueIsServedAsTheAuthoredNumber(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet,
		"/api/v1/experiment-runs/fixture-listening-exercise-completed-a")

	body := rec.Body.String()
	if !strings.Contains(body, `"value":72.50`) {
		t.Errorf("response does not carry the authored measurement value 72.50: %s", body)
	}
	if !strings.Contains(body, `"value":440`) {
		t.Errorf("response does not carry the authored numeric control value 440: %s", body)
	}
	if !strings.Contains(body, `"value":"position B"`) {
		t.Errorf("response does not carry the authored string control value: %s", body)
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Error("response is not valid JSON")
	}
}

// TestPlannedRunSerialisesAsNotPerformed asserts the JSON keeps the planned/performed split
// visible without a client having to interpret the status enum itself.
func TestPlannedRunSerialisesAsNotPerformed(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet,
		"/api/v1/experiment-runs/fixture-listening-exercise-planned-a")

	var body map[string]any
	decode(t, rec, &body)
	if body["performed"] != false {
		t.Errorf("performed = %v, want false", body["performed"])
	}
	if body["run_date"] != nil {
		t.Errorf("run_date = %v, want null", body["run_date"])
	}
	if got := body["observations"].([]any); len(got) != 0 {
		t.Errorf("planned run serialised observations: %v", got)
	}
}

func TestPracticeNotFoundContracts(t *testing.T) {
	handler := newHandler(t)
	cases := []struct{ target, code string }{
		{"/api/v1/vocabulary/no-such-term", "vocabulary_not_found"},
		{"/api/v1/experiments/no-such-experiment", "experiment_not_found"},
		{"/api/v1/experiment-runs/no-such-run", "experiment_run_not_found"},
		// Canonical identity is case-sensitive, so a case-drifted ID is a miss, not a match.
		{"/api/v1/vocabulary/Fixture-Term", "vocabulary_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, tc.target)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			assertErrorCode(t, rec, tc.code)
		})
	}
}

func TestPracticeInvalidQueries(t *testing.T) {
	handler := newHandler(t)
	cases := []string{
		// Unsupported parameters are refused rather than ignored.
		"/api/v1/vocabulary?related_term=fixture-term",
		"/api/v1/experiments?equipment=oscillator",
		"/api/v1/experiment-runs?q=steady",
		// Duplicate parameters are refused: which value applied would otherwise be arbitrary.
		"/api/v1/vocabulary?domain=dsp&domain=acoustics",
		"/api/v1/experiments?type=listening&type=hybrid",
		"/api/v1/experiment-runs?status=planned&status=completed",
		// Bounded filter values outside the contract.
		"/api/v1/experiments?status=provisional",
		"/api/v1/experiments?type=measurement",
		"/api/v1/experiments?difficulty=expert",
		"/api/v1/experiment-runs?status=finished",
		// The performed filter is strictly true or false.
		"/api/v1/experiment-runs?performed=1",
		"/api/v1/experiment-runs?performed=yes",
		"/api/v1/experiment-runs?performed=TRUE",
		// Paging bounds.
		"/api/v1/vocabulary?limit=abc",
		"/api/v1/vocabulary?offset=-1",
		// A detail route accepts no query string at all.
		"/api/v1/experiments/fixture-listening-exercise?type=listening",
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			assertErrorCode(t, rec, "invalid_query")
		})
	}
}

// TestRejectedPracticeFilterNamesTheContractValues asserts the 400 body lists the accepted
// values, which come from the canonical contract and are therefore repository data, and does not
// echo the caller's own value back.
func TestRejectedPracticeFilterNamesTheContractValues(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/experiments?status=provisional")

	body := rec.Body.String()
	for _, want := range []string{"proof", "established"} {
		if !strings.Contains(body, want) {
			t.Errorf("rejection does not name the accepted value %q: %s", want, body)
		}
	}
	if strings.Contains(body, "provisional") {
		t.Errorf("rejection echoes the caller's own value: %s", body)
	}
}

// TestPracticeIdentifiersCannotBecomeFilesystemPaths asserts no practice route can be turned
// into a file reader. The IDs below are only ever map keys; they are never joined to a path.
func TestPracticeIdentifiersCannotBecomeFilesystemPaths(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{
		"/api/v1/vocabulary/..%2f..%2fschemas%2fnode.schema.yaml",
		"/api/v1/experiments/..%5c..%5cwindows%5csystem32",
		"/api/v1/experiment-runs/fixture..listening",
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 400 or 404", rec.Code)
			}
			for _, leak := range []string{"schema: audiomuse", "C:\\", "/home/", "testdata"} {
				if strings.Contains(rec.Body.String(), leak) {
					t.Errorf("response leaked %q: %s", leak, rec.Body.String())
				}
			}
		})
	}
}

func TestPracticeRoutesRejectMutation(t *testing.T) {
	handler := newHandler(t)
	targets := []string{
		"/api/v1/vocabulary",
		"/api/v1/vocabulary/fixture-term",
		"/api/v1/experiments",
		"/api/v1/experiments/fixture-listening-exercise",
		"/api/v1/experiment-runs",
		"/api/v1/experiment-runs/fixture-listening-exercise-planned-a",
	}
	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

	for _, target := range targets {
		for _, method := range methods {
			t.Run(method+" "+target, func(t *testing.T) {
				rec := do(t, handler, method, target)
				if rec.Code != http.StatusMethodNotAllowed {
					t.Fatalf("status = %d, want 405", rec.Code)
				}
				if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
					t.Errorf("Allow = %q, want %q", got, want)
				}
				assertErrorCode(t, rec, "method_not_allowed")
			})
		}
	}
}

// TestPracticeResponsesAreByteIdentical is the determinism contract at the HTTP boundary: two
// identical requests against an unchanged corpus must produce the same bytes, not merely the
// same set of records.
func TestPracticeResponsesAreByteIdentical(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{
		"/api/v1/vocabulary",
		"/api/v1/vocabulary?domain=acoustics",
		"/api/v1/vocabulary/fixture-term",
		"/api/v1/experiments",
		"/api/v1/experiments/fixture-listening-exercise",
		"/api/v1/experiment-runs",
		"/api/v1/experiment-runs/fixture-listening-exercise-completed-a",
		"/api/v1/project",
		"/api/v1/diagnostics",
	} {
		t.Run(target, func(t *testing.T) {
			first := do(t, handler, http.MethodGet, target).Body.String()
			second := do(t, handler, http.MethodGet, target).Body.String()
			if first != second {
				t.Errorf("two identical requests returned different bytes:\n%s\n%s", first, second)
			}
		})
	}
}

// TestSeparateIndexesServeIdenticalPracticeBytes goes further than the same-handler check: two
// indexes built independently from an unchanged corpus must also serialise identically, which
// is what rules out Go map iteration order reaching the wire.
func TestSeparateIndexesServeIdenticalPracticeBytes(t *testing.T) {
	first, second := newHandler(t), newHandler(t)
	for _, target := range []string{
		"/api/v1/vocabulary",
		"/api/v1/experiments",
		"/api/v1/experiment-runs",
		"/api/v1/vocabulary/fixture-term",
	} {
		t.Run(target, func(t *testing.T) {
			a := do(t, first, http.MethodGet, target).Body.String()
			b := do(t, second, http.MethodGet, target).Body.String()
			if a != b {
				t.Errorf("two indexes served different bytes:\n%s\n%s", a, b)
			}
		})
	}
}

func TestProjectAndDiagnosticsExposeThePracticeLayer(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/project")
	var project service.ProjectSummary
	decode(t, rec, &project)
	if project.Counts.Vocabulary != 3 || project.Counts.Experiments != 2 || project.Counts.ExperimentRuns != 2 {
		t.Errorf("counts = %+v", project.Counts)
	}
	if project.ExperimentRuns.Planned != 1 || project.ExperimentRuns.Completed != 1 {
		t.Errorf("run totals = %+v", project.ExperimentRuns)
	}
	if len(project.Vocabulary.Experiment.Types) == 0 {
		t.Error("project does not publish the experiment contract vocabulary")
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/diagnostics")
	var diagnostics service.Diagnostics
	decode(t, rec, &diagnostics)
	if diagnostics.Corpus.Vocabulary != 3 || diagnostics.Corpus.Experiments != 2 ||
		diagnostics.Corpus.ExperimentRuns != 2 {
		t.Errorf("diagnostics corpus = %+v", diagnostics.Corpus)
	}
}
