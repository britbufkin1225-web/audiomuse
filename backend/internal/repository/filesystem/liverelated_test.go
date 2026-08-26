package filesystem_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
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

	starts, widest, widestRef, widestEvidence, widestScan := 0, 0, "", 0, 0
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
			if result.Bounds.RelationsScanned > widestScan {
				widestScan = result.Bounds.RelationsScanned
			}
			// The scan ceiling is declared with headroom over this corpus, so no canonical record
			// may reach it. A start that did would mean the eligible count it reported was a
			// floor, and the widest-scan number logged below is what a later reader checks that
			// headroom against as the encyclopedia grows.
			if result.Bounds.RelationsTruncated {
				t.Errorf("%s/%s scanned the ceiling of %d relations, so its eligible count is a floor",
					class, id, result.Bounds.MaxRelationsScanned)
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
	t.Logf("canonical corpus: %d discovery starts, widest result %d items at %s, widest evidence %d connections, widest scan %d of %d relations",
		starts, widest, widestRef, widestEvidence, widestScan, service.MaxRelatedRelationsScanned)
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
				"?relationship_types=conceptual,evidential",
				"?entity_types=claim&relationship_types=assertional",
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
		"/api/v1/related/node/frequency?relationship_types=conceptual,,evidential",
		"/api/v1/related/node/frequency?relationship_types=widget",
		"/api/v1/related/node/frequency?relationship_types=",
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

// TestLiveCorpusRelationshipScopePartitionsTheRealCorpus is the Phase 2B corpus-scale assertion.
//
// The fixture proves the filtering rule; this proves the rule holds over records the fixture does
// not contain, including the hub records where a single destination is reached by several
// canonical fields at once. For every discovery start the repository offers, the seven one-class
// results must together account for the unfiltered result exactly: every unfiltered connection
// appears in the result for its own class and in no other, and every destination the unfiltered
// result names is named by at least one class.
//
// It is the strongest available statement that the filter only ever removes. A class-scoped
// response carrying a connection the unfiltered response does not have would mean the filter
// invented one; a destination no class reaches would mean it silently dropped one.
func TestLiveCorpusRelationshipScopePartitionsTheRealCorpus(t *testing.T) {
	k := liveIndex(t)

	checked, filtered := 0, 0
	for _, class := range domain.SearchEntityTypes {
		for _, id := range liveRelatedStarts(t, k, class) {
			full, err := k.RelatedKnowledgeFor(string(class), id,
				service.RelatedQuery{Limit: service.MaxRelatedLimit})
			if err != nil {
				t.Fatalf("%s/%s: %v", class, id, err)
			}
			if full.RelationshipTypes != nil {
				t.Fatalf("%s/%s: an unfiltered request echoed a relationship scope", class, id)
			}
			checked++

			// The comparison below reads the unfiltered response as the complete set of this
			// record's connections, which it is only while nothing was cut. A start whose result
			// hit the item ceiling, and an item whose explanation hit the evidence cap, each
			// report a prefix rather than a total, and a class-scoped request may then surface a
			// connection the unfiltered response legitimately did not carry. Those cases are
			// excluded from the subset assertions rather than asserted loosely, because a weaker
			// assertion here would stop failing for the bug it exists to catch. Against the
			// corpus today neither cap binds, so nothing is skipped; the guards are what keep
			// this test correct as the encyclopedia grows past them.
			unfiltered := make(map[string]bool)
			destinations := make(map[string]bool)
			complete := !full.Truncated
			for _, item := range full.Items {
				if item.EvidenceTruncated {
					complete = false
				}
				destinations[string(item.EntityType)+"/"+item.ID] = true
				for _, reason := range append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...) {
					unfiltered[liveReasonKey(item, reason)] = true
				}
			}

			reached := make(map[string]bool, len(destinations))
			for _, priority := range domain.RelatedPriorityNames() {
				scoped, err := k.RelatedKnowledgeFor(string(class), id, service.RelatedQuery{
					Limit:             service.MaxRelatedLimit,
					RelationshipTypes: []string{priority},
				})
				if err != nil {
					t.Fatalf("%s/%s scope %s: %v", class, id, priority, err)
				}
				filtered++
				if !reflect.DeepEqual(scoped.RelationshipTypes, []string{priority}) {
					t.Errorf("%s/%s: relationship_types = %v, want [%s]",
						class, id, scoped.RelationshipTypes, priority)
				}
				// A filter may not enlarge the work behind an answer.
				if scoped.Bounds != full.Bounds {
					t.Errorf("%s/%s scope %s: bounds = %+v, want the unfiltered %+v",
						class, id, priority, scoped.Bounds, full.Bounds)
				}
				for _, item := range scoped.Items {
					ref := string(item.EntityType) + "/" + item.ID
					if complete && !destinations[ref] {
						t.Errorf("%s/%s scope %s reached %s, which the unfiltered result does not name",
							class, id, priority, ref)
					}
					reached[ref] = true
					for _, reason := range append([]domain.RelatedReason{item.Reason}, item.AdditionalEvidence...) {
						if string(reason.Priority) != priority {
							t.Errorf("%s/%s scope %s: %s carries a %s connection",
								class, id, priority, ref, reason.Priority)
						}
						if complete && !unfiltered[liveReasonKey(item, reason)] {
							t.Errorf("%s/%s scope %s: %s is not an unfiltered connection",
								class, id, priority, liveReasonKey(item, reason))
						}
						// Every result explains itself, and the sentence is the one the class
						// declares rather than anything derived from the records involved.
						if want := domain.RelatedPriorityExplanation(reason.Priority); reason.Explanation != want {
							t.Errorf("%s/%s: %s explained as %q, want %q",
								class, id, ref, reason.Explanation, want)
						}
					}
				}
			}
			// Every destination the unfiltered result names must be reachable by at least one
			// class, or a filter dropped a record that satisfies it. This half holds even when
			// the unfiltered result was cut, because a class-scoped result is never shorter than
			// its share of a prefix — but the evidence cap can hide the only class that reaches a
			// destination, so the guard applies here too.
			if complete {
				for ref := range destinations {
					if !reached[ref] {
						t.Errorf("%s/%s: destination %s is reached by no declared class, so a filter dropped it",
							class, id, ref)
					}
				}
			}
		}
	}
	if checked == 0 || filtered == 0 {
		t.Fatal("the canonical repository offered no discovery start, so this test would pass vacuously")
	}
	t.Logf("canonical corpus: %d starts checked against %d class-scoped discoveries", checked, filtered)
}

// liveReasonKey identifies one connection by all four canonical facts plus its destination, which
// is the granularity the context layer deduplicates at.
func liveReasonKey(item domain.RelatedItem, reason domain.RelatedReason) string {
	return string(item.EntityType) + "/" + item.ID + "|" + reason.Relation + "|" +
		reason.Origin + "|" + string(reason.Priority) + "|" + strconv.FormatBool(reason.Derived)
}
