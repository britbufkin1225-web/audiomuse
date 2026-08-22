// Package testsupport loads the synthetic fixture corpus used by the backend tests.
//
// Tests run the real parsing, validation and indexing code against this fixture rather than
// against the live AudioMuse repository, so a canonical content change cannot silently
// alter a unit-test expectation. One separate test exercises the live repository on purpose.
package testsupport

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// CorpusName is the label the fixture adapter reports.
const CorpusName = "fixture-corpus"

// CorpusRoot returns the absolute path of the fixture corpus, locating backend/testdata
// from whichever package directory the test is running in.
func CorpusRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "testdata", "corpus")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("fixture corpus not found from %s", dir)
		}
		dir = parent
	}
}

// CorpusFS returns the fixture corpus as a read-only filesystem.
func CorpusFS(t testing.TB) fs.FS {
	t.Helper()
	return os.DirFS(CorpusRoot(t))
}

// MutableCorpus copies the fixture into an in-memory filesystem so a test can introduce
// exactly one defect. Copying keeps the on-disk fixture valid and inspectable: a corpus
// full of deliberately broken files would be hard to read and easy to mistake for a bug.
func MutableCorpus(t testing.TB) fstest.MapFS {
	t.Helper()
	root := CorpusRoot(t)
	out := fstest.MapFS{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatalf("copy fixture corpus: %v", err)
	}
	return out
}

// Write replaces or adds one file in a copied corpus.
func Write(corpus fstest.MapFS, path, content string) {
	corpus[path] = &fstest.MapFile{Data: []byte(content)}
}

// ValidNode renders a schema-complete fixture node with the supplied front-matter body,
// so a defect test only has to state the one field it is changing.
func ValidNode(id, title, domainName, status, relationships, sources string) string {
	return "---\n" +
		"id: " + id + "\n" +
		"title: " + title + "\n" +
		"domain: " + domainName + "\n" +
		"status: " + status + "\n" +
		"session_origin: []\n" +
		"definition: Synthetic definition for " + id + ".\n" +
		"core_concepts: []\n" +
		"relationships: " + relationships + "\n" +
		"sources: " + sources + "\n" +
		"experiments: []\n" +
		"practical_applications: []\n" +
		"project_connections: []\n" +
		"future_questions: []\n" +
		"---\n\n# " + title + "\n\nBody.\n"
}

// ValidClaim renders a schema-complete fixture claim record with the supplied field values,
// so a defect test only has to state the one thing it is changing. Collection arguments are
// YAML flow sequences, matching the form claims/README.md describes for canonical records.
func ValidClaim(id, claimType, confidence, disputeStatus, evidence, attribution, derivedFrom, appearsIn string) string {
	return "---\n" +
		"id: \"" + id + "\"\n" +
		"statement: \"Synthetic fixture statement for " + id + ".\"\n" +
		"claim_type: \"" + claimType + "\"\n" +
		"confidence: \"" + confidence + "\"\n" +
		"confidence_basis: \"Synthetic fixture basis for " + id + ".\"\n" +
		"dispute_status: \"" + disputeStatus + "\"\n" +
		"temporal_precision: \"not_temporal\"\n" +
		"evidence: " + evidence + "\n" +
		"attribution: " + attribution + "\n" +
		"derived_from: " + derivedFrom + "\n" +
		"appears_in: " + appearsIn + "\n" +
		"open_questions: []\n"
}

// SupportedBy renders one evidence entry supporting a claim, for tests that only care that
// a source is cited at all.
func SupportedBy(sourceID string) string {
	return `[{"relation": "supported_by", "source_id": "` + sourceID + `", "note": "Fixture support note."}]`
}

// AppearsInNode renders a single node appearance site.
func AppearsInNode(nodeID string) string {
	return `[{"kind": "node", "ref": "` + nodeID + `"}]`
}

// quoted renders one canonical scalar field line in the JSON-compatible YAML form the practice
// contracts use.
func quoted(key, value string) string {
	return key + `: "` + value + "\"\n"
}

// raw renders one field line whose value is already a YAML flow value: a list, a null, or a
// quoted scalar the caller composed itself.
func raw(key, value string) string {
	return key + ": " + value + "\n"
}

// ValidVocabularyEntry renders a schema-complete fixture vocabulary entry, so a defect test only
// has to state the one field it is changing. Collection arguments are YAML flow sequences,
// matching the form vocabulary/README.md describes for canonical records.
func ValidVocabularyEntry(id, term, domainName, nodeRefs, sessionRefs, relatedTerms, tags string) string {
	return "---\n" +
		quoted("id", id) +
		quoted("term", term) +
		quoted("domain", domainName) +
		quoted("definition", "Synthetic fixture definition for "+id+".") +
		quoted("digital_relationship", "Synthetic fixture digital relationship for "+id+".") +
		quoted("best_use", "Synthetic fixture use for "+id+".") +
		raw("technologies", "[]") +
		raw("node_refs", nodeRefs) +
		raw("session_refs", sessionRefs) +
		raw("related_terms", relatedTerms) +
		raw("tags", tags)
}

// ValidExperiment renders a schema-complete fixture experiment definition.
//
// Experiment records are one per file rather than a stream, matching experiments/records/.
func ValidExperiment(id, status, expType, difficulty, nodeRefs, vocabularyRefs, sessionRefs, sourceRefs, relatedExperiments string) string {
	out := quoted("id", id) +
		quoted("title", "Fixture "+id) +
		quoted("status", status) +
		quoted("type", expType) +
		quoted("difficulty", difficulty) +
		quoted("purpose", "Synthetic fixture purpose for "+id+".") +
		raw("node_refs", nodeRefs) +
		raw("vocabulary_refs", vocabularyRefs) +
		raw("session_refs", sessionRefs) +
		raw("source_refs", sourceRefs)
	for _, field := range []string{
		"required_equipment", "optional_equipment", "safety", "setup", "procedure",
		"observations", "measurements", "expected_behavior", "interpretation",
		"limitations", "repeatability",
	} {
		out += raw(field, "[]")
	}
	return out + raw("related_experiments", relatedExperiments) + raw("project_connections", "[]")
}

// ValidExperimentRun renders a schema-complete fixture experiment-run record.
//
// runDate is written verbatim, so a test passes either null or a quoted ISO date and can
// therefore construct the planned/performed conflicts the lifecycle rules exist to reject.
func ValidExperimentRun(id, experimentID, runDate, status, observations, measurements, interpretation, sourceRefs string) string {
	return quoted("id", id) +
		quoted("experiment_id", experimentID) +
		raw("run_date", runDate) +
		quoted("status", status) +
		raw("environment_notes", "[]") +
		raw("equipment", "[]") +
		raw("software", "[]") +
		raw("procedure_deviations", "[]") +
		raw("control_settings", "[]") +
		raw("observations", observations) +
		raw("measurements", measurements) +
		raw("limitations", "[]") +
		raw("safety_notes", "[]") +
		raw("interpretation", interpretation) +
		raw("follow_up_questions", "[]") +
		raw("source_refs", sourceRefs)
}

// FixtureObservation renders one run observation, for tests that only care that qualitative
// evidence is present at all.
func FixtureObservation(statement string) string {
	return `[{"statement": "` + statement + `", "context": "Fixture observation context."}]`
}

// FixtureMeasurement renders one fully specified run measurement, for tests that need
// quantitative evidence rather than an observation.
func FixtureMeasurement(quantity, value, unit, calibration, limitations string) string {
	return `[{"quantity": "` + quantity + `", "value": ` + value + `, "unit": "` + unit +
		`", "method": "Fixture method.", "tool": "Fixture tool.", "calibration": "` + calibration +
		`", "uncertainty": null, "limitations": ` + limitations + `}]`
}
