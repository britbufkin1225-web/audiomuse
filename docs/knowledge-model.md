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

## Relationship pattern

`session -> concepts -> nodes -> related nodes -> experiments / research / builds`

Node-to-node relationships use stable node IDs rather than filenames. Source relationships use stable registry IDs. The schemas in `schemas/` define the allowed values and Phase 2 contracts.

A vocabulary term does not automatically need its own node. Promote concepts when they become important enough to support meaningful relationships, experiments, or future work.

The many-to-many contribution view for the first three sessions lives in `docs/session-node-map.md`. It complements rather than replaces node-level provenance.
