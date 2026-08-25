package domain

import (
	"reflect"
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
func TestRelatedReasonCarriesNoScore(t *testing.T) {
	want := []string{"Relation", "Origin", "Derived", "Priority", "PriorityRank"}
	got := fieldNames(RelatedReason{})
	if len(got) != len(want) {
		t.Fatalf("RelatedReason fields = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, got[i], want[i])
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
