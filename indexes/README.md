# AudioMuse Knowledge Index

This directory contains generated, read-only views of canonical AudioMuse repository content. Node, session, schema, and source/provenance files remain authoritative. These indexes are navigation and audit conveniences—not a database or a duplicate knowledge store.

Do not edit these files manually. Regenerate and validate them from the repository root:

```powershell
.\tools\build-knowledge-index.ps1
.\tools\validate-knowledge-index.ps1
.\tools\build-knowledge-coverage.ps1
.\tools\validate-knowledge-coverage.ps1
```

## Summary

- Nodes: 78
- Relationships: 220
- Relationship types represented: 11
- Sessions represented: 3
- Registered sources: 51
- Sources referenced by nodes: 50
- Domains represented: 12

## Views

- `nodes-by-domain.md` groups canonical nodes by canonical domain metadata.
- `relationships-by-type.md` groups explicit directed edges by canonical type.
- `node-connections.md` shows typed outbound and inbound navigation without synthesizing reverse edges.
- `session-coverage.md` shows the many-to-many session-to-node contribution map in both directions.
- `source-coverage.md` reports provenance presence and reuse without scoring source quality.
- `knowledge-coverage.md` summarizes explicit node, domain, session, vocabulary, experiment, run-evidence, provenance, and connectivity coverage and explains bounded research-gap candidates.
- `knowledge-coverage.json` is the machine-readable form of that same derived view.

## Knowledge coverage boundaries

Coverage is a measurable relationship between canonical entities; an observation is a deterministic interpretation of that structure; a research-gap candidate is a reason-backed signal for human review. None of those is a confirmed knowledge defect. Coverage does not measure correctness, research quality, truth, or universal completeness.

The bounded candidate rules flag nodes with zero sources, zero sessions, zero vocabulary cross-references, practical applications but zero linked experiment definitions, or at most one typed connection. They also flag canonical domains containing at most one node. These fixed rules were chosen because their evidence is directly auditable. Relative rankings, percentages, confidence scores, and inferred semantic matches were rejected because the repository has no defensible completeness denominator and does not authorize manufactured provenance.

Experiment definitions and completed runs remain distinct. A linked definition counts as experiment coverage; only a run whose canonical status is `completed` counts as performed evidence. A node with a linked definition and no completed run therefore reports `partial` run-evidence coverage whether its experiment has planned runs or no run records at all, which records the absence of performed evidence rather than the presence of any run. Canonical files remain authoritative, and humans decide whether any candidate warrants research.
