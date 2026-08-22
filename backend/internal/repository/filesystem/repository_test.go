package filesystem_test

import (
	"context"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

func load(t testing.TB, fsys fstest.MapFS) (*domain.ValidationReport, []domain.Node) {
	t.Helper()
	repo, err := filesystem.NewFromFS(fsys, testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open fixture corpus: %v", err)
	}
	corpus, report, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load fixture corpus: %v", err)
	}
	return report, corpus.Nodes
}

func TestLoadValidFixtureCorpus(t *testing.T) {
	repo, err := filesystem.NewFromFS(testsupport.CorpusFS(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open fixture corpus: %v", err)
	}
	corpus, report, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if report.HasFatal() {
		t.Fatalf("valid fixture corpus reported fatal issues: %v", report.Fatal())
	}
	if got, want := report.Status(), "WARN"; got != want {
		t.Errorf("status = %q, want %q (the fixture carries deliberate corpus gaps)", got, want)
	}
	if got, want := len(corpus.Nodes), 3; got != want {
		t.Fatalf("nodes = %d, want %d", got, want)
	}
	if got, want := len(corpus.Sources), 6; got != want {
		t.Errorf("sources = %d, want %d", got, want)
	}
	if got, want := len(corpus.Claims), 4; got != want {
		t.Errorf("claims = %d, want %d", got, want)
	}
	if got, want := len(corpus.Sessions), 2; got != want {
		t.Errorf("sessions = %d, want %d", got, want)
	}
	if got, want := len(corpus.RelationshipTypes), 3; got != want {
		t.Errorf("relationship types = %d, want %d", got, want)
	}

	if got, want := repo.Describe().Kind, "filesystem"; got != want {
		t.Errorf("descriptor kind = %q, want %q", got, want)
	}
}

func TestLoadParsesNodeFrontMatterAndBody(t *testing.T) {
	_, nodes := load(t, testsupport.MutableCorpus(t))

	var alpha domain.Node
	for _, node := range nodes {
		if node.ID == "alpha" {
			alpha = node
		}
	}
	if alpha.ID == "" {
		t.Fatal("alpha not loaded")
	}

	if got, want := alpha.Title, "Alpha"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if got, want := alpha.Domain, "acoustics"; got != want {
		t.Errorf("domain = %q, want %q", got, want)
	}
	if got, want := alpha.Status, "foundation"; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	wantRels := []domain.Relationship{
		{Target: "beta", Type: "produces"},
		{Target: "gamma", Type: "characterized_by"},
	}
	if !reflect.DeepEqual(alpha.Relationships, wantRels) {
		t.Errorf("relationships = %#v, want %#v", alpha.Relationships, wantRels)
	}
	if got, want := alpha.SessionOrigin, []string{"session-01-fixture"}; !reflect.DeepEqual(got, want) {
		t.Errorf("session_origin = %#v, want %#v", got, want)
	}
	if got, want := alpha.Path, "nodes/acoustics/alpha.md"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if alpha.Body == "" {
		t.Error("markdown body was not captured")
	}
}

// TestEmptyRelationshipListProjectsAsEmptySlice guards the JSON contract: a node that
// records relationships: [] must serialise as [] and never as null.
func TestEmptyRelationshipListProjectsAsEmptySlice(t *testing.T) {
	_, nodes := load(t, testsupport.MutableCorpus(t))
	for _, node := range nodes {
		if node.ID != "gamma" {
			continue
		}
		if node.Relationships == nil {
			t.Fatal("gamma relationships is nil, want an empty slice")
		}
		if len(node.Relationships) != 0 {
			t.Fatalf("gamma relationships = %#v, want empty", node.Relationships)
		}
		return
	}
	t.Fatal("gamma not loaded")
}

func TestSessionsDeriveFromRegistryAndNodeOrigin(t *testing.T) {
	repo, err := filesystem.NewFromFS(testsupport.CorpusFS(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	corpus, _, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := map[string]domain.Session{}
	for _, session := range corpus.Sessions {
		got[session.ID] = session
	}

	first, ok := got["session-01-fixture"]
	if !ok {
		t.Fatal("session-01-fixture missing")
	}
	if !first.DirectoryPresent {
		t.Error("session-01-fixture should report its directory as present")
	}
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(first.NodeIDs, want) {
		t.Errorf("node_ids = %#v, want %#v (sorted, derived from node session_origin)", first.NodeIDs, want)
	}

	second, ok := got["session-02-unused"]
	if !ok {
		t.Fatal("session-02-unused missing")
	}
	if second.DirectoryPresent {
		t.Error("session-02-unused should report its directory as absent")
	}
	if len(second.NodeIDs) != 0 {
		t.Errorf("node_ids = %#v, want empty", second.NodeIDs)
	}
}

func TestWarningsForCorpusGaps(t *testing.T) {
	report, _ := load(t, testsupport.MutableCorpus(t))
	if report.HasFatal() {
		t.Fatalf("unexpected fatal issues: %v", report.Fatal())
	}

	counts := map[string]int{}
	for _, issue := range report.Warnings() {
		counts[issue.Code]++
	}
	want := map[string]int{
		domain.CodeMissingLocator:    1, // session-02-unused transcript
		domain.CodeMissingSessionDir: 1, // sessions/session-02-unused/
		domain.CodeUncitedSource:     2, // session-02-unused, fixture-uncited-source
		domain.CodeUncitedSession:    1, // session-02-unused
	}
	if !reflect.DeepEqual(counts, want) {
		t.Errorf("warning counts = %#v, want %#v", counts, want)
	}
}

// TestFatalDefects drives one defect at a time through the real loader.
func TestFatalDefects(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(fstest.MapFS)
		wantErr string
	}{
		{
			name: "malformed front matter delimiters",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "nodes/dsp/delta.md", "id: delta\ntitle: Delta\n")
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "front matter is not valid yaml",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "nodes/dsp/delta.md", "---\nid: delta\n  title: [unclosed\n---\n")
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "core_concepts entry parses as a mapping",
			mutate: func(c fstest.MapFS) {
				node := testsupport.ValidNode("delta", "Delta", "dsp", "seed", "[]", "[]")
				node = replaceOnce(node, "core_concepts: []", "core_concepts:\n  - a phrase with: a colon")
				testsupport.Write(c, "nodes/dsp/delta.md", node)
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "missing required field",
			mutate: func(c fstest.MapFS) {
				node := testsupport.ValidNode("delta", "Delta", "dsp", "seed", "[]", "[]")
				testsupport.Write(c, "nodes/dsp/delta.md", replaceOnce(node, "experiments: []\n", ""))
			},
			wantErr: domain.CodeMissingField,
		},
		{
			name: "unknown top-level field",
			mutate: func(c fstest.MapFS) {
				node := testsupport.ValidNode("delta", "Delta", "dsp", "seed", "[]", "[]")
				testsupport.Write(c, "nodes/dsp/delta.md", replaceOnce(node, "experiments: []", "experiments: []\nrelated_nodes: []"))
			},
			wantErr: domain.CodeUnknownField,
		},
		{
			name: "non-canonical id",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "nodes/dsp/delta.md", testsupport.ValidNode("Delta_One", "Delta", "dsp", "seed", "[]", "[]"))
			},
			wantErr: domain.CodeInvalidID,
		},
		{
			name: "duplicate node id",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "nodes/dsp/alpha-again.md", testsupport.ValidNode("alpha", "Alpha Again", "dsp", "seed", "[]", "[]"))
			},
			wantErr: domain.CodeDuplicateID,
		},
		{
			name: "unresolved relationship target",
			mutate: func(c fstest.MapFS) {
				rels := "\n  - target: nowhere\n    type: produces"
				testsupport.Write(c, "nodes/dsp/delta.md", testsupport.ValidNode("delta", "Delta", "dsp", "seed", rels, "[]"))
			},
			wantErr: domain.CodeUnresolvedTarget,
		},
		{
			name: "relationship type outside the canonical vocabulary",
			mutate: func(c fstest.MapFS) {
				rels := "\n  - target: alpha\n    type: relates_to"
				testsupport.Write(c, "nodes/dsp/delta.md", testsupport.ValidNode("delta", "Delta", "dsp", "seed", rels, "[]"))
			},
			wantErr: domain.CodeInvalidRelationType,
		},
		{
			name: "self link",
			mutate: func(c fstest.MapFS) {
				rels := "\n  - target: delta\n    type: produces"
				testsupport.Write(c, "nodes/dsp/delta.md", testsupport.ValidNode("delta", "Delta", "dsp", "seed", rels, "[]"))
			},
			wantErr: domain.CodeSelfLink,
		},
		{
			name: "duplicate edge",
			mutate: func(c fstest.MapFS) {
				rels := "\n  - target: alpha\n    type: produces\n  - target: alpha\n    type: produces"
				testsupport.Write(c, "nodes/dsp/delta.md", testsupport.ValidNode("delta", "Delta", "dsp", "seed", rels, "[]"))
			},
			wantErr: domain.CodeDuplicateEdge,
		},
		{
			name: "unresolved source reference",
			mutate: func(c fstest.MapFS) {
				node := testsupport.ValidNode("delta", "Delta", "dsp", "seed", "[]", "[\"not-registered\"]")
				testsupport.Write(c, "nodes/dsp/delta.md", node)
			},
			wantErr: domain.CodeUnresolvedSource,
		},
		{
			name: "unresolved session_origin reference",
			mutate: func(c fstest.MapFS) {
				node := testsupport.ValidNode("delta", "Delta", "dsp", "seed", "[]", "[]")
				node = replaceOnce(node, "session_origin: []", "session_origin: [\"session-99-missing\"]")
				testsupport.Write(c, "nodes/dsp/delta.md", node)
			},
			wantErr: domain.CodeUnresolvedSession,
		},
		{
			name: "duplicate registered source id",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "sources/source-registry.yaml",
					"schema: audiomuse-source-registry\nversion: 1\nsources:\n"+
						"  - id: fixture-reference-work\n    type: book\n    title: One\n    locator: research/sources/fixture-reference-work.md\n    relationship: supporting\n"+
						"  - id: fixture-reference-work\n    type: book\n    title: Two\n    locator: research/sources/fixture-reference-work.md\n    relationship: supporting\n")
			},
			wantErr: domain.CodeDuplicateID,
		},
		{
			name: "registry locator escapes the repository root",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "sources/source-registry.yaml",
					"schema: audiomuse-source-registry\nversion: 1\nsources:\n"+
						"  - id: fixture-reference-work\n    type: book\n    title: One\n    locator: ../../etc/passwd\n    relationship: supporting\n")
			},
			wantErr: domain.CodeUnsafePath,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			corpus := testsupport.MutableCorpus(t)
			tc.mutate(corpus)
			report, _ := load(t, corpus)

			for _, issue := range report.Fatal() {
				if issue.Code == tc.wantErr {
					return
				}
			}
			t.Fatalf("want a fatal %s; got %v", tc.wantErr, report.Fatal())
		})
	}
}

// TestDeterministicOrdering runs the loader twice and requires identical output. Nothing
// may depend on map iteration order or on operating-system directory ordering.
func TestDeterministicOrdering(t *testing.T) {
	first, firstNodes := load(t, testsupport.MutableCorpus(t))
	second, secondNodes := load(t, testsupport.MutableCorpus(t))

	if !reflect.DeepEqual(firstNodes, secondNodes) {
		t.Error("node ordering differed between two loads of an unchanged corpus")
	}
	if !reflect.DeepEqual(first.Issues, second.Issues) {
		t.Error("validation report ordering differed between two loads of an unchanged corpus")
	}

	wantOrder := []string{"alpha", "beta", "gamma"}
	got := make([]string, 0, len(firstNodes))
	for _, node := range firstNodes {
		got = append(got, node.ID)
	}
	if !reflect.DeepEqual(got, wantOrder) {
		t.Errorf("node order = %v, want canonical ID order %v", got, wantOrder)
	}
}

func TestReadmeFilesAreNotParsedAsNodes(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "nodes/README.md", "# Nodes\n\nNot a canonical node record.\n")
	testsupport.Write(corpus, "nodes/dsp/README.md", "# DSP\n\nNot a canonical node record.\n")

	report, nodes := load(t, corpus)
	if report.HasFatal() {
		t.Fatalf("README files were parsed as node records: %v", report.Fatal())
	}
	if got, want := len(nodes), 3; got != want {
		t.Errorf("nodes = %d, want %d", got, want)
	}
}

func TestNewRejectsNonRepositoryRoot(t *testing.T) {
	if _, err := filesystem.NewFromFS(fstest.MapFS{"README.md": &fstest.MapFile{}}, "empty"); err == nil {
		t.Fatal("want an error for a directory that is not an AudioMuse repository")
	}
	if _, err := filesystem.New(""); err == nil {
		t.Fatal("want an error for an empty repository root")
	}
	if _, err := filesystem.New(t.TempDir()); err == nil {
		t.Fatal("want an error for an empty directory")
	}
}

func replaceOnce(text, old, replacement string) string {
	idx := indexOf(text, old)
	if idx < 0 {
		panic("fixture does not contain " + old)
	}
	return text[:idx] + replacement + text[idx+len(old):]
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
