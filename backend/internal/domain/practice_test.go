package domain

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCanonicalNumberAdversarialGrammar(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"0", true}, {"-0", true}, {"1e-2", true}, {"1E+03", true},
		{".5", false}, {"1.", false}, {"+1", false}, {"01", false}, {"-01", false},
		{"1e", false}, {"1e+", false}, {"0x10", false}, {"0o10", false},
		{".inf", false}, {"-.inf", false}, {".nan", false}, {"1_000", false},
		{"1:20", false}, {"1e999", false}, {"1] , \"injected\": true", false},
	}

	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			var number CanonicalNumber
			err := yaml.Unmarshal([]byte(tc.raw), &number)
			if (err == nil) != tc.want {
				t.Fatalf("yaml.Unmarshal(%q) error = %v, accepted = %v", tc.raw, err, err == nil)
			}
			if err != nil {
				return
			}
			encoded, err := json.Marshal(number)
			if err != nil {
				t.Fatalf("json.Marshal(%q): %v", tc.raw, err)
			}
			if string(encoded) != tc.raw || !json.Valid(encoded) {
				t.Fatalf("encoded = %q, want the authored valid JSON token %q", encoded, tc.raw)
			}
		})
	}
}
