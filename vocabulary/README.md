# AudioMuse Vocabulary

Vocabulary is AudioMuse's terminology and practical-context layer. It answers what a term means, how it relates to digital audio, why it is useful, what technologies use it, and where the existing atlas explains it further.

Canonical entries live in domain-grouped YAML files under `entries/` and conform to `schemas/vocabulary.schema.yaml`. The initial set is a bounded migration from the Session 1 Vocabulary Atlas, registered as the source `session-01-vocabulary-atlas`; the original DOCX remains source material, not a session and not a second canonical store. Because that atlas is the shared origin of every entry in this migration, it is recorded here once rather than repeated as a per-entry reference. Keep entries concise and preserve the distinctions among `definition`, `digital_relationship`, `best_use`, and `technologies`.

## Cross-reference semantics

- `node_refs` names existing canonical node IDs. A term can reference zero, one, or multiple nodes, but is not itself a node and does not own the durable concept.
- `session_refs` names registered session source IDs where the term was introduced or substantially discussed. A session records exploration chronology; it does not own the canonical definition. Only sources registered as `type: session` qualify, matching the rule node `session_origin` already follows. A term that no session has covered yet keeps an empty list rather than borrowing an unrelated source.
- `related_terms` names other vocabulary IDs for human navigation only. These associations are not typed graph relationships, do not imply equivalence, never create graph edges, and must not affect node degree or Phase 5 indexes.
- Nodes continue to own canonical typed relationships and source provenance. Vocabulary must not infer, mirror, or mutate those relationships.

The cross-layer path is therefore `Session -> Vocabulary -> Node -> Sources / typed graph relationships`, with each link retaining its own meaning.

## Authoring and validation

Use stable lowercase kebab-case IDs, a domain from `schemas/node.schema.yaml`, JSON-compatible YAML strings/arrays, and exactly the documented fields. Cross-references are curated rather than derived from keyword matches.

Run:

```powershell
pwsh -NoProfile -File tools/build-vocabulary-index.ps1
pwsh -NoProfile -File tools/validate-vocabulary.ps1
```

`index.md` is a deterministic read-only projection providing A-Z, domain, session, and canonical-node views. Do not edit it manually.
