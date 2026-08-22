package filesystem_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

const fixtureClaimsPath = "claims/records/fixture-claims.yaml"

func loadCorpus(t testing.TB, fsys fstest.MapFS) (*repository.Corpus, *domain.ValidationReport) {
	t.Helper()
	repo, err := filesystem.NewFromFS(fsys, testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open fixture corpus: %v", err)
	}
	corpus, report, err := repo.Load(context.Background())
	if err != nil {
		t.Fatalf("load fixture corpus: %v", err)
	}
	return corpus, report
}

func claimByID(t testing.TB, corpus *repository.Corpus, id string) domain.Claim {
	t.Helper()
	for _, claim := range corpus.Claims {
		if claim.ID == id {
			return claim
		}
	}
	t.Fatalf("claim %s was not loaded", id)
	return domain.Claim{}
}

// writeClaims replaces the fixture claim record file with a stream of the supplied records.
func writeClaims(corpus fstest.MapFS, records ...string) {
	testsupport.Write(corpus, fixtureClaimsPath, strings.Join(records, ""))
}

func TestClaimRecordsParse(t *testing.T) {
	corpus, report := loadCorpus(t, testsupport.MutableCorpus(t))
	if report.HasFatal() {
		t.Fatalf("valid fixture corpus reported fatal issues: %v", report.Fatal())
	}

	claim := claimByID(t, corpus, "beta-was-observed-in-1999")

	if got, want := claim.ClaimType, "historical_claim"; got != want {
		t.Errorf("claim_type = %q, want %q", got, want)
	}
	if got, want := claim.Confidence, "moderate"; got != want {
		t.Errorf("confidence = %q, want %q", got, want)
	}
	if got, want := claim.DisputeStatus, "disputed"; got != want {
		t.Errorf("dispute_status = %q, want %q", got, want)
	}
	if got, want := claim.TemporalPrecision, "year"; got != want {
		t.Errorf("temporal_precision = %q, want %q", got, want)
	}
	if claim.ConfidenceBasis == "" {
		t.Error("confidence_basis was not captured; a confidence level without its stated reason is the flattening the claim layer exists to prevent")
	}
	if got, want := claim.Path, fixtureClaimsPath; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}

	wantEvidence := []domain.ClaimEvidence{
		{Relation: "supported_by", SourceID: "fixture-archive-record", Note: "Dates the observation to 1999."},
		{Relation: "contradicted_by", SourceID: "fixture-reference-work", Note: "Dates the same observation to 1998."},
	}
	if !reflect.DeepEqual(claim.Evidence, wantEvidence) {
		t.Errorf("evidence = %#v, want %#v (declared order preserved)", claim.Evidence, wantEvidence)
	}
	wantAttribution := []domain.ClaimAttribution{
		{Actor: "Fixture Archivist", SourceID: "fixture-attribution-source"},
	}
	if !reflect.DeepEqual(claim.Attribution, wantAttribution) {
		t.Errorf("attribution = %#v, want %#v", claim.Attribution, wantAttribution)
	}
	wantAppearsIn := []domain.ClaimReference{
		{Kind: "node", Ref: "beta"},
		{Kind: "session", Ref: "session-01-fixture"},
	}
	if !reflect.DeepEqual(claim.AppearsIn, wantAppearsIn) {
		t.Errorf("appears_in = %#v, want %#v", claim.AppearsIn, wantAppearsIn)
	}
}

// TestClaimDerivationIsPreserved guards the separation the provenance model insists on:
// derived_from records what a claim was built from and is never merged into evidence.
func TestClaimDerivationIsPreserved(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	claim := claimByID(t, corpus, "gamma-follows-from-alpha-and-beta")

	want := []domain.ClaimReference{
		{Kind: "claim", Ref: "alpha-carries-energy"},
		{Kind: "node", Ref: "alpha"},
	}
	if !reflect.DeepEqual(claim.DerivedFrom, want) {
		t.Errorf("derived_from = %#v, want %#v", claim.DerivedFrom, want)
	}
	if len(claim.OpenQuestions) != 1 {
		t.Errorf("open_questions = %#v, want one entry", claim.OpenQuestions)
	}
}

// TestEmptyClaimCollectionsProjectAsEmptySlices guards the JSON contract: a canonically
// empty list must serialise as [] and never as null.
func TestEmptyClaimCollectionsProjectAsEmptySlices(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))
	claim := claimByID(t, corpus, "alpha-carries-energy")

	if claim.Attribution == nil || claim.DerivedFrom == nil || claim.OpenQuestions == nil {
		t.Fatalf("empty claim collections are nil: %#v", claim)
	}
	if len(claim.Attribution) != 0 || len(claim.DerivedFrom) != 0 || len(claim.OpenQuestions) != 0 {
		t.Errorf("expected empty collections, got %#v", claim)
	}
}

func TestContractVocabulariesAreReadFromTheSchemas(t *testing.T) {
	corpus, _ := loadCorpus(t, testsupport.MutableCorpus(t))

	claim := corpus.Vocabularies.Claim
	if got, want := len(claim.ClaimTypes), 9; got != want {
		t.Errorf("claim_types = %d, want %d", got, want)
	}
	if got, want := claim.ConfidenceLevels, []string{"high", "moderate", "low", "unknown"}; !reflect.DeepEqual(got, want) {
		t.Errorf("confidence_levels = %#v, want %#v (declared order preserved)", got, want)
	}
	if got, want := claim.EvidenceRelations, []string{"supported_by", "contradicted_by", "qualified_by"}; !reflect.DeepEqual(got, want) {
		t.Errorf("evidence_relations = %#v, want %#v", got, want)
	}

	source := corpus.Vocabularies.Source
	if got, want := len(source.Types), 13; got != want {
		t.Errorf("source types = %d, want %d", got, want)
	}
	if got, want := source.Retrievals, []string{"full_text", "partial_text", "citation_only"}; !reflect.DeepEqual(got, want) {
		t.Errorf("retrievals = %#v, want %#v", got, want)
	}
}

// TestClaimOrderingIsCanonicalAndStable runs the loader twice and requires identical
// output. Nothing may depend on map iteration or on operating-system directory ordering.
func TestClaimOrderingIsCanonicalAndStable(t *testing.T) {
	first, firstReport := loadCorpus(t, testsupport.MutableCorpus(t))
	second, secondReport := loadCorpus(t, testsupport.MutableCorpus(t))

	if !reflect.DeepEqual(first.Claims, second.Claims) {
		t.Error("claim ordering differed between two loads of an unchanged corpus")
	}
	if !reflect.DeepEqual(firstReport.Issues, secondReport.Issues) {
		t.Error("validation report ordering differed between two loads of an unchanged corpus")
	}

	want := []string{
		"alpha-carries-energy",
		"alpha-may-extend-to-gamma",
		"beta-was-observed-in-1999",
		"gamma-follows-from-alpha-and-beta",
	}
	got := make([]string, 0, len(first.Claims))
	for _, claim := range first.Claims {
		got = append(got, claim.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("claim order = %v, want canonical ID order %v", got, want)
	}
}

// TestCorpusWithoutClaimsLoads confirms the evidence layer is additive: a corpus that has
// no claims/records/ directory is not an error.
func TestCorpusWithoutClaimsLoads(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	delete(corpus, fixtureClaimsPath)

	loaded, report := loadCorpus(t, corpus)
	if report.HasFatal() {
		t.Fatalf("a corpus with no claim records reported fatal issues: %v", report.Fatal())
	}
	if got := len(loaded.Claims); got != 0 {
		t.Errorf("claims = %d, want 0", got)
	}
	if loaded.Claims == nil {
		t.Error("claims is nil, want an empty slice")
	}
}

// TestClaimCitationCountsAsCitation checks the Phase 1B widening of the uncited warning: a
// source no node lists topically but that several claims depend on is not uncited.
func TestClaimCitationCountsAsCitation(t *testing.T) {
	_, report := loadCorpus(t, testsupport.MutableCorpus(t))

	uncited := map[string]bool{}
	for _, issue := range report.Warnings() {
		if issue.Code == domain.CodeUncitedSource {
			uncited[issue.Ref] = true
		}
	}
	if uncited["fixture-archive-record"] {
		t.Error("fixture-archive-record is cited as claim evidence and must not be reported uncited")
	}
	if uncited["fixture-attribution-source"] {
		t.Error("fixture-attribution-source is cited as claim attribution and must not be reported uncited")
	}
	if !uncited["fixture-uncited-source"] {
		t.Error("fixture-uncited-source is cited by nothing and must still be reported uncited")
	}
}

// TestMissingAppearanceDocumentIsAWarning: an appearance site that is safe, canonical and
// simply absent is a corpus gap for a human, not a reason to refuse to serve the corpus.
func TestMissingAppearanceDocumentIsAWarning(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	writeClaims(corpus, testsupport.ValidClaim(
		"delta-appears-nowhere-real", "technical_fact", "moderate", "undisputed",
		testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
		`[{"kind": "document", "ref": "docs/fixture-not-written-yet.md"}]`))

	_, report := loadCorpus(t, corpus)
	if report.HasFatal() {
		t.Fatalf("an absent appearance document must not be fatal: %v", report.Fatal())
	}
	for _, issue := range report.Warnings() {
		if issue.Code == domain.CodeUnresolvedDocument {
			return
		}
	}
	t.Fatalf("want a %s warning; got %v", domain.CodeUnresolvedDocument, report.Warnings())
}

// TestClaimFatalDefects drives one evidence-integrity defect at a time through the real
// loader. Each case states the code it must fail with, so a fixture rejected for an
// unrelated reason shows up as a broken test rather than as a pass.
func TestClaimFatalDefects(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(fstest.MapFS)
		wantErr string
	}{
		{
			name: "blank claim id",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidID,
		},
		{
			name: "non-canonical claim id",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("Delta_Claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidID,
		},
		{
			name: "duplicate claim id across files",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "claims/records/fixture-duplicate.yaml",
					testsupport.ValidClaim("alpha-carries-energy", "technical_fact", "moderate", "undisputed",
						testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeDuplicateID,
		},
		{
			name: "duplicate claim id within one stream",
			mutate: func(c fstest.MapFS) {
				record := testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha"))
				writeClaims(c, record, record)
			},
			wantErr: domain.CodeDuplicateID,
		},
		{
			name: "missing required field",
			mutate: func(c fstest.MapFS) {
				record := testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha"))
				writeClaims(c, replaceOnce(record, "open_questions: []\n", ""))
			},
			wantErr: domain.CodeMissingField,
		},
		{
			name: "empty statement",
			mutate: func(c fstest.MapFS) {
				record := testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha"))
				writeClaims(c, replaceOnce(record, `statement: "Synthetic fixture statement for delta-claim."`, `statement: ""`))
			},
			wantErr: domain.CodeMissingField,
		},
		{
			name: "unknown top-level field",
			mutate: func(c fstest.MapFS) {
				record := testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha"))
				writeClaims(c, record+"verified: true\n")
			},
			wantErr: domain.CodeUnknownField,
		},
		{
			name: "malformed yaml",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, "---\nid: delta-claim\n  statement: [unclosed\n")
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "evidence entry declares an extra key",
			mutate: func(c fstest.MapFS) {
				evidence := `[{"relation": "supported_by", "source_id": "fixture-reference-work", "note": "n", "weight": 0.9}]`
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					evidence, "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "evidence entry is not a mapping",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					`["fixture-reference-work"]`, "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "evidence note is empty",
			mutate: func(c fstest.MapFS) {
				evidence := `[{"relation": "supported_by", "source_id": "fixture-reference-work", "note": ""}]`
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					evidence, "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeMissingField,
		},
		{
			name: "invalid claim type",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "disputed_claim", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "invalid confidence value",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "very-high", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "confidence differs only in case",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "Moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "invalid dispute status",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "contested",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "invalid evidence relation",
			mutate: func(c fstest.MapFS) {
				evidence := `[{"relation": "mentions", "source_id": "fixture-reference-work", "note": "n"}]`
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					evidence, "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "invalid appears_in kind",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "index", "ref": "alpha"}]`))
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "claim cites a source that is not registered",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("not-registered"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeUnresolvedSource,
		},
		{
			name: "source id differs only in case",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("Fixture-Reference-Work"), "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeUnresolvedSource,
		},
		{
			name: "attribution cites a source that is not registered",
			mutate: func(c fstest.MapFS) {
				attribution := `[{"actor": "Fixture Actor", "source_id": "not-registered"}]`
				writeClaims(c, testsupport.ValidClaim("delta-claim", "attributed_claim", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), attribution, "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeUnresolvedSource,
		},
		{
			name: "claim appears in a node that does not exist",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", testsupport.AppearsInNode("nowhere")))
			},
			wantErr: domain.CodeUnresolvedTarget,
		},
		{
			name: "claim appears in a session that is not registered",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "session", "ref": "session-99-missing"}]`))
			},
			wantErr: domain.CodeUnresolvedSession,
		},
		{
			name: "claim derives from a claim that does not exist",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "audiomuse_synthesis", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]",
					`[{"kind": "claim", "ref": "no-such-claim"}]`, testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeUnresolvedClaim,
		},
		{
			name: "claim derives from a node that does not exist",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "audiomuse_synthesis", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]",
					`[{"kind": "node", "ref": "nowhere"}]`, testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeUnresolvedTarget,
		},
		{
			name: "claim derives from itself",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "audiomuse_synthesis", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]",
					`[{"kind": "claim", "ref": "delta-claim"}]`, testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeClaimDerivationCycle,
		},
		{
			name: "claim derivation forms a two-record cycle",
			mutate: func(c fstest.MapFS) {
				writeClaims(c,
					testsupport.ValidClaim("delta-claim", "audiomuse_synthesis", "moderate", "undisputed",
						testsupport.SupportedBy("fixture-reference-work"), "[]",
						`[{"kind": "claim", "ref": "epsilon-claim"}]`, testsupport.AppearsInNode("alpha")),
					testsupport.ValidClaim("epsilon-claim", "audiomuse_synthesis", "moderate", "undisputed",
						testsupport.SupportedBy("fixture-reference-work"), "[]",
						`[{"kind": "claim", "ref": "delta-claim"}]`, testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeClaimDerivationCycle,
		},
		{
			name: "duplicate evidence entry",
			mutate: func(c fstest.MapFS) {
				evidence := `[{"relation": "supported_by", "source_id": "fixture-reference-work", "note": "a"},` +
					` {"relation": "supported_by", "source_id": "fixture-reference-work", "note": "b"}]`
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					evidence, "[]", "[]", testsupport.AppearsInNode("alpha")))
			},
			wantErr: domain.CodeDuplicateEvidence,
		},
		{
			name: "duplicate appearance site",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "node", "ref": "alpha"}, {"kind": "node", "ref": "alpha"}]`))
			},
			wantErr: domain.CodeDuplicateEvidence,
		},
		{
			name: "claim declares no appearance site",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]", "[]"))
			},
			wantErr: domain.CodeMissingAppearance,
		},
		{
			name: "appearance document escapes the repository root",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "document", "ref": "../../etc/passwd"}]`))
			},
			wantErr: domain.CodeUnsafePath,
		},
		{
			name: "appearance document is an external locator",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "document", "ref": "https://example.invalid/page"}]`))
			},
			wantErr: domain.CodeUnsafePath,
		},
		{
			name: "appearance document is a generated projection",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "document", "ref": "indexes/claim-index.md"}]`))
			},
			wantErr: domain.CodeGeneratedAppearance,
		},
		{
			name: "vocabulary reference is not a canonical identifier",
			mutate: func(c fstest.MapFS) {
				writeClaims(c, testsupport.ValidClaim("delta-claim", "technical_fact", "moderate", "undisputed",
					testsupport.SupportedBy("fixture-reference-work"), "[]", "[]",
					`[{"kind": "vocabulary", "ref": "Not A Term"}]`))
			},
			wantErr: domain.CodeInvalidID,
		},
		{
			name: "registry declares a source type outside the contract",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "sources/source-registry.yaml",
					"schema: audiomuse-source-registry\nversion: 1\nsources:\n"+
						"  - id: fixture-reference-work\n    type: blog\n    title: One\n"+
						"    locator: research/sources/fixture-reference-work.md\n    relationship: supporting\n")
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "registry declares a retrieval value outside the contract",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "sources/source-registry.yaml",
					"schema: audiomuse-source-registry\nversion: 1\nsources:\n"+
						"  - id: fixture-reference-work\n    type: book\n    title: One\n"+
						"    locator: research/sources/fixture-reference-work.md\n    relationship: supporting\n"+
						"    evidence_class: technical_reference\n    retrieval: skimmed\n")
			},
			wantErr: domain.CodeInvalidVocabulary,
		},
		{
			name: "claim contract is missing",
			mutate: func(c fstest.MapFS) {
				delete(c, "schemas/claim.schema.yaml")
			},
			wantErr: domain.CodeMalformedRecord,
		},
		{
			name: "source contract declares no vocabulary",
			mutate: func(c fstest.MapFS) {
				testsupport.Write(c, "schemas/source.schema.yaml", "schema: audiomuse-source\nversion: 1\nproperties: {}\n")
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

// TestLargeButValidRelationshipCollection checks that a claim citing every registered
// source and appearing in every node is loaded intact rather than truncated or reordered.
func TestLargeButValidRelationshipCollection(t *testing.T) {
	corpus := testsupport.MutableCorpus(t)
	sources := []string{
		"fixture-archive-record", "fixture-attribution-source", "fixture-reference-work",
		"fixture-uncited-source", "session-01-fixture", "session-02-unused",
	}
	evidence := make([]string, 0, len(sources))
	for _, id := range sources {
		evidence = append(evidence,
			`{"relation": "supported_by", "source_id": "`+id+`", "note": "Fixture support note."}`)
	}
	appears := `[{"kind": "node", "ref": "alpha"}, {"kind": "node", "ref": "beta"}, {"kind": "node", "ref": "gamma"}]`
	writeClaims(corpus, testsupport.ValidClaim("delta-cites-everything", "technical_fact", "moderate", "undisputed",
		"["+strings.Join(evidence, ", ")+"]", "[]", "[]", appears))

	loaded, report := loadCorpus(t, corpus)
	if report.HasFatal() {
		t.Fatalf("a large but valid claim reported fatal issues: %v", report.Fatal())
	}
	claim := claimByID(t, loaded, "delta-cites-everything")
	if got, want := len(claim.Evidence), len(sources); got != want {
		t.Errorf("evidence entries = %d, want %d", got, want)
	}
	if got, want := len(claim.AppearsIn), 3; got != want {
		t.Errorf("appearance sites = %d, want %d", got, want)
	}
}
