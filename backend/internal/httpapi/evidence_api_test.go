package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

func TestSourcesList(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/sources")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body service.SourceList
	decode(t, rec, &body)
	if got, want := body.Page.Total, 6; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := body.Sources[0].ID, "fixture-archive-record"; got != want {
		t.Errorf("first source = %q, want %q (canonical ID order)", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/sources?type=session")
	decode(t, rec, &body)
	if got, want := body.Page.Total, 2; got != want {
		t.Errorf("type-filtered total = %d, want %d", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/sources?claim_id=beta-was-observed-in-1999&retrieval=full_text")
	decode(t, rec, &body)
	if got, want := body.Page.Total, 1; got != want {
		t.Fatalf("AND-composed total = %d, want %d", got, want)
	}
	if got, want := body.Sources[0].ID, "fixture-archive-record"; got != want {
		t.Errorf("AND-composed source = %q, want %q", got, want)
	}

	rec = do(t, handler, http.MethodGet, "/api/v1/sources?session_id=session-01-fixture")
	decode(t, rec, &body)
	if got, want := body.Page.Total, 3; got != want {
		t.Errorf("session-filtered total = %d, want %d", got, want)
	}
}

func TestSourceByID(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/sources/fixture-reference-work")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var detail domain.SourceDetail
	decode(t, rec, &detail)

	if detail.ID != "fixture-reference-work" || detail.Type != "book" {
		t.Errorf("source = %#v", detail.Source)
	}
	if got, want := len(detail.Claims), 4; got != want {
		t.Fatalf("claims = %d, want %d", got, want)
	}
	if detail.Claims[2].Relation != "contradicted_by" {
		t.Errorf("claims = %#v, want the evidence relation carried on each entry", detail.Claims)
	}
	// Empty derived lists must render as [] rather than null.
	if !strings.Contains(rec.Body.String(), `"attributed_claim_ids":[]`) {
		t.Errorf("empty attributed_claim_ids did not render as []: %s", rec.Body.String())
	}
}

func TestSourceByIDUnknownReturns404(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/sources/no-such-source")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec, "source_not_found")
}

func TestClaimsList(t *testing.T) {
	handler := newHandler(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/claims")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body service.ClaimList
	decode(t, rec, &body)
	if got, want := body.Page.Total, 4; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
	if got, want := body.Claims[0].ID, "alpha-carries-energy"; got != want {
		t.Errorf("first claim = %q, want %q (canonical ID order)", got, want)
	}

	for _, tc := range []struct {
		target string
		total  int
	}{
		{"/api/v1/claims?confidence=high", 1},
		{"/api/v1/claims?claim_type=hypothesis", 1},
		{"/api/v1/claims?dispute_status=disputed", 1},
		{"/api/v1/claims?temporal_precision=year", 1},
		{"/api/v1/claims?relation=contradicted_by", 1},
		{"/api/v1/claims?source_id=fixture-archive-record", 3},
		{"/api/v1/claims?node_id=gamma", 2},
		{"/api/v1/claims?session_id=session-01-fixture", 1},
		{"/api/v1/claims?source_id=fixture-reference-work&relation=qualified_by", 2},
		{"/api/v1/claims?q=1999", 1},
		{"/api/v1/claims?node_id=no-such-node", 0},
	} {
		t.Run(tc.target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, tc.target)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var body service.ClaimList
			decode(t, rec, &body)
			if body.Page.Total != tc.total {
				t.Errorf("total = %d, want %d", body.Page.Total, tc.total)
			}
		})
	}
}

func TestClaimByID(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/claims/beta-was-observed-in-1999")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var detail domain.ClaimDetail
	decode(t, rec, &detail)

	if detail.ID != "beta-was-observed-in-1999" {
		t.Fatalf("claim = %#v", detail.Claim)
	}
	if detail.Confidence != "moderate" || detail.ConfidenceBasis == "" {
		t.Error("the confidence level and its stated basis must both be served; a level alone is the flattening the claim layer prevents")
	}
	if len(detail.Evidence) != 2 || detail.Evidence[1].Relation != "contradicted_by" {
		t.Errorf("evidence = %#v", detail.Evidence)
	}
	if len(detail.SourceIDs) != 3 || len(detail.NodeIDs) != 1 || len(detail.SessionIDs) != 1 {
		t.Errorf("flattened ID lists = %#v", detail)
	}
	// Provenance must never be collapsed into a boolean.
	if strings.Contains(rec.Body.String(), `"verified"`) {
		t.Error("the claim projection introduced a verified flag")
	}
}

func TestClaimByIDUnknownReturns404(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/claims/no-such-claim")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	assertErrorCode(t, rec, "claim_not_found")
}

// TestInvalidEvidenceFilters: an out-of-vocabulary filter value is refused with 400 rather
// than answered with an empty list a caller would read as "no such records".
func TestInvalidEvidenceFilters(t *testing.T) {
	handler := newHandler(t)

	for _, target := range []string{
		"/api/v1/claims?confidence=very-high",
		"/api/v1/claims?confidence=High",
		"/api/v1/claims?claim_type=disputed_claim",
		"/api/v1/claims?dispute_status=contested",
		"/api/v1/claims?temporal_precision=decade",
		"/api/v1/claims?relation=mentions",
		"/api/v1/sources?type=blog",
		"/api/v1/sources?relationship=related",
		"/api/v1/sources?evidence_class=hearsay",
		"/api/v1/sources?retrieval=skimmed",
		"/api/v1/claims?unknown=1",
		"/api/v1/sources?unknown=1",
		"/api/v1/claims?limit=abc",
		"/api/v1/sources?limit=-1",
		"/api/v1/claims?confidence=high&confidence=low",
		"/api/v1/sources/",
	} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, target)
			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 400 or 404", rec.Code)
			}
			if rec.Code == http.StatusBadRequest {
				assertErrorCode(t, rec, "invalid_query")
			}
		})
	}
}

// TestInvalidFilterErrorNamesTheVocabulary: the 400 body must say what the accepted values
// are, and must not echo the caller's own value back into the response.
func TestInvalidFilterErrorNamesTheVocabulary(t *testing.T) {
	rec := do(t, newHandler(t), http.MethodGet, "/api/v1/claims?confidence=%3Cscript%3E")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "moderate") || !strings.Contains(body, "unknown") {
		t.Errorf("body = %s, want the canonical confidence vocabulary", body)
	}
	if strings.Contains(body, "script") {
		t.Errorf("body echoed the caller-supplied value: %s", body)
	}
}

// TestEvidenceRoutesAreReadOnly asserts the method lock covers the new routes too.
func TestEvidenceRoutesAreReadOnly(t *testing.T) {
	handler := newHandler(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, target := range []string{
			"/api/v1/sources", "/api/v1/sources/fixture-reference-work",
			"/api/v1/claims", "/api/v1/claims/alpha-carries-energy",
		} {
			rec := do(t, handler, method, target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, target, rec.Code)
			}
			if got, want := rec.Header().Get("Allow"), "GET, HEAD"; got != want {
				t.Errorf("%s %s Allow = %q, want %q", method, target, got, want)
			}
		}
	}
}

// TestEvidenceResponsesDoNotLeakTheRepositoryPath: an absolute filesystem path identifies
// the operator's machine and must never reach a response body. Claim records carry a
// repository-relative path field, which is deliberately different.
func TestEvidenceResponsesDoNotLeakTheRepositoryPath(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{
		"/api/v1/sources", "/api/v1/sources/fixture-reference-work",
		"/api/v1/claims", "/api/v1/claims/alpha-carries-energy",
	} {
		rec := do(t, handler, http.MethodGet, target)
		if strings.Contains(rec.Body.String(), corpusRootForLeakCheck(t)) {
			t.Errorf("%s disclosed the absolute repository path", target)
		}
	}
}

// corpusRootForLeakCheck returns the absolute fixture path the responses must not contain.
func corpusRootForLeakCheck(t testing.TB) string {
	t.Helper()
	return testsupport.CorpusRoot(t)
}
