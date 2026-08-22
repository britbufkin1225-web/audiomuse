# AudioMuse Backend — Read-Only Knowledge API

A deterministic read-only HTTP projection of the canonical AudioMuse repository: nodes, sessions,
the typed relationship graph, the sources, claims and provenance that stand behind them, the
vocabulary, experiments and experiment runs that put them into practice, and one lexical search
surface spanning all of them.

**The repository remains the source of truth.** This service reads the corpus once at
startup, validates what it read, indexes it in memory, and serves JSON. It performs no
runtime writes of any kind.

`docs/backend-architecture.md` holds the full projection chain and the rationale behind
each architectural decision. This file is the operator's guide.

## Directory structure

```text
backend/
├── cmd/audiomuse-api/          process entry point, startup logging, graceful shutdown
├── internal/
│   ├── config/                 repository root and listen address resolution
│   ├── domain/                 typed AudioMuse records; no I/O, no HTTP
│   ├── repository/             KnowledgeRepository — a read-only interface with no write method
│   │   └── filesystem/         the only package that touches the corpus
│   ├── service/                immutable startup index, filtering, graph, evidence, traversal, practice and cross-layer discovery projections
│   ├── httpapi/                routing, query bounds, JSON envelopes, method lock
│   └── testsupport/            fixture corpus loading for tests
├── testdata/corpus/            synthetic fixture corpus (not canonical knowledge)
└── go.mod
```

Dependencies point one way and filesystem parsing never happens in an HTTP handler:

```text
httpapi  →  service  →  repository (interface)  →  repository/filesystem  →  canonical repository
```

The only external dependency is `gopkg.in/yaml.v3`, because the corpus is YAML and regex
parsing of structured records is exactly the brittleness this backend exists to replace.

## Running locally

From `backend/`:

```powershell
go version
```

```powershell
go mod tidy
```

```powershell
go vet ./...
```

```powershell
go test ./...
```

```powershell
go build ./...
```

Run the API. With no configuration it discovers the repository root by walking up from the
working directory, so this works from anywhere inside the repo:

```powershell
go run ./cmd/audiomuse-api
```

Point it at a specific repository with an environment variable:

```powershell
$env:AUDIOMUSE_REPO_ROOT = "C:\Users\britb\Documents\audiomuse"; go run ./cmd/audiomuse-api
```

Or with flags, which take precedence over the environment:

```powershell
go run ./cmd/audiomuse-api -repo-root C:\Users\britb\Documents\audiomuse -addr 127.0.0.1:8788
```

### Configuration

| Setting | Flag | Environment | Default |
| --- | --- | --- | --- |
| Repository root | `-repo-root` | `AUDIOMUSE_REPO_ROOT` | discovered upward from the working directory |
| Listen address | `-addr` | `AUDIOMUSE_ADDR` | `127.0.0.1:8788` |

Precedence is flag, then environment, then discovery. The root must contain `nodes/`,
`schemas/node.schema.yaml`, `schemas/relationship-types.yaml` and
`sources/source-registry.yaml`, or startup fails with a clear message. Discovery walks only
from the working directory to the filesystem root and never inspects a sibling tree.

Those four are the discovery markers and are deliberately kept minimal and stable.
`schemas/claim.schema.yaml`, `schemas/source.schema.yaml`, `schemas/experiment.schema.yaml` and
`schemas/experiment-run.schema.yaml` are not markers but are still required: the loader reads their
bounded vocabularies, and an unreadable one is reported as a fatal validation issue rather than as a
bad root. `schemas/vocabulary.schema.yaml` declares no enums and is not read directly; its domain
field explicitly reuses the enum in `schemas/node.schema.yaml`, which the loader reads for startup
record validation and vocabulary-filter validation.

`claims/records/`, `vocabulary/entries/`, `experiments/records/` and `experiment-runs/records/` are
each optional — a corpus predating one of those layers loads and serves it empty. A reference *into*
an absent layer still fails, because a claim that appears in a vocabulary entry which does not exist
is a broken record whether the layer is missing or the entry is.

The default address binds loopback: a knowledge corpus should not become reachable from the
network by accident.

### Startup output

```text
level=INFO msg="AudioMuse API" mode=read-only repository=... nodes=78 sessions=3 sources=51
  claims=48 vocabulary=165 experiments=10 experiment_runs=2 edges=220
  validation=WARN warnings=1 listen=127.0.0.1:8788
```

`/api/v1/project` reports the same counts plus the run tally split by lifecycle state, and
`/api/v1/diagnostics` reports the loaded corpus size so an operator can confirm which layers the
running process actually projected.

`validation` is `PASS`, `WARN` or `FAIL`. A `FAIL` aborts startup and prints every fatal
issue. The absolute repository path appears here, in the operator's terminal, and in no HTTP
response.

## API

Base path `/api/v1`. Every response is JSON.

| Method | Route | Returns |
| --- | --- | --- |
| GET | `/health` | process liveness; does not touch the corpus |
| GET | `/api/v1/project` | corpus summary, counts, domains, statuses, validation status |
| GET | `/api/v1/nodes` | node summaries, filtered, searched and paged |
| GET | `/api/v1/nodes/{id}` | one full node with its derived inbound relationships |
| GET | `/api/v1/sessions` | session summaries with their derived node contribution lists |
| GET | `/api/v1/sessions/{id}` | one session |
| GET | `/api/v1/sources` | registry entries with their evidential and topical citation counts |
| GET | `/api/v1/sources/{id}` | one registry entry with everything that cites it |
| GET | `/api/v1/claims` | claim summaries carrying all four provenance axes |
| GET | `/api/v1/claims/{id}` | one full claim with its evidence context |
| GET | `/api/v1/vocabulary` | vocabulary entries, filtered, searched and paged |
| GET | `/api/v1/vocabulary/{id}` | one entry with the experiments and claims that refer to it |
| GET | `/api/v1/experiments` | experiment definitions with their derived run tally |
| GET | `/api/v1/experiments/{id}` | one definition with the tally and IDs of its runs |
| GET | `/api/v1/experiment-runs` | run records with their lifecycle state and evidence counts |
| GET | `/api/v1/experiment-runs/{id}` | one full run |
| GET | `/api/v1/search` | one lexical query across every searchable canonical layer |
| GET | `/api/v1/graph` | the full read-only graph projection |
| GET | `/api/v1/graph/entities/{entity_type}/{id}/relationships` | the direct relationships of one graph entity |
| GET | `/api/v1/graph/entities/{entity_type}/{id}/traverse` | the bounded neighbourhood of one graph entity |
| GET | `/api/v1/diagnostics` | sanitized validation warnings |

### Node query parameters

| Parameter | Meaning |
| --- | --- |
| `q` | lexical substring search, case-insensitive, over id, title, domain, status, definition and core_concepts |
| `domain` | exact canonical domain |
| `status` | exact canonical status |
| `session` | exact registered session ID appearing in `session_origin` |
| `limit` | page size; default 50, clamped to 200 |
| `offset` | page start; default 0 |

`/api/v1/sessions` accepts `q`, `limit` and `offset`.

Every canonical filter on every endpoint matches exactly and case-sensitively, following the
canonical identity semantics in `docs/knowledge-model.md`. Only `q` is tolerant. An unrecognised
query parameter is refused with `400 invalid_query` rather than ignored, so a caller is never handed
a result set that silently dropped their filter.

### Source query parameters

| Parameter | Meaning |
| --- | --- |
| `q` | lexical substring search, case-insensitive, over id, title and author |
| `type` | exact registry type from `schemas/source.schema.yaml` |
| `relationship` | exact registry relationship from `schemas/source.schema.yaml` |
| `evidence_class` | exact evidence class; matches only sources that declare one |
| `retrieval` | exact retrieval status; matches only sources that declare one |
| `claim_id` | sources that claim cites, in either `evidence` or `attribution` |
| `node_id` | sources that node names in its canonical `sources:` list (topical) |
| `session_id` | sources cited by a claim that appears in that session (claim-mediated) |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

### Claim query parameters

| Parameter | Meaning |
| --- | --- |
| `q` | lexical substring search, case-insensitive, over id and statement |
| `claim_type` | exact type from `schemas/claim.schema.yaml` |
| `confidence` | exact confidence level from `schemas/claim.schema.yaml` |
| `dispute_status` | exact dispute status from `schemas/claim.schema.yaml` |
| `temporal_precision` | exact temporal precision from `schemas/claim.schema.yaml` |
| `relation` | claims carrying at least one evidence entry with that relation |
| `source_id` | claims citing that source, in either `evidence` or `attribution` |
| `node_id` | claims whose `appears_in` names that node |
| `session_id` | claims whose `appears_in` names that session |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

Multiple filters compose with AND. Every list is returned in canonical ID order; search never
reorders by relevance, so two requests against an unchanged corpus return byte-identical bodies.

A bounded filter value outside its canonical vocabulary is refused with `400 invalid_query` and the
response names the accepted values. An identifier filter such as `node_id` is not checked against
existence: an unknown ID means "no record stands in that relation", which is a legitimate empty
result rather than an error. `GET /api/v1/project` publishes both vocabularies under `vocabulary`,
so a client never has to discover them by trial and error.

### Vocabulary query parameters

| Parameter | Meaning |
| --- | --- |
| `q` | lexical substring search, case-insensitive, over id, term, domain, definition, digital_relationship, best_use, technologies and tags |
| `domain` | exact canonical domain, from the same enum nodes use |
| `node_id` | entries whose `node_refs` names that node |
| `session_id` | entries whose `session_refs` names that registered session |
| `tag` | exact canonical tag |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

### Experiment query parameters

| Parameter | Meaning |
| --- | --- |
| `q` | lexical substring search, case-insensitive, over id, title, status, type, difficulty and purpose |
| `status` | exact status from `schemas/experiment.schema.yaml` |
| `type` | exact type from `schemas/experiment.schema.yaml` |
| `difficulty` | exact difficulty from `schemas/experiment.schema.yaml` |
| `node_id` | definitions whose `node_refs` names that node |
| `vocabulary_id` | definitions whose `vocabulary_refs` names that entry |
| `session_id` | definitions whose `session_refs` names that registered session |
| `source_id` | definitions whose `source_refs` names that registered source |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

### Experiment run query parameters

| Parameter | Meaning |
| --- | --- |
| `experiment_id` | runs of that definition |
| `status` | exact lifecycle state from `schemas/experiment-run.schema.yaml` |
| `performed` | exactly `true` or `false`; the planned/executed split derived from status |
| `source_id` | runs whose `source_refs` names that registered source |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

There is no `q` on `/api/v1/experiment-runs`. A run has no authored prose identity, and searching
its observation statements would make a text hit mean "this run observed that" — an evidence
assertion a list projection has no business making. Runs are addressed by definition and state.

### Practice-layer representation

Four canonical distinctions are preserved in the types and visible in the JSON:

**Vocabulary cross-references are not graph edges.** `related_terms` is curated human navigation,
exactly as `vocabulary/README.md` states: it does not imply equivalence, does not create an edge,
and does not affect node degree. It is served as a plain ID list on the entry, never merged into
`node_refs`, and never projected into `GET /api/v1/graph`. `GET /api/v1/vocabulary/{id}` also
serves `experiment_ids` and `claim_ids`; those are reverse reads of references authored on the
experiment and claim records, and they are edges no more than `related_terms` is.

The same holds for the traversal routes, and it is the rule that keeps the two layers apart. The
backend resolves claim `appears_in: vocabulary` and `derived_from: experiment_run` references, and
an unresolvable one is fatal — but resolving a reference is not the same act as building an edge.
The traversal graph addresses four record classes, session, node, claim and source, and no
vocabulary entry, experiment or experiment run is ever a traversal entity, an edge endpoint, or a
filterable relationship name. Whether any practice reference should become a typed graph relation
is a graph-contract question, and reading the practice layer does not answer it.

**An experiment definition is not evidence of execution.** A definition's `observations` and
`measurements` are prose instructions about what a performer should record. A run's are typed
objects. The two never share a shape, so no client can mistake one for the other. The `runs` tally
and `run_ids` on a definition are derived from canonical run records at startup; they are never
written back, and `experiments/index.md` is not their source.

**A planned run is not a completed run.** The `runs` tally reports every lifecycle state
separately — `planned`, `completed`, `incomplete`, `invalid` — and never folds them into a single
"done" count. Each run also carries a derived `performed` boolean and, for a planned run, a
`run_date` of `null`, because a run that has not happened has no date.

**An observation is not a measurement.** An observation carries a statement and the context it was
noticed in, and nothing else — it cannot carry a value, because nothing was measured. A
measurement carries quantity, value, unit, method, tool, calibration, uncertainty and limitations,
and every one of them is served. Numeric values are served as the token the record was authored
with, so a measurement recorded as `72.50` is served as `72.50` and not as `72.5`: the trailing
figure is a precision claim, and re-encoding it would quietly weaken recorded evidence.

An experiment run is not a claim. The two layers connect only where a claim record explicitly says
so, through `derived_from: {kind: experiment_run}`, and the backend never synthesises that link.

### Provenance representation

The claim projection keeps the four axes `docs/claim-provenance-model.md` defines separate and never
collapses them into a score or a boolean. `claim_type` says what kind of statement it is,
`confidence` grades the repository evidence, `evidence[]` names which registered sources support,
contradict or qualify it, and `dispute_status` says whether registered sources conflict. The required
`confidence_basis` is served with the level, because a level without its stated reason is exactly the
flattening the claim layer exists to prevent. `attribution[]`, `derived_from[]` and `appears_in[]`
are served as authored. The `source_ids`, `node_ids` and `session_ids` fields on a claim detail are
convenience projections of those arrays, never a replacement for them.

On a source detail, `claims` is evidential — the claims citing it, each with its relation — while
`node_ids` is topical, the nodes whose `sources:` list names it. They are different relations and are
served under different names.

### Graph traversal

`/api/v1/graph` serves the node-to-node projection. The two entity routes above serve the
knowledge and evidence layers as one bounded graph, so a caller can move from a session to a
concept to a checkable statement to the source that stands behind it without reassembling four
projections by hand. The practice layer is served alongside it and is not part of it; see
"Practice-layer representation" above.

**Entities.** Four addressable classes, and identity is the pair `(type, id)`:

| Type | Canonical record |
| --- | --- |
| `session` | a registry entry of `type: session`, plus `sessions/<id>/` presence |
| `node` | `nodes/<domain>/<id>.md` |
| `claim` | one record in `claims/records/*.yaml` |
| `source` | one entry in `sources/source-registry.yaml` |

The classes stay distinct. A node is a concept, a claim is one checkable statement about
concepts, and a source is where the evidence lives; flattening them into generic graph nodes
would erase exactly the distinctions the knowledge and provenance models exist to make.
Vocabulary entries, experiments and experiment runs are canonical layers the backend does not
parse, so they are not addressable and no edge points at them.

A registered session is also a registry entry, so `session/session-01-what-is-sound` and
`source/session-01-what-is-sound` are two projections of one canonical record and are
addressed separately. No edge is emitted between them: they are the same record seen twice,
not two related things.

**Relationships.** Every edge is read from one canonical field, named in the edge's `origin`.
Nothing is inferred from shared keywords, similar titles, prose overlap or any similarity
measure. Each authored edge is emitted with its documented reverse, and a reverse edge is
marked `"derived": true` so it can never be mistaken for something a record declared.

| Canonical field | Forward edge | Reverse edge |
| --- | --- | --- |
| `node.relationships` | `node --<type>--> node` | the type's own `inverse` from `schemas/relationship-types.yaml` |
| `node.session_origin` | `node --originates_in--> session` | `session --contributed_to--> node` |
| `node.sources` | `node --sourced_from--> source` | `source --source_for--> node` |
| `claim.evidence` | `claim --supported_by\|contradicted_by\|qualified_by--> source` | `source --supports\|contradicts\|qualifies--> claim` |
| `claim.attribution` | `claim --attributed_to--> source` | `source --attribution_for--> claim` |
| `claim.appears_in` | `claim --appears_in--> node\|session` | `--appearance_site_of--> claim` |
| `claim.derived_from` | `claim --derived_from--> claim\|node` | `--basis_for--> claim` |

The evidence relation is carried through rather than flattened to a generic evidence edge: a
source that contradicts a claim must not look like one that supports it. Topical and
evidential source relations keep the separate names `sourced_from` and `supported_by` for the
same reason.

There is no direct source-to-session edge. `GET /api/v1/sources?session_id=` answers that
question through claims, and a traversal reaches it by actually walking the two hops, which
is what keeps `depth` meaning hops.

**An example path.** From a session outward to the evidence behind a concept it introduced:

```text
session  --contributed_to-->      node
node     --appearance_site_of--> claim
claim    --qualified_by-->       source
```

**Query parameters.** Both routes accept `relationship` and `target_type`; `traverse` also
accepts `depth`.

| Parameter | Meaning |
| --- | --- |
| `depth` | hops from the root. Default 1, minimum 1, maximum 3. `traverse` only |
| `relationship` | follow only edges with this relationship name |
| `target_type` | follow only edges pointing at this entity class |

Filters are applied while expanding, not to the finished result, so a filtered traversal is
the traversal of the filtered subgraph and `depth` still counts hops along matching edges.

**Bounds.** Traversal is breadth-first, so `distance` on each entity is its shortest hop
distance from the root. Cycles are normal — every authored edge has a reverse, so any pair of
related nodes is already a two-cycle — and an entity is expanded exactly once, at its shortest
distance. Beyond `depth`, one request is capped at 500 entities and 2000 relationships. Those
are service constants, not configuration: they are API safety invariants rather than
deployment choices. When a cap truncates a result the response says so with `"partial": true`
and a `truncation_reason` of `entity_limit_reached` or `edge_limit_reached`. Nothing is ever
dropped silently.

Ordering is fixed. Entities sort by distance, then entity class in model order
(session, node, claim, source), then ID; relationships sort by source entity, then
relationship name, then target class and ID. Two identical requests against an unchanged
corpus return byte-identical bodies.

**Errors.** An entity class outside the four is `400 invalid_query`, as is a depth outside
`1..3`, a non-integer depth, or a filter value outside the closed vocabulary. An ID that
resolves to no record is `404 entity_not_found`. An entity that exists but has no
relationships is `200` with empty lists — "this record has no edges" and "this record does not
exist" are different facts and are answered differently. A relationship name that is part of
the model but unused by the current corpus is a legitimate empty result, not an error.

This is a bounded read model, not a graph database. There is no query language, no mutation,
no persistence and no caller-supplied traversal program; the deliberate non-goals are listed
under Known limitations.

### Cross-layer search

`GET /api/v1/search` answers the question a reader has *before* they know which layer holds the
answer. Every other route requires the caller to pick a record class first; this one searches all
six searchable classes at once and reports which class each hit came from.

| Parameter | Meaning |
| --- | --- |
| `q` | **required.** Lexical substring search, case-insensitive, over the fields listed below |
| `type` | optional; restrict results to exactly one searchable class |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

`q` must be non-empty after trimming. An absent, empty or whitespace-only `q` is refused with
`400 invalid_query` rather than returning everything: each layer already has its own list endpoint,
and an empty search would be a second, slower whole-corpus dump that a caller who mistyped a
parameter name could not tell from a successful query. An unknown or duplicated query parameter is
refused exactly as it is on every other route.

#### Searchable classes and fields

Absence of a result means the text is not in one of these fields. It does not mean the corpus does
not know the thing, so the field set is documented rather than left implicit:

| Entity | Searchable fields | Display title | Summary |
| --- | --- | --- | --- |
| `session` | `id`, `title` | `title` | none |
| `node` | `id`, `title`, `domain`, `status`, `definition`, `core_concepts` | `title` | `definition` |
| `claim` | `id`, `statement` | `statement` | none |
| `source` | `id`, `title`, `author` | `title` | none |
| `vocabulary` | `id`, `term`, `domain`, `definition`, `digital_relationship`, `best_use`, `technologies`, `tags` | `term` | `definition` |
| `experiment` | `id`, `title`, `status`, `type`, `difficulty`, `purpose` | `title` | `purpose` |

These are the Phase 1A to 1D field sets unchanged. The backend defines them once, and each layer's own
`q` parameter reads the same definition, so a term that finds a record through `/api/v1/nodes` finds
it through `/api/v1/search` too. Each exclusion is the earlier layer's: source `notes` is prose about
retrieval and external locators; the cross-reference lists on a node, an entry or a definition are
another record's identity, not this one's; an experiment's `procedure` and `setup` describe what a
performer should do, so a hit there would return a definition that *mentions* a term in an
instruction rather than one that is *about* it. A class with no natural summary field returns a
smaller result rather than a fabricated one, and no summary is ever synthesised.

#### Experiment runs are deliberately absent

There is no `experiment_run` search class, and `?type=experiment_run` is refused with
`400 invalid_query` rather than answered with an empty set, so a caller can never read "no results"
as "no run mentions this".

A run's prose is its observations, measurements and interpretation. Those are evidence-bearing
records, and a generic free-text hit inside them would quietly mean "this run observed that" — an
evidence assertion a discovery surface has no standing to make. Runs stay addressable through
`/api/v1/experiment-runs`, where the caller states which definition and which lifecycle state they
are asking about, and through `/api/v1/experiment-runs/{id}` by exact ID.

#### Result shape

```json
{
  "entity_type": "vocabulary",
  "id": "resonance",
  "title": "Resonance",
  "summary": "...",
  "match_kind": "title_exact",
  "matched_fields": ["term", "definition"]
}
```

`matched_fields` names every canonical field the query actually matched, in the record's own field
order. It is the evidence for the hit: a client can answer "why did this appear" without a second
request, which matters more here than anywhere else in the API because a cross-layer result set is
the one place a reader cannot see the surrounding record. Nothing is highlighted, no source text is
rewritten, and no semantic category is inferred.

A result carries only its own record's fields. A claim hit never presents its source's title, an
experiment hit never presents its runs, and a vocabulary hit never presents the node it
cross-references. **A unified search surface is not a unified ontology** — it is one entry point
into six models that stay distinct. To follow a hit into its context, read the record itself:

| `entity_type` | Route |
| --- | --- |
| `session` | `/api/v1/sessions/{id}` |
| `node` | `/api/v1/nodes/{id}` |
| `claim` | `/api/v1/claims/{id}` |
| `source` | `/api/v1/sources/{id}` |
| `vocabulary` | `/api/v1/vocabulary/{id}` |
| `experiment` | `/api/v1/experiments/{id}` |

#### Ordering

Results are ordered by four mutually exclusive **categorical match classes**, then by the canonical
class order above, then by canonical ID:

| `match_kind` | Meaning |
| --- | --- |
| `id_exact` | the query is exactly the record's canonical ID |
| `title_exact` | the query is exactly the record's display field |
| `title_substring` | the query appears inside the record's display field |
| `field_substring` | the query appears only in some other searchable field |

This is **not a relevance score**, and it is deliberately not rendered as a number. There is no
weighting, no field boosting, no term frequency and no ranking model; it is a fixed hand-written
precedence, and calling it anything else would dress a priority list as information retrieval. The
class order and canonical ID break every tie, so the ordering is total: the same corpus and the same
query always return byte-identical results. Two record classes may share an ID — a session and its
registry entry do — so a hit is identified by `entity_type` **and** `id`, never by `id` alone.

#### Lexical search, not semantic retrieval

Matching is case-insensitive substring matching and nothing else. There is no stemming, no fuzzy or
Levenshtein matching, no BM25 or TF-IDF, no synonyms, no query rewriting, no embeddings and no
vector similarity. This is the point of the phase rather than a shortfall of it: deterministic
retrieval can be pinned by tests, so the contract is fixed *before* anything smarter is built on top
of it. Semantic retrieval is a later phase with its own contract.

The query is treated as plain text throughout. It is never compiled as a regular expression,
expanded as a glob, joined to a filesystem path, or handed to any interpreter, and no query is
stored: there is no search history, no query log and no analytics. A request exists only for the
life of that request.

A search that matches nothing is `200` with an empty `results` array. "The corpus contains no such
text" is an answer, not a `404`.

### Errors

```json
{ "error": { "code": "node_not_found", "message": "Node was not found." } }
```

Codes: `not_found`, `node_not_found`, `session_not_found`, `source_not_found`, `claim_not_found`,
`entity_not_found`, `vocabulary_not_found`, `experiment_not_found`, `experiment_run_not_found`,
`invalid_query`, `method_not_allowed`, `internal_error`. Go errors, stack traces and filesystem paths are
logged locally and never serialised into a response.

### Read-only enforcement

Only `GET` and `HEAD` are accepted, anywhere. Every other method returns `405` with
`Allow: GET, HEAD`, refused by middleware that runs before routing:

```powershell
Invoke-WebRequest -Method POST http://127.0.0.1:8788/api/v1/nodes
```

## Smoke test

```powershell
Invoke-RestMethod http://127.0.0.1:8788/health
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/project
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/nodes?q=sound&limit=5"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/nodes/sound
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/sessions
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/graph
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&type=vocabulary"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&limit=5&offset=5"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/diagnostics
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/sources?evidence_class=institutional_archive"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/sources/purves-neuroscience-auditory-system
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/claims?dispute_status=disputed"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/claims/screwed-up-records-1996-store-claim
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/claims?source_id=tsha-dj-screw&relation=contradicted_by"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/graph/entities/node/amplitude-envelope/relationships
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/graph/entities/session/session-01-what-is-sound/traverse?depth=2"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/graph/entities/node/amplitude-envelope/traverse?depth=2&target_type=claim"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/vocabulary?q=resonance"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/vocabulary?domain=psychoacoustics&limit=5"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/vocabulary/frequency
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/experiments?type=hybrid"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/experiments/near-frequency-beating
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/experiment-runs?experiment_id=near-frequency-beating"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/experiment-runs?performed=false"
```

## Validation

Startup separates two different failures. Fatal issues abort the process because the
projection would be wrong or ambiguous: malformed front matter, unparseable YAML, missing or
invalid ID, duplicate canonical ID, missing or unknown top-level field, unresolved
relationship target, relationship type outside the canonical vocabulary, self-link,
duplicate `(type, target)` pair, unresolved `session_origin` or `sources` reference, unsafe
path.

The evidence layer adds, at the same severity: a duplicate, blank or non-canonical claim ID; a claim
record or nested evidence, attribution, derivation or appearance object whose key set does not equal
the contract's; an empty required claim field; any value outside a bounded claim or source
vocabulary, including case drift; an unresolved evidence or attribution source; an unresolved
`appears_in` or `derived_from` node, session or claim reference; a duplicate evidence, attribution
or reference entry; a claim with no appearance site; a claim derivation cycle; an appearance
document that is an unsafe path, an external locator, or a generated projection under `indexes/`;
and an unreadable or vocabulary-less claim or source contract.

The traversal layer also requires every node relationship type and inverse label to be non-empty
canonical `snake_case`, distinct from itself, and unique across the complete forward/inverse label
namespace. A missing, malformed, duplicate, or colliding inverse is fatal because reverse traversal
could otherwise omit an edge, merge two predicates, or lose the authored-versus-derived marker.

The practice layer adds, again at the same severity: a duplicate, blank or non-canonical
vocabulary, experiment or run ID; a vocabulary term reused in any casing; a record or nested
control-setting, observation or measurement object whose key set does not equal the contract's; an
empty required field or an empty value inside any list; a value repeated within one list; a
vocabulary entry or experiment that references itself; an experiment status, type or difficulty, a
run status, or a measurement calibration outside its schema enum, including case drift; an
unresolved `node_refs`, `session_refs`, `source_refs`, `vocabulary_refs`, `related_terms`,
`related_experiments` or run `experiment_id`; a `run_date` that is not ISO `YYYY-MM-DD`; a
measurement value that is not a JSON number; an experiment or run file holding more than one
record; and an unreadable or enum-less experiment or experiment-run contract.

Phase 1D also upgrades two Phase 1B checks. Claim `appears_in: vocabulary` and
`derived_from: experiment_run` references were shape-checked and carried through unresolved,
because the backend did not read those layers. It reads them now, so both resolve, and a reference
that names nothing is fatal.

The run lifecycle rules are enforced at load and are the one place the backend deliberately
re-implements canonical semantics: a planned run may not carry a date, evidence, interpretation or
procedure deviations; a performed run must carry a date; a completed run must carry at least one
observation or measurement; an invalid run may not carry interpretation. The reason is the general
rule, not an exception to it — the API serves a derived `performed` flag and derived per-status run
counts, so a record that claimed `planned` while carrying measurements would make those derived
values assert evidence the repository says does not exist.

Warnings are served on `/api/v1/diagnostics` and do not stop startup: a registered locator that does
not exist, a registered session with no directory, a session no node cites, a registered source that
neither a node nor a claim cites, and a claim appearance document that is safe and canonical but has
not been written yet.

The diagnostics response labels this result as `validation_scope: runtime_projection` and labels
full repository semantic validation as an `external_precondition`. A running server therefore does
not imply that the PowerShell semantic validator was run by the process itself.

The semantic rules in `schemas/claim.schema.yaml` — what confidence a claim may carry given its
evidence, when an attribution is required, how dispute status must match the cited relations — stay
with `tools/validate-claims.ps1`, which is their canonical authority and gates every commit. The
backend checks what its own projection depends on and does not become a second, drifting copy.

The same boundary governs the practice layer. Vocabulary `domain` is declared by reference in
`schemas/vocabulary.schema.yaml` — "reuses a domain from `schemas/node.schema.yaml`" — so the
backend reads that node-domain enum for startup validation and exact-match filter validation, just
as `tools/validate-vocabulary.ps1` does. Calendar validity and the "a run cannot be recorded before it is
performed" rule stay with `tools/validate-experiment-runs.ps1`; the future-date bound in particular
depends on the wall clock, and a projection whose validity changed with the time of day would not
be the deterministic one this service promises. Generated indexes under `vocabulary/`,
`experiments/`, `experiment-runs/` and `indexes/` are never read: they are rebuildable views, and
reading them would destroy their value as an independent cross-check.

The backend never repairs a record and never writes to the corpus. Canonical
inconsistencies are reported for a human to decide about.

## Tests

`go test ./...` covers the front-matter parser, the claim and vocabulary record stream parsers, the
single-record experiment and run parsers, the filesystem adapter and every validation defect
including the whole run lifecycle contract, the service index, filtering, search, paging, the graph
projection and every evidence and practice reverse index, the traversal adjacency, depth semantics,
cycle termination, deduplication and truncation bounds, the cross-layer discovery projection, and
the HTTP routes including 404, 400 and 405 behaviour. Determinism is tested directly: the loader and the index are each built twice from an
unchanged corpus and the results compared. Unit tests run against `testdata/corpus/`, a
small synthetic fixture, so a canonical content change cannot silently move a unit-test
expectation.

Six tests run against the real repository on purpose: one asserts it loads with no fatal issues,
one asserts the evidence layer parses and resolves, one asserts the practice layer does, one walks
every canonical entity as a traversal root at maximum depth and asserts no practice record appears
anywhere in the result, and two snapshot the size, modification time and content digest of every
canonical file — one across a load, one across a full index build plus one request to every read
surface, including three searches, and a rejected request on each mutating method. All six skip if the canonical repository
is not found above the working directory.

Search is covered at both layers: class coverage, case insensitivity, exact-ID and match-class
precedence, matched-field correctness, class filtering, empty and rejected queries, paging and
clamped bounds, defensive copying, and determinism across two indexes built from one corpus. Two
assertions are guardrails rather than feature tests. One searches a run's own observation text and
requires it to match nothing, so a future change that starts indexing observations fails loudly.
The other cross-checks every layer's own `q` against `/api/v1/search` for the same term and requires
the same records, which is what keeps the two from drifting apart. Adversarial queries — regex,
glob, path, URL, SQL, shell and template shapes, Unicode, and oversized input — are asserted to be
treated as literal text.

A cross-phase suite covers the combined backend specifically: that every phase's routes coexist on
one router without shadowing, that mutation and duplicate-parameter rejection hold on all of them,
that the practice layer stays out of the traversal graph, that a record just found through search is
still refused as a traversal root and still absent from the graph, and that the shared index is
deterministic and hands out defensive copies across every layer at once.

## Known limitations

- Repository changes require a process restart. There is no watcher, no background sync and
  no filesystem polling, so a running process always serves one consistent snapshot.
- Search is lexical substring matching. `/api/v1/search` orders by a fixed categorical match
  precedence, not by a relevance score; the per-layer `q` parameters do not reorder at all. There
  is no stemming, no fuzzy matching, no semantic retrieval and no embedding.
- Search matches only the fields documented under "Cross-layer search". A record whose relevant
  text lives in an excluded field — source `notes`, an experiment's `procedure`, a node's markdown
  body — is not discoverable by that text.
- `q` accepts a single term and is matched literally. There is no phrase, boolean, wildcard or
  field-scoped query syntax, and `type` accepts one class rather than a set.
- Experiment runs are not searchable at all, by design. See "Experiment runs are deliberately
  absent"; they remain addressable through their own structured routes.
- No query is stored. There is no search history, no query log, no analytics and no
  personalisation, so nothing improves with use.
- No persistence and no database; the startup index is the only state.
- Node `experiments:` is the one canonical reference field still carried unresolved. No
  repository validator treats it as a reference list and every current node leaves it empty, so
  resolving it would be the backend inventing a contract rather than reading one.
- Vocabulary entries, experiments and experiment runs are read surfaces adjacent to the graph,
  not part of it. None of them becomes a vertex, an edge, a traversal entity, an edge endpoint or
  a filterable relationship name; `GET /api/v1/graph` and both traversal routes are unchanged by
  the practice layer being loaded. Claim `appears_in: vocabulary` and `derived_from:
  experiment_run` references do resolve, and resolving them is deliberately not the same as
  making them traversable — that would be a separate graph-contract decision.
- `appears_in: session` is a canonical reference kind no current claim record uses, so
  `?session_id=` on either evidence endpoint answers correctly and returns nothing against
  today's corpus.
- Graph traversal is bounded on purpose. There is no query language, no caller-supplied
  traversal program, and no `POST` traversal body; depth is capped at 3 and one request is
  capped at 500 entities and 2000 relationships. Anything beyond that is a repository query
  a client composes from several bounded requests.
- Traversal has no paging. A truncated result reports `partial` and its reason instead, so a
  caller narrows with `depth`, `relationship` or `target_type` rather than walking pages
  through a graph whose shape they cannot yet see.
- The graph is derived, never stored. There is no graph database, no persisted adjacency and
  no edge mutation of any kind.
- No frontend, no graph visualization, and no LLM integration.

## Future work

Deferred, not implemented: graph traversal across the practice layer, richer diagnostics, query
syntax beyond a single literal term, graph visualization, semantic retrieval, and MLLM
experimentation.

Semantic retrieval is deferred deliberately. The deterministic lexical surface exists first so that
the discovery contract — what is searchable, what a hit means, and what order results arrive in —
is fixed and test-covered before anything harder to reason about is layered on top of it.

Traversal over the practice layer is deferred deliberately, not incidentally. Vocabulary entries,
experiments and experiment runs are read surfaces adjacent to the graph, and their cross-references
are not canonical graph relationships; promoting them to traversal edges would assert a claim about
the corpus that the corpus does not make. See "Practice-layer representation" above.
