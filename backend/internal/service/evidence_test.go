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

func evidenceIndex(t testing.TB) *service.Knowledge {
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

func claimIDs(list service.ClaimList) []string {
	out := make([]string, 0, len(list.Claims))
	for _, claim := range list.Claims {
		out = append(out, claim.ID)
	}
	return out
}

func sourceIDs(list service.SourceList) []string {
	out := make([]string, 0, len(list.Sources))
	for _, source := range list.Sources {
		out = append(out, source.ID)
	}
	return out
}

func mustListClaims(t testing.TB, k *service.Knowledge, q service.ClaimQuery) service.ClaimList {
	t.Helper()
	list, err := k.ListClaims(q)
	if err != nil {
		t.Fatalf("list claims %#v: %v", q, err)
	}
	return list
}

func mustListSources(t testing.TB, k *service.Knowledge, q service.SourceQuery) service.SourceList {
	t.Helper()
	list, err := k.ListSources(q)
	if err != nil {
		t.Fatalf("list sources %#v: %v", q, err)
	}
	return list
}

func TestListSourcesIsCanonicalIDOrder(t *testing.T) {
	list := mustListSources(t, evidenceIndex(t), service.SourceQuery{})

	want := []string{
		"fixture-archive-record", "fixture-attribution-source", "fixture-reference-work",
		"fixture-uncited-source", "session-01-fixture", "session-02-unused",
	}
	if got := sourceIDs(list); !reflect.DeepEqual(got, want) {
		t.Errorf("source order = %v, want canonical ID order %v", got, want)
	}
	if got, want := list.Page.Total, len(want); got != want {
		t.Errorf("total = %d, want %d", got, want)
	}
}

func TestSourceSummaryCountsBothRelations(t *testing.T) {
	list := mustListSources(t, evidenceIndex(t), service.SourceQuery{})

	byID := map[string]domain.SourceSummary{}
	for _, source := range list.Sources {
		byID[source.ID] = source
	}

	// Evidential: cited by three fixture claims, listed topically by no node.
	if got, want := byID["fixture-archive-record"].ClaimCount, 3; got != want {
		t.Errorf("fixture-archive-record claim_count = %d, want %d", got, want)
	}
	if got, want := byID["fixture-archive-record"].NodeCount, 0; got != want {
		t.Errorf("fixture-archive-record node_count = %d, want %d", got, want)
	}
	// Topical: named by two nodes, and cited by all four claims.
	if got, want := byID["fixture-reference-work"].NodeCount, 2; got != want {
		t.Errorf("fixture-reference-work node_count = %d, want %d", got, want)
	}
	if got, want := byID["fixture-reference-work"].ClaimCount, 4; got != want {
		t.Errorf("fixture-reference-work claim_count = %d, want %d", got, want)
	}
	// Attribution alone still counts as a citation of the source.
	if got, want := byID["fixture-attribution-source"].ClaimCount, 1; got != want {
		t.Errorf("fixture-attribution-source claim_count = %d, want %d", got, want)
	}
	if got := byID["fixture-archive-record"].EvidenceClass; got == nil || *got != "institutional_archive" {
		t.Errorf("evidence_class = %v, want institutional_archive", got)
	}
	if got := byID["fixture-reference-work"].EvidenceClass; got != nil {
		t.Errorf("evidence_class = %v, want nil for a source no annotation was required for", *got)
	}
}

func TestSourceFilters(t *testing.T) {
	k := evidenceIndex(t)

	cases := []struct {
		name  string
		query service.SourceQuery
		want  []string
	}{
		{"type", service.SourceQuery{Type: "session"}, []string{"session-01-fixture", "session-02-unused"}},
		{"relationship", service.SourceQuery{Relationship: "historical"}, []string{"fixture-attribution-source"}},
		{"evidence class", service.SourceQuery{EvidenceClass: "institutional_archive"}, []string{"fixture-archive-record"}},
		{"retrieval", service.SourceQuery{Retrieval: "citation_only"}, []string{"fixture-attribution-source"}},
		{
			"cited by one claim",
			service.SourceQuery{ClaimID: "beta-was-observed-in-1999"},
			[]string{"fixture-archive-record", "fixture-attribution-source", "fixture-reference-work"},
		},
		{
			"named topically by one node",
			service.SourceQuery{NodeID: "alpha"},
			[]string{"fixture-reference-work", "session-01-fixture"},
		},
		{
			"claim-mediated session relation",
			service.SourceQuery{SessionID: "session-01-fixture"},
			[]string{"fixture-archive-record", "fixture-attribution-source", "fixture-reference-work"},
		},
		{
			"filters compose with AND",
			service.SourceQuery{ClaimID: "beta-was-observed-in-1999", Retrieval: "full_text"},
			[]string{"fixture-archive-record"},
		},
		{"lexical search over id, title and author", service.SourceQuery{Q: "participant"}, []string{"fixture-attribution-source"}},
		{"unknown claim id yields no matches", service.SourceQuery{ClaimID: "no-such-claim"}, []string{}},
		{"unknown node id yields no matches", service.SourceQuery{NodeID: "no-such-node"}, []string{}},
		{"session with no claims yields no matches", service.SourceQuery{SessionID: "session-02-unused"}, []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sourceIDs(mustListSources(t, k, tc.query))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("sources = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSourceByIDCarriesEveryReverseView(t *testing.T) {
	k := evidenceIndex(t)

	detail, err := k.SourceByID("fixture-reference-work")
	if err != nil {
		t.Fatalf("source lookup: %v", err)
	}

	// Ordered by claim ID, then relation, so two runs render identically.
	want := []domain.SourceClaimRef{
		{ClaimID: "alpha-carries-energy", Relation: "supported_by"},
		{ClaimID: "alpha-may-extend-to-gamma", Relation: "qualified_by"},
		{ClaimID: "beta-was-observed-in-1999", Relation: "contradicted_by"},
		{ClaimID: "gamma-follows-from-alpha-and-beta", Relation: "qualified_by"},
	}
	if !reflect.DeepEqual(detail.Claims, want) {
		t.Errorf("claims = %#v, want %#v", detail.Claims, want)
	}
	if got, want := detail.NodeIDs, []string{"alpha", "gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("node_ids = %v, want %v (topical node sources: lists)", got, want)
	}
	if got, want := detail.SessionIDs, []string{"session-01-fixture"}; !reflect.DeepEqual(got, want) {
		t.Errorf("session_ids = %v, want %v", got, want)
	}
	if got, want := detail.AttributedClaimIDs, []string{}; !reflect.DeepEqual(got, want) {
		t.Errorf("attributed_claim_ids = %#v, want an empty slice", got)
	}

	attributed, err := k.SourceByID("fixture-attribution-source")
	if err != nil {
		t.Fatalf("source lookup: %v", err)
	}
	if got, want := attributed.AttributedClaimIDs, []string{"beta-was-observed-in-1999"}; !reflect.DeepEqual(got, want) {
		t.Errorf("attributed_claim_ids = %v, want %v", got, want)
	}
	if len(attributed.Claims) != 0 {
		t.Errorf("claims = %#v, want empty: an attribution is not an evidence relation", attributed.Claims)
	}
}

func TestSourceByIDUnknown(t *testing.T) {
	if _, err := evidenceIndex(t).SourceByID("no-such-source"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListClaimsIsCanonicalIDOrder(t *testing.T) {
	list := mustListClaims(t, evidenceIndex(t), service.ClaimQuery{})

	want := []string{
		"alpha-carries-energy", "alpha-may-extend-to-gamma",
		"beta-was-observed-in-1999", "gamma-follows-from-alpha-and-beta",
	}
	if got := claimIDs(list); !reflect.DeepEqual(got, want) {
		t.Errorf("claim order = %v, want canonical ID order %v", got, want)
	}

	summary := list.Claims[2]
	if summary.Confidence != "moderate" || summary.DisputeStatus != "disputed" {
		t.Errorf("summary = %#v, want the provenance axes carried into the list projection", summary)
	}
	if summary.EvidenceCount != 2 || summary.AttributionCount != 1 || summary.AppearsInCount != 2 {
		t.Errorf("summary counts = %#v", summary)
	}
}

func TestClaimFilters(t *testing.T) {
	k := evidenceIndex(t)

	cases := []struct {
		name  string
		query service.ClaimQuery
		want  []string
	}{
		{"claim type", service.ClaimQuery{ClaimType: "hypothesis"}, []string{"alpha-may-extend-to-gamma"}},
		{"confidence", service.ClaimQuery{Confidence: "high"}, []string{"alpha-carries-energy"}},
		{"dispute status", service.ClaimQuery{DisputeStatus: "disputed"}, []string{"beta-was-observed-in-1999"}},
		{"temporal precision", service.ClaimQuery{TemporalPrecision: "year"}, []string{"beta-was-observed-in-1999"}},
		{"evidence relation", service.ClaimQuery{Relation: "contradicted_by"}, []string{"beta-was-observed-in-1999"}},
		{
			"cited source",
			service.ClaimQuery{SourceID: "fixture-archive-record"},
			[]string{"alpha-carries-energy", "beta-was-observed-in-1999", "gamma-follows-from-alpha-and-beta"},
		},
		{
			"attribution counts as citing the source",
			service.ClaimQuery{SourceID: "fixture-attribution-source"},
			[]string{"beta-was-observed-in-1999"},
		},
		{
			"appearance node",
			service.ClaimQuery{NodeID: "gamma"},
			[]string{"alpha-may-extend-to-gamma", "gamma-follows-from-alpha-and-beta"},
		},
		{"appearance session", service.ClaimQuery{SessionID: "session-01-fixture"}, []string{"beta-was-observed-in-1999"}},
		{
			"source and relation compose with AND",
			service.ClaimQuery{SourceID: "fixture-reference-work", Relation: "qualified_by"},
			[]string{"alpha-may-extend-to-gamma", "gamma-follows-from-alpha-and-beta"},
		},
		{"lexical search over id and statement", service.ClaimQuery{Q: "1999"}, []string{"beta-was-observed-in-1999"}},
		{"unknown source id yields no matches", service.ClaimQuery{SourceID: "no-such-source"}, []string{}},
		{"unknown node id yields no matches", service.ClaimQuery{NodeID: "no-such-node"}, []string{}},
		{
			"a node that no claim appears in yields no matches",
			service.ClaimQuery{NodeID: "beta", Confidence: "high"},
			[]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := claimIDs(mustListClaims(t, k, tc.query))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("claims = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestInvalidFilterValuesAreRejected: an out-of-vocabulary filter must be refused, not
// silently answered with an empty list that the caller would read as "no such claims".
func TestInvalidFilterValuesAreRejected(t *testing.T) {
	k := evidenceIndex(t)

	claimCases := []struct {
		param string
		query service.ClaimQuery
	}{
		{"claim_type", service.ClaimQuery{ClaimType: "disputed_claim"}},
		{"confidence", service.ClaimQuery{Confidence: "very-high"}},
		{"confidence", service.ClaimQuery{Confidence: "High"}},
		{"dispute_status", service.ClaimQuery{DisputeStatus: "contested"}},
		{"temporal_precision", service.ClaimQuery{TemporalPrecision: "decade"}},
		{"relation", service.ClaimQuery{Relation: "mentions"}},
	}
	for _, tc := range claimCases {
		t.Run("claims "+tc.param+"", func(t *testing.T) {
			_, err := k.ListClaims(tc.query)
			var invalid *service.InvalidFilterError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want InvalidFilterError", err)
			}
			if invalid.Param != tc.param {
				t.Errorf("param = %q, want %q", invalid.Param, tc.param)
			}
			if len(invalid.Allowed) == 0 {
				t.Error("the error must carry the canonical vocabulary so a caller can correct the request")
			}
		})
	}

	sourceCases := []struct {
		param string
		query service.SourceQuery
	}{
		{"type", service.SourceQuery{Type: "blog"}},
		{"relationship", service.SourceQuery{Relationship: "related"}},
		{"evidence_class", service.SourceQuery{EvidenceClass: "hearsay"}},
		{"retrieval", service.SourceQuery{Retrieval: "skimmed"}},
	}
	for _, tc := range sourceCases {
		t.Run("sources "+tc.param, func(t *testing.T) {
			_, err := k.ListSources(tc.query)
			var invalid *service.InvalidFilterError
			if !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want InvalidFilterError", err)
			}
			if invalid.Param != tc.param {
				t.Errorf("param = %q, want %q", invalid.Param, tc.param)
			}
		})
	}
}

func TestClaimByIDFlattensItsEvidenceContext(t *testing.T) {
	k := evidenceIndex(t)

	detail, err := k.ClaimByID("beta-was-observed-in-1999")
	if err != nil {
		t.Fatalf("claim lookup: %v", err)
	}

	want := []string{"fixture-archive-record", "fixture-attribution-source", "fixture-reference-work"}
	if !reflect.DeepEqual(detail.SourceIDs, want) {
		t.Errorf("source_ids = %v, want %v (evidence and attribution, distinct and sorted)", detail.SourceIDs, want)
	}
	if got, want := detail.NodeIDs, []string{"beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("node_ids = %v, want %v", got, want)
	}
	if got, want := detail.SessionIDs, []string{"session-01-fixture"}; !reflect.DeepEqual(got, want) {
		t.Errorf("session_ids = %v, want %v", got, want)
	}
	// The canonical arrays must still be present unflattened.
	if len(detail.Evidence) != 2 || len(detail.Attribution) != 1 {
		t.Errorf("canonical evidence and attribution were not carried through: %#v", detail)
	}

	// A derivation from a node must not appear as an appearance site.
	derived, err := k.ClaimByID("gamma-follows-from-alpha-and-beta")
	if err != nil {
		t.Fatalf("claim lookup: %v", err)
	}
	if got, want := derived.NodeIDs, []string{"gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("node_ids = %v, want %v: derived_from is not an appearance site", got, want)
	}
}

func TestClaimByIDUnknown(t *testing.T) {
	if _, err := evidenceIndex(t).ClaimByID("no-such-claim"); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestCrossResolution walks session -> claim -> source -> node using only the read API, to
// prove the evidence layer is traversable without a client guessing at joins.
func TestCrossResolution(t *testing.T) {
	k := evidenceIndex(t)

	claims := mustListClaims(t, k, service.ClaimQuery{SessionID: "session-01-fixture"})
	if len(claims.Claims) != 1 {
		t.Fatalf("session claims = %#v, want exactly one", claims.Claims)
	}
	claim, err := k.ClaimByID(claims.Claims[0].ID)
	if err != nil {
		t.Fatalf("claim lookup: %v", err)
	}
	if len(claim.SourceIDs) == 0 {
		t.Fatal("the claim cites no source, so the traversal cannot continue")
	}
	source, err := k.SourceByID(claim.SourceIDs[0])
	if err != nil {
		t.Fatalf("source lookup: %v", err)
	}
	if len(source.Claims) == 0 && len(source.AttributedClaimIDs) == 0 {
		t.Fatal("the source reports no claims, so the reverse leg of the traversal is broken")
	}
	back := mustListClaims(t, k, service.ClaimQuery{SourceID: source.ID})
	if !contains(claimIDs(back), claim.ID) {
		t.Errorf("claims?source_id=%s = %v, want it to contain %s", source.ID, claimIDs(back), claim.ID)
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestEvidencePaging(t *testing.T) {
	k := evidenceIndex(t)

	list := mustListClaims(t, k, service.ClaimQuery{Limit: 2})
	if got, want := claimIDs(list), []string{"alpha-carries-energy", "alpha-may-extend-to-gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("first page = %v, want %v", got, want)
	}
	if list.Page.Total != 4 || list.Page.Count != 2 || list.Page.Limit != 2 || list.Page.Offset != 0 {
		t.Errorf("page = %#v", list.Page)
	}

	list = mustListClaims(t, k, service.ClaimQuery{Limit: 2, Offset: 2})
	if got, want := claimIDs(list), []string{"beta-was-observed-in-1999", "gamma-follows-from-alpha-and-beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("second page = %v, want %v", got, want)
	}

	list = mustListClaims(t, k, service.ClaimQuery{Offset: 99})
	if got := claimIDs(list); len(got) != 0 {
		t.Errorf("past-the-end page = %v, want empty", got)
	}
	if list.Claims == nil {
		t.Error("claims is nil, want an empty slice so the JSON renders []")
	}

	sources := mustListSources(t, k, service.SourceQuery{Limit: 300})
	if sources.Page.Limit != service.MaxLimit {
		t.Errorf("limit = %d, want it clamped to %d", sources.Page.Limit, service.MaxLimit)
	}
}

// TestEvidenceProjectionsAreDeterministic builds the index twice and requires byte-identical
// projections. Every derived list is assembled from a Go map, whose iteration order is
// randomised, so this is the test that the sorting is real rather than incidental.
func TestEvidenceProjectionsAreDeterministic(t *testing.T) {
	first, second := evidenceIndex(t), evidenceIndex(t)

	if !reflect.DeepEqual(mustListClaims(t, first, service.ClaimQuery{}), mustListClaims(t, second, service.ClaimQuery{})) {
		t.Error("claim list differed between two builds of an unchanged corpus")
	}
	if !reflect.DeepEqual(mustListSources(t, first, service.SourceQuery{}), mustListSources(t, second, service.SourceQuery{})) {
		t.Error("source list differed between two builds of an unchanged corpus")
	}
	for _, id := range []string{"fixture-reference-work", "fixture-archive-record", "session-01-fixture"} {
		a, err := first.SourceByID(id)
		if err != nil {
			t.Fatalf("source lookup: %v", err)
		}
		b, err := second.SourceByID(id)
		if err != nil {
			t.Fatalf("source lookup: %v", err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("source detail for %s differed between two builds", id)
		}
	}
	for _, id := range []string{"alpha-carries-energy", "beta-was-observed-in-1999"} {
		a, err := first.ClaimByID(id)
		if err != nil {
			t.Fatalf("claim lookup: %v", err)
		}
		b, err := second.ClaimByID(id)
		if err != nil {
			t.Fatalf("claim lookup: %v", err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("claim detail for %s differed between two builds", id)
		}
	}
}

// TestVocabulariesAreDefensivelyCopied guards the immutability the index depends on for
// lock-free concurrent reads.
func TestVocabulariesAreDefensivelyCopied(t *testing.T) {
	k := evidenceIndex(t)

	vocab := k.Vocabularies()
	if len(vocab.Claim.ConfidenceLevels) == 0 {
		t.Fatal("claim vocabulary is empty")
	}
	vocab.Claim.ConfidenceLevels[0] = "tampered"

	if got := k.Vocabularies().Claim.ConfidenceLevels[0]; got != "high" {
		t.Errorf("confidence_levels[0] = %q after a caller mutated its copy, want %q", got, "high")
	}
}

func TestProjectReportsTheEvidenceLayer(t *testing.T) {
	project := evidenceIndex(t).Project()

	if got, want := project.Counts.Claims, 4; got != want {
		t.Errorf("claims = %d, want %d", got, want)
	}
	if !contains(project.CanonicalLayer, "claims") {
		t.Errorf("canonical_layers_served = %v, want it to include claims", project.CanonicalLayer)
	}
	if len(project.Vocabulary.Claim.ClaimTypes) == 0 || len(project.Vocabulary.Source.Types) == 0 {
		t.Error("the project summary must publish the filter vocabularies so a client need not guess them")
	}
}
