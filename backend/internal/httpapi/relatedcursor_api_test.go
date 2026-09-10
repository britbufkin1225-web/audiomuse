package httpapi_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/service"
)

// The Phase 2D continuation tests at the HTTP boundary.
//
// The cursor contract itself - what a token binds, how a traversal partitions the ordering, what a
// corrupted payload is refused for - is tested in the service, where a payload can be edited field
// by field. What matters here is the wire contract: that the parameter exists and is spelled the
// way this API spells parameters, that a token survives a round trip through a query string
// unescaped and unaltered, that the two new response fields serialise under the names clients will
// read, that every refusal lands in the stable error envelope, and that no refusal echoes the one
// input a caller controls.

// pagedRelatedOverHTTP walks a whole traversal through the handler and returns the concatenated
// item identities.
//
// It re-sends the same query string on every page with only the token changed, which is the client
// this contract is written for, and it fails rather than loops if the traversal does not terminate.
func pagedRelatedOverHTTP(
	t testing.TB, handler http.Handler, entityType, id string, params map[string]string,
) ([]string, int) {
	t.Helper()

	var refs []string
	pages := 0
	token := ""
	for {
		page := make(map[string]string, len(params)+1)
		for name, value := range params {
			page[name] = value
		}
		if token != "" {
			page["continuation_token"] = token
		}
		result := getRelated(t, handler, entityType, id, page)
		pages++

		if result.HasMore != (result.NextContinuationToken != "") {
			t.Fatalf("page %d: has_more = %v beside token %q", pages, result.HasMore, result.NextContinuationToken)
		}
		for _, item := range result.Items {
			refs = append(refs, string(item.EntityType)+"/"+item.ID)
		}
		if !result.HasMore {
			return refs, pages
		}
		token = result.NextContinuationToken
		if pages > result.Counts.Eligible+1 {
			t.Fatalf("traversal did not terminate after %d pages", pages)
		}
	}
}

// TestRelatedContinuationAPIAcceptsTheParameter pins the parameter's spelling and the shared
// query-string guard's treatment of it.
//
// The guard is the same one every route uses, so what is asserted here is that continuation_token
// was added to this route's accepted set and to nothing else: a neighbouring route must still
// refuse it, and this route must still refuse the near-misses a caller might guess at.
func TestRelatedContinuationAPIAcceptsTheParameter(t *testing.T) {
	handler := newHandler(t)

	t.Run("accepted-on-related", func(t *testing.T) {
		result := getRelated(t, handler, "node", "alpha", map[string]string{"limit": "2"})
		if result.NextContinuationToken == "" {
			t.Fatal("a two-item page of the fixture start issued no token")
		}
		getRelated(t, handler, "node", "alpha", map[string]string{
			"limit": "2", "continuation_token": result.NextContinuationToken,
		})
	})

	for _, name := range []string{"continuation", "continuationToken", "continuation-token", "cursor", "offset", "page"} {
		t.Run("rejected-"+name, func(t *testing.T) {
			status, code, _ := relatedErrorCode(t, handler,
				"/api/v1/related/node/alpha?"+name+"=x")
			if status != http.StatusBadRequest || code != "invalid_query" {
				t.Errorf("status = %d code = %q, want 400 invalid_query", status, code)
			}
		})
	}

	t.Run("rejected-on-search", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/search?q=alpha&continuation_token=x")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400: continuation_token is a related-knowledge parameter", rec.Code)
		}
	})

	t.Run("rejected-twice", func(t *testing.T) {
		status, code, _ := relatedErrorCode(t, handler,
			"/api/v1/related/node/alpha?continuation_token=a&continuation_token=b")
		if status != http.StatusBadRequest || code != "invalid_query" {
			t.Errorf("status = %d code = %q, want 400 invalid_query", status, code)
		}
	})
}

// TestRelatedContinuationAPIRefusesAnEmptyToken pins the present-but-empty case, which only the
// query string can express.
//
// A caller who wrote the parameter and supplied nothing to resume from is not asking for the first
// page, and answering with one would silently restart a traversal they believe they are part-way
// through. Whitespace trims to the same thing and is refused with it.
func TestRelatedContinuationAPIRefusesAnEmptyToken(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct{ name, target string }{
		{"empty", "/api/v1/related/node/alpha?continuation_token="},
		{"space", "/api/v1/related/node/alpha?continuation_token=%20"},
		{"tab", "/api/v1/related/node/alpha?continuation_token=%09"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, code, message := relatedErrorCode(t, handler, tc.target)
			if status != http.StatusBadRequest || code != "invalid_query" {
				t.Errorf("status = %d code = %q, want 400 invalid_query", status, code)
			}
			if !strings.Contains(message, "continuation_token") {
				t.Errorf("message %q does not name the parameter", message)
			}
		})
	}
}

// TestRelatedContinuationAPIWalksAWholeTraversal is the end-to-end assertion.
//
// A client that re-sends the same query string with each token it is handed must see exactly the
// records one large request returns, in the same order. Every page size the fixture start can be
// walked at is covered, so both the seam between full pages and the final partial page are
// asserted rather than one of them.
func TestRelatedContinuationAPIWalksAWholeTraversal(t *testing.T) {
	handler := newHandler(t)

	whole := getRelated(t, handler, "node", "alpha", map[string]string{
		"limit": strconv.Itoa(service.MaxRelatedLimit),
	})
	want := make([]string, 0, len(whole.Items))
	for _, item := range whole.Items {
		want = append(want, string(item.EntityType)+"/"+item.ID)
	}
	if len(want) < 3 {
		t.Fatalf("the fixture start returns %d items, too few to walk", len(want))
	}

	for _, size := range []int{1, 2, 3, len(want) - 1} {
		t.Run(fmt.Sprintf("limit-%d", size), func(t *testing.T) {
			got, pages := pagedRelatedOverHTTP(t, handler, "node", "alpha",
				map[string]string{"limit": strconv.Itoa(size)})
			if !reflect.DeepEqual(got, want) {
				t.Errorf("paged over %d pages:\n%s\nwant\n%s",
					pages, strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// TestRelatedContinuationAPIWalksAFilteredTraversal repeats the walk with both filters applied.
//
// The filters travel in the query string and the scope they resolved to travels in the token, and
// this is where the two must agree: a page four of a filtered traversal must be the same records a
// single filtered request would have returned at that position, and the echoed scope must not
// drift across pages.
func TestRelatedContinuationAPIWalksAFilteredTraversal(t *testing.T) {
	handler := newHandler(t)

	params := map[string]string{
		"limit":              "1",
		"entity_types":       "claim,node",
		"relationship_types": "conceptual,assertional",
	}
	whole := map[string]string{
		"limit":              strconv.Itoa(service.MaxRelatedLimit),
		"entity_types":       params["entity_types"],
		"relationship_types": params["relationship_types"],
	}
	full := getRelated(t, handler, "node", "alpha", whole)
	want := make([]string, 0, len(full.Items))
	for _, item := range full.Items {
		want = append(want, string(item.EntityType)+"/"+item.ID)
	}
	if len(want) < 2 {
		t.Fatalf("the filtered fixture start returns %d items, too few to walk", len(want))
	}

	got, pages := pagedRelatedOverHTTP(t, handler, "node", "alpha", params)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filtered walk over %d pages:\n%s\nwant\n%s",
			pages, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// The echoes are part of the page contract and must describe the same request on every page.
	second := getRelated(t, handler, "node", "alpha", map[string]string{
		"limit":              "1",
		"entity_types":       params["entity_types"],
		"relationship_types": params["relationship_types"],
		"continuation_token": getRelated(t, handler, "node", "alpha", params).NextContinuationToken,
	})
	if !reflect.DeepEqual(second.EntityTypes, full.EntityTypes) {
		t.Errorf("entity_types echo = %v, want %v", second.EntityTypes, full.EntityTypes)
	}
	if !reflect.DeepEqual(second.RelationshipTypes, full.RelationshipTypes) {
		t.Errorf("relationship_types echo = %v, want %v", second.RelationshipTypes, full.RelationshipTypes)
	}
}

// TestRelatedContinuationAPIOmittedLimitInheritsTheToken pins the limit contract as a client meets
// it.
//
// A traversal is expressible as "the same URL plus a token", so a continuation that names no limit
// uses the one the token carries rather than falling back to the default. Naming a different limit
// is refused, and the message says how to continue rather than only what was wrong.
func TestRelatedContinuationAPIOmittedLimitInheritsTheToken(t *testing.T) {
	handler := newHandler(t)

	first := getRelated(t, handler, "node", "alpha", map[string]string{"limit": "2"})
	if first.NextContinuationToken == "" {
		t.Fatal("a two-item page issued no token")
	}

	second := getRelated(t, handler, "node", "alpha", map[string]string{
		"continuation_token": first.NextContinuationToken,
	})
	if second.Limit != 2 {
		t.Errorf("limit = %d, want the token's 2 rather than the default %d",
			second.Limit, service.DefaultRelatedLimit)
	}
	if len(second.Items) != 2 {
		t.Errorf("items = %d, want 2", len(second.Items))
	}

	status, code, message := relatedErrorCode(t, handler,
		relatedURL("node", "alpha", map[string]string{
			"limit": "5", "continuation_token": first.NextContinuationToken,
		}))
	if status != http.StatusBadRequest || code != "invalid_query" {
		t.Errorf("status = %d code = %q, want 400 invalid_query", status, code)
	}
	if !strings.Contains(message, "Omit limit") {
		t.Errorf("message %q does not say how to continue", message)
	}
}

// TestRelatedContinuationAPIRefusalsAreStable walks every continuation refusal at the boundary.
//
// Each must be a 400 under the stable invalid_query code, must name the parameter so a client
// knows which of the four to change, and must never echo the token. The token is the one part of
// this request an attacker fully controls, and a message that reflected it would turn the error
// envelope into a mirror; a message that named the decoding step that failed would turn it into an
// oracle.
func TestRelatedContinuationAPIRefusalsAreStable(t *testing.T) {
	handler := newHandler(t)

	issued := map[string]string{
		"limit":              "1",
		"entity_types":       "claim,node",
		"relationship_types": "conceptual,assertional",
	}
	token := getRelated(t, handler, "node", "alpha", issued).NextContinuationToken
	if token == "" {
		t.Fatal("the filtered fixture start issued no token")
	}

	// A token this build cannot read, and one it can read but was issued under another version.
	garbage := "not-a-token"
	futureVersion := func() string {
		payload, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			t.Fatalf("decode token: %v", err)
		}
		var fields map[string]any
		if err := json.Unmarshal(payload, &fields); err != nil {
			t.Fatalf("unmarshal token: %v", err)
		}
		fields["v"] = 99
		body, err := json.Marshal(fields)
		if err != nil {
			t.Fatalf("marshal token: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(body)
	}()

	for _, tc := range []struct {
		name   string
		params map[string]string
		start  string
		expect string
	}{
		{
			name: "malformed", start: "alpha", expect: "not a valid continuation token",
			params: map[string]string{"limit": "1", "continuation_token": garbage},
		},
		{
			name: "oversized", start: "alpha", expect: "not a valid continuation token",
			params: map[string]string{
				"limit":              "1",
				"continuation_token": strings.Repeat("A", service.MaxRelatedContinuationTokenChars+1),
			},
		},
		{
			name: "unsupported-version", start: "alpha", expect: "continuation contract",
			params: map[string]string{
				"limit": "1", "entity_types": issued["entity_types"],
				"relationship_types": issued["relationship_types"], "continuation_token": futureVersion,
			},
		},
		{
			name: "different-start", start: "beta", expect: "different start record",
			params: map[string]string{
				"limit": "1", "entity_types": issued["entity_types"],
				"relationship_types": issued["relationship_types"], "continuation_token": token,
			},
		},
		{
			name: "different-entity-scope", start: "alpha", expect: "different entity_types scope",
			params: map[string]string{
				"limit": "1", "entity_types": "node",
				"relationship_types": issued["relationship_types"], "continuation_token": token,
			},
		},
		{
			name: "different-relationship-scope", start: "alpha", expect: "different relationship_types scope",
			params: map[string]string{
				"limit": "1", "entity_types": issued["entity_types"],
				"relationship_types": "conceptual", "continuation_token": token,
			},
		},
		{
			name: "different-limit", start: "alpha", expect: "different limit",
			params: map[string]string{
				"limit": "4", "entity_types": issued["entity_types"],
				"relationship_types": issued["relationship_types"], "continuation_token": token,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, code, message := relatedErrorCode(t, handler, relatedURL("node", tc.start, tc.params))
			if status != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", status)
			}
			if code != "invalid_query" {
				t.Errorf("code = %q, want invalid_query", code)
			}
			if !strings.Contains(message, "continuation_token") {
				t.Errorf("message %q does not name the parameter", message)
			}
			if !strings.Contains(message, tc.expect) {
				t.Errorf("message %q does not say %q", message, tc.expect)
			}
			for _, secret := range []string{
				tc.params["continuation_token"], "json", "base64", "decode", "struct", "relatedCursor",
			} {
				if secret != "" && strings.Contains(message, secret) {
					t.Errorf("message %q leaks %q", message, secret)
				}
			}
		})
	}
}

// TestRelatedContinuationAPISerialisesTheNewFields pins the wire names and their presence rule.
//
// has_more is always present, because a boolean that disappears when it is false cannot be told
// apart from one a server does not implement, and a client reading a missing has_more as "keep
// going" would loop. next_continuation_token is present exactly when has_more is true, so the two
// can never contradict each other on the wire.
func TestRelatedContinuationAPISerialisesTheNewFields(t *testing.T) {
	handler := newHandler(t)

	for _, tc := range []struct {
		name    string
		limit   string
		hasMore bool
	}{
		{"a page with more to come", "2", true},
		{"a complete page", strconv.Itoa(service.MaxRelatedLimit), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet,
				relatedURL("node", "alpha", map[string]string{"limit": tc.limit}))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			var body map[string]json.RawMessage
			decode(t, rec, &body)

			raw, present := body["has_more"]
			if !present {
				t.Fatal("has_more is absent from the response")
			}
			if string(raw) != strconv.FormatBool(tc.hasMore) {
				t.Errorf("has_more = %s, want %v", raw, tc.hasMore)
			}
			if _, present := body["next_continuation_token"]; present != tc.hasMore {
				t.Errorf("next_continuation_token present = %v, want %v", present, tc.hasMore)
			}
		})
	}
}

// TestRelatedContinuationAPITokenIsQuerySafe pins the encoding choice against the transport.
//
// A token is handed back to the caller to put in a URL, so it must survive that round trip without
// the caller having to know how to escape it. Raw base64url has no reserved character in it - no
// padding, no plus, no slash - which is exactly why it was chosen over the standard alphabet.
func TestRelatedContinuationAPITokenIsQuerySafe(t *testing.T) {
	handler := newHandler(t)

	seen := 0
	for _, start := range []struct{ class, id string }{
		{"node", "alpha"},
		{"claim", "alpha-carries-energy"},
		{"session", "session-01-fixture"},
	} {
		result := getRelated(t, handler, start.class, start.id, map[string]string{"limit": "1"})
		token := result.NextContinuationToken
		if token == "" {
			continue
		}
		seen++
		for _, r := range token {
			isBase64URL := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') || r == '-' || r == '_'
			if !isBase64URL {
				t.Errorf("token for %s/%s carries %q, which is not base64url", start.class, start.id, r)
				break
			}
		}
		// The unescaped token in a raw query string must reach the handler unaltered.
		rec := do(t, handler, http.MethodGet,
			"/api/v1/related/"+start.class+"/"+start.id+"?limit=1&continuation_token="+token)
		if rec.Code != http.StatusOK {
			t.Errorf("unescaped token for %s/%s: status = %d, want 200: %s",
				start.class, start.id, rec.Code, rec.Body.String())
		}
	}
	if seen == 0 {
		t.Fatal("no fixture start issued a token, so nothing was asserted")
	}
}

// TestRelatedContinuationAPIIsReadOnlyAndUnchangedForOldClients is the compatibility assertion.
//
// A client written against Phase 2A sends no token and must still receive the Phase 2A response.
// The route also stays read-only: continuation adds a parameter, not a method, and every mutating
// method is refused before routing as it always was.
func TestRelatedContinuationAPIIsReadOnlyAndUnchangedForOldClients(t *testing.T) {
	handler := newHandler(t)

	first := getRelated(t, handler, "node", "alpha", nil)
	if first.HasMore {
		t.Error("the fixture start's default page reports that more follow")
	}
	if first.NextContinuationToken != "" {
		t.Error("the fixture start's default page issued a token")
	}
	if first.Limit != service.DefaultRelatedLimit {
		t.Errorf("limit = %d, want the default %d", first.Limit, service.DefaultRelatedLimit)
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := do(t, handler, method, "/api/v1/related/node/alpha?continuation_token=x")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s = %d, want 405", method, rec.Code)
		}
	}
}
