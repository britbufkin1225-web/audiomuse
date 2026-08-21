package filesystem_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
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
// inspection: every canonical file's size and modification time must be unchanged by a
// full load. This does not depend on git, so it also catches a write that happened to
// reproduce identical content.
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
}

// snapshot records the size and modification time of every canonical record file.
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
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(rel)] = fileState{size: info.Size(), modTime: info.ModTime().UnixNano()}
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
