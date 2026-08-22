package filesystem_test

import (
	"context"
	"crypto/sha256"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/httpapi"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// liveRepoRoot locates the canonical AudioMuse repository containing this backend, or
// skips. Skipping keeps the suite runnable from a copy of backend/ on its own; every other
// test in the package runs against the synthetic fixture and does not need the real corpus.
func liveRepoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "sources", "source-registry.yaml")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "schemas", "node.schema.yaml")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("canonical AudioMuse repository not found from the test working directory")
		}
		dir = parent
	}
}

// TestLiveRepositoryLoadsCleanly is the repository regression check: the backend must be
// able to project the real corpus without a single fatal issue.
func TestLiveRepositoryLoadsCleanly(t *testing.T) {
	root := liveRepoRoot(t)

	repo, err := filesystem.New(root)
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	corpus, report, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load canonical repository: %v", err)
	}
	if report.HasFatal() {
		t.Fatalf("canonical repository reported fatal issues: %v", report.Fatal())
	}

	if len(corpus.Nodes) == 0 {
		t.Fatal("no canonical nodes were loaded")
	}
	if len(corpus.Sessions) == 0 {
		t.Fatal("no canonical sessions were derived")
	}
	edges := 0
	for _, node := range corpus.Nodes {
		edges += len(node.Relationships)
	}
	if edges == 0 {
		t.Fatal("no canonical relationships were loaded")
	}
	t.Logf("canonical corpus: nodes=%d sessions=%d sources=%d edges=%d validation=%s warnings=%d",
		len(corpus.Nodes), len(corpus.Sessions), len(corpus.Sources), edges,
		report.Status(), len(report.Warnings()))
}

// TestLiveRepositoryIsNotMutated asserts the read-only guarantee directly rather than by
// inspection: every canonical file's size, modification time, and content digest must be
// unchanged by a full load. This does not depend on git, and the digest catches a same-size
// rewrite even if its timestamp is preserved or the filesystem clock is coarse.
func TestLiveRepositoryIsNotMutated(t *testing.T) {
	root := liveRepoRoot(t)

	before := snapshot(t, root)

	repo, err := filesystem.New(root)
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	if _, _, err := repo.Load(context.Background()); err != nil {
		t.Fatalf("load canonical repository: %v", err)
	}

	after := snapshot(t, root)
	if !reflect.DeepEqual(before, after) {
		for path, state := range after {
			if prior, ok := before[path]; !ok {
				t.Errorf("load created %s", path)
			} else if prior != state {
				t.Errorf("load modified %s", path)
			}
		}
		for path := range before {
			if _, ok := after[path]; !ok {
				t.Errorf("load removed %s", path)
			}
		}
		t.Fatal("loading the corpus changed the repository working tree")
	}
}

type fileState struct {
	size    int64
	modTime int64
	digest  [sha256.Size]byte
}

// snapshot records metadata and a content digest for every canonical record file.
func snapshot(t testing.TB, root string) map[string]fileState {
	t.Helper()
	out := map[string]fileState{}
	for _, dir := range []string{"nodes", "sessions", "sources", "schemas", "claims", "experiments", "experiment-runs", "vocabulary", "indexes"} {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(rel)] = fileState{
				size: info.Size(), modTime: info.ModTime().UnixNano(), digest: sha256.Sum256(content),
			}
			return nil
		})
		if err != nil {
			t.Fatalf("snapshot %s: %v", dir, err)
		}
	}
	if len(out) == 0 {
		t.Fatal("snapshot captured no canonical files")
	}
	return out
}

// TestLiveRepositoryProjectsTheEvidenceLayer is the Phase 1B repository regression check:
// the real claim records must parse, their vocabularies must come from the canonical
// contracts, and every evidence reference must resolve with no fatal issue.
func TestLiveRepositoryProjectsTheEvidenceLayer(t *testing.T) {
	root := liveRepoRoot(t)

	repo, err := filesystem.New(root)
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	corpus, report, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load canonical repository: %v", err)
	}
	if report.HasFatal() {
		t.Fatalf("canonical evidence layer reported fatal issues: %v", report.Fatal())
	}

	if len(corpus.Claims) == 0 {
		t.Fatal("no canonical claims were loaded")
	}
	if len(corpus.Vocabularies.Claim.ClaimTypes) == 0 || len(corpus.Vocabularies.Source.Types) == 0 {
		t.Fatal("canonical contract vocabularies were not read")
	}

	evidence, attribution, appearances := 0, 0, 0
	confidences := map[string]int{}
	for _, claim := range corpus.Claims {
		evidence += len(claim.Evidence)
		attribution += len(claim.Attribution)
		appearances += len(claim.AppearsIn)
		confidences[claim.Confidence]++
	}
	if evidence == 0 || appearances == 0 {
		t.Fatal("canonical claims carry no evidence or appearance sites")
	}
	t.Logf("canonical evidence layer: claims=%d evidence=%d attribution=%d appearances=%d confidence=%v",
		len(corpus.Claims), evidence, attribution, appearances, confidences)
}

// TestLiveRepositoryProjectsThePracticeLayer is the Phase 1D repository regression check: the
// real vocabulary, experiment and experiment-run records must load, resolve and keep the
// planned/performed and observation/measurement distinctions intact.
func TestLiveRepositoryProjectsThePracticeLayer(t *testing.T) {
	root := liveRepoRoot(t)

	repo, err := filesystem.New(root)
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	corpus, report, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load canonical repository: %v", err)
	}
	if report.HasFatal() {
		t.Fatalf("canonical practice layer reported fatal issues: %v", report.Fatal())
	}

	if len(corpus.Vocabulary) == 0 {
		t.Fatal("no canonical vocabulary entries were loaded")
	}
	if len(corpus.Experiments) == 0 {
		t.Fatal("no canonical experiments were loaded")
	}
	if len(corpus.ExperimentRuns) == 0 {
		t.Fatal("no canonical experiment runs were loaded")
	}
	if len(corpus.Vocabularies.Experiment.Types) == 0 ||
		len(corpus.Vocabularies.ExperimentRun.Calibrations) == 0 {
		t.Fatal("canonical practice contract vocabularies were not read")
	}

	nodeRefs, sessionRefs, relatedTerms := 0, 0, 0
	for _, entry := range corpus.Vocabulary {
		nodeRefs += len(entry.NodeRefs)
		sessionRefs += len(entry.SessionRefs)
		relatedTerms += len(entry.RelatedTerms)
	}
	if relatedTerms == 0 {
		t.Fatal("canonical vocabulary declares no related terms")
	}

	statuses := map[string]int{}
	observations, measurements := 0, 0
	for _, run := range corpus.ExperimentRuns {
		statuses[run.Status]++
		observations += len(run.Observations)
		measurements += len(run.Measurements)
		if !run.Performed() && run.RunDate != nil {
			t.Errorf("run %s is planned but carries a date", run.ID)
		}
	}

	t.Logf("canonical practice layer: vocabulary=%d node_refs=%d session_refs=%d related_terms=%d "+
		"experiments=%d runs=%d statuses=%v observations=%d measurements=%d",
		len(corpus.Vocabulary), nodeRefs, sessionRefs, relatedTerms,
		len(corpus.Experiments), len(corpus.ExperimentRuns), statuses, observations, measurements)
}

// TestLiveRepositoryIsNotMutatedByAPIRequests extends the read-only proof past startup.
//
// TestLiveRepositoryIsNotMutated covers the load. This covers what a running process does after
// it: build the index, serve one representative request against every read surface, and assert
// the canonical tree is byte-for-byte unchanged. It is the direct answer to "does serving the
// API write anything", which inspection of the code can suggest but only a digest can settle.
func TestLiveRepositoryIsNotMutatedByAPIRequests(t *testing.T) {
	root := liveRepoRoot(t)
	before := snapshot(t, root)

	repo, err := filesystem.New(root)
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	handler := httpapi.NewServer(knowledge, slog.New(slog.NewTextHandler(io.Discard, nil)))

	targets := []string{
		"/health",
		"/api/v1/project",
		"/api/v1/diagnostics",
		"/api/v1/nodes",
		"/api/v1/sessions",
		"/api/v1/sources",
		"/api/v1/claims",
		"/api/v1/graph",
		"/api/v1/vocabulary",
		"/api/v1/vocabulary?q=resonance",
		"/api/v1/experiments",
		"/api/v1/experiment-runs",
		"/api/v1/experiment-runs?performed=false",
	}
	for _, target := range targets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", target, rec.Code)
		}
	}

	// A rejected request must not write either, so the mutating methods are exercised too.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/experiment-runs", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/v1/experiment-runs = %d, want 405", method, rec.Code)
		}
	}

	after := snapshot(t, root)
	if !reflect.DeepEqual(before, after) {
		for path, state := range after {
			if prior, ok := before[path]; !ok {
				t.Errorf("serving the API created %s", path)
			} else if prior != state {
				t.Errorf("serving the API modified %s", path)
			}
		}
		for path := range before {
			if _, ok := after[path]; !ok {
				t.Errorf("serving the API removed %s", path)
			}
		}
		t.Fatal("serving the read-only API changed the repository working tree")
	}
}
