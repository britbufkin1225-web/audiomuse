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

## Sources

Sources provide provenance and evidence. They answer: "Where did this knowledge come from?" Nodes reference stable source-registry IDs; the registry records human-readable titles and repository-relative locators. Provenance is curated explicitly, not reconstructed automatically.

## Vocabulary

Vocabulary is the terminology, definition, and practical-context layer. It answers what a term means, how it appears in digital audio, why it matters in practice, and which technologies commonly use it. Entries may explicitly reference canonical nodes and sessions, but vocabulary does not duplicate or outrank either layer.

`node_refs` are cross-layer pointers, not graph edges. `session_refs` identify substantial discussion, not definition ownership. `related_terms` are glossary navigation associations only: they are untyped, do not imply equivalence, do not affect node degree, and never alter the Phase 5 graph or indexes.

The layers remain distinct:

- Sessions = exploration chronology
- Nodes = durable concepts and typed graph relationships
- Vocabulary = terminology and practical reference
- Sources = provenance and evidence
- Indexes = deterministic read-only projections

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
