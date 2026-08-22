package filesystem

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// vocabularyEntriesDir is the canonical vocabulary root. Each file in it is a YAML stream of
// documents, one per entry, as vocabulary/README.md describes. The domain-named files under
// it are grouping only: an entry's domain comes from its own record, never from its filename.
const vocabularyEntriesDir = "vocabulary/entries"

// vocabularyRequiredFields is the required list from schemas/vocabulary.schema.yaml version 1.
// That contract also sets additional_properties: false and marks every property required, so
// this is simultaneously the required set and the allowed set: a record's key set must equal
// it exactly.
var vocabularyRequiredFields = []string{
	"id", "term", "domain", "definition", "digital_relationship", "best_use",
	"technologies", "node_refs", "session_refs", "related_terms", "tags",
}

// loadVocabulary reads every canonical vocabulary entry.
//
// The directory is walked in lexical order and results are sorted by ID afterwards, so
// neither traversal order nor map iteration can reach the projection. A corpus with no
// vocabulary/entries/ directory is not an error: the practice layer is additive, exactly as
// the claim layer is, and a corpus may legitimately predate it.
func (r *Repository) loadVocabulary(report *domain.ValidationReport) []domain.VocabularyEntry {
	if _, err := fs.Stat(r.fsys, vocabularyEntriesDir); err != nil {
		return []domain.VocabularyEntry{}
	}

	entries := make([]domain.VocabularyEntry, 0)
	seenID := make(map[string]string)
	// Term uniqueness is case-insensitive in tools/validate-vocabulary.ps1 while every
	// identifier comparison in this repository is case-sensitive. Both rules are kept as the
	// canonical validator states them: an ID is an identity, a term is a human label, and two
	// labels differing only in case would be indistinguishable in a rendered list.
	seenTerm := make(map[string]string)

	err := fs.WalkDir(r.fsys, vocabularyEntriesDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(p, ".yaml") {
			return nil
		}
		raw, readErr := fs.ReadFile(r.fsys, p)
		if readErr != nil {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path: p, Message: readErr.Error(),
			})
			return nil
		}
		for _, entry := range parseVocabularyStream(raw, p, report) {
			if prior, dup := seenID[entry.ID]; dup {
				report.Add(domain.ValidationIssue{
					Severity: domain.SeverityFatal, Code: domain.CodeDuplicateID,
					Ref: entry.ID, Path: p,
					Message: fmt.Sprintf("vocabulary id is already defined by %s", prior),
				})
				continue
			}
			folded := strings.ToLower(entry.Term)
			if prior, dup := seenTerm[folded]; dup {
				report.Add(domain.ValidationIssue{
					Severity: domain.SeverityFatal, Code: domain.CodeDuplicateTerm,
					Ref: entry.ID, Path: p,
					Message: fmt.Sprintf("vocabulary term %q is already defined by %s", entry.Term, prior),
				})
				continue
			}
			seenID[entry.ID] = p
			seenTerm[folded] = entry.ID
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: vocabularyEntriesDir, Message: err.Error(),
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries
}

// parseVocabularyStream decodes every YAML document in one vocabulary file.
//
// A file is a stream rather than a list because vocabulary/README.md and
// tools/validate-vocabulary.ps1 both define it that way. A document that fails to parse is
// reported and the stream is abandoned, matching parseClaimStream: once a decoder has lost
// its position, every subsequent document boundary is a guess.
func parseVocabularyStream(raw []byte, relPath string, report *domain.ValidationReport) []domain.VocabularyEntry {
	out := make([]domain.VocabularyEntry, 0)
	forEachDocument(raw, relPath, "vocabulary entry", report, func(doc *yaml.Node, index int) {
		if entry, ok := parseVocabularyEntry(doc, relPath, index, report); ok {
			out = append(out, entry)
		}
	})
	return out
}

// parseVocabularyEntry turns one YAML document into a domain.VocabularyEntry.
//
// Key-set checking happens before struct decoding so a missing or unknown field is reported as
// such rather than surfacing as a zero value that later looks like an authoring choice.
func parseVocabularyEntry(doc *yaml.Node, relPath string, index int, report *domain.ValidationReport) (domain.VocabularyEntry, bool) {
	position := fmt.Sprintf("vocabulary entry %d", index)
	fatal := func(code, msg, ref string) (domain.VocabularyEntry, bool) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: code, Ref: ref, Path: relPath,
			Message: position + ": " + msg,
		})
		return domain.VocabularyEntry{}, false
	}

	mapping, shapeErr := checkedMapping(doc, vocabularyRequiredFields)
	if shapeErr != nil {
		return fatal(shapeErr.code, shapeErr.message, "")
	}

	var entry domain.VocabularyEntry
	if err := mapping.Decode(&entry); err != nil {
		return fatal(domain.CodeMalformedRecord,
			"record does not match the vocabulary contract: "+err.Error(), "")
	}

	if !canonicalIDPattern.MatchString(entry.ID) {
		return fatal(domain.CodeInvalidID,
			fmt.Sprintf("vocabulary id %q is not canonical kebab-case", entry.ID), entry.ID)
	}
	// The key-set check proves each field was written; an empty value means the entry defines
	// nothing while still occupying an ID and a term, which is worse than an absent record.
	required := []struct{ name, value string }{
		{"term", entry.Term},
		{"domain", entry.Domain},
		{"definition", entry.Definition},
		{"digital_relationship", entry.DigitalRelationship},
		{"best_use", entry.BestUse},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fatal(domain.CodeMissingField, "required field "+field.name+" is empty", entry.ID)
		}
	}

	entry.Path = relPath
	normaliseVocabularySlices(&entry)
	checkVocabularyLists(entry, report)
	return entry, true
}

// normaliseVocabularySlices replaces nil slices with empty ones so the JSON projection renders
// [] rather than null for a canonically empty list. Representation only; no canonical value is
// altered.
func normaliseVocabularySlices(e *domain.VocabularyEntry) {
	ensureStrings(&e.Technologies)
	ensureStrings(&e.NodeRefs)
	ensureStrings(&e.SessionRefs)
	ensureStrings(&e.RelatedTerms)
	ensureStrings(&e.Tags)
}

// checkVocabularyLists enforces the per-list rules the canonical validator applies: no blank
// value, and no value repeated within one list.
//
// Duplicates are fatal rather than deduplicated for the reason claim evidence duplicates are:
// a reverse index built from a record that names the same reference twice would double-count
// it, and silently collapsing the pair would hide an authoring defect the canonical validator
// also rejects.
func checkVocabularyLists(entry domain.VocabularyEntry, report *domain.ValidationReport) {
	lists := []struct {
		name   string
		values []string
	}{
		{"technologies", entry.Technologies},
		{"node_refs", entry.NodeRefs},
		{"session_refs", entry.SessionRefs},
		{"related_terms", entry.RelatedTerms},
		{"tags", entry.Tags},
	}
	for _, list := range lists {
		checkDistinctNonEmpty(list.name, list.values, entry.ID, entry.Path, report)
	}
	for _, ref := range entry.RelatedTerms {
		if ref == entry.ID {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeSelfReference, Ref: entry.ID,
				Path:    entry.Path,
				Message: "vocabulary entry lists itself as a related term",
			})
		}
	}
}

// resolveVocabularyReferences checks every cross-record reference a vocabulary entry declares.
//
// node_refs resolve against canonical nodes and session_refs against sources registered as
// type: session, matching the two rules vocabulary/README.md states. related_terms resolve
// against other vocabulary entries and are checked here only to prove they point at something
// real — resolving a navigation cross-reference does not turn it into a graph edge, and
// nothing in this function or its callers adds a vertex or an edge to domain.Graph.
func resolveVocabularyReferences(
	entries []domain.VocabularyEntry,
	nodes []domain.Node,
	sessions []domain.Session,
	report *domain.ValidationReport,
) {
	vocabularyIDs := idSet(entries, func(e domain.VocabularyEntry) string { return e.ID })
	nodeIDs := idSet(nodes, func(n domain.Node) string { return n.ID })
	sessionIDs := idSet(sessions, func(s domain.Session) string { return s.ID })

	for _, entry := range entries {
		fatal := func(code, msg string) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: code, Ref: entry.ID, Path: entry.Path, Message: msg,
			})
		}
		for _, ref := range entry.NodeRefs {
			if !nodeIDs[ref] {
				fatal(domain.CodeUnresolvedTarget,
					fmt.Sprintf("node_refs %q does not resolve to a canonical node", ref))
			}
		}
		for _, ref := range entry.SessionRefs {
			if !sessionIDs[ref] {
				fatal(domain.CodeUnresolvedSession,
					fmt.Sprintf("session_refs %q is not a source registered as type: session", ref))
			}
		}
		for _, ref := range entry.RelatedTerms {
			// A self-reference is already reported by checkVocabularyLists, and it always
			// resolves, so it does not reach this branch twice.
			if !vocabularyIDs[ref] {
				fatal(domain.CodeUnresolvedVocabulary,
					fmt.Sprintf("related_terms %q does not resolve to a canonical vocabulary entry", ref))
			}
		}
	}
}
