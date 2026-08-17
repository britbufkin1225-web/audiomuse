# Nodes

Nodes are durable AudioMuse concepts: they answer, "What does AudioMuse currently know about this concept?" Sessions remain chronological exploration records, while sources record where knowledge came from.

Every node is a readable Markdown file with YAML front matter conforming to `schemas/node.schema.yaml`. Its stable lowercase kebab-case `id` is the relationship target; filenames and titles may change without breaking links. Phase 2 assigns one primary `domain` and one knowledge-maturity `status` to each node.

## Relationship and expansion rules

- Use stable node IDs in `related_nodes`, never filenames or paths.
- Use registry IDs from `sources/source-registry.yaml` in `sources`.
- Add a session ID to `session_origin` only when that session introduced or materially developed the node.
- Promote a vocabulary term to a node when it supports durable relationships, experiments, applications, or project work.
- Keep prose concise and durable; preserve chronological detail in sessions.
- Phase 2 proof nodes link only to other proof nodes. Future nodes may be introduced when their content is ready.

## Domains and statuses

Allowed domains and statuses are defined in `schemas/node.schema.yaml`. Domains are intentionally broad. Status describes knowledge maturity (`seed` through `project-applied`), not software progress.

## Phase 2 proof nodes

- `sound`
- `frequency`
- `resonance`
- `timbre`
- `sampling`
