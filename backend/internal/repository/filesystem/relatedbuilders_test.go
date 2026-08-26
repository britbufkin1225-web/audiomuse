package filesystem_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// Phase 2C: the related-knowledge contract against every canonical builder the repository ships.
//
// AudioMuse builds generated views of the corpus with the PowerShell builders under tools/. They
// are the repository's own cross-check: a builder reads the canonical records and writes an index
// a human can read, and `git diff` after a rebuild is what proves the canonical records and the
// generated view still agree. That check only means something while the generated view is not
// itself an input, and the backend documentation states that it never is - generated indexes under
// vocabulary/, experiments/, experiment-runs/, claims/ and indexes/ are never read.
//
// "Never read" is invisible in code. It is the absence of a path that nobody opens, which is
// exactly the kind of guarantee that is true until somebody adds a convenient shortcut. The tests
// here make it checkable from both directions: a builder's output cannot change what discovery
// answers, and a discovery request cannot change a builder's output.
//
// The builder inventory is derived from the repository rather than restated from a phase document,
// so a builder added later fails this file until somebody decides what it means for discovery.

// canonicalBuilders maps each builder script to the artifacts it writes under its default
// arguments, read from the scripts themselves at the time this phase was written.
//
// It is compared against the actual tools/build-*.ps1 inventory below rather than trusted, which is
// what makes it a checked statement about the repository rather than a copy of one.
var canonicalBuilders = map[string][]string{
	"build-claim-index.ps1":          {"claims/index.md"},
	"build-experiment-index.ps1":     {"experiments/index.md"},
	"build-experiment-run-index.ps1": {"experiment-runs/index.md"},
	"build-knowledge-coverage.ps1": {
		"indexes/knowledge-coverage.json",
		"indexes/knowledge-coverage.md",
	},
	"build-knowledge-index.ps1": {
		"indexes/README.md",
		"indexes/node-connections.md",
		"indexes/nodes-by-domain.md",
		"indexes/relationships-by-type.md",
		"indexes/session-coverage.md",
		"indexes/source-coverage.md",
	},
	"build-vocabulary-index.ps1": {"vocabulary/index.md"},
}

// repositoryBuilders lists the builder scripts the repository actually contains.
func repositoryBuilders(t testing.TB, root string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "tools", "build-*.ps1"))
	if err != nil {
		t.Fatalf("enumerate builders: %v", err)
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, filepath.Base(match))
	}
	sort.Strings(out)
	return out
}

// generatedArtifacts is every path the declared builders write, sorted.
func generatedArtifacts() []string {
	var out []string
	for _, paths := range canonicalBuilders {
		out = append(out, paths...)
	}
	sort.Strings(out)
	return out
}

// TestCanonicalBuilderInventoryIsCurrent requires the builder set this file reasons about to be the
// builder set the repository holds.
//
// It is the guard that keeps every other test here honest. A builder added to tools/ without being
// classified would otherwise leave its output silently outside the checks below, which is the same
// failure mode the precedence table's own coverage test exists to prevent for canonical fields.
func TestCanonicalBuilderInventoryIsCurrent(t *testing.T) {
	root := liveRepoRoot(t)

	found := repositoryBuilders(t, root)
	if len(found) == 0 {
		t.Fatal("the repository declares no builders, so this file is checking nothing")
	}
	declared := make([]string, 0, len(canonicalBuilders))
	for name := range canonicalBuilders {
		declared = append(declared, name)
	}
	sort.Strings(declared)

	if strings.Join(found, ",") != strings.Join(declared, ",") {
		t.Fatalf("builder inventory has changed:\n repository %v\n declared   %v", found, declared)
	}
	for name, artifacts := range canonicalBuilders {
		if len(artifacts) == 0 {
			t.Errorf("%s declares no generated artifact", name)
		}
		for _, artifact := range artifacts {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(artifact))); err != nil {
				t.Errorf("%s: generated artifact %s is not present: %v", name, artifact, err)
			}
		}
	}
}

// TestRelatedKnowledgeIgnoresEveryBuilderArtifact requires discovery to answer identically with and
// without the generated views present.
//
// The corpus is loaded twice: once as the repository stands, and once from an in-memory copy with
// every artifact the declared builders write removed. Every discovery start the repository offers
// is then swept, unfiltered and once per precedence class, and the two indexes must serialise the
// same bytes for all of it.
//
// Removing the artifacts rather than corrupting them is deliberate. A corrupted generated file
// would prove only that the loader does not parse it; an absent one proves that nothing in the
// discovery path needed it, which is the stronger statement and the one the documentation makes.
func TestRelatedKnowledgeIgnoresEveryBuilderArtifact(t *testing.T) {
	root := liveRepoRoot(t)

	withArtifacts := liveIndex(t)
	withoutArtifacts := indexWithoutGeneratedArtifacts(t, root)

	classes := []domain.SearchEntityType{
		domain.SearchSession, domain.SearchNode, domain.SearchClaim,
		domain.SearchSource, domain.SearchVocabulary, domain.SearchExperiment,
	}
	queries := []service.RelatedQuery{{Limit: service.MaxRelatedLimit}}
	for _, class := range domain.RelatedPriorities {
		queries = append(queries, service.RelatedQuery{
			Limit:             service.MaxRelatedLimit,
			RelationshipTypes: []string{string(class)},
		})
	}

	swept := 0
	for _, class := range classes {
		for _, id := range liveRelatedStarts(t, withArtifacts, class) {
			for _, query := range queries {
				want := liveRelatedJSON(t, withArtifacts, class, id, query)
				got := liveRelatedJSON(t, withoutArtifacts, class, id, query)
				if want != got {
					t.Fatalf("%s/%s %+v differs once the generated views are absent\n want %s\n  got %s",
						class, id, query, want, got)
				}
				swept++
			}
		}
	}
	if swept == 0 {
		t.Fatal("the sweep covered no discovery start")
	}
	t.Logf("compared %d discovery results across two indexes of the canonical repository", swept)
}

// TestBuilderArtifactsAreNotWrittenByDiscovery is the read-only proof aimed at the generated views
// specifically.
//
// The existing live tests snapshot the canonical record files across a request sweep. Generated
// artifacts are the files a writing backend would most plausibly touch, since they are the ones the
// repository itself regenerates, so they get their own snapshot: size, modification time and
// content digest, either side of a discovery sweep over every start and every relationship scope,
// plus the refusals a hostile caller would send.
func TestBuilderArtifactsAreNotWrittenByDiscovery(t *testing.T) {
	root := liveRepoRoot(t)
	artifacts := generatedArtifacts()

	before := artifactSnapshot(t, root, artifacts)
	k := liveIndex(t)

	classes := []domain.SearchEntityType{
		domain.SearchSession, domain.SearchNode, domain.SearchClaim,
		domain.SearchSource, domain.SearchVocabulary, domain.SearchExperiment,
	}
	for _, class := range classes {
		for _, id := range liveRelatedStarts(t, k, class) {
			if _, err := k.RelatedKnowledgeFor(string(class), id, service.RelatedQuery{Limit: service.MaxRelatedLimit}); err != nil {
				t.Fatalf("related %s/%s: %v", class, id, err)
			}
			for _, priority := range domain.RelatedPriorities {
				if _, err := k.RelatedKnowledgeFor(string(class), id, service.RelatedQuery{
					RelationshipTypes: []string{string(priority)},
				}); err != nil {
					t.Fatalf("related %s/%s scoped to %s: %v", class, id, priority, err)
				}
			}
		}
	}
	// The refusals too: a rejected request must be as read-only as an accepted one.
	for _, query := range []service.RelatedQuery{
		{RelationshipTypes: []string{"structural"}},
		{RelationshipTypes: []string{""}},
		{EntityTypes: []string{"experiment_run"}},
	} {
		if _, err := k.RelatedKnowledgeFor("node", "no-such-node", query); err == nil {
			t.Errorf("%+v was accepted against a missing start", query)
		}
	}

	after := artifactSnapshot(t, root, artifacts)
	for path, state := range before {
		other, present := after[path]
		if !present {
			t.Errorf("generated artifact %s disappeared during a discovery sweep", path)
			continue
		}
		if state != other {
			t.Errorf("generated artifact %s changed during a discovery sweep", path)
		}
	}
	if len(after) != len(before) {
		t.Errorf("the generated artifact set changed size: %d before, %d after", len(before), len(after))
	}
}

// artifactSnapshot records size, modification time and content digest for the named paths.
//
// It reuses the fileState shape the Phase 1A live tests already snapshot canonical records with, so
// "unchanged" means the same three things everywhere in this package.
func artifactSnapshot(t testing.TB, root string, paths []string) map[string]fileState {
	t.Helper()
	out := map[string]fileState{}
	for _, rel := range paths {
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err != nil {
			t.Fatalf("stat generated artifact %s: %v", rel, err)
		}
		content, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("read generated artifact %s: %v", rel, err)
		}
		out[rel] = fileState{size: info.Size(), modTime: info.ModTime().UnixNano(), digest: sha256.Sum256(content)}
	}
	if len(out) == 0 {
		t.Fatal("no generated artifact was snapshotted")
	}
	return out
}

// indexWithoutGeneratedArtifacts copies the repository into memory, drops every declared builder
// output, and builds an index over what is left.
//
// Copying rather than moving files keeps the working tree untouched, which is the same discipline
// every other live test in this package follows: a test that repaired or removed a canonical file
// to make its point would be the only writer in a read-only backend.
func indexWithoutGeneratedArtifacts(t testing.TB, root string) *service.Knowledge {
	t.Helper()
	excluded := map[string]bool{}
	for _, path := range generatedArtifacts() {
		excluded[path] = true
	}

	corpus := fstest.MapFS{}
	dropped := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		slashed := filepath.ToSlash(rel)
		if d.IsDir() {
			// The working tree carries directories the corpus does not: version control, the
			// backend module itself, and the agent scratch space. Skipping them keeps the copy to
			// the canonical record layers.
			switch {
			case slashed == ".":
				return nil
			case strings.HasPrefix(slashed, ".git"), slashed == "backend", slashed == "assets", slashed == "tools":
				return fs.SkipDir
			}
			return nil
		}
		if excluded[slashed] {
			dropped++
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		corpus[slashed] = &fstest.MapFile{Data: content}
		return nil
	})
	if err != nil {
		t.Fatalf("copy canonical repository: %v", err)
	}
	if dropped != len(excluded) {
		t.Fatalf("dropped %d generated artifacts, want %d", dropped, len(excluded))
	}

	repo, err := filesystem.NewFromFS(corpus, "canonical-without-generated-views")
	if err != nil {
		t.Fatalf("open the copied repository: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build an index without the generated views: %v", err)
	}
	return knowledge
}

// liveRelatedJSON runs one discovery and returns its serialisation, which is what two indexes have
// to agree on byte for byte.
func liveRelatedJSON(
	t testing.TB, k *service.Knowledge, class domain.SearchEntityType, id string, q service.RelatedQuery,
) string {
	t.Helper()
	result, err := k.RelatedKnowledgeFor(string(class), id, q)
	if err != nil {
		t.Fatalf("related %s/%s %+v: %v", class, id, q, err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal related %s/%s: %v", class, id, err)
	}
	return string(encoded)
}
