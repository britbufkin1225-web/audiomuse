package service_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 2D continuation tests.
//
// They assert three separable things. First, that a traversal is a partition: the pages of one
// paged request, concatenated, are exactly the prefix of the unpaged ordering that the same bounds
// allow, with no record repeated and none dropped at any seam. Second, that a token is bound to
// the request it was issued for: every scope component that could make it mean something else is
// checked, and a token presented against a changed one is refused rather than answered. Third,
// that an arbitrary string in the parameter is refused rather than trusted, cheaply, and without
// telling the sender anything about the encoding.
//
// The unpaged ordering is the oracle throughout. Nothing here asserts a page against a written-out
// list of records, because that would pin the fixture corpus rather than the contract; the
// assertion is always that paging produced what not paging produced.

// --------------------------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------------------------

// pagedRelated walks a whole traversal and returns the concatenated refs together with the number
// of pages it took.
//
// It fails rather than loops if the traversal exceeds the number of pages the eligible total could
// possibly need, so a contract change that made a token resume at or before its own position is a
// test failure instead of a hung suite. Each page's internal consistency is checked as it is read,
// because a token and a has_more that disagree is the failure most likely to be invisible in the
// concatenated result.
func pagedRelated(
	t testing.TB, k *service.Knowledge, class domain.SearchEntityType, id string, q service.RelatedQuery,
) ([]string, int) {
	t.Helper()

	var refs []string
	pages := 0
	token := ""
	for {
		page := q
		page.ContinuationToken = token
		result, err := k.RelatedKnowledgeFor(string(class), id, page)
		if err != nil {
			t.Fatalf("page %d of %s/%s: %v", pages+1, class, id, err)
		}
		pages++

		if result.HasMore != (result.NextContinuationToken != "") {
			t.Fatalf("page %d: has_more = %v beside token %q", pages, result.HasMore, result.NextContinuationToken)
		}
		if result.HasMore && !result.Truncated {
			t.Fatalf("page %d: has_more is true while truncated is false", pages)
		}
		if result.Counts.Returned != len(result.Items) {
			t.Fatalf("page %d: returned = %d, items = %d", pages, result.Counts.Returned, len(result.Items))
		}
		if len(result.Items) > result.Limit {
			t.Fatalf("page %d: %d items over a limit of %d", pages, len(result.Items), result.Limit)
		}
		if result.HasMore && len(result.Items) != result.Limit {
			t.Fatalf("page %d: %d items under the limit %d while claiming more follow",
				pages, len(result.Items), result.Limit)
		}
		refs = append(refs, relatedRefs(result)...)

		if !result.HasMore {
			return refs, pages
		}
		token = result.NextContinuationToken
		if pages > result.Counts.Eligible+1 {
			t.Fatalf("traversal did not terminate after %d pages of %d eligible items",
				pages, result.Counts.Eligible)
		}
	}
}

// firstPageToken returns the continuation token of one request's first page, failing if that
// request does not produce one.
func firstPageToken(
	t testing.TB, k *service.Knowledge, class domain.SearchEntityType, id string, q service.RelatedQuery,
) string {
	t.Helper()
	result := mustRelated(t, k, class, id, q)
	if result.NextContinuationToken == "" {
		t.Fatalf("related %s/%s %+v issued no continuation token", class, id, q)
	}
	return result.NextContinuationToken
}

// continuationError runs one request expected to be refused and returns the error.
func continuationError(
	t testing.TB, k *service.Knowledge, class domain.SearchEntityType, id string, q service.RelatedQuery,
) error {
	t.Helper()
	result, err := k.RelatedKnowledgeFor(string(class), id, q)
	if err == nil {
		t.Fatalf("related %s/%s %+v was answered with %d items, want a refusal",
			class, id, q, len(result.Items))
	}
	return err
}

// decodedToken renders a token's payload as a generic map, so a test can corrupt one field of a
// token this build actually issued rather than hand-writing a payload that may not resemble one.
func decodedToken(t testing.TB, token string) map[string]any {
	t.Helper()
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal token payload: %v", err)
	}
	return out
}

// encodedToken is the inverse, for a payload a test has edited.
func encodedToken(t testing.TB, payload map[string]any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal token payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(body)
}

// --------------------------------------------------------------------------------------------
// A. The first page
// --------------------------------------------------------------------------------------------

// TestRelatedContinuationFirstPageIsUnchangedWithoutAToken is the compatibility assertion.
//
// A caller who never sends a token must receive exactly the Phase 2A response, and the two new
// fields must not change what any old field says. The token-free result is compared against itself
// field by field rather than by prose: nothing follows a complete page, and a page that carried
// the whole eligible set is not truncated.
func TestRelatedContinuationFirstPageIsUnchangedWithoutAToken(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		t.Run(string(start.class), func(t *testing.T) {
			result := relatedOf(t, k, start.class, start.id)
			if result.Counts.Returned != result.Counts.Eligible {
				t.Fatalf("the fixture's %s start does not fit one default page: %d of %d",
					start.class, result.Counts.Returned, result.Counts.Eligible)
			}
			if result.HasMore {
				t.Error("a complete first page reports that more follow")
			}
			if result.NextContinuationToken != "" {
				t.Error("a complete first page issued a continuation token")
			}
			if result.Truncated {
				t.Error("a complete first page reports truncation")
			}
		})
	}
}

// TestRelatedContinuationFirstPageTokenPresence pins the token against the three sizes a first
// page can have relative to its limit.
//
// The boundary case is the one that matters: a result of exactly the limit must not issue a token,
// because the token would produce an empty page and a client would have to make a request to
// discover it had already finished.
func TestRelatedContinuationFirstPageTokenPresence(t *testing.T) {
	k := evidenceIndex(t)
	eligible := relatedOf(t, k, domain.SearchNode, "alpha").Counts.Eligible
	if eligible < 3 {
		t.Fatalf("the fixture start has %d related records, too few to page", eligible)
	}

	for _, tc := range []struct {
		name  string
		limit int
		token bool
	}{
		{"fewer-than-limit", eligible + 1, false},
		{"exactly-limit", eligible, false},
		{"more-than-limit", eligible - 1, true},
		{"one", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: tc.limit})
			if got := result.NextContinuationToken != ""; got != tc.token {
				t.Errorf("token present = %v, want %v", got, tc.token)
			}
			if result.HasMore != tc.token {
				t.Errorf("has_more = %v, want %v", result.HasMore, tc.token)
			}
			if want := result.Counts.Returned < result.Counts.Eligible; result.Truncated != want {
				t.Errorf("truncated = %v, want %v", result.Truncated, want)
			}
			if result.Counts.Eligible != eligible {
				t.Errorf("eligible = %d, want the unpaged %d", result.Counts.Eligible, eligible)
			}
		})
	}
}

// --------------------------------------------------------------------------------------------
// B. Whole traversals
// --------------------------------------------------------------------------------------------

// TestRelatedContinuationTraversalEqualsTheUnpagedOrdering is the central assertion of the phase.
//
// For every start and every page size, walking the traversal must produce exactly the ordering an
// unpaged request at the ceiling produces. That single equality carries three properties at once -
// no duplicate at any seam, no omission at any seam, and no reordering - and it carries them
// against the real ordering rather than against a list this test wrote down.
func TestRelatedContinuationTraversalEqualsTheUnpagedOrdering(t *testing.T) {
	k := evidenceIndex(t)

	for _, start := range everyStart {
		whole := relatedRefs(mustRelated(t, k, start.class, start.id,
			service.RelatedQuery{Limit: service.MaxRelatedLimit}))
		if len(whole) < 2 {
			continue
		}
		for _, size := range []int{1, 2, 3, len(whole) - 1, len(whole)} {
			t.Run(fmt.Sprintf("%s/limit-%d", start.class, size), func(t *testing.T) {
				got, pages := pagedRelated(t, k, start.class, start.id, service.RelatedQuery{Limit: size})
				if !reflect.DeepEqual(got, whole) {
					t.Errorf("paged at %d over %d pages:\n%s\nwant\n%s",
						size, pages, strings.Join(got, "\n"), strings.Join(whole, "\n"))
				}
				if want := (len(whole) + size - 1) / size; pages != want {
					t.Errorf("pages = %d, want %d for %d items at %d", pages, want, len(whole), size)
				}
			})
		}
	}
}

// TestRelatedContinuationTraversalTakesThreeOrMorePages walks a start that needs several pages at
// once, so the multi-seam case is asserted rather than inferred from the two-page one.
//
// The wide corpus is used because the fixture's most connected record still fits in a handful of
// pages at the smallest sizes; here the eligible set is the scan ceiling, so a traversal at a
// realistic page size crosses dozens of seams. The final page is checked to be a partial one, so
// the assertion covers the case where the last window does not fill the limit.
func TestRelatedContinuationTraversalTakesThreeOrMorePages(t *testing.T) {
	k := indexFrom(t, hubRelatedCorpus(t, 137))

	const size = 20
	whole := relatedRefs(mustRelated(t, k, domain.SearchNode, "hub",
		service.RelatedQuery{Limit: service.MaxRelatedLimit}))
	if len(whole) != service.MaxRelatedLimit {
		t.Fatalf("the wide start returned %d items at the ceiling, want %d", len(whole), service.MaxRelatedLimit)
	}

	got, pages := pagedRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: size})
	if pages < 3 {
		t.Fatalf("traversal took %d pages, want at least three", pages)
	}
	if len(got) != 137 {
		t.Errorf("traversal returned %d items, want the eligible 137", len(got))
	}
	seen := make(map[string]bool, len(got))
	for i, ref := range got {
		if seen[ref] {
			t.Fatalf("item %d, %s, was returned twice", i, ref)
		}
		seen[ref] = true
	}
	// The unpaged ceiling result is the front of the traversal, so a paged walk that agreed on
	// the set but not on the order would still fail here.
	if !reflect.DeepEqual(got[:len(whole)], whole) {
		t.Errorf("the traversal's first %d items are not the unpaged ceiling result", len(whole))
	}
}

// TestRelatedContinuationIsDeterministic pins that a page and its token are functions of the
// request.
//
// Two identical continuation requests must produce identical bytes, tokens included, and so must
// the same request against a separately built index over the same corpus. A token that varied
// across processes would be a session identifier wearing an encoding.
func TestRelatedContinuationIsDeterministic(t *testing.T) {
	corpus := hubRelatedCorpus(t, 60)
	first, second := indexFrom(t, corpus), indexFrom(t, corpus)

	render := func(k *service.Knowledge, token string) string {
		t.Helper()
		result := mustRelated(t, k, domain.SearchNode, "hub",
			service.RelatedQuery{Limit: 7, ContinuationToken: token})
		body, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal page: %v", err)
		}
		return string(body)
	}

	token := firstPageToken(t, first, domain.SearchNode, "hub", service.RelatedQuery{Limit: 7})
	if again := firstPageToken(t, first, domain.SearchNode, "hub", service.RelatedQuery{Limit: 7}); again != token {
		t.Fatal("one request issued two different tokens")
	}
	if other := firstPageToken(t, second, domain.SearchNode, "hub", service.RelatedQuery{Limit: 7}); other != token {
		t.Fatal("two indexes over one corpus issued different tokens")
	}

	want := render(first, token)
	if got := render(first, token); got != want {
		t.Error("one continuation request produced two different responses")
	}
	if got := render(second, token); got != want {
		t.Error("two indexes over one corpus produced different continuation responses")
	}
}

// --------------------------------------------------------------------------------------------
// C. Filters across pages
// --------------------------------------------------------------------------------------------

// TestRelatedContinuationCarriesEachFilterAcrossPages walks a traversal under each shape of filter
// the route accepts.
//
// A filter must mean the same thing on page four as on page one, which is what the equality
// against the unpaged filtered ordering asserts. The unfiltered case is included in the same table
// so that the filtered walks are compared against the same oracle rather than against each other.
func TestRelatedContinuationCarriesEachFilterAcrossPages(t *testing.T) {
	k := evidenceIndex(t)

	for _, tc := range []struct {
		name  string
		query service.RelatedQuery
	}{
		{"unfiltered", service.RelatedQuery{}},
		{"one-relationship", service.RelatedQuery{RelationshipTypes: []string{"assertional"}}},
		{"two-relationships", service.RelatedQuery{RelationshipTypes: []string{"conceptual", "assertional"}}},
		{"every-relationship", service.RelatedQuery{RelationshipTypes: domain.RelatedPriorityNames()}},
		{"one-destination", service.RelatedQuery{EntityTypes: []string{"claim"}}},
		{"two-destinations", service.RelatedQuery{EntityTypes: []string{"source", "node"}}},
		{"both-axes", service.RelatedQuery{
			EntityTypes:       []string{"claim", "node"},
			RelationshipTypes: []string{"conceptual", "assertional"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			whole := tc.query
			whole.Limit = service.MaxRelatedLimit
			want := relatedRefs(mustRelated(t, k, domain.SearchNode, "alpha", whole))
			if len(want) < 2 {
				t.Skipf("the fixture answers this scope with %d items, too few to page", len(want))
			}
			paged := tc.query
			paged.Limit = 1
			got, pages := pagedRelated(t, k, domain.SearchNode, "alpha", paged)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("paged over %d pages:\n%s\nwant\n%s",
					pages, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// TestRelatedContinuationAcceptsAnEquivalentFilterSpelling is the normalisation assertion.
//
// The token binds the scope that ran, not the string the caller typed, so a client that stores its
// filter as a set and re-renders it in another order between pages continues successfully. This is
// the one place the compatibility check must be tolerant, and it is tolerant of exactly one thing:
// a different spelling of the same set.
func TestRelatedContinuationAcceptsAnEquivalentFilterSpelling(t *testing.T) {
	k := evidenceIndex(t)

	issued := service.RelatedQuery{
		Limit:             1,
		EntityTypes:       []string{"claim", "node"},
		RelationshipTypes: []string{"conceptual", "assertional"},
	}
	token := firstPageToken(t, k, domain.SearchNode, "alpha", issued)

	// The same two sets, both written in the other order.
	reordered := service.RelatedQuery{
		Limit:             1,
		EntityTypes:       []string{"node", "claim"},
		RelationshipTypes: []string{"assertional", "conceptual"},
		ContinuationToken: token,
	}
	got := mustRelated(t, k, domain.SearchNode, "alpha", reordered)

	same := issued
	same.ContinuationToken = token
	want := mustRelated(t, k, domain.SearchNode, "alpha", same)
	if !reflect.DeepEqual(relatedRefs(got), relatedRefs(want)) {
		t.Errorf("a reordered filter continued differently: %v, want %v", relatedRefs(got), relatedRefs(want))
	}
}

// TestRelatedContinuationRefusesAChangedScope walks every way a caller can change the request the
// token was issued under.
//
// Each case must be refused with the error naming the axis that changed, so a client is told what
// to put back rather than being handed a page cut from an ordering it did not ask for. The table
// covers both filters in both directions - widening and narrowing - because a token that survived
// a widening would silently return records outside the scope of the request presenting it.
func TestRelatedContinuationRefusesAChangedScope(t *testing.T) {
	k := evidenceIndex(t)

	issued := service.RelatedQuery{
		Limit:             1,
		EntityTypes:       []string{"claim", "node"},
		RelationshipTypes: []string{"conceptual", "assertional"},
	}
	token := firstPageToken(t, k, domain.SearchNode, "alpha", issued)

	for _, tc := range []struct {
		name  string
		class domain.SearchEntityType
		id    string
		query service.RelatedQuery
		want  error
	}{
		{
			name: "different-start-class", class: domain.SearchClaim, id: "alpha-carries-energy",
			query: issued, want: service.ErrRelatedContinuationStart,
		},
		{
			name: "different-start-id", class: domain.SearchNode, id: "beta",
			query: issued, want: service.ErrRelatedContinuationStart,
		},
		{
			name: "narrowed-destinations", class: domain.SearchNode, id: "alpha",
			query: service.RelatedQuery{
				Limit: 1, EntityTypes: []string{"node"},
				RelationshipTypes: issued.RelationshipTypes,
			},
			want: service.ErrRelatedContinuationEntityScope,
		},
		{
			name: "widened-destinations", class: domain.SearchNode, id: "alpha",
			query: service.RelatedQuery{
				Limit: 1, EntityTypes: []string{"claim", "node", "source"},
				RelationshipTypes: issued.RelationshipTypes,
			},
			want: service.ErrRelatedContinuationEntityScope,
		},
		{
			name: "dropped-destinations", class: domain.SearchNode, id: "alpha",
			query: service.RelatedQuery{Limit: 1, RelationshipTypes: issued.RelationshipTypes},
			want:  service.ErrRelatedContinuationEntityScope,
		},
		{
			name: "narrowed-relationships", class: domain.SearchNode, id: "alpha",
			query: service.RelatedQuery{
				Limit: 1, EntityTypes: issued.EntityTypes,
				RelationshipTypes: []string{"conceptual"},
			},
			want: service.ErrRelatedContinuationRelationshipScope,
		},
		{
			name: "widened-relationships", class: domain.SearchNode, id: "alpha",
			query: service.RelatedQuery{
				Limit: 1, EntityTypes: issued.EntityTypes,
				RelationshipTypes: []string{"conceptual", "assertional", "contextual"},
			},
			want: service.ErrRelatedContinuationRelationshipScope,
		},
		{
			name: "dropped-relationships", class: domain.SearchNode, id: "alpha",
			query: service.RelatedQuery{Limit: 1, EntityTypes: issued.EntityTypes},
			want:  service.ErrRelatedContinuationRelationshipScope,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := tc.query
			query.ContinuationToken = token
			if err := continuationError(t, k, tc.class, tc.id, query); !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// --------------------------------------------------------------------------------------------
// D. The limit contract
// --------------------------------------------------------------------------------------------

// TestRelatedContinuationLimitContract pins the one rule the phase had a genuine choice about.
//
// A token binds the effective limit. A continuation naming no limit inherits it, so a traversal is
// expressible as "the same URL plus a token"; a continuation naming the same effective limit is
// accepted, including where the caller wrote a value the ceiling or the default normalises to that
// limit; and a continuation naming a different one is refused rather than silently re-windowing
// the remainder of the ordering.
func TestRelatedContinuationLimitContract(t *testing.T) {
	k := indexFrom(t, hubRelatedCorpus(t, 60))

	t.Run("omitted-limit-inherits-the-token", func(t *testing.T) {
		token := firstPageToken(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: 4})
		result := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{ContinuationToken: token})
		if result.Limit != 4 {
			t.Errorf("limit = %d, want the token's 4 rather than the default %d",
				result.Limit, service.DefaultRelatedLimit)
		}
		if len(result.Items) != 4 {
			t.Errorf("items = %d, want 4", len(result.Items))
		}
	})

	t.Run("repeated-limit-is-accepted", func(t *testing.T) {
		token := firstPageToken(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: 4})
		result := mustRelated(t, k, domain.SearchNode, "hub",
			service.RelatedQuery{Limit: 4, ContinuationToken: token})
		if result.Limit != 4 || len(result.Items) != 4 {
			t.Errorf("limit = %d with %d items, want 4 and 4", result.Limit, len(result.Items))
		}
	})

	t.Run("default-limit-round-trips", func(t *testing.T) {
		token := firstPageToken(t, k, domain.SearchNode, "hub", service.RelatedQuery{})
		result := mustRelated(t, k, domain.SearchNode, "hub",
			service.RelatedQuery{Limit: service.DefaultRelatedLimit, ContinuationToken: token})
		if result.Limit != service.DefaultRelatedLimit {
			t.Errorf("limit = %d, want %d", result.Limit, service.DefaultRelatedLimit)
		}
	})

	t.Run("ceiling-limit-round-trips", func(t *testing.T) {
		// A start with more eligible records than the ceiling, so a page at the ceiling is still
		// followed by another.
		wide := indexFrom(t, hubRelatedCorpus(t, service.MaxRelatedLimit+20))
		token := firstPageToken(t, wide, domain.SearchNode, "hub",
			service.RelatedQuery{Limit: service.MaxRelatedLimit})
		// A value above the ceiling normalises to the ceiling, which is the limit the token
		// carries, so the two spellings of "as much as allowed" continue interchangeably.
		result := mustRelated(t, wide, domain.SearchNode, "hub",
			service.RelatedQuery{Limit: service.MaxRelatedLimit * 10, ContinuationToken: token})
		if result.Limit != service.MaxRelatedLimit {
			t.Errorf("limit = %d, want the ceiling %d", result.Limit, service.MaxRelatedLimit)
		}
		if len(result.Items) != 20 {
			t.Errorf("items = %d, want the remaining 20", len(result.Items))
		}
	})

	for _, limit := range []int{3, 5, service.MaxRelatedLimit} {
		t.Run(fmt.Sprintf("changed-limit-%d-is-refused", limit), func(t *testing.T) {
			token := firstPageToken(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: 4})
			err := continuationError(t, k, domain.SearchNode, "hub",
				service.RelatedQuery{Limit: limit, ContinuationToken: token})
			if !errors.Is(err, service.ErrRelatedContinuationLimit) {
				t.Errorf("error = %v, want %v", err, service.ErrRelatedContinuationLimit)
			}
		})
	}
}

// TestRelatedContinuationFinalPageIsPartial pins the end of a traversal whose eligible total is
// not a multiple of the page size.
//
// The last page must carry the remainder, report that nothing follows, issue no token, and still
// report truncation - the response does not carry the whole eligible set, and saying otherwise
// would tell a client the page it is holding is the complete answer.
func TestRelatedContinuationFinalPageIsPartial(t *testing.T) {
	k := indexFrom(t, hubRelatedCorpus(t, 7))

	token := ""
	var last domain.RelatedKnowledge
	pages := 0
	for {
		result := mustRelated(t, k, domain.SearchNode, "hub",
			service.RelatedQuery{Limit: 3, ContinuationToken: token})
		pages++
		last = result
		if !result.HasMore {
			break
		}
		token = result.NextContinuationToken
	}

	if pages != 3 {
		t.Fatalf("pages = %d, want 3 for 7 items at 3", pages)
	}
	if len(last.Items) != 1 {
		t.Errorf("final page carried %d items, want the remaining 1", len(last.Items))
	}
	if last.NextContinuationToken != "" {
		t.Error("the final page issued a continuation token")
	}
	if !last.Truncated {
		t.Error("the final page of a 7-item result reports no truncation")
	}
	if last.Counts.Eligible != 7 {
		t.Errorf("eligible = %d, want 7", last.Counts.Eligible)
	}
}

// --------------------------------------------------------------------------------------------
// E. Ranking and ties across a seam
// --------------------------------------------------------------------------------------------

// TestRelatedContinuationResumesThroughEqualRanks is the tie-break assertion.
//
// Every spoke of the wide corpus reaches the hub over one canonical field, so all of them share a
// precedence class, a direction and a destination class: the entire ordering is decided by the
// canonical-ID tie-break alone, and every seam of a paged traversal falls between two items that
// compare equal on the first three keys. A cursor that bound only to a rank would be ambiguous at
// every one of them.
//
// The assertion is against a page size of one, so there is a seam between every adjacent pair.
func TestRelatedContinuationResumesThroughEqualRanks(t *testing.T) {
	k := indexFrom(t, hubRelatedCorpus(t, 24))

	whole := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: service.MaxRelatedLimit})
	ranks := make(map[string]bool)
	for _, item := range whole.Items {
		ranks[fmt.Sprintf("%d/%v/%s", item.Reason.PriorityRank, item.Reason.Derived, item.EntityType)] = true
	}
	if len(ranks) != 1 {
		t.Fatalf("the wide corpus produced %d distinct rank keys, so the tie-break is not exercised", len(ranks))
	}

	got, pages := pagedRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: 1})
	if pages != 24 {
		t.Errorf("pages = %d, want 24", pages)
	}
	if want := relatedRefs(whole); !reflect.DeepEqual(got, want) {
		t.Errorf("paged through equal ranks:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRelatedContinuationRefusesACursorTheOrderingDoesNotContain covers the stale-token case.
//
// A token is issued against one corpus and presented against another in which the record it names
// is no longer related to the start. There is no position "near" that record, so the token is
// refused rather than resumed from an approximation, which would silently skip or repeat items
// with nothing in the response to mark it.
func TestRelatedContinuationRefusesACursorTheOrderingDoesNotContain(t *testing.T) {
	wide := indexFrom(t, hubRelatedCorpus(t, 40))
	token := firstPageToken(t, wide, domain.SearchNode, "hub", service.RelatedQuery{Limit: 5})

	// A second corpus with the same start but a different, much smaller neighbourhood: the
	// cursor's spoke exists in neither the ordering nor the corpus.
	narrow := indexFrom(t, hubRelatedCorpus(t, 2))
	err := continuationError(t, narrow, domain.SearchNode, "hub",
		service.RelatedQuery{Limit: 5, ContinuationToken: token})
	if !errors.Is(err, service.ErrRelatedContinuationCursor) {
		t.Errorf("error = %v, want %v", err, service.ErrRelatedContinuationCursor)
	}
}

// TestRelatedContinuationRefusesACursorAtADifferentRank covers the subtler half of the same case.
//
// The record the cursor names is still in the ordering, but it now ranks differently, which means
// the ordering the token was cut from is not the ordering being resumed. Continuing across that
// would produce a page that is the next page of neither.
func TestRelatedContinuationRefusesACursorAtADifferentRank(t *testing.T) {
	k := evidenceIndex(t)

	token := firstPageToken(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: 1})
	payload := decodedToken(t, token)
	original, ok := payload["cp"].(float64)
	if !ok {
		t.Fatalf("token carries no numeric cursor rank: %#v", payload["cp"])
	}
	// Any other valid precedence rank: the record is still found, and the rank check refuses it.
	payload["cp"] = float64((int(original) + 1) % len(domain.RelatedPriorities))

	err := continuationError(t, k, domain.SearchNode, "alpha",
		service.RelatedQuery{Limit: 1, ContinuationToken: encodedToken(t, payload)})
	if !errors.Is(err, service.ErrRelatedContinuationCursor) {
		t.Errorf("error = %v, want %v", err, service.ErrRelatedContinuationCursor)
	}
}

// --------------------------------------------------------------------------------------------
// F. Token validation
// --------------------------------------------------------------------------------------------

// TestRelatedContinuationRefusesAMalformedToken walks the structural faults an arbitrary string
// can have.
//
// Every case is one refusal, ErrRelatedContinuationMalformed, because a caller cannot act
// differently on any of them and distinguishing them in the response would describe the decoder to
// the one input an attacker fully controls. What the table asserts is that each is refused at all,
// and that none of them reaches the corpus or panics.
func TestRelatedContinuationRefusesAMalformedToken(t *testing.T) {
	k := evidenceIndex(t)
	valid := firstPageToken(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: 1})

	oversized := strings.Repeat("A", service.MaxRelatedContinuationTokenChars+1)
	// A payload that is well formed base64 and well formed JSON but larger than the decoded bound,
	// so the decoded-size check is exercised rather than only the encoded one.
	bulky := base64.RawURLEncoding.EncodeToString([]byte(
		`{"v":1,"st":"node","si":"` + strings.Repeat("x", 900) + `"}`))

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"not-base64", "not a token"},
		{"base64-with-padding", base64.StdEncoding.EncodeToString([]byte(`{"v":1}`))},
		{"whitespace-payload", base64.RawURLEncoding.EncodeToString([]byte("   "))},
		{"invalid-utf8", base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xfe, 0xfd})},
		{"not-json", base64.RawURLEncoding.EncodeToString([]byte("nonsense"))},
		{"json-array", base64.RawURLEncoding.EncodeToString([]byte(`[1,2,3]`))},
		{"json-string", base64.RawURLEncoding.EncodeToString([]byte(`"token"`))},
		{"json-null", base64.RawURLEncoding.EncodeToString([]byte(`null`))},
		{"trailing-json", base64.RawURLEncoding.EncodeToString([]byte(`{"v":1} {"v":1}`))},
		{"oversized-encoded", oversized},
		{"oversized-decoded", bulky},
		{"truncated-valid-token", valid[:len(valid)/2]},
		{"valid-token-with-suffix", valid + "AAAA"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := continuationError(t, k, domain.SearchNode, "alpha",
				service.RelatedQuery{Limit: 1, ContinuationToken: tc.token})
			if !errors.Is(err, service.ErrRelatedContinuationMalformed) {
				t.Errorf("error = %v, want %v", err, service.ErrRelatedContinuationMalformed)
			}
		})
	}
}

// TestRelatedContinuationEmptyTokenIsTheFirstPage pins where "no token" is decided.
//
// The service reads an empty token as an absent one, which is what makes RelatedQuery's zero value
// a first-page request for every non-HTTP caller. Refusing a present-but-empty parameter is the
// HTTP layer's job, because only the query string can tell "supplied and blank" from "not
// supplied", and the corresponding assertion lives with that handler.
func TestRelatedContinuationEmptyTokenIsTheFirstPage(t *testing.T) {
	k := evidenceIndex(t)

	want := relatedOf(t, k, domain.SearchNode, "alpha")
	got := mustRelated(t, k, domain.SearchNode, "alpha", service.RelatedQuery{ContinuationToken: ""})
	if !reflect.DeepEqual(relatedRefs(got), relatedRefs(want)) {
		t.Errorf("an empty token returned %v, want the first page %v", relatedRefs(got), relatedRefs(want))
	}
}

// TestRelatedContinuationRefusesAnInvalidPayload edits one field of a token this build actually
// issued, so each case differs from a working token in exactly one way.
//
// Hand-written payloads would test a decoder against a shape no page ever produces; these test it
// against the shape every page produces, minus or plus one thing.
func TestRelatedContinuationRefusesAnInvalidPayload(t *testing.T) {
	k := evidenceIndex(t)
	valid := firstPageToken(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: 1})

	for _, tc := range []struct {
		name string
		edit func(map[string]any)
		want error
	}{
		{"unknown-field", func(p map[string]any) { p["surprise"] = 1 }, service.ErrRelatedContinuationMalformed},
		{"missing-version", func(p map[string]any) { delete(p, "v") }, service.ErrRelatedContinuationVersion},
		{"zero-version", func(p map[string]any) { p["v"] = 0 }, service.ErrRelatedContinuationVersion},
		{"future-version", func(p map[string]any) { p["v"] = 99 }, service.ErrRelatedContinuationVersion},
		{"version-wrong-type", func(p map[string]any) { p["v"] = "1" }, service.ErrRelatedContinuationMalformed},
		{"missing-start-type", func(p map[string]any) { delete(p, "st") }, service.ErrRelatedContinuationMalformed},
		{"missing-start-id", func(p map[string]any) { delete(p, "si") }, service.ErrRelatedContinuationMalformed},
		{"missing-cursor-type", func(p map[string]any) { delete(p, "ct") }, service.ErrRelatedContinuationMalformed},
		{"missing-cursor-id", func(p map[string]any) { delete(p, "ci") }, service.ErrRelatedContinuationMalformed},
		{"missing-limit", func(p map[string]any) { delete(p, "li") }, service.ErrRelatedContinuationMalformed},
		{"start-type-not-a-class", func(p map[string]any) { p["st"] = "experiment_run" }, service.ErrRelatedContinuationMalformed},
		{"start-type-arbitrary", func(p map[string]any) { p["st"] = "../../etc/passwd" }, service.ErrRelatedContinuationMalformed},
		{"cursor-type-not-a-class", func(p map[string]any) { p["ct"] = "graph" }, service.ErrRelatedContinuationMalformed},
		{"start-id-wrong-type", func(p map[string]any) { p["si"] = 7 }, service.ErrRelatedContinuationMalformed},
		{"start-id-oversized", func(p map[string]any) { p["si"] = strings.Repeat("a", 129) }, service.ErrRelatedContinuationMalformed},
		{"cursor-id-oversized", func(p map[string]any) { p["ci"] = strings.Repeat("a", 129) }, service.ErrRelatedContinuationMalformed},
		{"limit-zero", func(p map[string]any) { p["li"] = 0 }, service.ErrRelatedContinuationMalformed},
		{"limit-negative", func(p map[string]any) { p["li"] = -1 }, service.ErrRelatedContinuationMalformed},
		{"limit-over-ceiling", func(p map[string]any) { p["li"] = service.MaxRelatedLimit + 1 }, service.ErrRelatedContinuationMalformed},
		{"limit-wrong-type", func(p map[string]any) { p["li"] = "4" }, service.ErrRelatedContinuationMalformed},
		{"rank-negative", func(p map[string]any) { p["cp"] = -1 }, service.ErrRelatedContinuationMalformed},
		{"rank-past-the-table", func(p map[string]any) { p["cp"] = len(domain.RelatedPriorities) }, service.ErrRelatedContinuationMalformed},
		{"derived-wrong-type", func(p map[string]any) { p["cd"] = "yes" }, service.ErrRelatedContinuationMalformed},
		{"entity-scope-not-a-class", func(p map[string]any) { p["es"] = []string{"nonsense"} }, service.ErrRelatedContinuationMalformed},
		{"entity-scope-repeated", func(p map[string]any) { p["es"] = []string{"node", "node"} }, service.ErrRelatedContinuationMalformed},
		{"entity-scope-blank", func(p map[string]any) { p["es"] = []string{""} }, service.ErrRelatedContinuationMalformed},
		{"entity-scope-wrong-type", func(p map[string]any) { p["es"] = "node" }, service.ErrRelatedContinuationMalformed},
		{"relationship-scope-not-a-class", func(p map[string]any) { p["rs"] = []string{"unclassified"} }, service.ErrRelatedContinuationMalformed},
		{"relationship-scope-repeated", func(p map[string]any) { p["rs"] = []string{"conceptual", "conceptual"} }, service.ErrRelatedContinuationMalformed},
		{"relationship-scope-oversized", func(p map[string]any) {
			p["rs"] = append(domain.RelatedPriorityNames(), "conceptual")
		}, service.ErrRelatedContinuationMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := decodedToken(t, valid)
			tc.edit(payload)
			err := continuationError(t, k, domain.SearchNode, "alpha",
				service.RelatedQuery{Limit: 1, ContinuationToken: encodedToken(t, payload)})
			if !errors.Is(err, tc.want) {
				t.Errorf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestRelatedContinuationSurvivesArbitraryInput is the robustness assertion.
//
// A token is the one field of this route whose contents a caller composes freely, so an arbitrary
// string in it must be refused rather than crash, hang, or reach the corpus. Every input here is
// asserted to produce an error rather than a result, and the test passing at all is the assertion
// that none of them panicked.
func TestRelatedContinuationSurvivesArbitraryInput(t *testing.T) {
	k := evidenceIndex(t)

	inputs := []string{
		" ", "\x00", "\n", "%", "%%%%", "=", "==", "....", "/../../",
		"AAAA", "A", "AA", "AAA", "-", "_", "-_-_",
		strings.Repeat("A", service.MaxRelatedContinuationTokenChars),
		strings.Repeat("A", service.MaxRelatedContinuationTokenChars*4),
		base64.RawURLEncoding.EncodeToString([]byte("{")),
		base64.RawURLEncoding.EncodeToString([]byte("{}")),
		base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"st":null,"si":null}`)),
		base64.RawURLEncoding.EncodeToString([]byte(
			"[" + strings.Repeat("[", 200) + strings.Repeat("]", 200) + "]")),
		base64.RawURLEncoding.EncodeToString([]byte(`{"v":1e400}`)),
	}
	for i, token := range inputs {
		t.Run(fmt.Sprintf("input-%02d", i), func(t *testing.T) {
			result, err := k.RelatedKnowledgeFor(string(domain.SearchNode), "alpha",
				service.RelatedQuery{ContinuationToken: token})
			if err == nil {
				t.Fatalf("arbitrary token was answered with %d items", len(result.Items))
			}
		})
	}
}

// TestRelatedContinuationRefusesAMalformedTokenBeforeTheStartIsResolved pins the validation order.
//
// A caller who presented a stale token against a mistyped identifier must be told about the token,
// which is in the request they can see, rather than about a record that was never going to be
// found. It is the same rule the two filters already follow: the whole request is validated before
// the corpus is consulted.
func TestRelatedContinuationRefusesAMalformedTokenBeforeTheStartIsResolved(t *testing.T) {
	k := evidenceIndex(t)

	err := continuationError(t, k, domain.SearchNode, "no-such-node",
		service.RelatedQuery{ContinuationToken: "not a token"})
	if !errors.Is(err, service.ErrRelatedContinuationMalformed) {
		t.Errorf("error = %v, want %v", err, service.ErrRelatedContinuationMalformed)
	}
	if errors.Is(err, service.ErrNotFound) {
		t.Error("a malformed token on an unknown start was reported as a missing record")
	}
}

// TestRelatedContinuationRefusesAMalformedFilterBeforeTheToken keeps the other half of the order.
//
// The filters are validated before the token, so a request that is wrong in both ways reports the
// filter. That is not an arbitrary choice: the token's compatibility check is against the resolved
// scopes, so a scope that does not resolve has nothing for the token to be compared to.
func TestRelatedContinuationRefusesAMalformedFilterBeforeTheToken(t *testing.T) {
	k := evidenceIndex(t)

	err := continuationError(t, k, domain.SearchNode, "alpha", service.RelatedQuery{
		RelationshipTypes: []string{"nonsense"},
		ContinuationToken: "not a token",
	})
	var invalid *service.InvalidFilterError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %v, want an invalid filter error", err)
	}
	if invalid.Param != "relationship_types" {
		t.Errorf("param = %q, want relationship_types", invalid.Param)
	}
}

// TestRelatedContinuationTokensStayWithinTheirBound pins the encoded bound against the widest
// token this build can issue.
//
// The start and the cursor both carry long identifiers and both scopes are named in full, which is
// the worst case the payload has. If a later field pushes a real token past the ceiling, this
// fails here rather than in production on the one record whose identifier happens to be long.
func TestRelatedContinuationTokensStayWithinTheirBound(t *testing.T) {
	k := evidenceIndex(t)

	widest := service.RelatedQuery{
		Limit:             1,
		EntityTypes:       domain.SearchEntityTypeNames(),
		RelationshipTypes: domain.RelatedPriorityNames(),
	}
	token := firstPageToken(t, k, domain.SearchNode, "alpha", widest)
	if len(token) > service.MaxRelatedContinuationTokenChars {
		t.Errorf("a token of %d characters exceeds the %d ceiling",
			len(token), service.MaxRelatedContinuationTokenChars)
	}
	// The bound must also leave real headroom over the widest payload, so that a maximal
	// identifier - which the fixture corpus does not have - cannot cross it.
	const headroom = 2 * (128 + 128)
	if len(token)+headroom > service.MaxRelatedContinuationTokenChars {
		t.Errorf("a token of %d characters leaves under %d characters of identifier headroom",
			len(token), headroom)
	}
}

// TestRelatedContinuationTokenCarriesNothingItShouldNot inspects a decoded payload directly.
//
// The token is opaque by contract, and this is the one test that opens it - not to assert its
// shape as an interface, but to assert what it does not contain. A cursor carrying titles,
// summaries, evidence or counts would be a result set the client could edit; one carrying a path
// or a host would describe the operator's machine.
func TestRelatedContinuationTokenCarriesNothingItShouldNot(t *testing.T) {
	k := evidenceIndex(t)
	token := firstPageToken(t, k, domain.SearchNode, "alpha", service.RelatedQuery{Limit: 1})

	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	body := string(payload)
	for _, forbidden := range []string{
		"title", "summary", "evidence", "explanation", "eligible", "scanned",
		"/", "\\", "http", "audiomuse/backend", "internal", "Alpha",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("token payload carries %q: %s", forbidden, body)
		}
	}

	// Every key is accounted for by the documented payload, so a field added later without a
	// decision about what it exposes fails here.
	fields := decodedToken(t, token)
	known := map[string]bool{"v": true, "st": true, "si": true, "es": true, "rs": true,
		"li": true, "ct": true, "ci": true, "cp": true, "cd": true}
	for name := range fields {
		if !known[name] {
			t.Errorf("token payload carries an undocumented field %q", name)
		}
	}
}

// TestRelatedContinuationDoesNotChangeTheBounds is the regression guard for the phase.
//
// Paging is a window over the same bounded work, so none of the three caps the response reports
// may move when a token is present: the scan ceiling is the same, the scan count is the same work
// done, and per-item evidence is cut at the same cap on page four as on page one.
func TestRelatedContinuationDoesNotChangeTheBounds(t *testing.T) {
	k := indexFrom(t, wideRelatedCorpus(t, service.MaxRelatedRelationsScanned+1))

	first := mustRelated(t, k, domain.SearchNode, "hub", service.RelatedQuery{Limit: 10})
	if first.NextContinuationToken == "" {
		t.Fatal("the wide start issued no continuation token")
	}
	second := mustRelated(t, k, domain.SearchNode, "hub",
		service.RelatedQuery{Limit: 10, ContinuationToken: first.NextContinuationToken})

	if first.Bounds != second.Bounds {
		t.Errorf("bounds changed across a seam: %+v then %+v", first.Bounds, second.Bounds)
	}
	if first.Counts.Eligible != second.Counts.Eligible {
		t.Errorf("eligible = %d then %d", first.Counts.Eligible, second.Counts.Eligible)
	}
	if second.Bounds.MaxEvidencePerItem != service.MaxRelatedEvidencePerItem {
		t.Errorf("max_evidence_per_item = %d, want %d",
			second.Bounds.MaxEvidencePerItem, service.MaxRelatedEvidencePerItem)
	}
	if second.Bounds.MaxRelationsScanned != service.MaxRelatedRelationsScanned {
		t.Errorf("max_relations_scanned = %d, want %d",
			second.Bounds.MaxRelationsScanned, service.MaxRelatedRelationsScanned)
	}
	for _, item := range second.Items {
		if len(item.AdditionalEvidence) > service.MaxRelatedEvidencePerItem-1 {
			t.Errorf("item %s/%s carries %d further reasons past the cap",
				item.EntityType, item.ID, len(item.AdditionalEvidence))
		}
	}
}
