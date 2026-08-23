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

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
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

// TestLiveCorpusKeepsThePracticeLayerOutOfTheGraph is the cross-phase semantic lock, checked
// against the real corpus rather than the fixture.
//
// The fixture carries seven practice records; the canonical corpus carries an order of
// magnitude more, across 78 nodes and 48 claims whose reference shapes were authored by hand
// rather than constructed to exercise a code path. If loading the practice layer could pull a
// vocabulary entry or an experiment run into the traversal adjacency, the corpus is where it
// would happen first — some real claim naming a real vocabulary entry in appears_in.
//
// The assertion is over the whole adjacency: every entity reachable from every root, at the
// maximum documented depth, must be one of the four addressable classes, and no ID belonging
// to the practice layer may appear on either end of a relationship.
func TestLiveCorpusKeepsThePracticeLayerOutOfTheGraph(t *testing.T) {
	root := liveRepoRoot(t)

	repo, err := filesystem.New(root)
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	corpus, _, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load canonical repository: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	practice := map[string]string{}
	for _, entry := range corpus.Vocabulary {
		practice[entry.ID] = "vocabulary entry"
	}
	for _, experiment := range corpus.Experiments {
		practice[experiment.ID] = "experiment"
	}
	for _, run := range corpus.ExperimentRuns {
		practice[run.ID] = "experiment run"
	}
	if len(practice) == 0 {
		t.Fatal("no practice records loaded; this test would prove nothing")
	}

	// A canonical ID may legitimately be shared between a node and a vocabulary entry naming
	// the same concept, so an ID collision is not by itself a leak. Only IDs unique to the
	// practice layer can be used as evidence here.
	for _, node := range corpus.Nodes {
		delete(practice, node.ID)
	}
	for _, session := range corpus.Sessions {
		delete(practice, session.ID)
	}
	for _, claim := range corpus.Claims {
		delete(practice, claim.ID)
	}
	for _, source := range corpus.Sources {
		delete(practice, source.ID)
	}
	if len(practice) == 0 {
		t.Fatal("every practice ID is also a graph record ID; this test can no longer discriminate")
	}

	roots := make([][2]string, 0, len(corpus.Nodes)+len(corpus.Sessions)+len(corpus.Claims)+len(corpus.Sources))
	for _, node := range corpus.Nodes {
		roots = append(roots, [2]string{"node", node.ID})
	}
	for _, session := range corpus.Sessions {
		roots = append(roots, [2]string{"session", session.ID})
	}
	for _, claim := range corpus.Claims {
		roots = append(roots, [2]string{"claim", claim.ID})
	}
	for _, source := range corpus.Sources {
		roots = append(roots, [2]string{"source", source.ID})
	}

	entitiesSeen := 0
	for _, r := range roots {
		entityType, id := r[0], r[1]
		result, err := knowledge.Traverse(entityType, id, service.TraversalQuery{Depth: 3})
		if err != nil {
			t.Fatalf("traverse %s/%s: %v", entityType, id, err)
		}
		entitiesSeen += len(result.Entities)
		for _, e := range result.Entities {
			if kind, ok := practice[e.ID]; ok {
				t.Errorf("%s %q surfaced as a traversal entity from %s/%s", kind, e.ID, entityType, id)
			}
			if !domain.ValidEntityType(string(e.Type)) {
				t.Errorf("entity %q from %s/%s has non-canonical type %q", e.ID, entityType, id, e.Type)
			}
		}
		for _, edge := range result.Relationships {
			if kind, ok := practice[edge.From.ID]; ok {
				t.Errorf("%s %q appeared as an edge source from %s/%s", kind, edge.From.ID, entityType, id)
			}
			if kind, ok := practice[edge.To.ID]; ok {
				t.Errorf("%s %q appeared as an edge target from %s/%s", kind, edge.To.ID, entityType, id)
			}
		}
	}

	if entitiesSeen == 0 {
		t.Fatal("traversal returned no entities across the whole corpus")
	}
	t.Logf("canonical graph isolation: %d roots walked at depth 3, %d entity results, "+
		"%d practice-only IDs none of which appeared", len(roots), entitiesSeen, len(practice))
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
		// The traversal routes are exercised here too: they walk the whole adjacency and are
		// the most expansive read the API offers, so if any read surface were to touch the
		// corpus it would be this one.
		"/api/v1/graph/entities/node/frequency/relationships",
		"/api/v1/graph/entities/node/frequency/traverse?depth=3",
		"/api/v1/vocabulary",
		"/api/v1/vocabulary?q=resonance",
		"/api/v1/experiments",
		"/api/v1/experiment-runs",
		"/api/v1/experiment-runs?performed=false",
		// The discovery route scans every search document in the projection, so it is the
		// broadest single read after traversal and the one most worth proving inert.
		"/api/v1/search?q=resonance",
		"/api/v1/search?q=e&limit=200",
		"/api/v1/search?q=resonance&type=vocabulary",
		// Context resolution reads the adjacency and every practice reference list on top of
		// the search scan, so it is the broadest read the API performs in one request.
		"/api/v1/search?q=resonance&include_context=true",
		"/api/v1/search?q=e&limit=200&include_context=true",
		// Multi-term composition runs one substring pass per term over the same projection, so
		// it is the most work a single discovery request can do, and the combination below —
		// every term, every document, then context resolution over the returned page — is the
		// broadest read the API performs at all.
		"/api/v1/search?q=resonance+frequency&query_mode=all_terms",
		"/api/v1/search?q=resonance+frequency&query_mode=all_terms&include_context=true",
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
