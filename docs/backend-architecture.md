# AudioMuse Backend Architecture

## Why AudioMuse has a backend

AudioMuse is a repository-first knowledge atlas. The canonical corpus — nodes, sessions, sources,
claims, vocabulary, experiments, experiment runs, schemas, and research notes — lives in files and
stays authoritative. Direct filesystem browsing has become limiting as the corpus grew: the typed
relationship graph, the provenance registry, and the cross-layer reference structure are all real
data that no text editor can traverse.

Backend Phase 1A draws a computational boundary:

```text
CANONICAL KNOWLEDGE   →   SOFTWARE THAT INSPECTS THAT KNOWLEDGE
```

The architectural rule is narrow and load-bearing:

> The repository remains the source of truth. The Go backend is a deterministic read-only
> projection of repository state.

The backend never creates, rewrites, repairs, or infers canonical content. It reads, parses,
validates, indexes, resolves, filters, searches, projects, and serves JSON.

## Projection chain

Every field the API serves can be traced back through this chain to a canonical file.

| Repository source | Parsed representation | Go type | Service projection | API representation |
| --- | --- | --- | --- | --- |
| `nodes/<domain>/<id>.md` YAML front matter (`schemas/node.schema.yaml`) | front matter map + markdown body | `domain.Node` | `Index.nodesByID`, sorted `nodes`, `nodesByDomain` | `GET /api/v1/nodes`, `GET /api/v1/nodes/{id}` |
| `node.relationships[]` (`{target, type}`) | ordered edge list | `domain.Relationship` | outbound + derived inbound adjacency | `node.relationships`, `node.inbound_relationships`, `GET /api/v1/graph` |
| `schemas/relationship-types.yaml` | typed vocabulary list | `domain.RelationshipType` | edge-type validation set | edge `type` values; `GET /api/v1/project` |
| `sources/source-registry.yaml` (`schemas/source.schema.yaml`) | registry entry list | `domain.Source` | `sourcesByID` | provenance reference resolution; session identity |
| registry entries with `type: session`, plus `sessions/<id>/` on disk | registry entry + directory presence | `domain.Session` | `sessionsByID`, derived session→node contribution map | `GET /api/v1/sessions` |
| parse + reference resolution outcomes | issue list | `domain.ValidationIssue` | fatal/warning partition | startup log; `GET /api/v1/diagnostics` |

## Layering

Filesystem parsing never happens inside an HTTP handler. Dependencies point one way:

```text
httpapi  →  service  →  repository (interface)  →  repository/filesystem  →  canonical repository
```

- `internal/domain` — types only. No I/O, no HTTP, no filesystem.
- `internal/repository` — `KnowledgeRepository`, a read-only interface. It has no write method, so
  a mutation path cannot be added without changing the contract deliberately.
- `internal/repository/filesystem` — the only package that touches the corpus. Read calls only.
- `internal/service` — builds the immutable in-memory index once at startup and answers queries.
- `internal/httpapi` — routing, query parsing, bounds, JSON envelopes, method lock.

## Rationale

**Why the repository stays canonical.** The corpus is authored, reviewed, and phase-gated by humans
and validated by `tools/*.ps1`. A backend that could write would create a second source of truth and
a reconciliation problem that AudioMuse has explicitly avoided since Phase 1.

**Why the filesystem adapter sits behind an interface.** Nothing above `repository` knows a file
exists. A future SQLite, embedded-index, or Postgres adapter can be substituted without touching
service or HTTP code. The interface is the seam that keeps that promise honest.

**Why in-memory indexing is sufficient.** The corpus is 78 nodes, 220 edges, 3 sessions, and 51
registered sources. It loads in milliseconds and fits comfortably in memory. A database at this size
would add operational surface, a migration story, and a second copy of the truth, buying nothing.

**Why there is no file watcher.** A watcher implies partial reloads, invalidation ordering, and
mid-flight inconsistency between the graph and the validation report. Phase 1A instead guarantees
that a running process serves exactly one consistent snapshot. Restarting after a corpus change is
the documented and acceptable cost.

**Why graph edges are explicit-only.** `docs/knowledge-model.md` states that an edge is a claim, that
direction is part of the claim, and that inverse edges are derived for display and never stored. A
projection that manufactured edges from keyword overlap or embedding proximity would insert
unsourced claims into a corpus whose entire discipline is that claims carry provenance. Inbound
adjacency is served as `inbound_relationships`, clearly separated from the node's own
`relationships`, so a derived view can never be mistaken for authored data.

**Why the standard library.** `net/http` in current Go routes methods and path wildcards natively,
so a third-party router would add a dependency to replace one line of `http.ServeMux` setup. The one
external dependency, `gopkg.in/yaml.v3`, exists because the corpus is YAML and regex parsing of
structured records is exactly the brittleness `tools/validate-graph.ps1` works around today.

**Why lexical search only.** Semantic retrieval requires an embedding model, a similarity threshold,
and a decision about what "related" means — three unsourced judgments. Phase 1A search is
deterministic substring matching over declared fields, and it says so.

**Why mutating methods are rejected at the edge.** Read-only is asserted by a middleware that runs
before routing, not by the absence of write handlers. That makes the guarantee test-coverable and
makes an accidental future write route unreachable rather than merely unwritten.

## Validation severity

The backend separates two different failures.

**Fatal** — the projection would be wrong or ambiguous, so startup fails:
malformed front matter, unparseable YAML, missing or invalid `id`, duplicate canonical ID, missing
or unknown top-level field, unresolved relationship target, relationship type outside the canonical
vocabulary, self-link, duplicate `(type, target)` pair, unresolved `session_origin` or `sources`
reference, unsafe path.

**Warning** — the projection is correct but the corpus has a gap, so startup succeeds and reports:
a registered source whose repository-relative locator does not exist, a registered session with no
`sessions/<id>/` directory, a session no node cites, a registered source no node cites.

The backend never repairs a record and never writes to the corpus. Canonical inconsistencies are
reported for human decision.

## Known limitations (Phase 1A)

- Corpus changes require a process restart.
- Search is lexical substring matching only; there is no semantic retrieval, ranking model, or
  embedding.
- No persistence, no database, no cache beyond the startup index.
- No file watcher, background worker, or scheduled ingestion.
- No frontend and no graph visualization.
- No LLM or AI integration of any kind.
- Claims, experiments, experiment runs, and vocabulary are not yet parsed or served; node
  `experiments:` references are therefore carried through unresolved and unvalidated.

## Future work

Deferred, not implemented: a sources/claims/provenance read API, richer diagnostics, search
hardening, graph visualization, semantic retrieval, and MLLM experimentation.
