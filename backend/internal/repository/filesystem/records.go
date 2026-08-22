package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// This file holds the record-shape helpers shared by every canonical layer the backend parses.
//
// They exist because the layers share one authoring contract: a record is a YAML mapping whose
// key set must equal the schema's, nested objects declare additional_properties: false, and
// list values must be distinct and non-empty. Expressing that once means a new layer cannot
// quietly acquire weaker checking than an old one.

// recordError is a shape failure carrying the validation code it should be reported under, so
// a caller can attach the record's ID and path without losing the reason.
type recordError struct {
	code    string
	message string
}

func (e *recordError) Error() string { return e.message }

func malformed(format string, args ...any) *recordError {
	return &recordError{code: domain.CodeMalformedRecord, message: fmt.Sprintf(format, args...)}
}

// forEachDocument decodes every YAML document in one canonical record stream.
//
// A document that fails to parse is reported and the rest of the stream is abandoned: once the
// decoder has lost its position, every subsequent document boundary is a guess, and reporting
// invented records would be worse than reporting the one real defect. label names the record
// kind so the message says what failed to parse.
func forEachDocument(
	raw []byte,
	relPath, label string,
	report *domain.ValidationReport,
	visit func(doc *yaml.Node, index int),
) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	index := 0
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path:    relPath,
				Message: fmt.Sprintf("%s %d is not valid YAML: %s", label, index+1, err.Error()),
			})
			return
		}
		index++
		// A stream may end with a trailing separator, which yields an empty document.
		if doc.Kind == yaml.DocumentNode && len(doc.Content) == 0 {
			continue
		}
		visit(&doc, index)
	}
}

// checkedMapping unwraps a record document and proves its key set equals the contract's.
//
// The contracts this backend reads all mark every property required and set
// additional_properties: false, so required and allowed are the same list. Checking before
// struct decoding means a missing or unknown field is reported as such rather than surfacing
// as a zero value that later reads as an authoring choice.
func checkedMapping(doc *yaml.Node, fields []string) (*yaml.Node, *recordError) {
	mapping, err := documentMapping(doc)
	if err != nil {
		return nil, malformed("%s", err.Error())
	}
	keys, err := mappingKeys(mapping)
	if err != nil {
		return nil, malformed("%s", err.Error())
	}
	present := make(map[string]bool, len(keys))
	for _, k := range keys {
		present[k] = true
	}
	allowed := make(map[string]bool, len(fields))
	for _, f := range fields {
		allowed[f] = true
		if !present[f] {
			return nil, &recordError{code: domain.CodeMissingField, message: "missing required field " + f}
		}
	}
	for _, k := range keys {
		if !allowed[k] {
			return nil, &recordError{code: domain.CodeUnknownField, message: "unknown top-level field " + k}
		}
	}
	return mapping, nil
}

// checkObjectListShape enforces one array-of-objects field's item contract: every item must be
// a mapping whose key set is exactly the declared one.
//
// Struct decoding tolerates extra keys silently, so the shape is checked against the YAML tree
// rather than after decoding. want must be sorted; the callers declare it in contract order and
// the comparison sorts a copy of the record's keys to match.
func checkObjectListShape(mapping *yaml.Node, field string, want []string) error {
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedWant)
	expected := strings.Join(want, ", ")

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != field {
			continue
		}
		seq := mapping.Content[i+1]
		if seq.Kind == yaml.ScalarNode && seq.Tag == "!!null" {
			return nil
		}
		if seq.Kind != yaml.SequenceNode {
			return fmt.Errorf("%s must be a list", field)
		}
		for _, item := range seq.Content {
			if item.Kind != yaml.MappingNode {
				return fmt.Errorf("each %s entry must be a mapping with %s", field, expected)
			}
			keys, err := mappingKeys(item)
			if err != nil {
				return fmt.Errorf("%s entry: %w", field, err)
			}
			sorted := append([]string(nil), keys...)
			sort.Strings(sorted)
			if len(sorted) != len(sortedWant) {
				return fmt.Errorf("%s entry must declare exactly %s", field, expected)
			}
			for i, key := range sorted {
				if key != sortedWant[i] {
					return fmt.Errorf("%s entry must declare exactly %s", field, expected)
				}
			}
		}
		return nil
	}
	return nil
}

// ensureStrings replaces a nil slice with an empty one so the JSON projection renders [] rather
// than null. Representation only; no canonical value is altered.
func ensureStrings(values *[]string) {
	if *values == nil {
		*values = []string{}
	}
}

// checkDistinctNonEmpty enforces the list rule every canonical practice contract applies: no
// blank value, and no value repeated within one list.
//
// Comparison is exact and case-sensitive, matching the canonical identity semantics in
// docs/knowledge-model.md and the ordinal comparers the PowerShell validators use. Case drift
// in a reference is an unresolved reference, not a duplicate.
func checkDistinctNonEmpty(field string, values []string, ref, path string, report *domain.ValidationReport) {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMissingField, Ref: ref, Path: path,
				Message: field + " contains an empty value",
			})
			continue
		}
		if seen[value] {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeDuplicateReference, Ref: ref, Path: path,
				Message: fmt.Sprintf("duplicate %s value %q", field, value),
			})
		}
		seen[value] = true
	}
}

// idSet builds an exact, case-sensitive membership set of canonical identifiers.
func idSet[T any](items []T, key func(T) string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[key(item)] = true
	}
	return set
}
