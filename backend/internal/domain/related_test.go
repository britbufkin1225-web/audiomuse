package domain

import (
	"reflect"
	"strings"
	"testing"
)

// The Phase 2A precedence-model invariants.
//
// They are here rather than in the service because they are properties of the table itself, not
// of any corpus: a ranking policy that only holds for the records that happen to exist today is
// not a policy. Each asserts something a Go switch cannot assert on its own.

// TestEveryRelatedPriorityOriginIsClassified holds the closed set to the table.
//
// A canonical field that reached the context layer without being classified here would rank at
// the very end of every discovery list, quietly, and the only symptom would be a suggestion in an
// unexpected place. Walking the declared set turns that into a build-time failure.
func TestEveryRelatedPriorityOriginIsClassified(t *testing.T) {
	for _, origin := range RelatedPriorityOrigins {
		if got := RelatedPriorityFor(origin); got == PriorityUnclassified {
			t.Errorf("canonical field %q is not classified by the precedence table", origin)
		}
	}
	if len(RelatedPriorityOrigins) == 0 {
		t.Fatal("the origin set is empty, so this test would pass vacuously")
	}
}

// TestRelatedPriorityOriginsAreDistinct asserts no canonical field is listed twice, so the set is
// a set and a later reader can trust its length as the count of fields the model can emit.
func TestRelatedPriorityOriginsAreDistinct(t *testing.T) {
	seen := make(map[string]bool, len(RelatedPriorityOrigins))
	for _, origin := range RelatedPriorityOrigins {
		if seen[origin] {
			t.Errorf("canonical field %q is listed twice", origin)
		}
		seen[origin] = true
	}
}

// TestUnknownOriginIsUnclassifiedRatherThanGuessed asserts the fallback does not pick a class.
//
// A default that resolved to any real precedence would give an unclassified field the standing of
// a classified one, which is the failure mode the fallback exists to prevent.
func TestUnknownOriginIsUnclassifiedRatherThanGuessed(t *testing.T) {
	for _, origin := range []string{"", "node.definition", "node", "relationships", "vocabulary"} {
		if got := RelatedPriorityFor(origin); got != PriorityUnclassified {
			t.Errorf("unknown field %q classified as %s", origin, got)
		}
	}
}

// TestRelatedPriorityRankIsATotalOrder asserts the ranks are the list positions, are strictly
// increasing, and place the fallback last.
//
// The ordering of a discovery result is a comparison of these integers, so a duplicate or an
// out-of-sequence rank would make two precedence classes indistinguishable to the sort while
// still reading as distinct in the response.
func TestRelatedPriorityRankIsATotalOrder(t *testing.T) {
	for i, priority := range RelatedPriorities {
		if got := RelatedPriorityRank(priority); got != i {
			t.Errorf("rank of %s = %d, want its position %d", priority, got, i)
		}
	}
	last := RelatedPriorityRank(PriorityUnclassified)
	if last != len(RelatedPriorities) {
		t.Errorf("unclassified ranks %d, want %d so it sorts after every class",
			last, len(RelatedPriorities))
	}
	for _, priority := range RelatedPriorities {
		if RelatedPriorityRank(priority) >= last {
			t.Errorf("%s does not outrank the unclassified fallback", priority)
		}
	}
}

// TestRelatedPriorityNamesMatchTheModelOrder asserts the rendered set is the model rather than an
// alphabetisation of it, which is what makes a document or an error message readable as policy.
func TestRelatedPriorityNamesMatchTheModelOrder(t *testing.T) {
	names := RelatedPriorityNames()
	if len(names) != len(RelatedPriorities) {
		t.Fatalf("names = %d, priorities = %d", len(names), len(RelatedPriorities))
	}
	for i, name := range names {
		if name != string(RelatedPriorities[i]) {
			t.Errorf("name %d = %q, want %q", i, name, RelatedPriorities[i])
		}
	}
}

// TestNewRelatedReasonDerivesPrecedenceFromTheOriginItReports is the consistency assertion.
//
// A reason's priority and the canonical field it claims to come from must never disagree, because
// a client reading one and trusting the other would be reading two different explanations of one
// connection. Constructing every reason through this one function is what guarantees it; this
// checks the guarantee over the whole closed set rather than over the fields a fixture happens to
// exercise.
func TestNewRelatedReasonDerivesPrecedenceFromTheOriginItReports(t *testing.T) {
	for _, origin := range append([]string{"unheard-of-field"}, RelatedPriorityOrigins...) {
		for _, derived := range []bool{false, true} {
			reason := NewRelatedReason(ContextRelation{
				Relation: "fixture_relation",
				Entity:   ContextRef{EntityType: SearchNode, ID: "alpha", Label: "Alpha"},
				Origin:   origin,
				Derived:  derived,
			})
			if reason.Origin != origin || reason.Relation != "fixture_relation" || reason.Derived != derived {
				t.Errorf("reason %+v did not carry the relation it was built from", reason)
			}
			if want := RelatedPriorityFor(origin); reason.Priority != want {
				t.Errorf("origin %q classified as %s, want %s", origin, reason.Priority, want)
			}
			if want := RelatedPriorityRank(reason.Priority); reason.PriorityRank != want {
				t.Errorf("origin %q ranked %d, want %d", origin, reason.PriorityRank, want)
			}
		}
	}
}

// TestRelatedReasonCarriesNoScore is the no-fabrication assertion at the type level.
//
// The reason struct must not acquire a confidence, a similarity, a relevance or any other number
// that reads as a measurement of how related two records are, because nothing in the corpus
// measures that. The field set is asserted exactly, so adding one is a deliberate act with a test
// to change rather than an incremental widening nobody reviews.
//
// Explanation was added by Phase 2B and is the deliberate act this test exists to force. It is a
// string rather than a number and is a lookup over the precedence class rather than a computation
// over the pair, so it states the class in words and measures nothing; the assertion below that
// every rendering is one of the eight fixed strings is what keeps it that way.
func TestRelatedReasonCarriesNoScore(t *testing.T) {
	want := []string{"Relation", "Origin", "Derived", "Priority", "PriorityRank", "Explanation"}
	got := fieldNames(RelatedReason{})
	if len(got) != len(want) {
		t.Fatalf("RelatedReason fields = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
		}
	}
	// No field of the reason is numeric except the precedence ordinal, which is a rank rather
	// than a measurement. A float anywhere in this struct would be a score whatever it was named.
	for i, field := range reflect.VisibleFields(reflect.TypeOf(RelatedReason{})) {
		switch field.Type.Kind() {
		case reflect.Float32, reflect.Float64:
			t.Errorf("field %d %q is a float; a reason measures nothing", i, field.Name)
		case reflect.Int:
			if field.Name != "PriorityRank" {
				t.Errorf("field %d %q is an integer other than the precedence rank", i, field.Name)
			}
		}
	}
}

// fieldNames lists a struct's exported fields in declaration order, so a test can assert a
// contract's whole field set rather than only the fields it remembered to check.
func fieldNames(value any) []string {
	t := reflect.TypeOf(value)
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out = append(out, t.Field(i).Name)
	}
	return out
}

// --------------------------------------------------------------------------------------------
// Phase 2B: the filter allowlist and the explanation table.
//
// Both are properties of the model rather than of a corpus, for the reason the precedence
// assertions above are: a filter vocabulary that only holds for the records that happen to exist
// today is not a vocabulary, and an explanation that varies with anything other than the class it
// explains is not deterministic.
// --------------------------------------------------------------------------------------------

// TestValidRelatedPriorityIsTheModelAndNothingElse holds the filter allowlist to the ranking
// table.
//
// The values a caller may filter by and the classes the ranking is defined over must be one list,
// or a caller could name a scope the ranking has no rank for. Every declared class is accepted,
// and nothing outside the declared set is: the unclassified fallback is refused with the invented
// names, because it is what a canonical field the model does not name degrades into rather than a
// scope anybody can ask for.
func TestValidRelatedPriorityIsTheModelAndNothingElse(t *testing.T) {
	for _, p := range RelatedPriorities {
		if !ValidRelatedPriority(string(p)) {
			t.Errorf("declared class %q is not accepted as a filter value", p)
		}
	}
	for _, name := range RelatedPriorityNames() {
		if !ValidRelatedPriority(name) {
			t.Errorf("rendered name %q is not accepted as a filter value", name)
		}
	}
	for _, bad := range []string{
		string(PriorityUnclassified), "", " ", "conceptual ", " conceptual",
		"Conceptual", "CONCEPTUAL", "conceptuals", "concept", "node.relationships",
		"produces", "explicit_related_node", "shared_claim",
	} {
		if ValidRelatedPriority(bad) {
			t.Errorf("%q was accepted as a filter value", bad)
		}
	}
}

// TestRelatedPriorityNamesExcludeTheUnclassifiedFallback pins the one deliberate omission from the
// rendered set, which Phase 2B turned from a documentation detail into a contract: the rendered
// names are the allowlist an error message lists, so a value appearing there is a value a caller
// is being told to send.
func TestRelatedPriorityNamesExcludeTheUnclassifiedFallback(t *testing.T) {
	for _, name := range RelatedPriorityNames() {
		if name == string(PriorityUnclassified) {
			t.Error("the rendered class list offers unclassified as a filter value")
		}
	}
}

// TestEveryPriorityExplanationIsPresentAndDistinct asserts the explanation table covers the model.
//
// A class rendering an empty string would produce an item that ranks but does not explain itself,
// and two classes sharing a sentence would tell a reader that two different canonical fields said
// the same thing. Walking the closed set turns either into a build-time failure rather than a
// response nobody inspects.
func TestEveryPriorityExplanationIsPresentAndDistinct(t *testing.T) {
	seen := make(map[string]RelatedPriority, len(RelatedPriorities))
	for _, p := range RelatedPriorities {
		text := RelatedPriorityExplanation(p)
		if text == "" {
			t.Errorf("class %q has no explanation", p)
			continue
		}
		if other, clash := seen[text]; clash {
			t.Errorf("classes %q and %q share one explanation", other, p)
		}
		seen[text] = p
	}
	// An unrecognised class explains itself as unrecognised rather than as nothing, which is the
	// same degradation RelatedPriorityFor already chooses.
	if RelatedPriorityExplanation(PriorityUnclassified) == "" {
		t.Error("the unclassified fallback has no explanation")
	}
	if RelatedPriorityExplanation(RelatedPriority("invented")) != RelatedPriorityExplanation(PriorityUnclassified) {
		t.Error("an invented class explained itself as something other than unclassified")
	}
}

// TestPriorityExplanationsAreStableAndSelfContained is the determinism and no-fabrication
// assertion on the explanation text.
//
// Repeated calls must return identical bytes, because the sentence is a lookup rather than a
// generated string. The sentence must also name no record: an explanation that mentioned an
// endpoint would have had to be built from one, and a template with a record's title interpolated
// into it is exactly the kind of prose this layer cannot check. And none of them may claim the
// destination is relevant, similar or important, because nothing in the corpus says so.
func TestPriorityExplanationsAreStableAndSelfContained(t *testing.T) {
	forbidden := []string{
		"similar", "relevant", "relevance", "confidence", "score", "likely", "probably",
		"important", "recommend", "suggest", "because it", "%s", "{",
	}
	for _, p := range append(append([]RelatedPriority(nil), RelatedPriorities...), PriorityUnclassified) {
		first := RelatedPriorityExplanation(p)
		for run := 0; run < 10; run++ {
			if again := RelatedPriorityExplanation(p); again != first {
				t.Fatalf("class %q rendered %q then %q", p, first, again)
			}
		}
		lower := strings.ToLower(first)
		for _, word := range forbidden {
			if strings.Contains(lower, word) {
				t.Errorf("class %q explains itself with %q: %q", p, word, first)
			}
		}
		// The fixture identifiers of every test corpus, and the canonical class names, are the
		// things a template would have interpolated. None may appear in a sentence that is
		// supposed to be about a canonical field rather than about a pair of records.
		for _, leak := range []string{"alpha", "beta", "gamma", "fixture", "/", "\\"} {
			if strings.Contains(lower, leak) {
				t.Errorf("class %q leaks %q into its explanation: %q", p, leak, first)
			}
		}
		if !strings.HasSuffix(first, ".") {
			t.Errorf("class %q explains itself with a fragment: %q", p, first)
		}
	}
}

// TestNewRelatedReasonExplainsItselfFromItsOwnPrecedence is the consistency assertion Phase 2B
// adds to the one above it.
//
// A reason's explanation and the class it reports must never disagree, because a client reading
// one and trusting the other would be reading two accounts of one connection. Constructing every
// reason through one function is what guarantees it; this checks the guarantee over the whole
// closed set of canonical fields rather than over the ones a fixture happens to exercise.
func TestNewRelatedReasonExplainsItselfFromItsOwnPrecedence(t *testing.T) {
	for _, origin := range append([]string{"unheard-of-field"}, RelatedPriorityOrigins...) {
		reason := NewRelatedReason(ContextRelation{
			Relation: "fixture_relation",
			Entity:   ContextRef{EntityType: SearchNode, ID: "alpha", Label: "Alpha"},
			Origin:   origin,
		})
		if want := RelatedPriorityExplanation(reason.Priority); reason.Explanation != want {
			t.Errorf("origin %q explained as %q, want %q", origin, reason.Explanation, want)
		}
		if reason.Explanation == "" {
			t.Errorf("origin %q produced a reason that does not explain itself", origin)
		}
	}
}
