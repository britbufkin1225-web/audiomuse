# AudioMuse Knowledge Model

## Sessions

Sessions are chronological exploration records. They answer: "What did AudioMuse explore during this session?" A session may introduce, revisit, deepen, or apply many concepts.

## Nodes

Nodes are the durable conceptual units of AudioMuse. They answer: "What does AudioMuse currently know about this concept?" A node can be expanded by many sessions over time and has one primary Phase 2 domain.

Canonical knowledge-maturity statuses:

- `seed`
- `foundation`
- `developed`
- `deep-dive`
- `practical`
- `project-applied`

## Historical and cultural nodes

Most AudioMuse nodes are concept nodes derived from session exploration. Phase 12C introduced a second
shape: nodes in `history-culture`, and history-bearing nodes in other domains, which describe places,
institutions, people, practices, and objects rather than physical or perceptual concepts. They obey the
same schema and the same relationship vocabulary, and differ in three declared ways.

First, provenance replaces session origin. A historical node records `session_origin: []` where no
registered session developed it, and cites external registered sources instead. That is not missing
provenance; it is provenance from a different layer, and the knowledge-coverage view reports it as a
session-coverage candidate rather than as a defect.

Second, factual claims carry their evidence and their uncertainty in the node prose. Dates,
attributions, and firsts are not promoted from repetition, and where sources disagree the node says so
and adopts neither version. `docs/houston-musical-cartography.md` holds the chronology and the
dispute register those nodes point at.

Third, relationship types are chosen conservatively. A widely credited origin is stored as
`contributes_to` rather than `produces` when the evidence attributes rather than demonstrates, and no
new relationship type is invented to express a historical nuance. Where no directed claim is
supported, the node records `relationships: []`.

Phase 12D supplies the fourth thing those nodes needed: claim confidence as stored data rather than as
prose. Evidence type, corroboration, chronology precision, disputed status, and whether the cited text
was retrieved are now validated fields on canonical claim records, and the provenance Phase 12C
preserved in `research/sources/` is what makes them applicable retroactively. The prose markers in
`docs/houston-musical-cartography.md` remain; annotation proceeds by priority rather than in one
sweep.

## Claims

Claims are the epistemic layer. A node answers what AudioMuse knows about a concept; a claim answers
what one checkable statement asserts, what kind of statement it is, which registered sources support,
contradict, or qualify it, how strong that evidence is, and whether the sources conflict. Those four
things are stored independently and are never combined into a single score, and confidence grades the
repository's evidence rather than anyone's certainty.

Not every sentence becomes a claim. The layer exists for statements that are externally checkable and
that matter if they are wrong: dates, attributions, origin and priority assertions, contested
chronology, actionable technical statements, and conclusions AudioMuse reached by combining sources.
Definitions, explanations, and framing stay as prose.

Claims name their appearance sites; nodes and documents do not list their claims in return. The
reverse view is derived into `claims/index.md`, following the same rule the graph uses for inverse
edges. Node `sources:` lists are unchanged and keep their topical meaning: a source relevant to a node
is not the same statement as a source supporting a claim, and only the second is an evidence
relationship. `schemas/claim.schema.yaml` holds the contract and every bounded vocabulary;
`docs/claim-provenance-model.md` holds the taxonomy, the semantics, the origin-claim rule, and the
migration plan. Run `pwsh -NoProfile -File tools/validate-claims.ps1` after changing claims, and
`pwsh -NoProfile -File tools/test-claim-validator.ps1` after changing the validator.

## Sources

Sources provide provenance and evidence. They answer: "Where did this knowledge come from?" Nodes reference stable source-registry IDs; the registry records human-readable titles and repository-relative locators. Provenance is curated explicitly, not reconstructed automatically.

## Vocabulary

Vocabulary is the terminology, definition, and practical-context layer. It answers what a term means, how it appears in digital audio, why it matters in practice, and which technologies commonly use it. Entries may explicitly reference canonical nodes and sessions, but vocabulary does not duplicate or outrank either layer.

`node_refs` are cross-layer pointers, not graph edges. `session_refs` identify substantial discussion, not definition ownership, and may name only sources registered as `type: session` — the same constraint node `session_origin` obeys, so reference documents stay in the provenance layer instead of appearing as chronology. `related_terms` are glossary navigation associations only: they are untyped, do not imply equivalence, do not affect node degree, and never alter the Phase 5 graph or indexes. They are curated one-directional pointers and are deliberately not required to be symmetric.

The layers remain distinct:

- Sessions = exploration chronology
- Nodes = durable concepts and typed graph relationships
- Vocabulary = terminology and practical reference
- Sources = provenance and evidence
- Indexes = deterministic read-only projections
- Experiments = reproducible listening, observation, visualization, and measurement exercises
- Experiment runs = separate records of planned or performed experiment executions and their evidence
- Claims = checkable statements with typed provenance, graded evidence, and dispute status

The intended path is `Session -> Vocabulary -> Node -> Sources / Graph Relationships`; no step collapses one layer into another.

## Typed relationship pattern

`session -> concepts -> nodes --typed relationship--> nodes -> experiments / research / builds`

Each node's `relationships` array stores directed statements with a stable target node ID and a canonical relationship-type ID. For example, `frequency --influences--> pitch` means that frequency is the source, `pitch` is the target, and `influences` supplies the semantics. It does not imply the reverse statement.

The bounded vocabulary in `schemas/relationship-types.yaml` defines eleven approved types: `produces`, `characterized_by`, `influences`, `contributes_to`, `studies`, `captures`, `represents`, `processes`, `controls`, `enables`, and `used_in`. Each definition says when to use and avoid the type, records its conceptual inverse, and gives repository-supported examples. The vocabulary stays deliberately small so authors choose meaningful, comparable edges instead of inventing near-synonyms.

Inverse labels are descriptive metadata, not additional valid stored types. AudioMuse stores each supported claim once in its clearest direction; readers and future tools may derive an inverse display. Reciprocal edges are required only if both directions express independently useful claims. No current type is symmetrical.

Source relationships use stable registry IDs. The schemas in `schemas/` define the allowed values and contracts.

### Relationship integrity

- Target an existing node's stable `id`; never use a title, filename, path, or future placeholder.
- Do not create self-links or repeat an exact `(type, target)` pair in one node.
- Treat direction as part of the claim. Choose the source and target that make the canonical verb read naturally.
- Do not store a mirror edge solely for navigation; derive the documented inverse when presenting the graph.
- Add an edge only when node prose, session content, registered sources, or straightforward definitional semantics supports it.
- Trivial definitional edges may rely on the node's existing provenance. Factual, historical, causal, or non-obvious edges need appropriate source support.
- Cross-domain edges follow the same evidence rules; domain boundaries neither require nor prohibit a relationship.
- Propose future types by documenting a distinct recurring meaning, usage boundary, inverse, and repository examples. Do not add a synonym for an existing type.
- A node with no defensible outbound claim records `relationships: []`. Inbound edges from other nodes still connect it, so never invent an edge merely to avoid an empty list.
- Run `pwsh -NoProfile -File tools/validate-graph.ps1` after changing nodes or relationship types. It reports every integrity violation it finds, prints the graph counts, and exits non-zero while any violation remains.

Phase 4 cleanly replaces `related_nodes`; the repository does not maintain parallel typed and untyped systems. During migration, reciprocal adjacency pairs and weak associations were reviewed and normalized into single supported claims. Links that conveyed only redundant traversal or lacked a defensible typed meaning were removed rather than relabeled generically.

A vocabulary term does not automatically need its own node. Promote concepts when they become important enough to support meaningful relationships, experiments, or future work.

The many-to-many contribution view for the first three sessions lives in `docs/session-node-map.md`. It complements rather than replaces node-level provenance.

## Derived read-only views

The files in `indexes/` are deterministic views built from canonical nodes, sessions, schemas, and source/provenance records. They support navigation, audit, analysis, and future tooling without becoming authoritative or mutating graph semantics. They are not a database or duplicate knowledge store, and they must not be edited manually.

Run `pwsh -NoProfile -File tools/build-knowledge-index.ps1` to rebuild them and `pwsh -NoProfile -File tools/validate-knowledge-index.ps1` to verify canonical integrity, edge reconciliation, and generated-file currency. Inbound displays are derived navigation views only; the generator never synthesizes reverse canonical edges.

The generated `vocabulary/index.md` is a separate restrained projection of canonical vocabulary entries. It offers A-Z, domain, session, and node lookup without importing vocabulary associations into graph semantics. Build it with `tools/build-vocabulary-index.ps1` and validate it with `tools/validate-vocabulary.ps1`.

## Experiments

Experiments keep AudioMuse from becoming an encyclopedia that only accumulates descriptions. They connect supported concepts to something a reader can hear, observe, manipulate, record, or—when the method warrants it—measure. Canonical experiment records are separate from nodes because a procedure can span several concepts, sessions, and vocabulary terms and has its own safety and repeatability concerns. Embedding it in one node would imply false ownership and make reuse difficult.

Experiment references are cross-layer pointers, not canonical graph edges. The graph continues to own conceptual meaning; experiments provide practical routes through that meaning. Their generated index is disposable for the same reason as the knowledge and vocabulary indexes: navigation can be rebuilt, while authored records remain authoritative.

Observation means a qualitative report. Measurement means a numeric result with enough method and equipment context to interpret it. Keeping them separate prevents subjective impressions from acquiring false precision and avoids forcing measurement hardware into critical-listening exercises.

## Experiment runs

Experiment definitions say what to do; experiment runs say whether one execution was planned, completed, left incomplete, or invalidated. A definition may have no runs or many runs without acquiring mutable result state. Run records preserve environment, equipment/software, deviations, controls, direct observations, measurements, confounds, safety notes, interpretation, and follow-up questions in a separate repository layer.

Observations are direct qualitative or perceptual evidence, including subjective listening notes. Measurements are numeric evidence with a unit, method, tool, calibration state, uncertainty, and limitations. Nominal generator settings and DAW controls remain control settings, not measured acoustic output. Neither evidence form changes node maturity, creates a graph edge, or becomes scientific fact automatically; interpretation is explicit and remains bounded by recorded limitations.

## Canonical identity semantics

Canonical repository IDs and contract-defined enum values are compared using exact, case-sensitive ordinal semantics. IDs are data contracts: silently normalizing their case hides authoring defects, and locale-sensitive or case-insensitive matching is inappropriate for repository identity. Human-friendly search is a separate concern and may be tolerant without weakening canonical-reference validation.

| Validator | Reference type | Previous behavior | Phase 9 behavior |
| --- | --- | --- | --- |
| `validate-graph.ps1` | node IDs, relationship targets, relationship types | default hashtables and case-insensitive operators | ordinal dictionary/set lookup and case-sensitive enum equality |
| `validate-experiments.ps1`, `validate-experiment-runs.ps1` | duplicate detection within reference and prose arrays | default hashtables | ordinal sets, matching `validate-vocabulary.ps1` |
| `validate-graph.ps1`, `validate-vocabulary.ps1` | reported per-type and per-domain count ordering | `Sort-Object` (current culture) | ordinal sort |
| `validate-knowledge-index.ps1` | generated node membership and filenames | case-insensitive membership | exact case-sensitive reconciliation |
| `validate-vocabulary.ps1` | vocabulary IDs, node IDs, session IDs, related vocabulary IDs, domains | default hashtables and case-insensitive membership | ordinal sets/dictionary and case-sensitive domain validation |
| `validate-experiments.ps1` | experiment IDs, node IDs, vocabulary IDs, session/source IDs, related experiment IDs, closed enums | default hashtables and case-insensitive membership | ordinal sets/dictionary and case-sensitive enum validation |
| `validate-experiment-runs.ps1` | run IDs, experiment IDs, source IDs, status and calibration enums | experiment/source sets and enums were ordinal; run IDs and status-dependent rules were not fully cohesive | all canonical IDs and closed-enum decisions use exact case-sensitive semantics |

In `validate-graph.ps1` the relationship parser already restricts targets and types to lowercase, so
case drift is rejected before reference resolution runs and reports a malformed-entry defect. The
ordinal lookup behind it is defence in depth: it keeps identity exact if that pattern is ever widened.

Duplicate detection is ordinal for the same reason resolution is. A case-insensitive set reports a
case-drifted canonical reference as a duplicate rather than as an unresolved reference, and rejects
two prose entries that legitimately differ only by case. Vocabulary terms remain the one deliberate
exception: they are human-readable labels rather than identifiers, so `term` uniqueness stays
case-insensitive while every ID, reference, and closed-enum comparison is ordinal.
