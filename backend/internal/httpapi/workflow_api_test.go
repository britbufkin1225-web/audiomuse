package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/httpapi"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/repository/filesystem"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
	"github.com/britbufkin1225-web/audiomuse/backend/internal/testsupport"
)

// Phase 1J: the merged search workflow, tested as one system rather than as six features.
//
// Phases 1E to 1I each pinned the contract they introduced, and they pinned it well. What none of
// them could pin is the property that only exists once they are all present: that query
// composition, scope filtering, facet counting, context resolution, ranking, explanation and
// paging compose into one deterministic, bounded, documented request-to-response pipeline. A
// per-feature suite can be complete and still leave that unowned, because every one of those tests
// is entitled to assume the other five behave.
//
// So the tests here are deliberately not more feature tests. They are invariants stated once and
// required to hold over a shared matrix of requests that exercises the features together:
//
//	facet sum == page.total                      the facet object describes the set that was paged
//	len(results) <= page.limit <= MaxLimit        no request can produce an unbounded response
//	relevance_score == sum(match_signals)         the explanation and the score are one fact
//	the ordering is total and monotone            no pair of results compares equal
//	page == full result set[offset:offset+limit]  bounding happens after ranking, never before
//	byte-identical repeats                        nothing read a map, a clock or a directory order
//
// An invariant that holds over a matrix catches an interaction a hand-written expectation cannot:
// it is checked against every combination in the table, including the ones nobody thought to write
// down. Where an expectation is genuinely per-case — which status a malformed request earns, which
// signal a query fires — the case is a table row rather than a separate function, so the contract
// reads as a list of decisions instead of as a wall of prose.
//
// Nothing here is a new search feature, a new endpoint or a new fixture. Every request below is one
// a client could already send, and every corpus record it reads is the fixture corpus the earlier
// phases already ship.

// workflowCase is one request in the shared matrix, named so a failure reports which combination of
// controls broke rather than only which URL.
type workflowCase struct {
	name   string
	target string
}

// searchTarget composes a search URL from encoded parameters, so a test states the request it means
// rather than a hand-escaped string that could quietly differ from it.
func searchTarget(q string, params ...string) string {
	target := httpapi.APIBase + "/search?q=" + url.QueryEscape(q)
	for _, param := range params {
		target += "&" + param
	}
	return target
}

// workflowMatrix is the shared request set: the whole pipeline, exercised with its controls in
// combination rather than one at a time.
//
// It is built in a fixed order from fixed literals, so the matrix is the same list on every run and
// a failure names a reproducible request. The queries are chosen to span the retrieval space the
// fixture corpus can express — an exact ID, an exact display field, a name prefix, prose-only text,
// text that matches nothing — and each is then crossed with the composition, scope, context and
// paging controls.
func workflowMatrix() []workflowCase {
	// Literal queries, chosen for what they demonstrate rather than for how many hits they return.
	literals := []struct{ name, q string }{
		{"id-exact", "alpha"},
		{"display-exact", "fixture term"},
		{"display-prefix", "Fixture Te"},
		{"prose-only", "synthetic"},
		{"broad", "fixture"},
		{"mixed-case", "FIXTURE"},
		{"punctuated", "fixture-term"},
		{"no-hits", "nothing-in-this-corpus-matches-this"},
	}
	// Composed queries: terms within one field, terms spread across fields, a contiguous phrase in a
	// field that is not the display field, and a pair no single record carries.
	composed := []string{
		"fixture term",
		"synthetic root",
		"fixture harness",
		"alpha gamma",
		"fixture nothing-in-this-corpus-matches-this",
	}
	// The scope spellings: unscoped, single classes through both filters, a genuine subset, and the
	// complete set.
	scopes := []struct{ name, param string }{
		{"unscoped", ""},
		{"type-node", "type=node"},
		{"type-vocabulary", "type=vocabulary"},
		{"entity-types-one", "entity_types=claim"},
		{"entity-types-subset", "entity_types=node,claim"},
		{"entity-types-all", "entity_types=session,node,claim,source,vocabulary,experiment"},
	}

	var cases []workflowCase
	add := func(name, target string) {
		cases = append(cases, workflowCase{name: name, target: target})
	}

	for _, lit := range literals {
		for _, scope := range scopes {
			var params []string
			if scope.param != "" {
				params = append(params, scope.param)
			}
			add("literal/"+lit.name+"/"+scope.name, searchTarget(lit.q, params...))
			add("literal/"+lit.name+"/"+scope.name+"/context",
				searchTarget(lit.q, append(append([]string(nil), params...), "include_context=true")...))
		}
		// Paging windows, including one that starts past the end of the result set.
		for _, paging := range []string{"limit=1", "limit=2&offset=1", "limit=200", "limit=3&offset=99"} {
			add("literal/"+lit.name+"/"+paging, searchTarget(lit.q, paging))
			add("literal/"+lit.name+"/"+paging+"/context",
				searchTarget(lit.q, paging, "include_context=true"))
		}
	}

	for _, q := range composed {
		add("all_terms/"+q, searchTarget(q, "query_mode=all_terms"))
		add("all_terms/"+q+"/context", searchTarget(q, "query_mode=all_terms", "include_context=true"))
		add("all_terms/"+q+"/scoped", searchTarget(q, "query_mode=all_terms", "entity_types=node,vocabulary"))
		add("all_terms/"+q+"/paged", searchTarget(q, "query_mode=all_terms", "limit=1&offset=1"))
		add("all_terms/"+q+"/everything",
			searchTarget(q, "query_mode=all_terms", "type=vocabulary", "include_context=true", "limit=2"))
	}
	// The explicit default: a request naming literal is the same request as one naming no mode, and
	// it belongs in the matrix so every invariant covers it too.
	add("literal/explicit-mode", searchTarget("fixture", "query_mode=literal"))
	return cases
}

// workflowSearch runs one matrix request and decodes the typed contract. A non-200 is fatal: every
// request in the matrix is well formed, so a refusal is a defect rather than a case.
func workflowSearch(t testing.TB, handler http.Handler, target string) service.SearchResults {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
	}
	var results service.SearchResults
	decode(t, rec, &results)
	return results
}

// refusedSearch requires the established validation response: 400, the stable envelope, the existing
// invalid_query code, a message that discloses nothing about the process, and no key outside the
// documented envelope. It returns the message so a case may additionally assert which rule it cited.
func refusedSearch(t testing.TB, handler http.Handler, target string) string {
	t.Helper()
	rec := do(t, handler, http.MethodGet, target)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET %s = %d, want 400: %s", target, rec.Code, rec.Body.String())
	}

	// The envelope is asserted structurally as well as by type, so an error response cannot grow a
	// field — a hint, a caller echo, a trace id — without this failing.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("GET %s: decode envelope: %v", target, err)
	}
	if len(raw) != 1 || raw["error"] == nil {
		t.Fatalf("GET %s: error envelope has keys %v, want exactly [error]", target, sortedRawKeys(raw))
	}
	var detail map[string]string
	if err := json.Unmarshal(raw["error"], &detail); err != nil {
		t.Fatalf("GET %s: decode error detail: %v", target, err)
	}
	if len(detail) != 2 {
		t.Errorf("GET %s: error detail has keys %v, want exactly [code message]", target, sortedTextKeys(detail))
	}
	if detail["code"] != httpapi.CodeInvalidQuery {
		t.Errorf("GET %s: error code = %q, want %q", target, detail["code"], httpapi.CodeInvalidQuery)
	}
	if strings.TrimSpace(detail["message"]) == "" {
		t.Errorf("GET %s: empty error message", target)
	}
	assertNoInternalDisclosure(t, target, detail["message"])
	return detail["message"]
}

// assertNoInternalDisclosure is the leak check applied to every string this phase reads back out of
// the API. Go package names, file positions, goroutine dumps and filesystem paths describe the
// operator's machine, and none of them can help a client fix their query.
func assertNoInternalDisclosure(t testing.TB, target, value string) {
	t.Helper()
	for _, leak := range []string{
		"service.", "domain.", "httpapi.", "filesystem.", "goroutine", ".go:",
		`C:\`, "/Users/", "internal/", "testdata", "0x",
	} {
		if strings.Contains(value, leak) {
			t.Errorf("GET %s disclosed %q: %s", target, leak, value)
		}
	}
}

func sortedRawKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedTextKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// resultKey identifies one hit. Two classes may share an ID, so identity is the pair.
func resultKey(r domain.SearchResult) string {
	return string(r.EntityType) + "/" + r.ID
}

func resultKeys(results []domain.SearchResult) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, resultKey(r))
	}
	return out
}

func containsSignal(results []domain.SearchResult, signal string) bool {
	for _, r := range results {
		for _, fired := range r.MatchSignals {
			if fired == signal {
				return true
			}
		}
	}
	return false
}

// TestSearchWorkflowInvariantsHoldAcrossTheMatrix is the phase's central regression.
//
// Every property below is a statement about the pipeline as a whole rather than about one stage,
// and each is required of every request in the matrix. A per-feature test could not state most of
// them: "the facets count the set that was paged" is only meaningful once facets and paging are
// both present, and "the score is the sum of the signals listed beside it" is only checkable once
// ranking and explanation are serialised together.
func TestSearchWorkflowInvariantsHoldAcrossTheMatrix(t *testing.T) {
	handler := newHandler(t)

	signalRank := make(map[string]int, len(domain.SearchMatchSignals))
	for i, signal := range domain.SearchMatchSignals {
		signalRank[signal] = i
	}
	matchKinds := map[string]bool{
		domain.MatchIDExact: true, domain.MatchTitleExact: true, domain.MatchTitleSubstring: true,
		domain.MatchFieldSubstring: true, domain.MatchAllTerms: true,
	}

	for _, tc := range workflowMatrix() {
		t.Run(tc.name, func(t *testing.T) {
			res := workflowSearch(t, handler, tc.target)

			// Bounding. The default and the ceiling are enforced whatever the caller wrote, so no
			// request in the matrix — including the one asking for 200 — can return an unbounded page
			// or a page larger than the limit it reports.
			if res.Page.Limit <= 0 || res.Page.Limit > service.MaxLimit {
				t.Errorf("page.limit = %d, want 1..%d", res.Page.Limit, service.MaxLimit)
			}
			if len(res.Results) > res.Page.Limit {
				t.Errorf("%d results exceed page.limit %d", len(res.Results), res.Page.Limit)
			}
			if res.Page.Count != len(res.Results) {
				t.Errorf("page.count = %d, len(results) = %d", res.Page.Count, len(res.Results))
			}
			if res.Page.Total < len(res.Results) {
				t.Errorf("page.total %d is smaller than the page it carries (%d)", res.Page.Total, len(res.Results))
			}
			if res.Page.Offset < 0 {
				t.Errorf("page.offset = %d", res.Page.Offset)
			}

			// Facets describe the complete filtered set, so their sum is page.total on every page of
			// every search — including a page that starts past the end of the result set.
			if got := res.Facets.EntityTypes.Total(); got != res.Page.Total {
				t.Errorf("facet sum = %d, page.total = %d (%+v)", got, res.Page.Total, res.Facets)
			}
			for _, class := range domain.SearchEntityTypes {
				if res.Facets.EntityTypes.Count(class) < 0 {
					t.Errorf("facet %s is negative", class)
				}
			}

			seen := make(map[string]bool, len(res.Results))
			for i, r := range res.Results {
				// No duplicate candidates. Matching, scope filtering and context enrichment are each
				// capable of introducing one, and none may.
				if key := resultKey(r); seen[key] {
					t.Errorf("result %d repeats %s", i, key)
				} else {
					seen[key] = true
				}
				if !domain.ValidSearchEntityType(string(r.EntityType)) {
					t.Errorf("result %d has unsearchable class %q", i, r.EntityType)
				}
				if r.ID == "" || r.Title == "" {
					t.Errorf("result %d has no identity: %+v", i, r)
				}
				if !matchKinds[r.MatchKind] {
					t.Errorf("result %d has unknown match_kind %q", i, r.MatchKind)
				}

				// The explanation reconciles with the score exactly. This is the property that makes
				// the ranking auditable from the response alone, so it is required of every hit of
				// every mode rather than spot-checked.
				if got := domain.SearchRelevanceScore(r.MatchSignals); got != r.RelevanceScore {
					t.Errorf("result %d: relevance_score %d != sum of %v (%d)",
						i, r.RelevanceScore, r.MatchSignals, got)
				}
				assertSignalsAreWellFormed(t, i, r.MatchSignals, signalRank)

				if len(r.MatchedFields) == 0 {
					t.Errorf("result %d matched no field yet was returned", i)
				}
				// Evidence is the record's own. A composed hit's union must be exactly the union of its
				// per-term lists: a field in one and not the other would mean the explanation and the
				// match disagree about which fields the query reached.
				if len(r.TermMatches) > 0 {
					assertTermEvidenceReconciles(t, i, r)
				} else if res.QueryMode == string(domain.SearchModeAllTerms) {
					t.Errorf("result %d carries no term_matches under all_terms", i)
				}

				assertContextIsWellFormed(t, tc.target, i, r, res.IncludeContext)

				// Ordering, checked pairwise so a failure names the pair that broke it. The order is
				// required to be total: relevance descending, then the canonical class order, then the
				// canonical ID, with no pair comparing equal.
				if i == 0 {
					continue
				}
				prev := res.Results[i-1]
				switch {
				case prev.RelevanceScore < r.RelevanceScore:
					t.Errorf("result %d outscores its predecessor (%d > %d)",
						i, r.RelevanceScore, prev.RelevanceScore)
				case prev.RelevanceScore > r.RelevanceScore:
				default:
					prevRank := domain.SearchEntityRank(prev.EntityType)
					rank := domain.SearchEntityRank(r.EntityType)
					if prevRank > rank || (prevRank == rank && prev.ID >= r.ID) {
						t.Errorf("tie between %s and %s is not broken canonically",
							resultKey(prev), resultKey(r))
					}
				}
			}
		})
	}
}

// assertSignalsAreWellFormed requires a signal list to be the minimal true description of a hit:
// drawn from the closed set, free of repetition, ordered by descending weight, never empty, and
// carrying at most one member of each mutually exclusive group.
func assertSignalsAreWellFormed(t testing.TB, index int, signals []string, rank map[string]int) {
	t.Helper()
	if len(signals) == 0 {
		t.Errorf("result %d has no match_signals", index)
		return
	}
	fired := make(map[string]bool, len(signals))
	last := -1
	for _, signal := range signals {
		position, known := rank[signal]
		if !known {
			t.Errorf("result %d fired unknown signal %q", index, signal)
			continue
		}
		if fired[signal] {
			t.Errorf("result %d repeats signal %q", index, signal)
		}
		fired[signal] = true
		if position <= last {
			t.Errorf("result %d signals are not in descending weight order: %v", index, signals)
		}
		last = position
	}
	// The three exclusive groups. Each states one fact about the hit, and two members of one group
	// would be two answers to one question — which is also how a score gets inflated.
	for _, group := range [][]string{
		{domain.SignalIDExact, domain.SignalIDSubstring},
		{domain.SignalTitleExact, domain.SignalTitlePrefix, domain.SignalTitleSubstring},
		{domain.SignalTitleAllTerms, domain.SignalTitleTerms},
	} {
		count := 0
		for _, signal := range group {
			if fired[signal] {
				count++
			}
		}
		if count > 1 {
			t.Errorf("result %d fired %d of the exclusive group %v: %v", index, count, group, signals)
		}
	}
}

// assertTermEvidenceReconciles requires matched_fields to be exactly the union of the per-term field
// lists, with every term satisfied and none repeated.
func assertTermEvidenceReconciles(t testing.TB, index int, r domain.SearchResult) {
	t.Helper()
	union := map[string]bool{}
	terms := make(map[string]bool, len(r.TermMatches))
	for _, match := range r.TermMatches {
		if match.Term == "" {
			t.Errorf("result %d carries an unnamed term match", index)
		}
		if terms[match.Term] {
			t.Errorf("result %d repeats term %q", index, match.Term)
		}
		terms[match.Term] = true
		if len(match.MatchedFields) == 0 {
			t.Errorf("result %d reports term %q with no field", index, match.Term)
		}
		for _, field := range match.MatchedFields {
			union[field] = true
		}
	}
	if len(union) != len(r.MatchedFields) {
		t.Errorf("result %d: matched_fields %v is not the union of its term evidence %v",
			index, r.MatchedFields, r.TermMatches)
		return
	}
	for _, field := range r.MatchedFields {
		if !union[field] {
			t.Errorf("result %d: matched_fields names %q, which no term matched", index, field)
		}
	}
}

// assertContextIsWellFormed requires context to be present exactly when it was requested, to be
// internally consistent, to stay inside its per-result bound, and to name only addressable records.
func assertContextIsWellFormed(t testing.TB, target string, index int, r domain.SearchResult, requested bool) {
	t.Helper()
	if !requested {
		if r.Context != nil {
			t.Errorf("result %d carries context that was not requested", index)
		}
		return
	}
	if r.Context == nil {
		t.Errorf("result %d has no context object although context was requested", index)
		return
	}
	ctx := r.Context
	if ctx.Returned != len(ctx.Related) {
		t.Errorf("result %d: context returned = %d, len(related) = %d", index, ctx.Returned, len(ctx.Related))
	}
	if ctx.Returned > ctx.Count {
		t.Errorf("result %d: context returned %d of a count of %d", index, ctx.Returned, ctx.Count)
	}
	if ctx.Truncated != (ctx.Returned < ctx.Count) {
		t.Errorf("result %d: truncated = %v with returned %d of %d",
			index, ctx.Truncated, ctx.Returned, ctx.Count)
	}
	if ctx.Returned > service.MaxContextRelationsPerResult {
		t.Errorf("result %d: context of %d exceeds the per-result bound %d",
			index, ctx.Returned, service.MaxContextRelationsPerResult)
	}
	for _, relation := range ctx.Related {
		if relation.Relation == "" || relation.Origin == "" {
			t.Errorf("result %d: context relation is unlabelled: %+v", index, relation)
		}
		if relation.Entity.ID == "" || relation.Entity.Label == "" {
			t.Errorf("result %d: context entity is not addressable: %+v", index, relation.Entity)
		}
		if !domain.ValidSearchEntityType(string(relation.Entity.EntityType)) {
			t.Errorf("result %d: context names %q, which is not a searchable class",
				index, relation.Entity.EntityType)
		}
		// A context item names another record. It must never name the hit itself, which would make a
		// result its own context and inflate the count with a fact the corpus did not state.
		if relation.Entity.EntityType == r.EntityType && relation.Entity.ID == r.ID {
			t.Errorf("result %d lists itself as its own context", index)
		}
		assertNoInternalDisclosure(t, target, relation.Origin)
	}
}

// TestSearchWorkflowContextIsBoundedPerResponse requires the whole-response context budget to hold
// over the widest request the API accepts, which is the one place the per-result bound alone is not
// enough: a full page of well-connected records multiplies it.
func TestSearchWorkflowContextIsBoundedPerResponse(t *testing.T) {
	handler := newHandler(t)
	for _, target := range []string{
		searchTarget("fixture", "include_context=true", "limit=200"),
		searchTarget("e", "include_context=true", "limit=200"),
	} {
		res := workflowSearch(t, handler, target)
		total := 0
		for _, r := range res.Results {
			if r.Context != nil {
				total += r.Context.Returned
			}
		}
		if total > service.MaxContextRelationsPerResponse {
			t.Errorf("GET %s serialised %d context relations, over the bound of %d",
				target, total, service.MaxContextRelationsPerResponse)
		}
	}
}

// TestSearchWorkflowRepeatedRequestsAreByteIdentical is the stability regression, and it compares
// raw bytes rather than decoded values on purpose: a decode normalises away exactly the differences
// — key order, a re-serialised number, an omitted-versus-empty slice — that a client diffing two
// responses would see.
//
// One handler answering the same request repeatedly rules out per-request nondeterminism: Go map
// iteration inside a stage, a sort that is not total, a budget that is not reset.
func TestSearchWorkflowRepeatedRequestsAreByteIdentical(t *testing.T) {
	handler := newHandler(t)
	for _, tc := range workflowMatrix() {
		first := do(t, handler, http.MethodGet, tc.target).Body.String()
		for run := 1; run <= 3; run++ {
			if got := do(t, handler, http.MethodGet, tc.target).Body.String(); got != first {
				t.Fatalf("%s: run %d differs from run 0\nfirst: %s\nlater: %s", tc.name, run, first, got)
			}
		}
	}
}

// TestSearchWorkflowDoesNotDependOnHowTheCorpusWasEnumerated is the second half of determinism, and
// the half a repeated request cannot reach.
//
// One index answering twice proves the query path is deterministic. It cannot prove the index itself
// is, because both answers come from the same startup. Building a second index from a different
// filesystem implementation — an in-memory tree rather than the directory on disk — drives a
// different enumeration through the same loaders, so a stage that had come to depend on the order
// records arrived in would produce two different orderings here while remaining perfectly stable on
// either one alone.
func TestSearchWorkflowDoesNotDependOnHowTheCorpusWasEnumerated(t *testing.T) {
	fromDisk := newHandler(t)

	repo, err := filesystem.NewFromFS(testsupport.MutableCorpus(t), testsupport.CorpusName)
	if err != nil {
		t.Fatalf("open in-memory fixture corpus: %v", err)
	}
	knowledge, err := service.New(context.Background(), repo)
	if err != nil {
		t.Fatalf("build in-memory index: %v", err)
	}
	fromMemory := httpapi.NewServer(knowledge, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, tc := range workflowMatrix() {
		disk := do(t, fromDisk, http.MethodGet, tc.target).Body.String()
		memory := do(t, fromMemory, http.MethodGet, tc.target).Body.String()
		if disk != memory {
			t.Fatalf("%s differs between independently built indexes\ndisk:   %s\nmemory: %s",
				tc.name, disk, memory)
		}
	}
}

// TestSearchWorkflowIsStableUnderConcurrentReaders runs the whole matrix from several goroutines and
// requires every answer to equal the single-threaded one.
//
// The index is built once and never written to afterwards, and the per-request work — the result
// slice, the context budget, the facet counts — is all request-local. That is an invariant of the
// design rather than of any one phase, so it belongs to the workflow suite; under -race this is also
// where a shared buffer or a mutated index entry would surface.
func TestSearchWorkflowIsStableUnderConcurrentReaders(t *testing.T) {
	handler := newHandler(t)
	matrix := workflowMatrix()

	expected := make([]string, len(matrix))
	for i, tc := range matrix {
		expected[i] = do(t, handler, http.MethodGet, tc.target).Body.String()
	}

	const readers = 8
	var wg sync.WaitGroup
	for reader := 0; reader < readers; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, tc := range matrix {
				if got := do(t, handler, http.MethodGet, tc.target).Body.String(); got != expected[i] {
					t.Errorf("%s answered differently under concurrent load", tc.name)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestSearchWorkflowMalformedRequestsAreRefusedConsistently pins the whole validation surface in one
// table, including the cases that exist only because several phases' controls sit on one route.
//
// Every row is a 400 carrying the established envelope and the existing invalid_query code; this
// phase adds no search-specific error code and no second error shape. The expectations are
// substrings rather than exact strings, so the table pins which rule was cited without freezing the
// prose that cites it.
func TestSearchWorkflowMalformedRequestsAreRefusedConsistently(t *testing.T) {
	handler := newHandler(t)
	base := httpapi.APIBase + "/search"

	cases := []struct {
		name   string
		target string
		cites  string
	}{
		// The query itself.
		{"q absent", base, "q is required"},
		{"q empty", base + "?q=", "q is required"},
		{"q spaces only", base + "?q=+++", "q is required"},
		{"q tab and newline only", base + "?q=%09%0A", "q is required"},
		{"q carries a NUL", base + "?q=" + url.QueryEscape("alpha\x00beta"), "invalid character"},
		{"q past the length bound", base + "?q=" + strings.Repeat("a", service.MaxQueryChars+1), "exceeds"},
		{"q supplied twice", base + "?q=alpha&q=beta", "exactly once"},

		// The query string as a whole.
		{"unknown parameter", searchTarget("fixture", "bogus=1"), "Unsupported query parameter"},
		{"parameter names are case sensitive", base + "?Q=fixture", "Unsupported query parameter"},
		{"limit supplied twice", searchTarget("fixture", "limit=1", "limit=2"), "exactly once"},

		// Composition.
		{"query_mode blank", searchTarget("fixture", "query_mode="), "query_mode must be one of"},
		{"query_mode unknown", searchTarget("fixture", "query_mode=fuzzy"), "Unsupported value for query_mode"},
		{"query_mode wrong case", searchTarget("fixture", "query_mode=ALL_TERMS"), "Unsupported value for query_mode"},
		{"query_mode boolean spelling", searchTarget("fixture", "query_mode=and"), "Unsupported value for query_mode"},
		{"all_terms with one distinct term", searchTarget("fixture", "query_mode=all_terms"), "at least"},
		{"all_terms with one term repeated", searchTarget("fixture fixture", "query_mode=all_terms"), "at least"},
		{"all_terms past the term ceiling", searchTarget("a b c d e f g h i", "query_mode=all_terms"), "at most"},

		// Scope.
		{"type unknown", searchTarget("fixture", "type=notaclass"), "Unsupported value for type"},
		{"type wrong case", searchTarget("fixture", "type=Node"), "Unsupported value for type"},
		{"type names the unsearchable class", searchTarget("fixture", "type=experiment_run"), "Unsupported value for type"},
		{"entity_types blank", searchTarget("fixture", "entity_types="), "comma-separated list"},
		{"entity_types whitespace only", searchTarget("fixture", "entity_types=+"), "comma-separated list"},
		{"entity_types trailing comma", searchTarget("fixture", "entity_types=node,"), "empty value"},
		{"entity_types leading comma", searchTarget("fixture", "entity_types=,node"), "empty value"},
		{"entity_types doubled comma", searchTarget("fixture", "entity_types=node,,claim"), "empty value"},
		{"entity_types blank member", searchTarget("fixture", "entity_types=node,+,claim"), "empty value"},
		{"entity_types unknown member", searchTarget("fixture", "entity_types=node,notaclass"), "Unsupported value for entity_types"},
		{"entity_types wrong case", searchTarget("fixture", "entity_types=NODE"), "Unsupported value for entity_types"},
		{"entity_types plural spelling", searchTarget("fixture", "entity_types=nodes"), "Unsupported value for entity_types"},
		{"entity_types names the unsearchable class", searchTarget("fixture", "entity_types=experiment_run"), "Unsupported value for entity_types"},
		{"entity_types repeats a class", searchTarget("fixture", "entity_types=node,claim,node"), "must not repeat"},
		{"entity_types repeats after trimming", searchTarget("fixture", "entity_types=node,+node"), "must not repeat"},

		// Context.
		{"include_context blank", searchTarget("fixture", "include_context="), "exactly true or false"},
		{"include_context numeric", searchTarget("fixture", "include_context=1"), "exactly true or false"},
		{"include_context wrong case", searchTarget("fixture", "include_context=TRUE"), "exactly true or false"},
		{"include_context yes", searchTarget("fixture", "include_context=yes"), "exactly true or false"},

		// Paging.
		{"limit negative", searchTarget("fixture", "limit=-1"), "non-negative integer"},
		{"limit not a number", searchTarget("fixture", "limit=all"), "non-negative integer"},
		{"limit fractional", searchTarget("fixture", "limit=5.5"), "non-negative integer"},
		{"offset negative", searchTarget("fixture", "offset=-1"), "non-negative integer"},
		{"offset not a number", searchTarget("fixture", "offset=last"), "non-negative integer"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := refusedSearch(t, handler, tc.target)
			if !strings.Contains(message, tc.cites) {
				t.Errorf("GET %s cited %q, want a message naming %q", tc.target, message, tc.cites)
			}
		})
	}
}

// TestSearchWorkflowQueryBoundCountsUTF8Bytes pins the unit used by the shared text safety bound.
// A multibyte query at exactly the byte ceiling is accepted, while adding one ASCII byte is
// refused. This distinguishes the implemented byte contract from a rune-counting implementation.
func TestSearchWorkflowQueryBoundCountsUTF8Bytes(t *testing.T) {
	handler := newHandler(t)
	atBound := strings.Repeat("é", service.MaxQueryChars/len("é"))
	if len(atBound) != service.MaxQueryChars {
		t.Fatalf("test query is %d bytes, want %d", len(atBound), service.MaxQueryChars)
	}

	if rec := do(t, handler, http.MethodGet, searchTarget(atBound)); rec.Code != http.StatusOK {
		t.Fatalf("a %d-byte UTF-8 query returned %d, want 200: %s",
			len(atBound), rec.Code, rec.Body.String())
	}

	message := refusedSearch(t, handler, searchTarget(atBound+"a"))
	if !strings.Contains(message, strconv.Itoa(service.MaxQueryChars)+" UTF-8 bytes") {
		t.Errorf("the byte-bound refusal was not explicit about its unit: %q", message)
	}
}

// TestSearchWorkflowConflictingFiltersAreRefusedRatherThanReconciled covers the combinations that
// exist only because the scope filter has two spellings.
//
// A structurally invalid combination is refused. It is never repaired by preferring one filter, by
// intersecting the two, or by dropping the one that matched nothing — each of which would answer a
// question the caller did not ask, and would do so with a response that looks correct.
func TestSearchWorkflowConflictingFiltersAreRefusedRatherThanReconciled(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct{ name, target string }{
		{"agreeing filters are still two filters", searchTarget("fixture", "type=node", "entity_types=node")},
		{"disagreeing filters", searchTarget("fixture", "type=node", "entity_types=claim")},
		{"the single filter inside the list", searchTarget("fixture", "type=node", "entity_types=node,claim")},
		{"order does not matter", searchTarget("fixture", "entity_types=claim", "type=node")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := refusedSearch(t, handler, tc.target)
			if !strings.Contains(message, "must not be combined") {
				t.Errorf("GET %s cited %q, want the combination rule", tc.target, message)
			}
		})
	}

	// A blank type is absent, exactly as a blank filter is on every other route, so it does not
	// conflict with a scope list. This is the one place both spellings can appear in one query string
	// without being a conflict, and it is pinned so the rule stays a decision rather than an accident
	// of parameter parsing.
	t.Run("a blank type is absent rather than conflicting", func(t *testing.T) {
		res := workflowSearch(t, handler, searchTarget("fixture", "type=", "entity_types=claim"))
		if res.Type != "" {
			t.Errorf("a blank type was echoed as %q", res.Type)
		}
		if want := []string{"claim"}; !reflect.DeepEqual(res.EntityTypes, want) {
			t.Errorf("entity_types echoed %v, want %v", res.EntityTypes, want)
		}
		if res.Facets.EntityTypes.Total() != res.Page.Total {
			t.Errorf("facets did not describe the scoped set: %+v", res.Facets)
		}
		for _, r := range res.Results {
			if r.EntityType != domain.SearchClaim {
				t.Errorf("scope leaked %s", resultKey(r))
			}
		}
	})
}

// TestSearchWorkflowUnsatisfiableRequestsAreAnswered separates the two ways a search returns nothing,
// which is the distinction this phase most needs pinned: a malformed request is refused, and a
// well-formed request the corpus cannot satisfy is answered.
//
// Answered means the complete response shape — the echo, the page metadata, the full facet structure
// with every count at zero, and an empty results array — not a 404 and not a partial body. "The
// corpus contains no such text" is an answer, and a client must be able to read it with the same
// code path it reads a hit with.
func TestSearchWorkflowUnsatisfiableRequestsAreAnswered(t *testing.T) {
	handler := newHandler(t)

	cases := []struct{ name, target string }{
		{"text nothing carries", searchTarget("nothing-in-this-corpus-matches-this")},
		{"control characters are searched, not refused", searchTarget("fixture\x01term")},
		{"a legal scope the query cannot satisfy", searchTarget("fixture term", "type=session")},
		{"a legal scope list the query cannot satisfy", searchTarget("synthetic root", "entity_types=claim,source")},
		{"a composed query no single record satisfies",
			searchTarget("alpha nothing-in-this-corpus-matches-this", "query_mode=all_terms")},
		{"scope and composition together exclude everything",
			searchTarget("fixture term", "query_mode=all_terms", "entity_types=session")},
		{"a window past the end of an empty set",
			searchTarget("nothing-in-this-corpus-matches-this", "limit=5", "offset=50")},
		{"context requested on an empty set",
			searchTarget("nothing-in-this-corpus-matches-this", "include_context=true")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, tc.target)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200: %s", tc.target, rec.Code, rec.Body.String())
			}
			var res service.SearchResults
			decode(t, rec, &res)

			if len(res.Results) != 0 || res.Page.Total != 0 || res.Page.Count != 0 {
				t.Fatalf("GET %s returned %d of %d results", tc.target, len(res.Results), res.Page.Total)
			}
			if res.Facets.EntityTypes.Total() != 0 {
				t.Errorf("GET %s reported facets on an empty set: %+v", tc.target, res.Facets)
			}

			// The whole facet structure is serialised on an empty search, and the results array is
			// present rather than null: a class with no hits is a fact the caller asked for, and an
			// absent key could not tell it from a class this build does not search.
			var body struct {
				Results *[]json.RawMessage         `json:"results"`
				Facets  map[string]json.RawMessage `json:"facets"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("GET %s: decode: %v", tc.target, err)
			}
			if body.Results == nil {
				t.Errorf("GET %s omitted the results array instead of returning it empty", tc.target)
			}
			raw, present := body.Facets["entity_types"]
			if !present {
				t.Fatalf("GET %s omitted the facet object", tc.target)
			}
			classes := map[string]int{}
			if err := json.Unmarshal(raw, &classes); err != nil {
				t.Fatalf("GET %s: decode facets: %v", tc.target, err)
			}
			for _, class := range domain.SearchEntityTypes {
				if _, counted := classes[string(class)]; !counted {
					t.Errorf("GET %s omitted the %s facet from an empty result set", tc.target, class)
				}
			}
			if len(classes) != len(domain.SearchEntityTypes) {
				t.Errorf("GET %s facet object carries %d classes, want %d",
					tc.target, len(classes), len(domain.SearchEntityTypes))
			}
		})
	}
}

// TestSearchWorkflowResultBoundsAreEnforced states the paging contract in one table, including the
// spellings a caller reaches for when trying to remove the bound.
func TestSearchWorkflowResultBoundsAreEnforced(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		name      string
		target    string
		wantLimit int
	}{
		{"an absent limit is the default", searchTarget("fixture"), service.DefaultLimit},
		{"a blank limit is the default", searchTarget("fixture", "limit="), service.DefaultLimit},
		{"a zero limit is the default", searchTarget("fixture", "limit=0"), service.DefaultLimit},
		{"a limit inside the bound is honoured", searchTarget("fixture", "limit=2"), 2},
		{"the maximum is accepted", searchTarget("fixture", "limit=200"), service.MaxLimit},
		{"one past the maximum clamps", searchTarget("fixture", "limit=201"), service.MaxLimit},
		{"a very large limit clamps", searchTarget("fixture", "limit=1000000"), service.MaxLimit},
		{"the bound survives every other control",
			searchTarget("fixture term", "query_mode=all_terms", "entity_types=vocabulary",
				"include_context=true", "limit=999"), service.MaxLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := workflowSearch(t, handler, tc.target)
			if res.Page.Limit != tc.wantLimit {
				t.Errorf("GET %s reported limit %d, want %d", tc.target, res.Page.Limit, tc.wantLimit)
			}
			if len(res.Results) > tc.wantLimit {
				t.Errorf("GET %s returned %d results under a limit of %d",
					tc.target, len(res.Results), tc.wantLimit)
			}
		})
	}

	// An offset past the end is a legal window onto a non-empty set: the page is empty, and the facets
	// and the total still describe the whole set rather than the window.
	t.Run("a window past the end still describes the whole set", func(t *testing.T) {
		full := workflowSearch(t, handler, searchTarget("fixture", "limit=200"))
		past := workflowSearch(t, handler, searchTarget("fixture", "limit=5", "offset=1000"))
		if len(past.Results) != 0 {
			t.Errorf("a window past the end returned %d results", len(past.Results))
		}
		if past.Page.Total != full.Page.Total {
			t.Errorf("page.total changed with the window: %d then %d", full.Page.Total, past.Page.Total)
		}
		if past.Facets != full.Facets {
			t.Errorf("facets changed with the window: %+v then %+v", full.Facets, past.Facets)
		}
	})
}

// TestSearchWorkflowBoundingFollowsRanking requires every page to be a contiguous window onto the one
// ranked result set rather than a separately ordered subset.
//
// This is the property that makes paging safe to reason about: a page boundary can never hide a
// record that satisfied the query, a client walking the pages sees each hit exactly once, and the
// bound is applied after ranking rather than as a shortcut inside it. It is checked by comparing each
// window against the corresponding slice of the complete set, which no per-feature test could do
// without also owning the ranking contract.
func TestSearchWorkflowBoundingFollowsRanking(t *testing.T) {
	handler := newHandler(t)

	for _, base := range []struct {
		name  string
		query string
		extra []string
	}{
		{"literal", "fixture", nil},
		{"literal with context", "fixture", []string{"include_context=true"}},
		{"scoped", "fixture", []string{"entity_types=claim,source"}},
		{"composed", "fixture term", []string{"query_mode=all_terms"}},
	} {
		t.Run(base.name, func(t *testing.T) {
			full := workflowSearch(t, handler,
				searchTarget(base.query, append(append([]string(nil), base.extra...), "limit=200")...))
			if full.Page.Total < 2 {
				t.Fatalf("the fixture corpus returned %d results for %q; at least two are needed",
					full.Page.Total, base.query)
			}

			for offset := 0; offset <= full.Page.Total; offset++ {
				for _, limit := range []int{1, 2, full.Page.Total} {
					params := append(append([]string(nil), base.extra...),
						"limit="+strconv.Itoa(limit), "offset="+strconv.Itoa(offset))
					page := workflowSearch(t, handler, searchTarget(base.query, params...))

					if page.Page.Total != full.Page.Total {
						t.Fatalf("offset %d limit %d: page.total %d, want %d",
							offset, limit, page.Page.Total, full.Page.Total)
					}
					if page.Facets != full.Facets {
						t.Fatalf("offset %d limit %d: the facets moved with the page", offset, limit)
					}

					end := offset + limit
					if end > full.Page.Total {
						end = full.Page.Total
					}
					var want []domain.SearchResult
					if offset < full.Page.Total {
						want = full.Results[offset:end]
					}
					if len(want) == 0 && len(page.Results) == 0 {
						continue
					}
					if !reflect.DeepEqual(page.Results, want) {
						t.Fatalf("offset %d limit %d is not the matching window of the ranked set:\n got %v\nwant %v",
							offset, limit, resultKeys(page.Results), resultKeys(want))
					}
				}
			}
		})
	}
}

// TestSearchWorkflowContextIsAdditive requires the context control to change what a result carries
// and nothing else: not which records matched, not their order, not their scores, not their evidence,
// not the facets and not the page metadata.
//
// It is checked by taking the context off the enriched response and requiring what remains to be
// exactly the plain one. That is a stronger statement than comparing identifiers, because it also
// catches a resolver that quietly rewrote a title, reordered a matched-field list, or spent part of
// its budget changing a score.
func TestSearchWorkflowContextIsAdditive(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		name   string
		query  string
		params []string
	}{
		{"literal", "fixture", nil},
		{"scoped", "fixture", []string{"entity_types=node,claim"}},
		{"single class", "fixture", []string{"type=vocabulary"}},
		{"composed", "fixture term", []string{"query_mode=all_terms"}},
		{"paged", "fixture", []string{"limit=2", "offset=1"}},
		{"empty", "nothing-in-this-corpus-matches-this", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plain := workflowSearch(t, handler, searchTarget(tc.query, tc.params...))
			enriched := workflowSearch(t, handler,
				searchTarget(tc.query, append(append([]string(nil), tc.params...), "include_context=true")...))

			if !enriched.IncludeContext {
				t.Fatalf("the context control was not echoed")
			}
			if plain.IncludeContext {
				t.Fatalf("a request without the control echoed it")
			}
			if enriched.Page != plain.Page {
				t.Errorf("context changed the page: %+v then %+v", plain.Page, enriched.Page)
			}
			if enriched.Facets != plain.Facets {
				t.Errorf("context changed the facets: %+v then %+v", plain.Facets, enriched.Facets)
			}
			if enriched.Query != plain.Query || enriched.QueryMode != plain.QueryMode ||
				enriched.Type != plain.Type || !reflect.DeepEqual(enriched.EntityTypes, plain.EntityTypes) {
				t.Errorf("context changed the echo of the query that ran")
			}

			stripped := make([]domain.SearchResult, len(enriched.Results))
			copy(stripped, enriched.Results)
			for i := range stripped {
				if stripped[i].Context == nil {
					t.Errorf("result %d was returned without context", i)
				}
				stripped[i].Context = nil
			}
			if !reflect.DeepEqual(stripped, plain.Results) {
				t.Errorf("context changed the results themselves:\n got %v\nwant %v",
					resultKeys(stripped), resultKeys(plain.Results))
			}
		})
	}
}

// TestSearchWorkflowMultiTermCompositionIsNormalised pins the composition semantics the rest of the
// pipeline inherits: what a term is, what repetition means, and what whitespace means.
//
// Each row states two requests that must produce byte-identical responses. Comparing whole responses
// rather than result identifiers means the echo, the facets and the evidence are all covered by the
// same assertion.
func TestSearchWorkflowMultiTermCompositionIsNormalised(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct{ name, left, right string }{
		{"surrounding whitespace is trimmed", "  fixture term  ", "fixture term"},
		{"runs of whitespace collapse", "fixture     term", "fixture term"},
		{"tabs and newlines separate terms", "fixture\tterm", "fixture term"},
		{"a repeated term collapses to its first occurrence", "fixture term fixture", "fixture term"},
		{"repetition anywhere collapses", "term fixture term", "term fixture"},
		{"case is folded", "FIXTURE Term", "fixture term"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left := do(t, handler, http.MethodGet, searchTarget(tc.left, "query_mode=all_terms")).Body.String()
			right := do(t, handler, http.MethodGet, searchTarget(tc.right, "query_mode=all_terms")).Body.String()
			if left != right {
				t.Errorf("%q and %q are different requests:\n%s\n%s", tc.left, tc.right, left, right)
			}
		})
	}

	// Term order does not change which records match — every term must occur somewhere in the record
	// either way — but it does change the phrase, and therefore the phrase-derived signals. Both
	// halves are asserted: a test that only checked the set would pass a build that had quietly
	// stopped ranking the phrase, and one that only compared bytes would read the documented
	// order-sensitivity as a defect.
	t.Run("term order changes the phrase and not the match set", func(t *testing.T) {
		forward := workflowSearch(t, handler, searchTarget("synthetic root", "query_mode=all_terms"))
		reversed := workflowSearch(t, handler, searchTarget("root synthetic", "query_mode=all_terms"))

		forwardKeys := resultKeys(forward.Results)
		reversedKeys := resultKeys(reversed.Results)
		sort.Strings(forwardKeys)
		sort.Strings(reversedKeys)
		if !reflect.DeepEqual(forwardKeys, reversedKeys) {
			t.Errorf("term order changed the match set: %v then %v", forwardKeys, reversedKeys)
		}
		if forward.Query != "synthetic root" || reversed.Query != "root synthetic" {
			t.Errorf("the echoed phrase did not follow the caller's order: %q then %q",
				forward.Query, reversed.Query)
		}
		if !containsSignal(forward.Results, domain.SignalPhraseMatch) {
			t.Errorf("the contiguous phrase fired no phrase_match: %v", resultKeys(forward.Results))
		}
		if containsSignal(reversed.Results, domain.SignalPhraseMatch) {
			t.Errorf("the reversed phrase fired phrase_match although it is not contiguous")
		}
	})

	// Composition is not a query language. An operator-looking token is an ordinary term, required
	// literally like any other, so inserting one can only narrow the query — never combine, negate or
	// widen it. The check states that directly: the token appears in the echoed term list, every hit
	// carries it as satisfied evidence, and the result set is a subset of the query without it.
	t.Run("operator-looking tokens are ordinary terms", func(t *testing.T) {
		withOperator := workflowSearch(t, handler, searchTarget("fixture AND term", "query_mode=all_terms"))
		without := workflowSearch(t, handler, searchTarget("fixture term", "query_mode=all_terms"))

		if withOperator.Query != "fixture and term" {
			t.Errorf("echo = %q, want the normalised three-term list", withOperator.Query)
		}
		if withOperator.Page.Total > without.Page.Total {
			t.Errorf("adding a required term widened the query: %d then %d",
				without.Page.Total, withOperator.Page.Total)
		}

		kept := make(map[string]bool, len(without.Results))
		for _, r := range without.Results {
			kept[resultKey(r)] = true
		}
		for _, r := range withOperator.Results {
			if !kept[resultKey(r)] {
				t.Errorf("%s matched only once 'and' was required, so the token was not a term",
					resultKey(r))
			}
			satisfied := false
			for _, match := range r.TermMatches {
				if match.Term == "and" && len(match.MatchedFields) > 0 {
					satisfied = true
				}
			}
			if !satisfied {
				t.Errorf("%s was returned without evidence for the required term %q: %+v",
					resultKey(r), "and", r.TermMatches)
			}
		}
	})

	// The two modes are the same retrieval over the same fields, differing only in composition. Under
	// literal, whitespace is part of the phrase; under all_terms it separates terms.
	t.Run("literal keeps whitespace inside the phrase", func(t *testing.T) {
		literal := workflowSearch(t, handler, searchTarget("synthetic root"))
		composed := workflowSearch(t, handler, searchTarget("synthetic root", "query_mode=all_terms"))
		if literal.QueryMode != "" {
			t.Errorf("a literal response echoed query_mode = %q", literal.QueryMode)
		}
		if composed.QueryMode != string(domain.SearchModeAllTerms) {
			t.Errorf("a composed response echoed query_mode = %q", composed.QueryMode)
		}
		for _, r := range literal.Results {
			if len(r.TermMatches) != 0 {
				t.Errorf("a literal result carried term_matches: %+v", r.TermMatches)
			}
		}
	})
}

// TestSearchWorkflowEveryDocumentedSignalIsReachable walks the closed signal set and requires each
// member to be produced by some request against the fixture corpus.
//
// A weight table nothing can fire is dead contract: it would be documented, internally consistent
// under the weight invariant, and still describe a ranking the API cannot express. This is the
// regression that ties the published signal table to observable behaviour, and it fails loudly if a
// later change makes a signal unreachable without also removing it from the set.
func TestSearchWorkflowEveryDocumentedSignalIsReachable(t *testing.T) {
	handler := newHandler(t)

	// Each row names the request that demonstrates the signal, so the table doubles as a worked
	// example of what fires each one.
	demonstrations := []struct {
		signal string
		target string
	}{
		{domain.SignalIDExact, searchTarget("alpha")},
		{domain.SignalTitleExact, searchTarget("fixture term")},
		{domain.SignalTitlePrefix, searchTarget("Fixture Te")},
		{domain.SignalTitleSubstring, searchTarget("ompanion")},
		{domain.SignalPhraseMatch, searchTarget("synthetic root", "query_mode=all_terms")},
		{domain.SignalTitleAllTerms, searchTarget("fixture term", "query_mode=all_terms")},
		{domain.SignalTitleTerms, searchTarget("fixture harness", "query_mode=all_terms")},
		{domain.SignalIDSubstring, searchTarget("fixture")},
		{domain.SignalFieldMatch, searchTarget("synthetic")},
	}

	reached := make(map[string]bool, len(demonstrations))
	for _, demo := range demonstrations {
		res := workflowSearch(t, handler, demo.target)
		if !containsSignal(res.Results, demo.signal) {
			t.Errorf("GET %s fired no %s: %v", demo.target, demo.signal, resultKeys(res.Results))
			continue
		}
		reached[demo.signal] = true
	}
	for _, signal := range domain.SearchMatchSignals {
		if !reached[signal] {
			t.Errorf("signal %s is documented and carries weight %d, but no request demonstrates it",
				signal, domain.SearchSignalWeight(signal))
		}
	}
}

// TestSearchWorkflowRankingIsUnaffectedByScopeContextAndPaging requires a hit's score and explanation
// to be a function of the query and that record alone.
//
// A score that moved when other records were filtered out, when the page window changed, or when
// context was resolved would not be explainable from the response, because none of those inputs
// appears in match_signals. The check compares each hit's whole explanation across requests that
// differ only in controls the ranking policy must not read.
func TestSearchWorkflowRankingIsUnaffectedByScopeContextAndPaging(t *testing.T) {
	handler := newHandler(t)

	type explanation struct {
		score   int
		kind    string
		signals string
		fields  string
	}
	explain := func(res service.SearchResults) map[string]explanation {
		out := make(map[string]explanation, len(res.Results))
		for _, r := range res.Results {
			out[resultKey(r)] = explanation{
				score:   r.RelevanceScore,
				kind:    r.MatchKind,
				signals: strings.Join(r.MatchSignals, ","),
				fields:  strings.Join(r.MatchedFields, ","),
			}
		}
		return out
	}

	baseline := explain(workflowSearch(t, handler, searchTarget("fixture", "limit=200")))
	if len(baseline) < 2 {
		t.Fatalf("the fixture corpus returned %d hits for the baseline query", len(baseline))
	}

	for _, variant := range []struct{ name, target string }{
		{"scoped to one class", searchTarget("fixture", "limit=200", "type=vocabulary")},
		{"scoped to a class list", searchTarget("fixture", "limit=200", "entity_types=claim,source")},
		{"context resolved", searchTarget("fixture", "limit=200", "include_context=true")},
		{"a narrow page", searchTarget("fixture", "limit=2", "offset=3")},
		{"a narrow page with context", searchTarget("fixture", "limit=2", "offset=3", "include_context=true")},
	} {
		t.Run(variant.name, func(t *testing.T) {
			for key, got := range explain(workflowSearch(t, handler, variant.target)) {
				want, known := baseline[key]
				if !known {
					t.Errorf("%s appeared under %q but not in the unfiltered search", key, variant.name)
					continue
				}
				if got != want {
					t.Errorf("%s was explained differently under %q:\n got %+v\nwant %+v",
						key, variant.name, got, want)
				}
			}
		})
	}
}

// TestSearchWorkflowResponsesDiscloseNothingInternal sweeps the matrix and requires no successful
// response to carry a Go identifier, a source position or a filesystem path.
//
// The error path is covered by refusedSearch; this is the success path, where the risk is different:
// a result's title, summary, matched-field list, signal list and context origin are all strings
// assembled from repository content, and any of them could become a disclosure channel through a
// careless change.
func TestSearchWorkflowResponsesDiscloseNothingInternal(t *testing.T) {
	handler := newHandler(t)
	for _, tc := range workflowMatrix() {
		res := workflowSearch(t, handler, tc.target)
		for _, r := range res.Results {
			assertNoInternalDisclosure(t, tc.target, r.Title)
			assertNoInternalDisclosure(t, tc.target, r.Summary)
			assertNoInternalDisclosure(t, tc.target, strings.Join(r.MatchedFields, " "))
			assertNoInternalDisclosure(t, tc.target, strings.Join(r.MatchSignals, " "))
		}
	}
}

// TestSearchWorkflowAcceptsOnlyTheDocumentedParameters ties the published parameter table to the set
// the router actually accepts.
//
// The refusal lists what the endpoint supports, so it is the endpoint's own statement of its surface
// rather than a second copy maintained by hand. A parameter added to the handler without being
// documented, or documented without being accepted, fails here.
func TestSearchWorkflowAcceptsOnlyTheDocumentedParameters(t *testing.T) {
	handler := newHandler(t)
	documented := []string{"q", "type", "entity_types", "query_mode", "include_context", "limit", "offset"}

	message := refusedSearch(t, handler, searchTarget("fixture", "undocumented=1"))
	_, listed, found := strings.Cut(message, "Supported: ")
	if !found {
		t.Fatalf("the refusal did not list the supported parameters: %s", message)
	}
	got := strings.Split(strings.TrimSuffix(strings.TrimSpace(listed), "."), ", ")
	if !reflect.DeepEqual(got, documented) {
		t.Fatalf("the endpoint accepts %v, but the API documentation describes %v", got, documented)
	}

	// Every documented parameter is genuinely accepted, so the list is not merely internally
	// consistent. Each is supplied with a value the contract calls valid.
	for _, target := range []string{
		searchTarget("fixture"),
		searchTarget("fixture", "type=node"),
		searchTarget("fixture", "entity_types=node,claim"),
		searchTarget("fixture term", "query_mode=all_terms"),
		searchTarget("fixture", "include_context=true"),
		searchTarget("fixture", "limit=5"),
		searchTarget("fixture", "offset=1"),
	} {
		if rec := do(t, handler, http.MethodGet, target); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200: %s", target, rec.Code, rec.Body.String())
		}
	}
}

// TestSearchWorkflowStaysReadOnly requires the read-only guarantee to hold across the whole search
// surface, with every control present, and to be enforced before the query is parsed.
//
// A mutating method is refused with the documented Allow header whether or not the request that
// carried it would have been valid, so a caller cannot learn anything about the corpus — or about
// which of their parameters was wrong — by probing with POST.
func TestSearchWorkflowStaysReadOnly(t *testing.T) {
	handler := newHandler(t)
	targets := []string{
		searchTarget("fixture"),
		searchTarget("fixture term", "query_mode=all_terms", "entity_types=vocabulary",
			"include_context=true", "limit=2"),
		httpapi.APIBase + "/search",
		searchTarget("fixture", "bogus=1"),
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		for _, target := range targets {
			rec := do(t, handler, method, target)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, target, rec.Code)
			}
			if got := rec.Header().Get("Allow"); got != "GET, HEAD" {
				t.Errorf("%s %s Allow = %q, want %q", method, target, got, "GET, HEAD")
			}
		}
	}

	// HEAD is served from the GET handler, so it must agree with it on status and on headers.
	for _, target := range targets[:2] {
		get := do(t, handler, http.MethodGet, target)
		head := do(t, handler, http.MethodHead, target)
		if get.Code != head.Code {
			t.Errorf("HEAD %s = %d, GET = %d", target, head.Code, get.Code)
		}
		if get.Header().Get("Content-Type") != head.Header().Get("Content-Type") {
			t.Errorf("HEAD %s content-type = %q, GET = %q", target,
				head.Header().Get("Content-Type"), get.Header().Get("Content-Type"))
		}
	}
}
