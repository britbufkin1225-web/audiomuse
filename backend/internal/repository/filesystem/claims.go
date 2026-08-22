package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/britbufkin1225-web/audiomuse/backend/internal/domain"
)

// claimRecordsDir is the canonical claim record root. Each file in it is a YAML stream of
// documents, one per claim, as claims/README.md describes.
const claimRecordsDir = "claims/records"

// generatedDirs are repository roots whose contents are derived projections.
//
// schemas/claim.schema.yaml rejects them as appearance sites: a generated view cannot be
// evidence that a claim exists, because it was itself produced from the claim.
var generatedDirs = []string{"indexes/"}

// claimRequiredFields is the required list from schemas/claim.schema.yaml version 1. That
// contract also sets additional_properties: false and marks every property required, so
// this is simultaneously the required set and the allowed set: a record's key set must
// equal it exactly.
var claimRequiredFields = []string{
	"id", "statement", "claim_type", "confidence", "confidence_basis",
	"dispute_status", "temporal_precision", "evidence", "attribution",
	"derived_from", "appears_in", "open_questions",
}

// Item key sets from schemas/claim.schema.yaml. Each nested object also declares
// additional_properties: false, and struct decoding tolerates extra keys silently, so the
// shapes are checked against the YAML tree before decoding.
var (
	evidenceItemFields    = []string{"note", "relation", "source_id"}
	attributionItemFields = []string{"actor", "source_id"}
	referenceItemFields   = []string{"kind", "ref"}
)

// loadClaims reads every canonical claim record.
//
// The directory is walked in lexical order and results are sorted by ID afterwards, so
// neither traversal order nor map iteration can reach the projection. A corpus with no
// claims/records/ directory is not an error: the evidence layer is additive and a corpus
// may legitimately predate it.
func (r *Repository) loadClaims(vocab domain.ClaimVocabulary, report *domain.ValidationReport) []domain.Claim {
	if _, err := fs.Stat(r.fsys, claimRecordsDir); err != nil {
		return []domain.Claim{}
	}

	var claims []domain.Claim
	seen := make(map[string]string)

	err := fs.WalkDir(r.fsys, claimRecordsDir, func(p string, d fs.DirEntry, walkErr error) error {
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
		for _, claim := range parseClaimStream(raw, p, vocab, report) {
			if prior, dup := seen[claim.ID]; dup {
				report.Add(domain.ValidationIssue{
					Severity: domain.SeverityFatal, Code: domain.CodeDuplicateID,
					Ref: claim.ID, Path: p,
					Message: fmt.Sprintf("claim id is already defined by %s", prior),
				})
				continue
			}
			seen[claim.ID] = p
			claims = append(claims, claim)
		}
		return nil
	})
	if err != nil {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
			Path: claimRecordsDir, Message: err.Error(),
		})
	}

	sort.Slice(claims, func(i, j int) bool { return claims[i].ID < claims[j].ID })
	if claims == nil {
		claims = []domain.Claim{}
	}
	return claims
}

// parseClaimStream decodes every YAML document in one claim record file.
//
// A file is a stream rather than a list because claims/README.md defines it that way. A
// document that fails to parse is reported and skipped; the rest of the stream is still
// read, so one broken record does not hide every record after it.
func parseClaimStream(raw []byte, relPath string, vocab domain.ClaimVocabulary, report *domain.ValidationReport) []domain.Claim {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	out := make([]domain.Claim, 0)
	index := 0
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: domain.CodeMalformedRecord,
				Path:    relPath,
				Message: fmt.Sprintf("claim record %d is not valid YAML: %s", index+1, err.Error()),
			})
			break
		}
		index++
		// A stream may end with a trailing separator, which yields an empty document.
		if doc.Kind == yaml.DocumentNode && len(doc.Content) == 0 {
			continue
		}
		if claim, ok := parseClaim(&doc, relPath, index, vocab, report); ok {
			out = append(out, claim)
		}
	}
	return out
}

// parseClaim turns one YAML document into a domain.Claim.
//
// Key-set checking happens before struct decoding so a missing or unknown field is reported
// as such rather than surfacing as a zero value that later looks like an authoring choice.
func parseClaim(doc *yaml.Node, relPath string, index int, vocab domain.ClaimVocabulary, report *domain.ValidationReport) (domain.Claim, bool) {
	position := fmt.Sprintf("claim record %d", index)
	fatal := func(code, msg, ref string) (domain.Claim, bool) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: code, Ref: ref, Path: relPath,
			Message: position + ": " + msg,
		})
		return domain.Claim{}, false
	}

	mapping, err := documentMapping(doc)
	if err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}
	keys, err := mappingKeys(mapping)
	if err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}
	present := make(map[string]bool, len(keys))
	for _, k := range keys {
		present[k] = true
	}
	allowed := make(map[string]bool, len(claimRequiredFields))
	for _, f := range claimRequiredFields {
		allowed[f] = true
		if !present[f] {
			return fatal(domain.CodeMissingField, "missing required field "+f, "")
		}
	}
	for _, k := range keys {
		if !allowed[k] {
			return fatal(domain.CodeUnknownField, "unknown top-level field "+k, "")
		}
	}

	if err := checkObjectListShape(mapping, "evidence", evidenceItemFields); err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}
	if err := checkObjectListShape(mapping, "attribution", attributionItemFields); err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}
	if err := checkObjectListShape(mapping, "derived_from", referenceItemFields); err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}
	if err := checkObjectListShape(mapping, "appears_in", referenceItemFields); err != nil {
		return fatal(domain.CodeMalformedRecord, err.Error(), "")
	}

	var claim domain.Claim
	if err := mapping.Decode(&claim); err != nil {
		return fatal(domain.CodeMalformedRecord, "record does not match the claim contract: "+err.Error(), "")
	}

	if !canonicalIDPattern.MatchString(claim.ID) {
		return fatal(domain.CodeInvalidID,
			fmt.Sprintf("claim id %q is not canonical kebab-case", claim.ID), claim.ID)
	}

	// Required string fields must carry a value. The key-set check proves the field was
	// written; an empty value means the record asserts nothing, which for a claim layer is
	// worse than an absent record because it still occupies an ID.
	required := []struct{ name, value string }{
		{"statement", claim.Statement},
		{"claim_type", claim.ClaimType},
		{"confidence", claim.Confidence},
		{"confidence_basis", claim.ConfidenceBasis},
		{"dispute_status", claim.DisputeStatus},
		{"temporal_precision", claim.TemporalPrecision},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fatal(domain.CodeMissingField, "required field "+field.name+" is empty", claim.ID)
		}
	}

	claim.Path = relPath
	normaliseClaimSlices(&claim)
	checkClaimVocabulary(claim, vocab, report)
	checkClaimShape(claim, report)
	return claim, true
}

// checkObjectListShape enforces one array-of-objects field's item contract: every item must
// be a mapping whose key set is exactly the declared one.
func checkObjectListShape(mapping *yaml.Node, field string, want []string) error {
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
		expected := strings.Join(want, ", ")
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
			if len(sorted) != len(want) {
				return fmt.Errorf("%s entry must declare exactly %s", field, expected)
			}
			for i, key := range sorted {
				if key != want[i] {
					return fmt.Errorf("%s entry must declare exactly %s", field, expected)
				}
			}
		}
		return nil
	}
	return nil
}

// normaliseClaimSlices replaces nil slices with empty ones so the JSON projection renders
// [] rather than null for a canonically empty list. Representation only; no canonical value
// is altered.
func normaliseClaimSlices(c *domain.Claim) {
	if c.Evidence == nil {
		c.Evidence = []domain.ClaimEvidence{}
	}
	if c.Attribution == nil {
		c.Attribution = []domain.ClaimAttribution{}
	}
	if c.DerivedFrom == nil {
		c.DerivedFrom = []domain.ClaimReference{}
	}
	if c.AppearsIn == nil {
		c.AppearsIn = []domain.ClaimReference{}
	}
	if c.OpenQuestions == nil {
		c.OpenQuestions = []string{}
	}
}

// checkClaimVocabulary validates every bounded field against the contract vocabularies.
//
// This runs at startup rather than at query time because the API exposes claim_type,
// confidence, dispute_status, temporal_precision and evidence relation as filters. A value
// outside the vocabulary would produce a claim that no valid filter can reach and that no
// invalid filter is allowed to name — invisible in every projection, which is the worst
// possible outcome for an evidence layer.
func checkClaimVocabulary(claim domain.Claim, vocab domain.ClaimVocabulary, report *domain.ValidationReport) {
	add := func(field, value string) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: domain.CodeInvalidVocabulary, Ref: claim.ID,
			Path:    claim.Path,
			Message: fmt.Sprintf("%s %q is not declared in %s", field, value, claimSchemaPath),
		})
	}
	if !newSet(vocab.ClaimTypes)[claim.ClaimType] {
		add("claim_type", claim.ClaimType)
	}
	if !newSet(vocab.ConfidenceLevels)[claim.Confidence] {
		add("confidence", claim.Confidence)
	}
	if !newSet(vocab.DisputeStatuses)[claim.DisputeStatus] {
		add("dispute_status", claim.DisputeStatus)
	}
	if !newSet(vocab.TemporalPrecisions)[claim.TemporalPrecision] {
		add("temporal_precision", claim.TemporalPrecision)
	}
	relations := newSet(vocab.EvidenceRelations)
	for _, e := range claim.Evidence {
		if !relations[e.Relation] {
			add("evidence relation", e.Relation)
		}
	}
	derivedKinds := newSet(vocab.DerivedFromKinds)
	for _, d := range claim.DerivedFrom {
		if !derivedKinds[d.Kind] {
			add("derived_from kind", d.Kind)
		}
	}
	appearsKinds := newSet(vocab.AppearsInKinds)
	for _, a := range claim.AppearsIn {
		if !appearsKinds[a.Kind] {
			add("appears_in kind", a.Kind)
		}
	}
}

// checkClaimShape enforces the structural rules that the reverse indexes depend on.
//
// Duplicates are fatal rather than deduplicated: a reverse index built from a record that
// states the same relationship twice would double-count it, and silently collapsing the
// pair would hide an authoring defect the canonical validator also rejects.
func checkClaimShape(claim domain.Claim, report *domain.ValidationReport) {
	add := func(code, msg string) {
		report.Add(domain.ValidationIssue{
			Severity: domain.SeverityFatal, Code: code, Ref: claim.ID, Path: claim.Path, Message: msg,
		})
	}

	if len(claim.AppearsIn) == 0 {
		add(domain.CodeMissingAppearance,
			"claim declares no appearance site; a claim that appears nowhere in the repository is not a claim")
	}

	seenEvidence := make(map[string]bool, len(claim.Evidence))
	for _, e := range claim.Evidence {
		if strings.TrimSpace(e.SourceID) == "" {
			add(domain.CodeMissingField, "evidence entry has an empty source_id")
			continue
		}
		if strings.TrimSpace(e.Note) == "" {
			add(domain.CodeMissingField,
				fmt.Sprintf("evidence entry for source %q has an empty note", e.SourceID))
		}
		key := e.Relation + "|" + e.SourceID
		if seenEvidence[key] {
			add(domain.CodeDuplicateEvidence,
				fmt.Sprintf("duplicate evidence entry %s %s", e.Relation, e.SourceID))
		}
		seenEvidence[key] = true
	}

	seenAttribution := make(map[string]bool, len(claim.Attribution))
	for _, a := range claim.Attribution {
		if strings.TrimSpace(a.Actor) == "" || strings.TrimSpace(a.SourceID) == "" {
			add(domain.CodeMissingField, "attribution entry has an empty actor or source_id")
			continue
		}
		key := a.Actor + "|" + a.SourceID
		if seenAttribution[key] {
			add(domain.CodeDuplicateEvidence,
				fmt.Sprintf("duplicate attribution entry %s %s", a.Actor, a.SourceID))
		}
		seenAttribution[key] = true
	}

	checkReferenceList(claim, "derived_from", claim.DerivedFrom, add)
	checkReferenceList(claim, "appears_in", claim.AppearsIn, add)
}

func checkReferenceList(claim domain.Claim, field string, refs []domain.ClaimReference, add func(code, msg string)) {
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.Ref) == "" {
			add(domain.CodeMissingField, field+" entry has an empty ref")
			continue
		}
		key := ref.Kind + "|" + ref.Ref
		if seen[key] {
			add(domain.CodeDuplicateEvidence,
				fmt.Sprintf("duplicate %s entry %s %s", field, ref.Kind, ref.Ref))
		}
		seen[key] = true
	}
}

// resolveClaimReferences checks every cross-record reference a claim declares.
//
// Only the layers this backend parses are resolved: sources, claims, nodes and sessions.
// Vocabulary entries and experiment runs are canonical layers the backend still does not
// read, so their references are shape-checked and carried through unresolved, exactly as
// Phase 1A carries node experiments: references. Claiming to have validated a reference
// against a layer that was never loaded would be worse than saying so.
//
// The semantic confidence and dispute rules in schemas/claim.schema.yaml are deliberately
// not reimplemented here. tools/validate-claims.ps1 is the canonical authority for them and
// gates every commit; a second Go implementation would be a second authority free to drift
// from the first. The backend validates what its own projection depends on — identity,
// vocabulary, shape and reference resolution — and no more.
func (r *Repository) resolveClaimReferences(
	claims []domain.Claim,
	nodes []domain.Node,
	sources []domain.Source,
	sessions []domain.Session,
	report *domain.ValidationReport,
) {
	claimIDs := make(map[string]bool, len(claims))
	for _, c := range claims {
		claimIDs[c.ID] = true
	}
	nodeIDs := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	sourceIDs := make(map[string]bool, len(sources))
	for _, s := range sources {
		sourceIDs[s.ID] = true
	}
	sessionIDs := make(map[string]bool, len(sessions))
	for _, s := range sessions {
		sessionIDs[s.ID] = true
	}

	for _, claim := range claims {
		fatal := func(code, msg string) {
			report.Add(domain.ValidationIssue{
				Severity: domain.SeverityFatal, Code: code, Ref: claim.ID, Path: claim.Path, Message: msg,
			})
		}

		for _, e := range claim.Evidence {
			if e.SourceID != "" && !sourceIDs[e.SourceID] {
				fatal(domain.CodeUnresolvedSource,
					fmt.Sprintf("evidence source %q is not registered in %s", e.SourceID, sourceRegistryPath))
			}
		}
		for _, a := range claim.Attribution {
			if a.SourceID != "" && !sourceIDs[a.SourceID] {
				fatal(domain.CodeUnresolvedSource,
					fmt.Sprintf("attribution source %q is not registered in %s", a.SourceID, sourceRegistryPath))
			}
		}

		for _, ref := range claim.DerivedFrom {
			switch ref.Kind {
			case domain.ClaimKindClaim:
				// A self-reference is a one-element cycle and is reported by
				// reportDerivationCycles, so it is not also reported here.
				if !claimIDs[ref.Ref] {
					fatal(domain.CodeUnresolvedClaim,
						fmt.Sprintf("derived_from claim %q does not resolve to a canonical claim", ref.Ref))
				}
			case domain.ClaimKindNode:
				if !nodeIDs[ref.Ref] {
					fatal(domain.CodeUnresolvedTarget,
						fmt.Sprintf("derived_from node %q does not resolve to a canonical node", ref.Ref))
				}
			case domain.ClaimKindExperimentRun:
				checkUnparsedRef(claim, "derived_from experiment_run", ref.Ref, fatal)
			}
		}

		for _, ref := range claim.AppearsIn {
			switch ref.Kind {
			case domain.ClaimKindNode:
				if !nodeIDs[ref.Ref] {
					fatal(domain.CodeUnresolvedTarget,
						fmt.Sprintf("appears_in node %q does not resolve to a canonical node", ref.Ref))
				}
			case domain.ClaimKindSession:
				if !sessionIDs[ref.Ref] {
					fatal(domain.CodeUnresolvedSession,
						fmt.Sprintf("appears_in session %q is not a source registered as type: session", ref.Ref))
				}
			case domain.ClaimKindVocabulary:
				checkUnparsedRef(claim, "appears_in vocabulary", ref.Ref, fatal)
			case domain.ClaimKindDocument:
				r.checkDocumentAppearance(claim, ref.Ref, report)
			}
		}
	}

	reportDerivationCycles(claims, report)
}

// checkUnparsedRef bounds a reference into a canonical layer the backend does not parse.
//
// The target cannot be resolved, so the only honest check is that the reference is a
// canonical identifier at all. A malformed one is fatal because it could never resolve
// under any later phase either.
func checkUnparsedRef(claim domain.Claim, field, ref string, fatal func(code, msg string)) {
	if !canonicalIDPattern.MatchString(ref) {
		fatal(domain.CodeInvalidID, fmt.Sprintf("%s %q is not a canonical identifier", field, ref))
	}
}

// checkDocumentAppearance validates a repository-relative document appearance site.
//
// An unsafe path is fatal: joining it to the corpus root is exactly what must never happen.
// A generated projection is fatal because schemas/claim.schema.yaml rejects it outright — a
// derived view cannot be evidence that a claim exists. A path that is safe, canonical, and
// simply absent is a warning, matching the Phase 1A treatment of a registered locator that
// does not resolve: it is a corpus gap for a human to decide about and it does not make the
// claim projection wrong.
func (r *Repository) checkDocumentAppearance(claim domain.Claim, ref string, report *domain.ValidationReport) {
	add := func(severity domain.Severity, code, msg, path string) {
		report.Add(domain.ValidationIssue{
			Severity: severity, Code: code, Ref: claim.ID, Path: path, Message: msg,
		})
	}
	if isExternalLocator(ref) {
		add(domain.SeverityFatal, domain.CodeUnsafePath,
			fmt.Sprintf("appears_in document %q is an external locator, not a repository-relative path", ref),
			claim.Path)
		return
	}
	rel, err := safeRelPath(ref)
	if err != nil {
		add(domain.SeverityFatal, domain.CodeUnsafePath,
			fmt.Sprintf("appears_in document %q is not a safe repository-relative path", ref), claim.Path)
		return
	}
	for _, dir := range generatedDirs {
		if strings.HasPrefix(rel+"/", dir) || strings.HasPrefix(rel, dir) {
			add(domain.SeverityFatal, domain.CodeGeneratedAppearance,
				fmt.Sprintf("appears_in document %q is a generated projection and may not be an appearance site", rel),
				claim.Path)
			return
		}
	}
	if _, err := fs.Stat(r.fsys, rel); err != nil {
		add(domain.SeverityWarning, domain.CodeUnresolvedDocument,
			"claim names an appearance document that does not exist in the repository", rel)
	}
}

// reportDerivationCycles rejects a claim derivation graph that is not acyclic.
//
// schemas/claim.schema.yaml states the rule, and the projection depends on it: a cycle
// would make "what was this built from" unanswerable and would not terminate under any
// later traversal. Detection walks claims in canonical ID order and each record's
// derived_from list in declared order, so the reported cycle is the same on every run.
func reportDerivationCycles(claims []domain.Claim, report *domain.ValidationReport) {
	derivations := make(map[string][]string, len(claims))
	paths := make(map[string]string, len(claims))
	for _, claim := range claims {
		refs := make([]string, 0, len(claim.DerivedFrom))
		for _, ref := range claim.DerivedFrom {
			if ref.Kind == domain.ClaimKindClaim {
				refs = append(refs, ref.Ref)
			}
		}
		derivations[claim.ID] = refs
		paths[claim.ID] = claim.Path
	}

	const (
		unvisited = 0
		onStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(claims))
	reported := make(map[string]bool, len(claims))

	var walk func(id string, stack []string)
	walk = func(id string, stack []string) {
		state[id] = onStack
		stack = append(stack, id)
		for _, next := range derivations[id] {
			if _, known := derivations[next]; !known {
				continue // unresolved reference; already reported by resolveClaimReferences
			}
			switch state[next] {
			case unvisited:
				walk(next, stack)
			case onStack:
				if !reported[next] {
					reported[next] = true
					report.Add(domain.ValidationIssue{
						Severity: domain.SeverityFatal, Code: domain.CodeClaimDerivationCycle,
						Ref: next, Path: paths[next],
						Message: "claim derivation forms a cycle: " + strings.Join(append(stack, next), " -> "),
					})
				}
			}
		}
		state[id] = done
	}

	for _, claim := range claims {
		if state[claim.ID] == unvisited {
			walk(claim.ID, nil)
		}
	}
}
