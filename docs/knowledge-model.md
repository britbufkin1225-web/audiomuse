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
- Run `pwsh -NoProfile -File tools/validate-graph.ps1` after changing nodes or relationship types.

Phase 4 cleanly replaces `related_nodes`; the repository does not maintain parallel typed and untyped systems. During migration, reciprocal adjacency pairs and weak associations were reviewed and normalized into single supported claims. Links that conveyed only redundant traversal or lacked a defensible typed meaning were removed rather than relabeled generically.

A vocabulary term does not automatically need its own node. Promote concepts when they become important enough to support meaningful relationships, experiments, or future work.

The many-to-many contribution view for the first three sessions lives in `docs/session-node-map.md`. It complements rather than replaces node-level provenance.
