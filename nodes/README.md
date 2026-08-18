# Nodes

Nodes are durable AudioMuse concepts: they answer, "What does AudioMuse currently know about this concept?" Sessions remain chronological exploration records, while sources record where knowledge came from.

Every node is a readable Markdown file with YAML front matter conforming to `schemas/node.schema.yaml`. Its stable lowercase kebab-case `id` is the relationship target; filenames and titles may change without breaking links. Phase 2 assigns one primary `domain` and one knowledge-maturity `status` to each node.

A node file lives at `nodes/<domain>/<id>.md`, where `<domain>` is the node's primary domain from the schema enum. Domain subdirectories are created as nodes need them; empty placeholders are not required for every domain.

## Relationship and expansion rules

- Use stable node IDs as relationship `target` values, never filenames or paths.
- Use registry IDs from `sources/source-registry.yaml` in `sources`.
- Add a session ID to `session_origin` only when that session introduced or materially developed the node.
- Promote a vocabulary term to a node when it supports durable relationships, experiments, applications, or project work.
- Keep prose concise and durable; preserve chronological detail in sessions.
- Keep relationships curated, typed, and conceptually meaningful. Read each entry as `current node --type--> target`.
- Use only IDs from `schemas/relationship-types.yaml`. Do not store inverse or reciprocal edges only to improve traversal.
- Keep relationships in deterministic author-curated order and run `pwsh -NoProfile -File tools/validate-graph.ps1` after edits.

## Domains and statuses

Allowed domains and statuses are defined in `schemas/node.schema.yaml`. Domains are intentionally broad. Status describes knowledge maturity (`seed` through `project-applied`), not software progress.

## Phase 2 proof nodes

- `sound`
- `frequency`
- `resonance`
- `timbre`
- `sampling`

These nodes remain canonical and were reused in the Phase 3 expansion. See `docs/session-node-map.md` for the first session-derived graph overview.
