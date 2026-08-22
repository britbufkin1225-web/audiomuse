package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

func newKnowledge(t testing.TB) *service.Knowledge {
	t.Helper()
	repo, err := filesystem.NewFromFS(testsupport.CorpusFS(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open fixture corpus: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	return knowledge
}

func TestNodeLookup(t *testing.T) {
	k := newKnowledge(t)

	node, err := k.NodeByID("alpha")
	if err != nil {
		t.Fatalf("NodeByID(alpha): %v", err)
	}
	if got, want := node.Title, "Alpha"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if len(node.InboundRelationships) != 0 {
		t.Errorf("alpha inbound = %#v, want none", node.InboundRelationships)
	}
}

func TestMissingNodeReturnsNotFound(t *testing.T) {
	k := newKnowledge(t)
	if _, err := k.NodeByID("does-not-exist"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := k.SessionByID("does-not-exist"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestInboundRelationshipsAreDerivedNotStored checks that the reverse view is present,
// sorted, and kept out of the node's own authored relationships list.
func TestInboundRelationshipsAreDerivedNotStored(t *testing.T) {
	k := newKnowledge(t)

	gamma, err := k.NodeByID("gamma")
	if err != nil {
		t.Fatalf("NodeByID(gamma): %v", err)
	}
	if len(gamma.Relationships) != 0 {
		t.Errorf("gamma authored relationships = %#v, want empty", gamma.Relationships)
	}
	want := []domain.InboundRelationship{
		{Source: "alpha", Type: "characterized_by"},
		{Source: "beta", Type: "characterized_by"},
	}
	if !reflect.DeepEqual(gamma.InboundRelationships, want) {
		t.Errorf("gamma inbound = %#v, want %#v", gamma.InboundRelationships, want)
	}
}

func TestListNodesFilters(t *testing.T) {
	k := newKnowledge(t)

	cases := []struct {
		name  string
		query service.NodeQuery
		want  []string
	}{
		{"no filter", service.NodeQuery{}, []string{"alpha", "beta", "gamma"}},
		{"domain", service.NodeQuery{Domain: "acoustics"}, []string{"alpha", "beta"}},
		{"status", service.NodeQuery{Status: "seed"}, []string{"gamma"}},
		{"session", service.NodeQuery{Session: "session-01-fixture"}, []string{"alpha", "beta"}},
		{"domain and status together", service.NodeQuery{Domain: "acoustics", Status: "developed"}, []string{"beta"}},
		{"unknown domain", service.NodeQuery{Domain: "no-such-domain"}, []string{}},
		// Canonical filters are exact and case-sensitive, per docs/knowledge-model.md.
		{"domain case drift does not match", service.NodeQuery{Domain: "Acoustics"}, []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(k.ListNodes(tc.query))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("nodes = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLexicalSearch(t *testing.T) {
	k := newKnowledge(t)

	cases := []struct {
		name string
		q    string
		want []string
	}{
		{"matches id", "gamm", []string{"gamma"}},
		{"matches title case-insensitively", "BETA", []string{"beta"}},
		{"matches definition text", "resonant", []string{"beta"}},
		{"matches core concepts", "empty relationship list", []string{"gamma"}},
		{"matches nothing", "no-such-term-anywhere", []string{}},
		{"blank query matches everything", "   ", []string{"alpha", "beta", "gamma"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(k.ListNodes(service.NodeQuery{Q: tc.q}))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("q=%q gave %v, want %v", tc.q, got, tc.want)
			}
		})
	}

	// The node markdown body is not part of the search corpus; only front matter is.
	if got := ids(k.ListNodes(service.NodeQuery{Q: "Prose body for the alpha"})); len(got) != 0 {
		t.Errorf("body text matched %v, want no matches", got)
	}
}

func TestPagingIsBounded(t *testing.T) {
	k := newKnowledge(t)

	page := k.ListNodes(service.NodeQuery{Limit: 2})
	if got, want := ids(page), []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("first page = %v, want %v", got, want)
	}
	if page.Page.Total != 3 || page.Page.Count != 2 || page.Page.Limit != 2 || page.Page.Offset != 0 {
		t.Errorf("page = %#v", page.Page)
	}

	page = k.ListNodes(service.NodeQuery{Limit: 2, Offset: 2})
	if got, want := ids(page), []string{"gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("second page = %v, want %v", got, want)
	}

	page = k.ListNodes(service.NodeQuery{Offset: 99})
	if got := ids(page); len(got) != 0 {
		t.Errorf("past-the-end offset gave %v, want none", got)
	}
	if page.Page.Total != 3 {
		t.Errorf("total = %d, want 3", page.Page.Total)
	}

	if got, want := k.ListNodes(service.NodeQuery{Limit: 10_000}).Page.Limit, service.MaxLimit; got != want {
		t.Errorf("limit was not clamped: %d, want %d", got, want)
	}
	if got, want := k.ListNodes(service.NodeQuery{}).Page.Limit, service.DefaultLimit; got != want {
		t.Errorf("default limit = %d, want %d", got, want)
	}
	if got := k.ListNodes(service.NodeQuery{Offset: -5}).Page.Offset; got != 0 {
		t.Errorf("negative offset = %d, want 0", got)
	}
}

func TestOverlongQueryIsBoundedNotRejected(t *testing.T) {
	k := newKnowledge(t)
	long := make([]byte, service.MaxQueryChars*4)
	for i := range long {
		long[i] = 'z'
	}
	if got := ids(k.ListNodes(service.NodeQuery{Q: string(long)})); len(got) != 0 {
		t.Errorf("overlong query matched %v, want none", got)
	}
}

func TestGraphProjection(t *testing.T) {
	k := newKnowledge(t)
	graph := k.Graph()

	if got, want := graph.Metadata.NodeCount, 3; got != want {
		t.Errorf("node_count = %d, want %d", got, want)
	}
	if got, want := graph.Metadata.EdgeCount, 3; got != want {
		t.Errorf("edge_count = %d, want %d", got, want)
	}

	wantEdges := []domain.GraphEdge{
		{Source: "alpha", Target: "gamma", Type: "characterized_by"},
		{Source: "alpha", Target: "beta", Type: "produces"},
		{Source: "beta", Target: "gamma", Type: "characterized_by"},
	}
	if !reflect.DeepEqual(graph.Edges, wantEdges) {
		t.Errorf("edges = %#v, want %#v (sorted by source, type, target)", graph.Edges, wantEdges)
	}

	// Every edge must originate from an authored relationship. Nothing may be inferred,
	// and no reverse edge may be synthesised into the canonical projection.
	authored := map[string]bool{}
	for _, page := range []service.NodeList{k.ListNodes(service.NodeQuery{})} {
		for _, summary := range page.Nodes {
			node, err := k.NodeByID(summary.ID)
			if err != nil {
				t.Fatalf("NodeByID(%s): %v", summary.ID, err)
			}
			for _, rel := range node.Relationships {
				authored[node.ID+"|"+rel.Type+"|"+rel.Target] = true
			}
		}
	}
	for _, edge := range graph.Edges {
		if !authored[edge.Source+"|"+edge.Type+"|"+edge.Target] {
			t.Errorf("graph edge %v was not authored on any node", edge)
		}
	}
	if len(authored) != len(graph.Edges) {
		t.Errorf("authored edges = %d, graph edges = %d", len(authored), len(graph.Edges))
	}
}

func TestGraphIsDeterministicAcrossBuilds(t *testing.T) {
	first := newKnowledge(t).Graph()
	second := newKnowledge(t).Graph()
	if !reflect.DeepEqual(first, second) {
		t.Error("graph projection differed between two builds of an unchanged corpus")
	}
}

// TestGraphAccessorReturnsACopy protects the immutability the package doc relies on for
// lock-free concurrent reads.
func TestGraphAccessorReturnsACopy(t *testing.T) {
	k := newKnowledge(t)
	graph := k.Graph()
	if len(graph.Edges) == 0 {
		t.Fatal("fixture graph has no edges")
	}
	graph.Edges[0].Target = "mutated"
	if k.Graph().Edges[0].Target == "mutated" {
		t.Fatal("mutating a returned graph changed the shared index")
	}
}

func TestListSessions(t *testing.T) {
	k := newKnowledge(t)

	all := k.ListSessions(service.SessionQuery{})
	if got, want := all.Page.Total, 2; got != want {
		t.Fatalf("sessions = %d, want %d", got, want)
	}
	if got, want := all.Sessions[0].ID, "session-01-fixture"; got != want {
		t.Errorf("first session = %q, want %q", got, want)
	}

	filtered := k.ListSessions(service.SessionQuery{Q: "unused"})
	if got, want := filtered.Page.Total, 1; got != want {
		t.Fatalf("filtered sessions = %d, want %d", got, want)
	}
	if got, want := filtered.Sessions[0].ID, "session-02-unused"; got != want {
		t.Errorf("filtered session = %q, want %q", got, want)
	}
}

func TestProjectSummaryDoesNotLeakAPath(t *testing.T) {
	k := newKnowledge(t)
	project := k.Project()

	if got, want := project.Name, service.ProjectName; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := project.Mode, service.ModeReadOnly; got != want {
		t.Errorf("mode = %q, want %q", got, want)
	}
	if got, want := project.Counts.Nodes, 3; got != want {
		t.Errorf("nodes = %d, want %d", got, want)
	}
	if got, want := project.Counts.Edges, 3; got != want {
		t.Errorf("edges = %d, want %d", got, want)
	}
	if got, want := project.Counts.Sessions, 2; got != want {
		t.Errorf("sessions = %d, want %d", got, want)
	}
	if got, want := project.Counts.Sources, 6; got != want {
		t.Errorf("sources = %d, want %d", got, want)
	}
	if got, want := project.Domains, []string{"acoustics", "dsp"}; !reflect.DeepEqual(got, want) {
		t.Errorf("domains = %v, want %v", got, want)
	}
	if got, want := project.Repository.Name, testsupport.CorpusName; got != want {
		t.Errorf("repository name = %q, want %q", got, want)
	}
}

func TestDiagnosticsReportsWarningsOnly(t *testing.T) {
	k := newKnowledge(t)
	diagnostics := k.Diagnostics()

	if diagnostics.Counts.Fatal != 0 {
		t.Errorf("fatal = %d, want 0 (a running process cannot have fatal issues)", diagnostics.Counts.Fatal)
	}
	if diagnostics.Counts.Warning == 0 {
		t.Fatal("fixture corpus should report warnings")
	}
	for _, warning := range diagnostics.Warnings {
		if warning.Severity != domain.SeverityWarning {
			t.Errorf("diagnostics exposed a %s issue", warning.Severity)
		}
	}
}

// TestNewRefusesAFatallyInvalidCorpus checks that the service will not serve a projection
// it knows to be wrong.
func TestNewRefusesAFatallyInvalidCorpus(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	testsupport.Write(corpus, "nodes/dsp/delta.md",
		testsupport.ValidNode("delta", "Delta", "dsp", "seed", "\n  - target: nowhere\n    type: produces", "[]"))

	repo, err := filesystem.NewFromFS(corpus, testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := service.New(context.Background(), repo); err == nil {
		t.Fatal("want an error for a corpus with fatal validation issues")
	}
}

func TestNewHonoursACancelledContext(t *testing.T) {
	repo, err := filesystem.NewFromFS(testsupport.CorpusFS(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.New(ctx, repo); err == nil {
		t.Fatal("want an error for a cancelled context")
	}
}

func ids(list service.NodeList) []string {
	out := make([]string, 0, len(list.Nodes))
	for _, node := range list.Nodes {
		out = append(out, node.ID)
	}
	return out
}
