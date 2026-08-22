# AudioMuse Backend Architecture

## Why AudioMuse has a backend

AudioMuse is a repository-first knowledge atlas. The canonical corpus — nodes, sessions, sources,
claims, vocabulary, experiments, experiment runs, schemas, and research notes — lives in files and
stays authoritative. Direct filesystem browsing has become limiting as the corpus grew: the typed
relationship graph, the provenance registry, and the cross-layer reference structure are all real
data that no text editor can traverse.

Backend Phase 1A drew a computational boundary, Phase 1B extended it through the evidence
layer, and Phase 1C connected the two into a bounded traversal layer over the relationships
they already resolve:

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
| the fields above, read as one graph | canonical field references | `domain.GraphRelationship`, `domain.EntityRef` | `Knowledge.adjacency`, one entry per `(type, id)` | `GET /api/v1/graph/entities/{entity_type}/{id}/relationships`, `.../traverse` |
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
- `internal/service` — builds the immutable in-memory index once at startup and answers queries,
  including the bounded breadth-first traversal over the relationship adjacency.
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

**Why Phase 1C exposes traversal as a bounded read model rather than a graph database.** The
relationships were already resolved: Phase 1A resolves node edges and session origin, Phase 1B
resolves claim evidence, attribution, appearance and derivation. What was missing was the ability to
follow more than one of them per request. Building that as an adjacency index over the records
already in the startup index costs one pass at load and adds no new authority; introducing SQLite,
Neo4j or a persisted graph store would introduce a second copy of the truth, a migration story, and
the reconciliation problem the repository-first rule exists to avoid. The corpus is 78 nodes, 48
claims and 51 sources; a depth-3 traversal of it completes in memory in well under a second.

**Why traversal edges are explicit-only, again.** The Phase 1A rule that no edge may be manufactured
from keyword overlap or embedding proximity does not weaken because an edge crosses a layer
boundary. Every Phase 1C edge names the canonical field it was read from in its `origin`, so "why
does this edge exist" is answerable from the edge itself. No edge is derived from a shared word, a
similar title, overlapping prose, co-occurrence or any similarity measure, and there is no automatic
edge discovery of any kind.

**Why every authored edge is emitted with a reverse, and why the reverse is labelled.** A traversal
that could only follow authored direction would leave every source unable to reach the claims that
cite it, which is the question the provenance layer is most often asked. The reverse of a node edge
uses that relationship type's own `inverse` from `schemas/relationship-types.yaml`; the reverse of an
evidence relation is the same verb in the active voice; the cross-layer reverses restate the field
they come from. None is invented. Each carries `"derived": true`, because `docs/knowledge-model.md`
states that AudioMuse stores each claim once in its clearest direction and that inverse labels are
descriptive metadata rather than storable edges — a reverse edge presenting itself as authored would
misrepresent the corpus.

**Why the four entity classes stay distinct instead of becoming generic nodes.** A node named
"Room Mode", a claim that standing-wave behaviour produces location-dependent pressure peaks, and
the reference work that supports it are three different kinds of thing, and the difference is the
entire point of the knowledge and provenance models. Collapsing them into one graph-node type would
make a traversal cheaper to render and would destroy the distinction that claim confidence,
provenance inspection, contradiction analysis and source-quality analysis all depend on. Identity is
therefore the pair `(type, id)`, which also keeps a registry entry of `type: session`
distinguishable from the session projected from it.

**Why topical and evidential source edges have different names.** `sourced_from` comes from a node's
`sources:` list and means the source is relevant to the concept. `supported_by` comes from a claim's
`evidence` and means the source materially supports that statement. They are the same distinction
`GET /api/v1/sources/{id}` keeps between `node_ids` and `claims`, carried into the graph. The
evidence relation itself is preserved rather than flattened into a generic evidence edge, so a
source that contradicts a claim can never look like one that supports it.

**Why there is no direct source-to-session edge.** Phase 1B answers `?session_id=` on the source
list through claims, because there is no canonical edge between a registry entry and a session. A
traversal reaches the same fact by walking session, claim, source, which is two hops and reports
itself as two hops. Emitting it additionally as one direct edge would make the same relation
countable twice and would make `depth` mean something other than hops.

**Why breadth-first.** Depth then means shortest hop distance, which is what a caller exploring
outward from a concept expects and what makes the `distance` field a fact about the graph rather
than an artefact of the walk. A depth-first traversal would report an entity at whatever distance it
happened to be reached first. Breadth-first also degrades honestly under the result bounds: what a
truncated response loses is the far edge of the neighbourhood, not an arbitrary branch.

**Why depth and fan-out are both bounded, and bounded in code.** Depth alone is not a bound — one
hub entity can have hundreds of neighbours, so a depth-2 request over a large corpus can cost far
more than a depth-3 request over a sparse one. Depth is capped at 3 because that is the length of
the epistemic path the model is built around, session to node to claim to source; a fourth hop buys
reach that is no longer explainable as one question. The entity and edge caps are service constants
rather than configuration because they are API safety invariants: a caller who could raise them
could ask one request to serialise the corpus, and an operator who could lower them would change
what the documented contract means. A truncated result always reports `partial` and its reason;
silently dropping results while claiming completeness would be a wrong answer rather than a small
one.

**Why cycles are expected rather than prevented.** Every authored edge has a reverse, so any related
pair is already a two-cycle, and claim derivation can close longer loops. The traversal keeps a
visited set and expands each entity exactly once at its shortest distance, so a cyclic corpus
terminates. Cycles are a property of a knowledge graph, not a defect to be validated away.

**Why a generic graph-query language is deferred.** A `MATCH ... WHERE ... RETURN` surface, or a
JSON body describing arbitrary traversal steps, is a program the caller supplies and the server
executes, which means unbounded cost, an evaluator to secure, and a query semantics to specify and
version. The current need is to follow known relationships safely, and two bounded GET routes answer
that. A bounded surface can be widened later on evidence of a real query a client cannot compose; a
query engine cannot be narrowed once clients depend on it.

**Why traversal has no paging.** Paging a graph requires a stable cursor over a result whose shape
the caller cannot see before requesting it, and a page boundary through a neighbourhood is not a
meaningful unit. A caller narrows with `depth`, `relationship` or `target_type` instead, and a
result that hit a bound says so.

**Why filters are applied during expansion.** A filter applied to the finished result would let
`depth` count hops along edges that were then discarded, so a depth-2 request could return entities
that are not two matching hops away. Filtering while expanding makes a filtered traversal the
traversal of the filtered subgraph, which is the only reading of `depth` that stays true.

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

Phase 1C additionally validates the executable inverse contract in
`schemas/relationship-types.yaml`: every forward and inverse label must be non-empty canonical
`snake_case`, a directed predicate may not be self-inverse, and labels must be unique across the
forward/inverse namespace. Violations are fatal because an ambiguous inverse would make traversal
semantics and the `derived` provenance marker untrustworthy.

**Warning** — the projection is correct but the corpus has a gap, so startup succeeds and reports:
a registered source whose repository-relative locator does not exist, a registered session with no
`sessions/<id>/` directory, a session no node cites, a registered source that neither a node nor a
claim cites, and a claim appearance document that is safe and canonical but does not exist.

The backend never repairs a record and never writes to the corpus. Canonical inconsistencies are
reported for human decision.

`/api/v1/diagnostics` makes this boundary machine-readable: `validation_scope` is
`runtime_projection`, while `repository_semantic_validation` is `external_precondition`. Its
`valid` status must not be interpreted as an in-process execution of the PowerShell semantic rules.

## Known limitations (through Phase 1C)

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
- Graph traversal is deliberately bounded: depth 1 to 3, 500 entities and 2000 relationships per
  request, two GET routes, no query language, no caller-supplied traversal program, no paging and
  no mutation. The adjacency is derived at startup and never persisted.
- The traversal graph addresses only the four record classes the backend parses. A claim
  `appears_in: vocabulary`, `appears_in: document` or `derived_from: experiment_run` reference is
  carried through unresolved and produces no graph entity and no edge.
- A registered session and its registry entry are addressed as two entities that share an ID, and
  no edge is emitted between them: they are one canonical record seen through two projections.

## Future work

Deferred, not implemented: vocabulary and experiment-run parsing, richer diagnostics, search
hardening, graph visualization, semantic retrieval, and MLLM experimentation. The Phase 1C contract
is shaped to be useful to a future read-only graph inspector, provenance-path view or
claim-confidence overlay without any of them being implemented here, and without the backend being
distorted around a hypothetical frontend.
