# Knowledge Coverage and Research-Gap Analysis

Phase 10 derives a read-only view from canonical nodes, relationships, sources, sessions, vocabulary, experiments, and experiment runs. Canonical content remains authoritative.

## Semantic boundary

- **Coverage** is a measurable explicit relationship, such as a node referencing two registered sources.
- **Observation** is a deterministic interpretation of repository structure, such as a node having no vocabulary cross-reference.
- **Research-gap candidate** is a bounded, evidence-backed signal that human review may be useful.
- **Knowledge defect** is a confirmed canonical error. Phase 10 never infers one from coverage statistics.

States are `covered`, `partial`, `unlinked`, and `not_applicable`. `partial` is currently used for practical run evidence when an experiment definition exists but no completed run exists. Coverage is not a quality score, and AudioMuse has no defensible denominator for universal completeness.

## Decisions and rationale

**What:** Use fixed zero/one-count rules with explicit evidence and prose reasons. **Why:** They are reproducible and traceable to canonical facts. **Rejected:** percentile rankings and weighted scores. **Why rejected:** the small corpus and absent universal denominator would make them falsely authoritative.

**What:** Treat declared practical applications as the applicability gate for experiment candidates. **Why:** it is an explicit canonical signal that practical work fits the node. **Rejected:** requiring an experiment for every concept. **Why rejected:** not every concept reasonably needs an experiment.

**What:** Count only `completed` runs as performed evidence. **Why:** definitions, planned executions, observations, and measurements have distinct contracts. **Rejected:** treating any run record as evidence. **Why rejected:** planned and incomplete records do not establish performed evidence.

**What:** Generate Markdown plus JSON in `indexes/`. **Why:** Markdown supports human review and JSON supports strict reconciliation while matching existing generated-index conventions. **Rejected:** a database, API, or duplicated canonical store. **Why rejected:** repository files are the authoritative system.

## Rules and limitations

Candidates are emitted for zero node sources, zero node sessions, zero vocabulary links, practical applications with zero linked experiments, at most one total typed connection, and domains with at most one node. Reasons report the triggering facts. These thresholds describe current representation only. They do not infer semantic similarity, source support, missing edges, research priority, correctness, or quality.

Rebuild and validate from the repository root:

```powershell
.\tools\build-knowledge-coverage.ps1
.\tools\validate-knowledge-coverage.ps1
.\tools\test-knowledge-coverage.ps1
```
