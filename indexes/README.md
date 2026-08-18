# AudioMuse Knowledge Index

This directory contains generated, read-only views of canonical AudioMuse repository content. Node, session, schema, and source/provenance files remain authoritative. These indexes are navigation and audit conveniences—not a database or a duplicate knowledge store.

Do not edit these files manually. Regenerate and validate them from the repository root:

```powershell
.\tools\build-knowledge-index.ps1
.\tools\validate-knowledge-index.ps1
```

## Summary

- Nodes: 15
- Relationships: 45
- Relationship types represented: 11
- Sessions represented: 3
- Registered sources: 5
- Sources referenced by nodes: 4
- Domains represented: 7

## Views

- `nodes-by-domain.md` groups canonical nodes by canonical domain metadata.
- `relationships-by-type.md` groups explicit directed edges by canonical type.
- `node-connections.md` shows typed outbound and inbound navigation without synthesizing reverse edges.
- `session-coverage.md` shows the many-to-many session-to-node contribution map in both directions.
- `source-coverage.md` reports provenance presence and reuse without scoring source quality.
