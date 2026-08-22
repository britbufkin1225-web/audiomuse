# AudioMuse Backend Architecture

## Why AudioMuse has a backend

AudioMuse is a repository-first knowledge atlas. The canonical corpus — nodes, sessions, sources,
claims, vocabulary, experiments, experiment runs, schemas, and research notes — lives in files and
stays authoritative. Direct filesystem browsing has become limiting as the corpus grew: the typed
relationship graph, the provenance registry, and the cross-layer reference structure are all real
data that no text editor can traverse.

Backend Phase 1A drew a computational boundary and Phase 1B extended it through the evidence
layer:

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
| `claims/records/*.yaml` YAML document streams (`schemas/claim.schema.yaml`) | one mapping per claim | `domain.Claim` | `claimsByID`, sorted `claims` | `GET /api/v1/claims`, `GET /api/v1/claims/{id}` |
| `claim.evidence[]` (`{relation, source_id, note}`) and `claim.attribution[]` | ordered citation lists | `domain.ClaimEvidence`, `domain.ClaimAttribution` | `claimIDsBySourceID`, `sourceClaims`, `attributedClaimIDs` | `claim.evidence`, `source.claims`, `?source_id=`, `?relation=` |
| `claim.appears_in[]` and `claim.derived_from[]` (`{kind, ref}`) | kind-qualified reference lists | `domain.ClaimReference` | `claimIDsByNodeID`, `claimIDsBySessionID` | `claim.appears_in`, `?node_id=`, `?session_id=` |
| `schemas/claim.schema.yaml` and `schemas/source.schema.yaml` bounded enums | vocabulary lists | `domain.Vocabularies` | evidence filter validation set | `project.vocabulary`; `400 invalid_query` |
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

**Why Phase 1B extended the Phase 1A machinery instead of adding a subsystem.** Sources, claims,
and provenance are not an independent content type sitting beside the knowledge graph; they are
relationships inside it. A claim's evidence points at the same registry a node's `sources:` points
at, and its `appears_in` points at the same nodes and sessions the graph already indexes. A second
parser hierarchy, a second startup index, and a second resolution pass would have produced two
answers to "does this ID resolve", free to disagree. Claims therefore load through the same
filesystem adapter, resolve in the same pass, and live in the same immutable index as everything
else. There is one read model.

**Why the contract vocabularies are read rather than compiled in.** `schemas/claim.schema.yaml`
states the reason for its own validator: a vocabulary change must be a schema change and must not be
possible to make silently inside code. The backend now exposes `claim_type`, `confidence`,
`dispute_status`, `temporal_precision`, evidence `relation`, source `type`, `relationship`,
`evidence_class` and `retrieval` as API filters, so a compiled-in copy of any of those lists would
be a second authority that could drift from the schema and silently answer a filter with the wrong
set. Both contract files are read at startup and an unreadable one is fatal.

**Why the backend does not reimplement the semantic claim rules.** `schemas/claim.schema.yaml` also
declares rules about what confidence a claim may carry given its evidence, when an attribution is
required, and how dispute status must match the cited relations. `tools/validate-claims.ps1` is the
canonical authority for those and gates every commit. A second Go implementation would be a second
authority with its own bugs and its own drift. The backend validates exactly what its own projection
depends on — identity, vocabulary, record shape, and reference resolution — and no more.

**Why topical and evidential source relations are kept apart.** `docs/claim-provenance-model.md`
distinguishes a node `sources:` list, which says a source is relevant to a concept, from claim
`evidence`, which says a source materially supports a specific statement. `GET /api/v1/sources/{id}`
therefore serves `node_ids` (topical) and `claims` (evidential, each carrying its relation) as
separate fields. Merging them into one "related nodes" list would erase the distinction the entire
provenance layer exists to make.

**Why reverse indexes were added only where an endpoint needs one.** Each map built at startup —
claims by source, by node, by session; claims and attributions by source; nodes by source; sessions
by source and its inverse — answers exactly one filter or one detail field. Claim to source and
claim to node need no index because those are fields on the claim record itself. Nothing was built
speculatively for a traversal a later phase might want.

**Why source and session are related through claims.** There is no canonical edge between a registry
entry and a session. `GET /api/v1/sources?session_id=` therefore means "sources cited by a claim that
appears in that session", which is the evidence-layer question. Deriving it instead from node
`session_origin` would have merged the topical and evidential relations back together.

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

Phase 1B adds, at the same severity: a claim record whose key set does not equal the contract's, a
duplicate claim ID, a blank or non-canonical claim ID, an empty required claim field, a claim
`evidence`, `attribution`, `derived_from` or `appears_in` item whose key set does not equal the
contract's, a value outside any bounded claim or source vocabulary, an unresolved evidence or
attribution source, an unresolved `appears_in` or `derived_from` node, session or claim reference, a
duplicate evidence, attribution or reference entry, a claim with no appearance site, a derivation
cycle, an appearance document that is an unsafe path or an external locator, an appearance document
under `indexes/`, and an unreadable or vocabulary-less `schemas/claim.schema.yaml` or
`schemas/source.schema.yaml`.

**Warning** — the projection is correct but the corpus has a gap, so startup succeeds and reports:
a registered source whose repository-relative locator does not exist, a registered session with no
`sessions/<id>/` directory, a session no node cites, a registered source that neither a node nor a
claim cites, and a claim appearance document that is safe and canonical but does not exist.

The backend never repairs a record and never writes to the corpus. Canonical inconsistencies are
reported for human decision.

`/api/v1/diagnostics` makes this boundary machine-readable: `validation_scope` is
`runtime_projection`, while `repository_semantic_validation` is `external_precondition`. Its
`valid` status must not be interpreted as an in-process execution of the PowerShell semantic rules.

## Known limitations (through Phase 1B)

- Corpus changes require a process restart.
- Search is lexical substring matching only; there is no semantic retrieval, ranking model, or
  embedding.
- No persistence, no database, no cache beyond the startup index.
- No file watcher, background worker, or scheduled ingestion.
- No frontend and no graph visualization.
- No LLM or AI integration of any kind.
- Experiments, experiment runs, and vocabulary entries are canonical layers the backend still does
  not parse. Node `experiments:` references and claim `appears_in: vocabulary` and
  `derived_from: experiment_run` references are therefore checked for identifier shape only and
  carried through unresolved. Claiming to have validated a reference against a layer that was never
  loaded would be worse than saying so.
- The semantic confidence, dispute, attribution and origin-term rules in
  `schemas/claim.schema.yaml` are enforced by `tools/validate-claims.ps1`, not by the backend.
- `appears_in: session` is a canonical reference kind that no current claim record uses, so
  `GET /api/v1/claims?session_id=` and `GET /api/v1/sources?session_id=` answer correctly and
  return nothing against today's corpus.
- Graph traversal across the evidence layer is not implemented. Phase 1B resolves the
  session/node/claim/source relations deterministically; walking them is a later decision.

## Future work

Deferred, not implemented: vocabulary and experiment-run parsing, evidence-layer graph traversal,
richer diagnostics, search hardening, graph visualization, semantic retrieval, and MLLM
experimentation.
