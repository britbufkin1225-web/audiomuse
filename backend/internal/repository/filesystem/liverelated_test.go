package filesystem_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/httpapi"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 2A checks against the real corpus.
//
// Every other related-knowledge test runs against the synthetic fixture, so that a canonical
// content change cannot silently alter a unit-test expectation. They exist for the questions
// the fixture cannot answer: whether the precedence table covers the canonical fields the actual
// repository uses, whether the declared bounds are the right size for it, and whether the new
// route leaves the corpus untouched. They skip when the canonical repository is not present, so
// the suite still runs from a copy of backend/ on its own.

// liveRelatedStarts returns every record of one searchable class the live index holds.
//
// Each class is enumerated through its own list projection, at the paging ceiling, so the set is
// the complete canonical set rather than whatever a lexical query happened to reach. Enumerating
// by walking the corpus directory instead would let this test disagree with the contract it is
// checking, since a record the loader skipped is not a discovery start either.
func liveRelatedStarts(t testing.TB, k *service.Knowledge, class domain.SearchEntityType) []string {
	t.Helper()
	out := []string{}
	switch class {
	case domain.SearchSession:
		for _, record := range k.ListSessions(service.SessionQuery{Limit: service.MaxLimit}).Sessions {
			out = append(out, record.ID)
		}
	case domain.SearchNode:
		for _, record := range k.ListNodes(service.NodeQuery{Limit: service.MaxLimit}).Nodes {
			out = append(out, record.ID)
		}
	case domain.SearchClaim:
		list, err := k.ListClaims(service.ClaimQuery{Limit: service.MaxLimit})
		if err != nil {
			t.Fatalf("list claims: %v", err)
		}
		for _, record := range list.Claims {
			out = append(out, record.ID)
		}
	case domain.SearchSource:
		list, err := k.ListSources(service.SourceQuery{Limit: service.MaxLimit})
		if err != nil {
			t.Fatalf("list sources: %v", err)
		}
		for _, record := range list.Sources {
			out = append(out, record.ID)
		}
	case domain.SearchVocabulary:
		list, err := k.ListVocabulary(service.VocabularyQuery{Limit: service.MaxLimit})
		if err != nil {
			t.Fatalf("list vocabulary: %v", err)
		}
		for _, record := range list.Vocabulary {
			out = append(out, record.ID)
		}
	case domain.SearchExperiment:
		list, err := k.ListExperiments(service.ExperimentQuery{Limit: service.MaxLimit})
		if err != nil {
			t.Fatalf("list experiments: %v", err)
		}
		for _, record := range list.Experiments {
			out = append(out, record.ID)
		}
	default:
		t.Fatalf("no live enumeration for searchable class %s", class)
	}
	if len(out) == 0 {
		t.Fatalf("the canonical repository holds no %s record", class)
	}
	return out
}

// liveIndex builds the index over the canonical repository, or skips.
func liveIndex(t testing.TB) *service.Knowledge {
	t.Helper()
	repo, err := filesystem.New(liveRepoRoot(t))
	if err != nil {
		t.Fatalf("open canonical repository: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}
	return knowledge
}

// TestLiveCorpusRelatedKnowledgeIsClassifiedAndBounded is the corpus-scale assertion.
//
// It walks every discovery start the real repository offers — every session, node, claim, source,
// vocabulary entry and experiment definition — and requires four things of each result: it
// resolves, every reason names a canonical field the precedence table classifies, no item is the
// record it started from, and every item is a record the index can name. A canonical field
// reaching the discovery contract without a precedence decision would surface here as an
// unclassified reason rather than as a suggestion in a quietly wrong place.
//
// The bounds are checked against the corpus rather than merely declared: the widest result is
// reported so a later reader can see how much headroom the ceiling has, and the request cost is
// held to one map lookup and a sort by the absence of any per-request corpus read.
func TestLiveCorpusRelatedKnowledgeIsClassifiedAndBounded(t *testing.T) {
	k := liveIndex(t)

	starts, widest, widestRef, widestEvidence := 0, 0, "", 0
	for _, class := range domain.SearchEntityTypes {
		ids := liveRelatedStarts(t, k, class)
		if len(ids) == 0 {
			continue
		}
		for _, id := range ids {
			result, err := k.RelatedKnowledgeFor(string(class), id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
			if err != nil {
				t.Fatalf("%s/%s: %v", class, id, err)
			}
			starts++
			if result.Counts.Eligible > widest {
				widest, widestRef = result.Counts.Eligible, string(class)+"/"+id
			}
			if len(result.Items) > service.MaxRelatedLimit {
				t.Fatalf("%s/%s returned %d items, past the ceiling", class, id, len(result.Items))
			}
			for _, item := range result.Items {
				if item.EntityType == class && item.ID == id {
					t.Errorf("%s/%s was offered as related knowledge about itself", class, id)
				}
				if item.ID == "" || item.Title == "" {
					t.Errorf("%s/%s produced an unnamed item %+v", class, id, item)
				}
				if item.EvidenceCount > widestEvidence {
					widestEvidence = item.EvidenceCount
				}
				reasons := append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...)
				if len(reasons) > service.MaxRelatedEvidencePerItem {
					t.Errorf("%s/%s -> %s/%s carried %d reasons, past the cap",
						class, id, item.EntityType, item.ID, len(reasons))
				}
				for _, reason := range reasons {
					if reason.Priority == domain.PriorityUnclassified {
						t.Errorf("%s/%s -> %s/%s is explained by unclassified canonical field %q",
							class, id, item.EntityType, item.ID, reason.Origin)
					}
				}
			}
		}
	}
	if starts == 0 {
		t.Fatal("the canonical repository offered no discovery start, so this test would pass vacuously")
	}
	t.Logf("canonical corpus: %d discovery starts, widest result %d items at %s, widest evidence %d connections",
		starts, widest, widestRef, widestEvidence)
}

// TestLiveCorpusRelatedKnowledgeIsDeterministic asserts the real corpus, not just the fixture,
// produces one answer.
//
// The fixture is small enough that an accidental ordering could survive it. The canonical
// repository has hub records referenced by dozens of others, which is where an unordered read
// would actually show, so every start is requested twice and the two results must be identical
// values.
func TestLiveCorpusRelatedKnowledgeIsDeterministic(t *testing.T) {
	k := liveIndex(t)

	for _, class := range domain.SearchEntityTypes {
		for _, id := range liveRelatedStarts(t, k, class) {
			first, err := k.RelatedKnowledgeFor(string(class), id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
			if err != nil {
				t.Fatalf("%s/%s: %v", class, id, err)
			}
			again, err := k.RelatedKnowledgeFor(string(class), id, service.RelatedQuery{Limit: service.MaxRelatedLimit})
			if err != nil {
				t.Fatalf("%s/%s: %v", class, id, err)
			}
			if !reflect.DeepEqual(first, again) {
				t.Fatalf("%s/%s changed between two identical requests", class, id)
			}
		}
	}
}

// TestLiveRepositoryIsNotMutatedByRelatedKnowledgeRequests is the no-write assertion for the new
// route, checked the way the earlier phases check theirs: hash the corpus, serve, hash again.
//
// The requests below are the broadest the route can make — every start class, the ceiling limit,
// a scoped variant — plus the refusals, because a rejected request must not write either.
func TestLiveRepositoryIsNotMutatedByRelatedKnowledgeRequests(t *testing.T) {
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

	served := 0
	for _, class := range domain.SearchEntityTypes {
		for _, id := range liveRelatedStarts(t, knowledge, class) {
			for _, query := range []string{
				"?limit=100",
				"?entity_types=node,claim,source",
				"",
			} {
				target := "/api/v1/related/" + string(class) + "/" + id + query
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
				if rec.Code != http.StatusOK {
					t.Errorf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
				}
				served++
			}
		}
	}
	if served == 0 {
		t.Fatal("no discovery request was served, so this test would pass vacuously")
	}

	for _, target := range []string{
		"/api/v1/related/experiment_run/anything",
		"/api/v1/related/node/no-such-node-exists-here",
		"/api/v1/related/node/frequency?entity_types=node,,claim",
		"/api/v1/related/node/frequency?limit=-1",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code == http.StatusOK {
			t.Errorf("GET %s = 200, want a refusal", target)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/related/node/frequency", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /api/v1/related/node/frequency = %d, want 405", method, rec.Code)
		}
	}

	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Error("serving related-knowledge discovery changed the canonical repository")
	}
}
