package domain

import (
	"reflect"
	"testing"
)

// The ranking weights are a policy, and a policy is only deterministic if its arithmetic is.
// These tests hold the weight table to the one property the whole design rests on — that a
// summed score behaves as a precedence order — so a later weight edit fails here rather than
// silently reordering search results nobody was looking at.

// TestSearchSignalWeightsAreLexicographic is the load-bearing invariant: every signal outweighs
// the sum of every weaker signal, so no accumulation of weak evidence overtakes one strong piece.
func TestSearchSignalWeightsAreLexicographic(t *testing.T) {
	if !SearchSignalWeightsAreLexicographic() {
		t.Fatal("the signal weights are no longer lexicographic; a summed score no longer orders " +
			"results the way SearchMatchSignals reads")
	}

	// Spelled out independently of the helper, so a bug in the helper cannot make the invariant
	// report itself as held.
	below := 0
	for i := len(SearchMatchSignals) - 1; i >= 0; i-- {
		signal := SearchMatchSignals[i]
		weight := SearchSignalWeight(signal)
		if weight <= 0 {
			t.Errorf("signal %q has weight %d; every signal must carry positive weight", signal, weight)
		}
		if weight <= below {
			t.Errorf("signal %q weighs %d but the signals below it sum to %d; a record carrying "+
				"every weaker signal would outrank one carrying this", signal, weight, below)
		}
		below += weight
	}
}

// TestSearchSignalWeightsDescendInDeclaredOrder pins SearchMatchSignals as the ranking policy
// written down: the declared order is the weight order, so a result's signal list always leads
// with the strongest thing true of it.
func TestSearchSignalWeightsDescendInDeclaredOrder(t *testing.T) {
	previous := 0
	seen := map[string]bool{}
	for i, signal := range SearchMatchSignals {
		if seen[signal] {
			t.Fatalf("signal %q is declared twice", signal)
		}
		seen[signal] = true

		weight := SearchSignalWeight(signal)
		if i > 0 && weight >= previous {
			t.Errorf("signal %q weighs %d, not less than the %d before it; the declared order is "+
				"no longer the weight order", signal, weight, previous)
		}
		previous = weight
	}
}

// TestSearchRelevanceScoreIsTheSumOfItsSignals asserts the one arithmetic guarantee the response
// makes to a client: match_signals and relevance_score are the same fact twice, so a client can
// recompute the score and check it.
func TestSearchRelevanceScoreIsTheSumOfSignals(t *testing.T) {
	cases := []struct {
		name    string
		signals []string
		want    int
	}{
		{"no signals", nil, 0},
		{"empty list", []string{}, 0},
		{"one signal", []string{SignalFieldMatch}, SearchSignalWeight(SignalFieldMatch)},
		{
			"a literal name hit",
			[]string{SignalTitlePrefix, SignalIDSubstring, SignalFieldMatch},
			SearchSignalWeight(SignalTitlePrefix) + SearchSignalWeight(SignalIDSubstring) +
				SearchSignalWeight(SignalFieldMatch),
		},
		{
			"a composed name hit",
			[]string{SignalTitleExact, SignalTitleAllTerms, SignalIDSubstring, SignalFieldMatch},
			SearchSignalWeight(SignalTitleExact) + SearchSignalWeight(SignalTitleAllTerms) +
				SearchSignalWeight(SignalIDSubstring) + SearchSignalWeight(SignalFieldMatch),
		},
		{"every signal at once", SearchMatchSignals, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if tc.name == "every signal at once" {
				for _, signal := range SearchMatchSignals {
					want += SearchSignalWeight(signal)
				}
			}
			if got := SearchRelevanceScore(tc.signals); got != want {
				t.Errorf("SearchRelevanceScore(%v) = %d, want %d", tc.signals, got, want)
			}
		})
	}
}

// TestSearchSignalWeightRejectsUnknownNames asserts a score cannot be inflated by a name this
// build does not define. An unknown signal contributes nothing rather than a default.
func TestSearchSignalWeightRejectsUnknownNames(t *testing.T) {
	for _, unknown := range []string{"", "popularity", "click_rate", "ID_EXACT", "title", "  "} {
		if got := SearchSignalWeight(unknown); got != 0 {
			t.Errorf("SearchSignalWeight(%q) = %d, want 0", unknown, got)
		}
	}
	if got := SearchRelevanceScore([]string{"popularity", SignalFieldMatch}); got != SearchSignalWeight(SignalFieldMatch) {
		t.Errorf("an unknown signal contributed weight: %d", got)
	}
}

// TestSearchMatchSignalsIsTheClosedSet asserts every declared constant appears in the ordered set
// and nothing else does, so a signal cannot be emitted that the ordering policy never ranked.
func TestSearchMatchSignalsIsTheClosedSet(t *testing.T) {
	want := []string{
		SignalIDExact, SignalTitleExact, SignalTitlePrefix, SignalTitleSubstring,
		SignalPhraseMatch, SignalTitleAllTerms, SignalTitleTerms, SignalIDSubstring,
		SignalFieldMatch,
	}
	if !reflect.DeepEqual(SearchMatchSignals, want) {
		t.Errorf("SearchMatchSignals = %v, want %v", SearchMatchSignals, want)
	}
}
