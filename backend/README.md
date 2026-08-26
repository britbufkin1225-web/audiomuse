# AudioMuse Backend — Read-Only Knowledge API

A deterministic read-only HTTP projection of the canonical AudioMuse repository: nodes, sessions,
the typed relationship graph, the sources, claims and provenance that stand behind them, the
vocabulary, experiments and experiment runs that put them into practice, and one lexical search
surface spanning all of them that can compose several terms into one request, rank what it finds by
an explicit and reproducible relevance policy, explain why each result matched and where it ranked,
and resolve the canonical context around it — and, from any one of those records, a bounded ranked
list of what to read next, each entry naming the canonical field that connected it.

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
│   ├── service/                immutable startup index, filtering, graph, evidence, traversal, practice, cross-layer discovery and related-knowledge projections
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
| GET | `/api/v1/search` | one lexical query across every searchable canonical layer, optionally composed from several terms and with the bounded canonical context of each hit |
| GET | `/api/v1/related/{entity_type}/{id}` | the bounded, ranked, explained set of canonical records related to one record |
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

Multiple filters compose with AND. Every list is returned in canonical ID order, and the per-layer
`q` filter never reorders it: relevance ranking exists on `/api/v1/search` alone, so two requests
against an unchanged corpus return byte-identical bodies here.

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
| `entity_types` | optional; restrict results to a comma-separated **set** of searchable classes. Not combinable with `type` |
| `query_mode` | optional; exactly `literal` or `all_terms`. How `q` is composed into a query. Default `literal` |
| `include_context` | optional; exactly `true` or `false`. Resolve the canonical context of each returned hit |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

`q` must be non-empty after trimming. An absent, empty or whitespace-only `q` is refused with
`400 invalid_query` rather than returning everything: each layer already has its own list endpoint,
and an empty search would be a second, slower whole-corpus dump that a caller who mistyped a
parameter name could not tell from a successful query. Values longer than the length bound below
are refused rather than truncated. An unknown or duplicated query parameter is refused exactly as
it is on every other route.

#### Empty, malformed and unsatisfiable requests

These are three different things, and the difference between them is the difference between a `400`
and a `200`:

| Request | Response |
| --- | --- |
| `q` absent, empty, or only whitespace | `400 invalid_query` |
| `q` past the length bound | `400 invalid_query`; never truncated to a shorter query |
| `q` containing a NUL byte | `400 invalid_query` |
| `q` containing any other control character | `200`; it is ordinary text, matched literally like any other character |
| any parameter supplied twice | `400 invalid_query` |
| any parameter outside the table above | `400 invalid_query`, listing the accepted parameters |
| a well-formed query the corpus does not answer | `200`, empty `results`, all-zero facets |
| a legal scope or composition nothing satisfies | `200`, empty `results`, all-zero facets |

The length bound is **128 bytes of the trimmed UTF-8 value**, and it is the same bound every other
route applies to its own `q`. For an ASCII query that is 128 characters, which is what the refusal
message says; a query written in a script whose characters take more than one byte reaches the bound
sooner. The bound exists to keep the cost of one request knowable, so it is deliberately a bound on
the bytes the service was handed rather than on how they decompose.

A present but **empty** parameter is not uniformly "absent", and the split is deliberate. `type=` is
absent, exactly as a blank filter is on every other route — including for the purpose of the
`type`/`entity_types` conflict below, so `?q=x&type=&entity_types=node` is a scoped search rather
than a refused combination. `entity_types=`, `query_mode=` and `include_context=` are refused
instead, because for those three a blank value would silently return a differently shaped response
than the caller asked for.

An unsatisfiable search is a complete response, not a truncated one. `?q=nothing-matches-this`:

```json
{
  "query": "nothing-matches-this",
  "page": { "total": 0, "count": 0, "limit": 50, "offset": 0 },
  "facets": {
    "entity_types": {
      "session": 0,
      "node": 0,
      "claim": 0,
      "source": 0,
      "vocabulary": 0,
      "experiment": 0
    }
  },
  "results": []
}
```

A malformed one is the stable error envelope and nothing else. `?q=resonance&entity_types=node,`:

```json
{
  "error": {
    "code": "invalid_query",
    "message": "Parameter entity_types must not contain an empty value."
  }
}
```

Every refusal on this route carries the existing `invalid_query` code; search introduces no error
code and no second error shape of its own. The message states the rule that was broken and never
echoes the caller's own value, which is the one part of a response an attacker controls.

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
it through `/api/v1/search` too. Matching occurs within one canonical scalar or list value; separator
text between fields or list entries is never searchable. Each exclusion is the earlier layer's:
source `notes` is prose about retrieval and external locators; the cross-reference lists on a node,
an entry or a definition are
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

#### Result scope

`entity_types` restricts a search to a **set** of searchable classes, which is the question
`type` cannot ask: "answer this from the evidence layer only" is two classes, not one.

```text
?q=acoustic&entity_types=node,claim
```

| Rule | Behaviour |
| --- | --- |
| omitted | every searchable class, exactly as before this parameter existed |
| one or more classes | results are restricted to those classes |
| all six classes | the same result set, facets and page as omitting the parameter; the response additionally echoes the list |
| unknown class | `400 invalid_query`, listing the accepted classes |
| `experiment_run` | `400 invalid_query`; it is not a searchable class |
| empty member | `400 invalid_query`; a leading, trailing or doubled comma produces one |
| whitespace-only member | `400 invalid_query` |
| repeated class | `400 invalid_query` |
| supplied with `type` | `400 invalid_query`; supply one or the other |

Members are trimmed, so `node, claim` and `node,claim` are the same request, and are then
compared **exactly** against the six canonical class names. `Node`, `NODE`, `nodes`, `vocab` and
`run` are refused rather than folded or guessed at, for the reason `query_mode` refuses `AND`: a
filter that guesses at a spelling can guess wrong and answer a different question. There are no
aliases and no plural forms, because the API has no alias convention to follow and inventing one
here would make this the only place a canonical name is not written as the model spells it.

Nothing malformed is repaired. `?entity_types=node,` is a caller mistake, and answering it with
a working search would hide the mistake behind a correct-looking response — the same reason an
unknown query parameter is refused rather than ignored.

**`type` and `entity_types` are alternatives, not layers.** They are two spellings of one
restriction, and there is no reading of both at once that is not a guess: intersecting them can
produce an empty set that looks like "the corpus holds nothing of that kind", and honouring
either one alone would silently discard a filter the caller believes is applied. `type` is
unchanged and is not deprecated; a single-class search may be written either way, and the two
return the same records. A response echoes only the filter it was given.

The list needs no length bound of its own: the set is closed at six and repetition is refused,
so a valid list cannot be longer than the model.

**Scope filters; it does not search, and it does not rank.** Restricting classes changes nothing
about matching — the same searchable fields, the same case-insensitive substring test, the same
`match_kind`, the same `relevance_score`, the same `match_signals`, the same `matched_fields`, the
same `term_matches` and the same relative order. A score is a function of the query and one record,
so removing other records cannot change it. A scoped result set
is exactly the unscoped one with the other classes removed, which is what makes `entity_types`
safe to add to a request whose results a client has already reasoned about. A record outside the
scope is skipped as the corpus is walked, and everything after that — ordering, facet counting,
paging, context resolution — runs over the filtered set, so `page.total` is the size of the full
**filtered** set and a page boundary can never hide a record the filter kept.

#### Result facets

Every successful search carries a `facets` object describing the class composition of the
complete result set:

```json
{
  "query": "acoustic",
  "entity_types": ["node", "claim"],
  "page": { "total": 10, "count": 10, "limit": 50, "offset": 0 },
  "facets": {
    "entity_types": {
      "session": 0,
      "node": 7,
      "claim": 3,
      "source": 0,
      "vocabulary": 0,
      "experiment": 0
    }
  },
  "results": []
}
```

The `results` array above is abridged; the ten hits it stands for are what the facets count.

| Rule | Behaviour |
| --- | --- |
| coverage | every searchable class, always, including classes with no hits |
| key order | the canonical class order, not alphabetical |
| scope | the complete filtered result set, **before** paging |
| sum | equals `page.total` on every page of one search |
| empty search | the complete structure with every count at `0` |
| `experiment_run` | absent, because it is not a searchable class |

**Counts are of the whole result set, never of the page.** `?limit=1&offset=1` and `?limit=200`
report identical facets for the same query and the same filters, because a breakdown that moved
with the page would describe the one thing the caller can already count for themselves. This is
what makes the object useful before narrowing: a reader who searches a term and is shown 7 nodes
and 3 claims can ask for `entity_types=claim` knowing what it will return.

A zero is an answer, and is why every class is always present: "nothing of this kind matched" and
"this build does not search this kind" are different facts, and an omitted key could not tell
them apart. Under a scope, the classes outside it are reported as `0` for the same reason.

**Facets are counts, not scores.** Nothing here ranks a class, reorders results, or implies that
a class with more hits is a better answer. They are the same deterministic retrieval facts the
result list carries, tallied.

`facets` is additive: it is the one key Phase 1H adds to a search response, and no existing key
was renamed, removed or reshaped to make room for it. `entity_types` is echoed only when the
caller supplied it, in the canonical class order rather than the caller's, so — like `query` —
the response describes the search that ran.

#### Query composition

`query_mode` decides how the text in `q` becomes a query. It changes composition only: both modes
run the same case-insensitive substring test over the same field set, and neither one widens what
is searchable.

| `query_mode` | Meaning |
| --- | --- |
| omitted | Identical to `literal`. This is the default and is what every request before this contract existed already meant |
| `literal` | The whole normalised query is **one contiguous needle**. Whitespace inside `q` is part of the phrase |
| `all_terms` | `q` is split on whitespace and **every term must occur somewhere in the same record** |

Only those two spellings are accepted. `AND`, `and`, `all`, `any`, `ALL_TERMS`, `semantic`, `fuzzy`,
`1` and `true` are refused with `400 invalid_query` rather than guessed at, and a present but empty
`query_mode=` is refused for the same reason: a mode that quietly resolved to something else would
answer a different question than the one asked.

**Whitespace is never silently reinterpreted.** `?q=room resonance` still searches for the phrase
`room resonance`, exactly as it did before `query_mode` existed. A response to a literal request —
omitted mode or explicit — carries no `query_mode` key and no `term_matches` key on any result, so
an existing client's responses are unchanged.

##### `all_terms`

`q` is trimmed, lower-cased and split with whitespace as the only separator. A record matches when
**every** term appears as a substring of at least one of that record's own searchable values.
Terms may land in the same canonical field or in different ones:

```text
node "room-mode"
  title:      Room Mode
  definition: A standing-wave resonance determined by the dimensions of the space.

?q=room resonance                        -> no hit; the phrase is not contiguous anywhere
?q=room resonance&query_mode=all_terms   -> hit; "room" in title, "resonance" in definition
```

What the composition may **not** do:

- It may not span two records. If one record holds `room` and a different record holds
  `resonance`, neither is a hit — joining them would assert a relationship the corpus did not make.
- It may not be satisfied by context. `include_context` resolves *after* matching, for the
  returned page only, so a term carried by a related record never completes a query.
- It may not reach an unsearchable field. The field set under "Searchable classes and fields" is
  unchanged: node markdown bodies, source `notes`, experiment `procedure` and `setup`, and every
  experiment run remain outside discovery in both modes.

**Tokenisation is whitespace and nothing else.** There is no stemming, no accent folding, no
Unicode normalisation pipeline and no punctuation stripping, so punctuation stays part of a term:
`room,` and `room` are two different terms, and only the second finds a record that wrote the bare
word. That is a documented limit of a deterministic composition rather than an oversight — every
heuristic that would smooth it over is an unsourced judgement about language.

**Duplicates collapse.** `?q=resonance resonance room` executes as `resonance room`: the first
occurrence of each term wins, and repetition cannot change the result set, the ordering, the totals
or the evidence.

**Term bounds.**

| Bound | Value |
| --- | --- |
| minimum distinct terms | 2 |
| maximum distinct terms | 8 |
| total `q` length | 128 bytes of the trimmed UTF-8 value, as in every mode |

Both term bounds apply to the *distinct* terms that actually execute. One term under `all_terms` is
refused rather than run: it would be literal search under a second name, and an explicit mode
should mean what its name says. All four bounds are enforced in the service as well as at the HTTP
edge, so a direct `Knowledge.Search` caller inherits the same contract.

##### Composed response

```json
{
  "query": "room resonance",
  "query_mode": "all_terms",
  "page": { "total": 1, "count": 1, "limit": 50, "offset": 0 },
  "facets": {
    "entity_types": {
      "session": 0,
      "node": 1,
      "claim": 0,
      "source": 0,
      "vocabulary": 0,
      "experiment": 0
    }
  },
  "results": [
    {
      "entity_type": "node",
      "id": "room-mode",
      "title": "Room Mode",
      "summary": "A standing-wave resonance determined by the dimensions of the space.",
      "match_kind": "all_terms",
      "relevance_score": 28,
      "match_signals": ["title_terms", "id_substring", "field_match"],
      "matched_fields": ["id", "title", "definition"],
      "term_matches": [
        { "term": "room", "matched_fields": ["id", "title"] },
        { "term": "resonance", "matched_fields": ["definition"] }
      ]
    }
  ]
}
```

`query` echoes the normalised term list joined by single spaces, so the response describes the
query that ran rather than the caller's spacing. `query_mode` is present only for `all_terms`;
`term_matches` is present only on composed results.

The signals above are the composed-mode ones: `room` occurs in the display field and `resonance`
does not, which is partial coverage (`title_terms`, 16); the whole phrase `room resonance` occurs
contiguously nowhere, so there is no `phrase_match`; `room` also occurs inside the canonical ID
(`id_substring`, 8); and the `definition` hit contributes `field_match` (4). `16 + 8 + 4 = 28`.

`term_matches` is the evidence for a composed hit, and it is bounded to exactly what the backend
knows: which of this record's canonical fields each term was found in. Terms follow the normalised
query order, fields follow the canonical field order, and there are no snippets, no offsets, no
highlighting, no occurrence counts and no rewritten prose. Every field named belongs to the hit's
own record — nothing is borrowed from a related one, including when `include_context=true`
resolved that record on the same response. `matched_fields` keeps its Phase 1E meaning and is the
union of the per-term lists in canonical field order.

##### Composed ordering

Composed results are ordered by `relevance_score`, then by the canonical class order, then by
canonical ID. Every hit still carries the one composed class `all_terms` — each returned record
satisfies every term, so no match kind separates them — but the Phase 1I signals do: a record
carrying the caller's words contiguously, or carrying all of them in its display field, is ranked
above one that spreads them across five prose fields. See "Relevance ranking". The same corpus and
the same normalised query always produce the same bytes.

Term **order** is significant to the phrase-derived signals and to nothing else. Which records match
is order-independent — every term must occur somewhere in the record either way — but `fixture term`
and `term fixture` are different phrases, so they can rank the same matched records differently. The
response echoes the normalised term list in the caller's own order, so the phrase that was ranked is
always visible in `query`.

`type`, `entity_types`, `limit` and `offset` compose unchanged, and the order of operations is
fixed: normalise, match, filter by class, order the **complete** match set, count its facets, page
it, and only then resolve context. Paging never precedes matching, so `page.total` is always the
size of the full composed result set rather than of the page, and the facets always describe that
same full set.

#### Result shape

For `q=resonance`:

```json
{
  "entity_type": "vocabulary",
  "id": "acoustic-resonance",
  "title": "Resonance",
  "summary": "...",
  "match_kind": "title_exact",
  "relevance_score": 524,
  "match_signals": ["title_exact", "id_substring", "field_match"],
  "matched_fields": ["id", "term", "definition"]
}
```

The query is exactly this entry's `term`, occurs inside its ID without being all of it, and also
occurs in its `definition`: `512 + 8 + 4 = 524`.

`relevance_score` and `match_signals` are the Phase 1I additions and are always present on every
result of every mode. They are covered in "Relevance ranking" below; in short, the score is the
exact sum of the weights of the signals listed beside it, so a client can recompute it and check it.

`matched_fields` names every canonical field the query actually matched, in the record's own field
order. It is the evidence for the hit: a client can answer "why did this appear" without a second
request, which matters more here than anywhere else in the API because a cross-layer result set is
the one place a reader cannot see the surrounding record. Nothing is highlighted, no source text is
rewritten, and no semantic category is inferred.

A result's own fields carry only its own record. A claim hit never presents its source's title as
its own, an experiment hit never presents its runs, and a vocabulary hit never presents the
definition of the node it cross-references. **A unified search surface is not a unified ontology**
— it is one entry point into six models that stay distinct. To read a record itself rather than
its identity, follow it to its own route:

| `entity_type` | Route |
| --- | --- |
| `session` | `/api/v1/sessions/{id}` |
| `node` | `/api/v1/nodes/{id}` |
| `claim` | `/api/v1/claims/{id}` |
| `source` | `/api/v1/sources/{id}` |
| `vocabulary` | `/api/v1/vocabulary/{id}` |
| `experiment` | `/api/v1/experiments/{id}` |

#### Result context

A search result identifies a record. `include_context=true` additionally resolves, for each hit on
the returned page, the canonical records it directly references and the canonical records that
directly reference it — enough to decide what to open next without one request per candidate.

Context is **absent by default**. A request without `include_context` returns the Phase 1E body
unchanged: no `context` key on any result and no `include_context` key on the response.
`include_context=false` is identical to omitting it, and any other spelling — `1`, `yes`, `TRUE` —
is refused with `400 invalid_query` rather than guessed at.

```json
{
  "entity_type": "claim",
  "id": "beta-was-observed-in-1999",
  "title": "The beta fixture phenomenon is recorded as having been observed during 1999.",
  "match_kind": "id_exact",
  "relevance_score": 1024,
  "match_signals": ["id_exact"],
  "matched_fields": ["id"],
  "context": {
    "related": [
      {
        "relation": "appears_in",
        "entity": { "entity_type": "node", "id": "beta", "label": "Beta" },
        "origin": "claim.appears_in",
        "derived": false
      },
      {
        "relation": "contradicted_by",
        "entity": {
          "entity_type": "source",
          "id": "fixture-reference-work",
          "label": "A Fixture Reference Work"
        },
        "origin": "claim.evidence",
        "derived": false
      }
    ],
    "count": 5,
    "returned": 5,
    "truncated": false
  }
}
```

The `related` array above is abridged to two of this claim's five context items.

| Field | Meaning |
| --- | --- |
| `relation` | the canonical relationship name; see the traversal vocabulary and the reference relations below |
| `entity` | the identity of the other record: its searchable class, its canonical ID, and its display label |
| `origin` | the canonical field the relation was read from |
| `derived` | `false` if the hit's own record authored the reference, `true` if this is the documented reverse read of one |
| `count` | the size of this record's full canonical context |
| `returned` | how much of it this response carries |
| `truncated` | `true` whenever `returned` is less than `count` |

`entity` is an identity, not an embedded record and not a URL. It carries the same display label
the record would carry as a search title, and nothing else; a client navigates by `entity_type`
and `id` through the route table above. Two classes may share an ID — a session and its registry
entry do — so a context ref is never resolved by ID alone, and the two have genuinely different
contexts.

#### What context resolves

Every context item is read from exactly one canonical field of exactly one canonical record.
Nothing is inferred from shared keywords, similar titles, overlapping prose or co-occurrence.
There is no "you may also like", no "conceptually related to" and no similarity of any kind: a
context item is a reference some record actually wrote down, or the documented reverse read of one,
and `origin` says which field it came from.

For `session`, `node`, `claim` and `source`, context **is** the graph traversal adjacency, reused
unchanged — same relation names, same origins, same `derived` flag — so one repository connection
keeps one meaning wherever it is read. Those canonical fields are listed under "Graph traversal".

Vocabulary entries and experiment definitions are not graph entities, so their references are
resolved separately, from the reference fields the graph deliberately does not model:

| Canonical field | Relation | Reverse |
| --- | --- | --- |
| `vocabulary.node_refs` | `references` | `referenced_by` |
| `vocabulary.session_refs` | `references` | `referenced_by` |
| `vocabulary.related_terms` | `related_term` | none |
| `experiment.node_refs` | `references` | `referenced_by` |
| `experiment.vocabulary_refs` | `references` | `referenced_by` |
| `experiment.session_refs` | `references` | `referenced_by` |
| `experiment.source_refs` | `references` | `referenced_by` |
| `experiment.related_experiments` | `related_experiment` | none |
| `claim.appears_in` (kind `vocabulary`) | `appears_in` | `appearance_site_of` |

`references` is one name rather than one per field because these fields are literally reference
lists and `origin` already says which one was read; inventing `describes_node` and
`applies_to_session` would assert relations the corpus does not state. The two symmetric lists emit
no reverse: `vocabulary/README.md` states that related terms are human navigation only and imply
neither equivalence nor a graph edge, and neither contract requires the pairing to be authored on
both sides, so a reverse would turn "A listed B" into "B is related to A".

Resolving a reference is still not the same as making it traversable. A context item is a
navigation identity; a vocabulary entry or experiment definition named in one is not a graph
vertex, is not an edge endpoint, and is still refused as a traversal root. `GET /api/v1/graph` and
both traversal routes are unchanged by context being requested.

#### Context is not traversal, and does not score

Context stops at the hit's own direct references. There is no depth parameter, no expansion and no
caller-supplied walk: it answers "what is this record connected to", once. For a neighbourhood —
several hops, filtered by relationship or target type — use
`/api/v1/graph/entities/{entity_type}/{id}/traverse`, which is shaped for that question.

Context resolves provenance; it never evaluates it. A contradicting source is presented exactly
like a supporting one, under its own canonical relation, and nothing here judges whether a source
is reliable, whether a claim is well supported, or whether one relation matters more than another.

Requesting context **cannot change which results match or in what order**. It is resolved after
matching, ordering and paging, and only for the results being returned. Relationship count is not
a ranking signal: a record does not sort higher for being well connected, and does not appear at
all for being connected to something that matched. Context text is not searchable either — a query
that appears only inside a related record is not a hit on the referring one, which is what keeps
`matched_fields` a complete explanation of every result.

#### Context bounds and ordering

`related` is ordered by relation, then by the canonical class order above, then by canonical ID,
then by `origin` and `derived`. The ordering is total, so the same corpus and the same request
always return byte-identical context.

| Bound | Value |
| --- | --- |
| context items per result | 25 |
| context items per response | 2000 |

Neither is configurable; both are API safety invariants rather than deployment choices, and both
are sized against the real corpus so that only genuine hub records are ever shortened: its widest
single context is 75 references, and the broadest possible page — 200 results — asks for roughly
1,800 in total, of which about a dozen results reach the per-result cap and none reach the
per-response one. The per-response
figure is deliberately the same as the traversal edge cap, so one statement covers the whole API —
no single request serialises more than 2,000 canonical relationships, whichever route asked for
them. The budget is spent in result order, so one heavily referenced hit cannot fill a page and a
full page of well-connected records cannot become an unbounded payload. Truncation keeps the
head of the ordered list, so it is deterministic rather than an arbitrary subset, and a truncated
context always reports its full `count` alongside a smaller `returned` and `truncated: true`. The
rest of a truncated context is read through the record's own route or through traversal.

A record that references nothing and is referenced by nothing returns `"related": []` with
`"count": 0`, not a missing object: "this record has no canonical context" and "context was not
requested" are different facts and are answered differently.

#### Experiment runs stay out of context too

No context item names an experiment run, under any relation, from any origin. An experiment
definition resolves the records it references and stops there; its runs are reached through
`/api/v1/experiment-runs?experiment_id=`. A run is not a searchable class, so there is no context
ref that could address one, and the Phase 1E boundary therefore holds structurally rather than by
convention: context resolves identities, and it must not become the route by which observation and
measurement prose re-enters generic search. A claim's `appears_in: document` reference resolves to
nothing for the related reason that no layer addresses it.

#### Result bounds

Every search response is bounded, and no spelling of `limit` removes the bound:

| `limit` | Effective page size |
| --- | --- |
| omitted | 50 |
| `limit=` (present but blank) | 50 |
| `limit=0` | 50; zero means "use the default", it does not mean "return nothing" |
| `1` to `200` | as supplied |
| above `200` | clamped to 200 |
| negative, fractional, or not a number | `400 invalid_query` |

`offset` follows the same rules minus the ceiling: absent, blank or `0` starts at the beginning,
a negative or non-numeric value is `400 invalid_query`, and an offset past the end of the result
set is a legal empty window — `results` is `[]` while `page.total` and `facets` still describe the
whole set, because they always describe the set rather than the window.

**The bound is applied after ranking, never inside it.** The order of operations is fixed: normalise,
match, restrict to the requested classes, order the complete match set, count its facets, page it,
and only then resolve context. Every page is therefore a contiguous window onto the one ranked result
set — the returned `results` are exactly `[offset:offset+limit]` of the complete ordering — so a page
boundary can never hide a record the query matched, a client walking the pages sees each hit exactly
once, and `page.total` and `facets` are identical on every page of one search.

#### Ordering

Every result set, in both modes, is ordered by:

1. `relevance_score`, descending;
2. the canonical class order above;
3. canonical ID, ascending.

The last key is unique within a class, so the order is **total**: no two distinct results compare
equal, and the same corpus and the same query always return byte-identical results. Two record
classes may share an ID — a session and its registry entry do — so a hit is identified by
`entity_type` **and** `id`, never by `id` alone.

`match_kind` remains the four mutually exclusive **categorical match classes** it has been since
Phase 1E, and it is still not a number:

| `match_kind` | Meaning |
| --- | --- |
| `id_exact` | the query is exactly the record's canonical ID |
| `title_exact` | the query is exactly the record's display field |
| `title_substring` | the query appears inside the record's display field |
| `field_substring` | the query appears only in some other searchable field |
| `all_terms` | every term of a composed query was found in this record |

Since Phase 1I it is **derived from** `match_signals` rather than computed separately, so the coarse
class and the score can never describe one hit differently. For a literal search the score is a
strict refinement of the class precedence above: every class-defining signal outweighs the sum of
every weaker signal, so literal results still emerge in `id_exact`, `title_exact`, `title_substring`,
`field_substring` order, now sorted *within* each class as well.

#### Relevance ranking

A **signal** is one named, yes-or-no fact about how the normalised query reached one record's own
canonical fields, carrying a fixed integer weight. `relevance_score` is the sum of the weights of the
signals that are true, and `match_signals` lists exactly those, in descending weight order:

| Signal | Weight | Fires when |
| --- | --- | --- |
| `id_exact` | 1024 | the query is exactly the record's canonical ID |
| `title_exact` | 512 | the query is exactly the record's display field |
| `title_prefix` | 256 | the display field begins with the query and continues past it |
| `title_substring` | 128 | the query occurs inside the display field, neither at its start nor as all of it |
| `phrase_match` | 64 | *(composed only)* the whole query occurs contiguously in a field other than the display field |
| `title_all_terms` | 32 | *(composed only)* every term occurs in the display field |
| `title_terms` | 16 | *(composed only)* at least one term, but not every term, occurs in the display field |
| `id_substring` | 8 | the query — composed, at least one term — occurs inside the ID without being all of it |
| `field_match` | 4 | the query reached a field that is neither the ID nor the display field |

Three groups are mutually exclusive and never appear together: `{id_exact, id_substring}`,
`{title_exact, title_prefix, title_substring}`, and `{title_all_terms, title_terms}`. No signal is
emitted twice, the list is never empty — a record that matched nothing is not a result — and every
name comes from the closed set above.

**Why powers of two.** Each weight is strictly greater than the sum of every weight below it. That
one property is the whole soundness argument: it makes the integer sum behave exactly as a
lexicographic comparison of the signal list, so a record carrying a stronger signal outranks a
record carrying every weaker signal at once, and no accumulation of weak evidence can overtake one
strong piece. A weight table without that property would be a set of magic numbers whose ordering
nobody could predict from reading it. It is enforced by a test rather than asserted in a comment.

**Why the score is explainable rather than opaque.** Because it is defined as the sum of its
signals, `match_signals` and `relevance_score` are the same fact stated twice and cannot disagree.
A client can add up the weights above and recover the number the backend sent, and can compare two
results' signal lists to see exactly which signal separated them. Nothing else contributes: the
score is a function of the query and that one record, so it cannot change because some other record
also matched, and it is not affected by the scope filter, the page window or the context control.

**What is deliberately absent.** There is no term frequency, no field-length normalisation, no
inverse document frequency, no popularity, no click or usage data, no recency, no user or session
weighting, no editorial boost, and no learned model. Every signal is a statement about the text of
one record and the text of one query, and nothing in ranking reads a clock, a counter, a random
source or another record. `relevance_score` is comparable only within one response: nothing
calibrates it across queries, and a larger number on a different query does not mean a better answer.

The score orders results; it does not filter them. A hit scoring 4 is returned exactly as a hit
scoring 1024 is, in its correct position. There is no relevance threshold, no cutoff and no
"did you mean".

#### Lexical search, not semantic retrieval

Matching is case-insensitive substring matching and nothing else, in both query modes. There is no
stemming, no fuzzy or Levenshtein matching, no BM25 or TF-IDF, no synonyms, no query rewriting, no
embeddings and no vector similarity. This is the point of the contract rather than a shortfall of
it: deterministic retrieval can be pinned by tests, so what is searchable and what a hit means are
fixed *before* anything smarter is built on top of them. Semantic retrieval is a later phase with
its own contract.

`all_terms` does not change that, and the distinction is worth stating plainly:

```text
term A and term B were both found in one canonical record
```

is a deterministic retrieval fact about text. It does **not** mean the backend understands any
relationship between A and B, that the record is *about* both, or that it is a better answer than
a record containing one of them. `all_terms` is a bounded composition of literal tests, not a
query language and not a step towards one: there is no `AND`/`OR`/`NOT` parser, no parentheses, no
quoting, no `+`/`-` operators, no wildcards, no regular expressions and no caller-supplied field
names. A token that looks like an operator is a term like any other, required literally.

The query is treated as plain text throughout. It is never compiled as a regular expression,
expanded as a glob, joined to a filesystem path, or handed to any interpreter, and no query is
stored: there is no search history, no query log and no analytics. A request exists only for the
life of that request.

A search that matches nothing is `200` with an empty `results` array. "The corpus contains no such
text" is an answer, not a `404`.

### Related-knowledge discovery

`GET /api/v1/related/{entity_type}/{id}` answers the question a reader has *while holding a
record*: given this, what else in AudioMuse should I read, and why that. Search requires the caller
to know what to type; traversal requires them to know which graph vertex to walk from. This route
requires neither — it takes one canonical record and returns a short, ranked, explained list of
other canonical records.

It is **derivation-free**. Every item is reached over a reference some canonical record actually
authored, or the documented reverse read of one, and every item says which canonical field
connected it. Nothing is inferred from similar wording, shared keywords, prose overlap,
co-occurrence, embeddings or any model output. See "Why explicit references and not similarity"
below.

**Starting records.** The six searchable classes, addressed by the `(entity_type, id)` pair every
other identity on this API uses:

| Start | Canonical record |
| --- | --- |
| `session` | a registry entry of `type: session` |
| `node` | `nodes/<domain>/<id>.md` |
| `claim` | one record in `claims/records/*.yaml` |
| `source` | one entry in `sources/source-registry.yaml` |
| `vocabulary` | one entry in `vocabulary/entries/*.yaml` |
| `experiment` | one record in `experiments/records/*.yaml` |

This is the **search** class set, not the **graph** class set: vocabulary entries and experiment
definitions are discovery starts and are still not graph vertices, exactly as
"Graph traversal" above states. `experiment_run` is not a start and is refused with
`400 invalid_query` rather than answered with an empty list, for the reason it is not a search
class either — a zero would read as "this run is connected to nothing" rather than "runs are not
a discovery start". Runs stay reachable through their own routes.

**Query parameters.**

| Parameter | Meaning |
| --- | --- |
| `entity_types` | optional; restrict *destinations* to a comma-separated **set** of searchable classes |
| `limit` | optional; how many items to return. Default 25, clamped to 100 |

`entity_types` is the same filter, with the same semantics and the same refusals, as the one on
`/api/v1/search`: members are trimmed and compared exactly, a blank member or a repeated class is
refused rather than repaired, and the response echoes the scope in canonical class order. There is
deliberately no single-class `type` alongside it — `/api/v1/search` carries both spellings only
because `type` predates the list.

There is no `depth`, no `offset` and no `q`, and each absence is the contract rather than an
omission. Text would make this search, which exists. Depth would make it traversal, which exists.
Paging would make a discovery list a cursor over a derived ordering, and a reader deciding where to
go next is not working through a result set — the response reports the exact eligible total
instead, so a caller can see what was left without walking it.

#### Response

`GET /api/v1/related/node/rhythm?limit=2` against the canonical repository:

```json
{
  "start": { "entity_type": "node", "id": "rhythm", "title": "Rhythm" },
  "limit": 2,
  "counts": { "eligible": 35, "returned": 2 },
  "truncated": true,
  "items": [
    {
      "entity_type": "node",
      "id": "beatmatching",
      "title": "Beatmatching",
      "summary": "Aligning the tempo and beat positions of two recordings so that they can play together in time.",
      "reason": {
        "relation": "influences",
        "origin": "node.relationships",
        "derived": false,
        "priority": "conceptual",
        "priority_rank": 0
      },
      "evidence_count": 1,
      "evidence_truncated": false
    },
    {
      "entity_type": "node",
      "id": "rhythmic-entrainment",
      "title": "Rhythmic Entrainment",
      "summary": "The alignment of a listener's internal periodic activity — bodily, motor, or neural — with a periodicity in the music, used both as a proposed emotion-induction mechanism and as a contested technical term in the neuroscience literature.",
      "reason": {
        "relation": "influences",
        "origin": "node.relationships",
        "derived": false,
        "priority": "conceptual",
        "priority_rank": 0
      },
      "evidence_count": 1,
      "evidence_truncated": false
    }
  ]
}
```

`start` echoes the record the backend resolved, so a caller who navigated by ID alone can confirm
they landed where they meant. `counts.eligible` is the exact number of distinct records related to
the start after the destination scope was applied, counted **before** the limit; `truncated` is
never `false` when anything was cut.

An item carries its own record's identity and its own display fields and nothing else. `title` and
`summary` are the same fields the search layer already uses for that class — a related item and a
search hit name one record identically by construction — and a class with no summary field returns
none rather than acquiring an empty one. An item is **not** the record: reading it is a request to
that record's own route.

#### Why each item is there

`reason` is the strongest canonical connection between the start and that item, and it is what
decided the item's position. It is structured rather than prose because prose would have to be
generated, and generated prose is the one kind of explanation this backend cannot check.

| Field | Meaning |
| --- | --- |
| `relation` | the canonical relation name, the same one traversal and search context use |
| `origin` | the canonical field the connection was read from |
| `derived` | `false` if the start authored the reference, `true` if it is another record's reference read backwards |
| `priority` | the precedence class of that canonical field |
| `priority_rank` | that class's integer rank; lower is stronger |

There is no confidence, no similarity, no relevance percentage and no probability anywhere in a
discovery response, and the omission is deliberate: a number there would be read as a measurement,
and nothing in the corpus measures how related two records are.

When two records are connected by more than one canonical field, the item appears **once**. The
strongest connection becomes `reason` and the others are reported in `additional_evidence`, in the
same precedence order, bounded at five connections in total per item including the primary.
`evidence_count` is the true total and `evidence_truncated` says whether the list was cut, so a
shortened explanation is visibly shortened rather than looking like a record with fewer
connections than it has. `additional_evidence` is absent entirely when there is only one
connection, so the common case is not padded with an empty array.

#### Relationship precedence

Precedence is a property of the **canonical field**, never of the records at either end and never
of their text. That is what makes the ordering explainable: the answer to "why is this above that"
is always "because AudioMuse wrote the connection down in this field rather than that one".

| Rank | `priority` | Canonical fields | What those fields assert |
| --- | --- | --- | --- |
| 0 | `conceptual` | `node.relationships` | a typed edge between two concepts — the only connection AudioMuse authors specifically as a knowledge relation |
| 1 | `evidential` | `claim.evidence` | what materially supports, contradicts or qualifies a statement |
| 2 | `attributive` | `claim.attribution` | who a statement is credited to |
| 3 | `assertional` | `claim.appears_in`, `claim.derived_from` | which records a statement is about, or rests on |
| 4 | `contextual` | `node.sources`, `node.session_origin` | a concept's topical provenance and its chronological origin |
| 5 | `referential` | `vocabulary.node_refs`, `vocabulary.session_refs`, `experiment.node_refs`, `experiment.vocabulary_refs`, `experiment.session_refs`, `experiment.source_refs` | a practice record pointing into another layer |
| 6 | `navigational` | `vocabulary.related_terms`, `experiment.related_experiments` | curated human navigation, which those contracts say implies neither equivalence nor a graph edge |

Evidence is kept ahead of attribution because `docs/claim-provenance-model.md` treats "what stands
behind this" and "who says so" as different facts and the first is the one a reader checks. Topical
and evidential source relations keep the separate precedence their separate relation names already
record, so a source that merely bears on a concept never outranks one that supports a statement.

Precedence is keyed on `origin` rather than on `relation` because relation names are not a closed
set — node-to-node edges use the relationship-type IDs from `schemas/relationship-types.yaml` and
their declared inverses, so the vocabulary grows whenever the corpus adds a type. Canonical field
names are closed, and a test walks that closed set and requires every member to be classified, so a
new canonical field cannot reach this contract without a deliberate decision about where it ranks.

#### Ordering

Items are ordered by four keys, in this order:

1. the primary reason's `priority_rank`, ascending — strongest connection first;
2. authored connections before derived ones, because an authored reference is something the start
   itself wrote down;
3. the destination's class in canonical model order — `session`, `node`, `claim`, `source`,
   `vocabulary`, `experiment`;
4. the destination's canonical ID, ascending.

A destination appears exactly once, so the last two keys are unique and the order is **total**: no
two items ever compare equal. Two identical requests against an unchanged corpus therefore return
byte-identical bodies, and two indexes built independently from the same corpus agree. Nothing is
left to Go map iteration or to the order the filesystem returned records in.

Truncation is applied last: a bounded result is always the **front** of the complete ordering, never
a different selection. A scoped result is likewise the unscoped one with other classes removed —
same items, same reasons, same relative order — so a filter never changes the ranking.

#### Bounds

| Bound | Value | What it protects |
| --- | --- | --- |
| default `limit` | 25 | one navigable list rather than an index |
| maximum `limit` | 100 | one heavily referenced record cannot become a corpus dump |
| connections per item | 5 | evidence is a list inside a list, so it needs its own cap |

These are service constants, not configuration: they are API safety invariants rather than
deployment choices, exactly as the traversal and context bounds are. The whole response is bounded
by their product — at most 500 canonical connections, a quarter of the 2,000 the API already states
no single request exceeds.

The Phase 1C traversal depth limits are deliberately **not** reused, because there is no traversal
here to bound. Discovery is one hop by construction: the canonical references of every searchable
record are resolved once at startup by the context layer, and a request is a map lookup, a group, a
sort and a slice. There is no frontier, no visited set, no recursion and no expansion, so a depth
parameter would have nothing to control.

Against the canonical repository today the widest discovery result is 75 items (from
`session/session-01-what-is-sound`) and the widest single item carries 3 connections, so the
ceiling and the evidence cap both have real headroom while the default limit does shorten the
handful of genuine hub records — which is what a default is for.

#### Empty results, and errors

| Request | Answer |
| --- | --- |
| a record that exists and is related to nothing | `200`, `"items": []`, zero counts |
| a scope naming a class the record is not connected to | `200`, `"items": []`, zero counts |
| a start class outside the six | `400 invalid_query`, listing the accepted classes |
| a malformed or repeated `entity_types` member | `400 invalid_query`, stating the rule |
| a negative or non-integer `limit` | `400 invalid_query` |
| a `limit` above the maximum | `200`, clamped, with the applied value echoed |
| an identifier containing `/`, `\`, `..` or a NUL | `400 invalid_query` |
| a well-formed identifier naming no record | `404 related_start_not_found` |
| a malformed request that also names no record | `400 invalid_query` — the request is refused before the lookup |

"This record is related to nothing" and "this record does not exist" are different facts and are
answered differently. A registered source nothing cites is a successful empty discovery, not a
`404`.

The whole request is validated before the start is looked up, so a request that is both malformed
and names a record the corpus does not contain is answered as malformed. A caller told only about
the identifier would fix it, resend, and be refused a second time for a mistake that was already
visible in the request they sent. The start *class* is decided before either, because a request
that does not name a discovery class is not a discovery request. A refusal never echoes the
identifier that was not found, so a `400` cannot become a way to learn whether a record exists.

The traversal routes are the partial precedent rather than the model: an out-of-range `depth` is
already a `400` on a root that does not resolve, but a mistyped `relationship` or `target_type` is
still answered as `404 entity_not_found`, because those vocabularies are checked after the root is
resolved. Discovery does not copy that half.

`limit` follows the paging contract every other route on this API uses — clamped, with the applied
value echoed — rather than the traversal `depth` contract, where an out-of-range value is refused.
The two differ because a silently reduced *depth* would let a caller believe they had seen a whole
neighbourhood, while a clamped *limit* returns the front of the same ordering they asked for and
the response says both what was applied and what was left.

`related_start_not_found` is a distinct code from `entity_not_found`. A discovery start may be a
vocabulary entry or an experiment definition, neither of which is a graph vertex, so reporting a
graph miss would tell a client the lookup failed in a layer it never asked about.

Canonical IDs are ASCII kebab-case by contract, enforced at load by every layer, so a Unicode
identifier is well-formed input naming a record the corpus cannot contain and is answered as a
miss. Record *titles* carry whatever the corpus authored, Unicode included, and are served
verbatim as UTF-8.

#### Why explicit references and not similarity

`docs/backend-architecture.md` states that a projection manufacturing edges from keyword overlap or
embedding proximity would insert unsourced claims into a corpus whose entire discipline is that
claims carry provenance. A projection that ranked *authored* references by their apparent
similarity would do the same thing one step later: it would present a machine's guess about meaning
as though AudioMuse had said it.

The practical case is the same one that justified lexical search first. A deterministic baseline is
testable and a semantic one is not, yet: an explicit-reference ranking produces the same bytes for
the same corpus on every run, which means the contract can be pinned by tests before any embedding,
similarity threshold or retrieval model is introduced on top of it. Every reason a caller receives
here is checkable against the canonical file it names.

Embedding-based or AI-generated discovery is **out of scope** for this phase and is not implemented
anywhere in this backend. Nothing in this route calls a model, computes a vector, or consults
anything outside the canonical repository.

#### Scope of this phase, and what it is not

Implemented: one read-only route, six starting classes, the seven-class precedence table above,
deduplication with bounded evidence, destination-scope filtering, a default and maximum limit, and
an exact eligible count.

Deliberately not implemented, and not planned as part of it:

- multi-hop or transitive discovery — the traversal routes serve that question;
- caller-supplied ranking, weights, or a relationship-priority parameter, which would make the
  ordering a property of the request rather than of the corpus;
- personalisation, reading history, popularity or any per-caller state; no request is stored;
- similarity, embeddings, vectors or generated explanations, as above.

Reasonable future extensions, none of which exist today: exposing the precedence table itself as a
read-only contract endpoint so a client can render the classes without hard-coding them; a
per-class breakdown of the eligible set, in the shape search facets already use; and a bounded
"related to both of these" intersection. Each would be its own phase with its own contract, and
none is a quiet widening of this one.

### Errors

```json
{ "error": { "code": "node_not_found", "message": "Node was not found." } }
```

Codes: `not_found`, `node_not_found`, `session_not_found`, `source_not_found`, `claim_not_found`,
`entity_not_found`, `vocabulary_not_found`, `experiment_not_found`, `experiment_run_not_found`,
`related_start_not_found`, `invalid_query`, `method_not_allowed`, `internal_error`. Go errors, stack traces and filesystem paths are
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
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=room%20resonance&query_mode=all_terms"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=frequency%20pitch&query_mode=all_terms&type=node"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=sampling%20audio&query_mode=all_terms&include_context=true"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&entity_types=node,claim"
```

```powershell
(Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance").facets.entity_types
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&include_context=true"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&type=claim&include_context=true"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/related/node/rhythm
```

```powershell
(Invoke-RestMethod "http://127.0.0.1:8788/api/v1/related/node/rhythm?limit=5").items | Select-Object entity_type, id, title
```

```powershell
(Invoke-RestMethod "http://127.0.0.1:8788/api/v1/related/node/rhythm?limit=5").items.reason
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/related/node/rhythm?entity_types=claim,source"
```

```powershell
Invoke-RestMethod http://127.0.0.1:8788/api/v1/related/session/session-01-what-is-sound
```

```powershell
(Invoke-RestMethod http://127.0.0.1:8788/api/v1/related/session/session-01-what-is-sound).counts
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
cycle termination, deduplication and truncation bounds, the cross-layer discovery projection with
its relevance ranking, match explanations and context resolver, the related-knowledge precedence
table with its deduplication and bounds, and the HTTP routes including 404, 400 and 405 behaviour. Determinism is tested directly: the loader and the index are each built twice from an
unchanged corpus and the results compared. Unit tests run against `testdata/corpus/`, a
small synthetic fixture, so a canonical content change cannot silently move a unit-test
expectation.

Nine tests run against the real repository on purpose: one asserts it loads with no fatal issues,
one asserts the evidence layer parses and resolves, one asserts the practice layer does, one walks
every canonical entity as a traversal root at maximum depth and asserts no practice record appears
anywhere in the result, and two snapshot the size, modification time and content digest of every
canonical file — one across a load, one across a full index build plus one request to every read
surface, including three searches and two context resolutions, and a rejected request on each
mutating method. Three more cover related-knowledge discovery at corpus scale, which the fixture
cannot: one walks every discovery start the repository offers and requires each reason to name a
canonical field the precedence table classifies, each result to stay inside the declared bounds, and
no start to appear in its own list; one requires every one of those starts to answer identically
twice; and a third snapshots the corpus across a discovery request for every start, plus the
refusals and a rejected request on each mutating method. All nine skip if the canonical repository
is not found above the working directory.

Search is covered at both layers: class coverage, case insensitivity, exact-ID and match-class
precedence, matched-field correctness, class filtering, empty and rejected queries, paging and
clamped bounds, defensive copying, and determinism across two indexes built from one corpus.
Query composition has its own suite at both layers. It pins the compatibility half first — that an
omitted mode and an explicit `literal` are the same response, that two words are still a phrase,
and that no literal result gains a `query_mode` or `term_matches` key — and then the composed half:
terms in one field and in separate fields of one record, a missing term rejecting the record, terms
held by two different records refusing to combine, case, whitespace and duplicate normalisation,
both term bounds and the UTF-8 byte ceiling at the service rather than only over HTTP, per-term
evidence in canonical field and query order, class-then-ID ordering across two indexes, `type`,
`limit` and `offset` composition, and defensive copying through the per-term evidence. Three are
guardrails rather than feature tests: one requires a record whose context names a term to still be
refused when its own fields lack that term, so context can never complete a query; one requires
operator-shaped tokens to be required as literal text, so no Boolean syntax can creep in; and one
re-checks that a composed query still cannot reach an experiment run. Two
assertions are guardrails rather than feature tests. One searches a run's own observation text and
requires it to match nothing, so a future change that starts indexing observations fails loudly.
The other cross-checks every layer's own `q` against `/api/v1/search` for the same term and requires
the same records, which is what keeps the two from drifting apart. Adversarial queries — regex,
glob, path, URL, SQL, shell and template shapes, Unicode, and oversized input — are asserted to be
treated as literal text.

Result scope and facets have their own suite at both layers, written as invariants against the
unfiltered search rather than as expected record lists: a scoped result set must be the unscoped
one with other classes removed, in the same order and with the same match evidence. It covers each
class alone, several classes at once, a scope naming all six being identical to no scope at all,
canonical scope ordering regardless of how the caller spelled the list, composition with both query
modes, with `include_context` and with paging, the facet sum equalling `page.total`, facets
unchanged across every page of one search, the complete zero-valued structure on an empty result
set, and defensive copying of the echoed scope, the facets and the results. Every malformed
spelling is asserted separately — empty, leading, trailing and doubled separators, a whitespace-only
member, a duplicate, a duplicate after trimming, a wrong-case or plural class name, and the two
class filters supplied together — as is the refusal of a malformed scope on an otherwise valid
request. Two are guardrails rather than feature tests: one requires `experiment_run` to be refused
in a class list and absent from the accepted set, so scoping cannot become the route by which runs
enter discovery; the other walks the closed searchable set and requires every member to be counted
by the facet structure, so a class added to the model without a facet cannot silently break the sum
invariant. At the HTTP layer the facet object is pinned as a wire contract: exactly the six class
keys, in canonical order, present at zero, with no `experiment_run` key.

Relevance ranking and match explainability have their own suite at three layers. The domain tests
hold the weight table to the property the design rests on — every signal outweighing the sum of
every weaker signal — plus the declared order matching the weight order, the score being exactly the
sum of its signals, and an unknown signal name contributing nothing. The service tests cover each
tier in turn: an exact ID strictly outscoring every other hit, an exact name beating descriptive
text, a name **prefix** beating an interior name match inside the one `title_substring` class, an
ID-only hit ranking last and still being returned, a contiguous phrase beating scattered terms, and
complete term coverage of a display field beating partial. Ordering is pinned as a whole list, not
only as pairs, so the tie-break — score, then canonical class, then canonical ID — is asserted where
it actually fires; a test fails if the chosen query stops producing an equal-score pair at all.
Determinism is asserted across repeated runs and across two indexes built from one corpus. The
explanation itself is held to a contract: signals in declared weight order, no repetition, every
name inside the closed set, the three mutually exclusive tiers never firing twice, composed-only
signals never appearing on a literal hit, and `match_kind` always agreeing with the signals it is
derived from. Normalisation is covered as identity of whole responses — case, surrounding and
repeated whitespace, and a repeated term all rank identically — and term **order** is pinned as
significant to the phrase signals and to nothing else. Integration is asserted against the phases
it composes with: one scope, several scopes, all six, the single-class `type`, a zero-result scope,
facets still counting the complete filtered set, paging applied strictly after ranking, and
`include_context` leaving every score, signal and position untouched. One is a guardrail rather than
a feature test: it requires every emitted signal to come from the closed compile-time vocabulary and
to carry no record content, which is what keeps the two new fields outside the provenance-flattening
rule they were added against. At the HTTP layer the wire contract is pinned separately: both keys
present on every result of every mode, the score recomputable from the signals over the wire, the
serialised order being the ranking, byte-identical bodies across repeated requests and across two
handlers, page windows matching the ranked set position for position, and the result object carrying
exactly the documented keys and no more.

Context resolution is covered at both layers. The service tests assert the exact resolved set, in
order, for a representative record of every searchable class — including the canonical
`claim -> source` provenance in both directions — plus the deterministic empty case for records
nothing refers to, stability across eight rebuilt indexes, defensive copying, and both bounds
against a synthetic densely connected corpus the real one does not yet contain. Three are
guardrails rather than feature tests: one requires the result set and every result field to be
identical with and without context, so context can never become a ranking or matching signal; one
cross-checks every graph-class context item against the traversal adjacency for the same record,
so the two layers cannot drift into two meanings for one connection; and one takes the vocabulary
entries and experiment definitions a context just named and asserts each is still refused as a
traversal root and still absent from the graph. The HTTP tests cover the wire contract: that a
request without the control is byte-identical to the Phase 1E body, that every resolved identity
is fetchable through its own route, that the serialised objects carry exactly the documented keys,
that a malformed or duplicated control is refused without leaking internals, and that
`include_context` is refused on every other route.

A cross-phase suite covers the combined backend specifically: that every phase's routes coexist on
one router without shadowing, that mutation and duplicate-parameter rejection hold on all of them,
that the practice layer stays out of the traversal graph, that a record just found through search —
or just named as another record's context — is still refused as a traversal root and still absent
from the graph, and that the shared index is deterministic and hands out defensive copies across
every layer at once.

A search-workflow suite covers the composed pipeline rather than any one of its stages. The suites
above each pin the contract of the feature they introduced; this one pins what is only true once all
of them are present, by stating each property once and requiring it of a shared matrix of requests
that crosses both query modes with every scope spelling, the context control and a range of page
windows. The invariants are that the facet sum equals `page.total` on every page, that a page never
exceeds its reported limit and the limit never exceeds the ceiling, that no result repeats, that
`relevance_score` is exactly the sum of the weights of the `match_signals` beside it, that the signal
list is closed, unique, ordered by descending weight and free of two members of one exclusive group,
that `matched_fields` is exactly the union of the per-term evidence, that the ordering is total and
monotone, and that context is present exactly when requested and internally consistent inside both
its bounds.

Determinism is asserted three ways there: repeated identical requests are compared as raw bytes
rather than as decoded values; the same matrix is answered by an index built from an in-memory copy
of the fixture and required to be byte-identical, which is what rules out a dependency on how the
corpus was enumerated rather than merely on the query path; and the matrix is replayed from eight
concurrent readers. Four further regressions state contract decisions that span stages: every page is
required to be exactly the matching window of the complete ranked set, so bounding demonstrably
follows ranking; taking the context off an enriched response is required to leave exactly the plain
one, so context is additive and never a ranking signal; every hit's whole explanation is required to
be unchanged by scope, paging and context, so a score is a function of the query and one record; and
every signal in the published weight table is required to be fired by a named request, so the table
cannot become documentation of a ranking the API can no longer express. The validation surface is one
table of malformed requests — the query itself, the query string, composition, scope, context and
paging — each required to answer `400` with the existing `invalid_query` code and an envelope
carrying exactly `code` and `message`, and separated from the well-formed-but-unsatisfiable requests
that must answer `200` with an empty result list and the complete all-zero facet structure.

Related-knowledge discovery has its own suite at four layers, and the split follows what each layer
can actually prove. The domain tests hold the precedence table to properties no Go switch asserts on
its own: every canonical field in the declared closed set classifies, the set contains no duplicate,
an unknown field falls through to `unclassified` rather than being guessed at, the ranks are the
list positions with the fallback strictly last, and a reason's precedence is always derived from the
origin it reports. One is a guardrail rather than a feature test: it asserts the reason struct's
exact field set, so a confidence, similarity or relevance number cannot be added to an explanation
without a test to change.

The service tests pin the whole contract for a start of every searchable class as an ordered list of
`item <-relation- canonical-field (priority, derived)` lines, so identity, ranking and explanation
are asserted together and a projection returning the right records for invented reasons fails. They
cover the precedence order end to end from a node, provenance leading from a claim, evidence
outranking attribution, a source read backwards to everything that cites it, a vocabulary entry
reaching the concept layer, and an experiment definition reaching what it demonstrates without ever
reaching a run. Bounds are asserted against the unbounded result rather than a hard-coded list: a cut
list must be the front of the complete ordering at every limit, must report the true eligible total,
and must say it was cut. The limit contract is pinned as clamping rather than refusal, including the
exactly maximal value. Scope is asserted as an invariant — a scoped result must be the unscoped one
with other classes removed, same reasons, same relative order — and the echoed scope is required to
be in canonical order regardless of how the caller spelled it.

Determinism is asserted three ways: twenty-five repeated calls per start compared as whole values;
the same starts answered by an index built from an in-memory copy of the fixture and required to
match the one built from disk, which is what rules out a dependency on how the corpus was
enumerated; and the ordering required to be total, with no two items of any result sharing a sort
key. Both tie-breaks are pinned where they actually fire — two sources separated by nothing but
their IDs, and a session and a source separated by the canonical class order.

Evidence integrity has its own group. Every reason of every item of every start is required to name
a canonical field from the declared set, to carry a classified precedence, and to report the rank
that precedence actually has. The grounding test is the load-bearing one: every reason is looked up
in the Phase 1F context the search route independently serves for the same record, matched on
relation, destination, canonical field and direction together, so a connection this layer cannot
point at in the context projection is a fabrication no matter how reasonable it looks. That is the
test that fails if a future edit ever starts inferring a connection from similarity, prose overlap
or co-occurrence. Deduplication is covered against real canonical data — a claim that names one node
in both `appears_in` and `derived_from` — and against two constructed corpora the fixture cannot
supply: one claim that both cites and credits the same source, which pins that the stronger
precedence class wins rather than field order or alphabetical relation name, and two nodes joined by
all three canonical relationship types in both directions, which pins the per-item evidence cap, the
kept order, and that the primary reason is never repeated inside the additional list.

Four are guardrails rather than feature tests. One requires the loader to refuse a node self-link
and then requires no start to appear in its own discovery, so the exclusion is checked at the layer
that owns the rule as well as at the layer that depends on it. One requires the session and the
registry entry that share an ID to be offered as two separate items with their own canonical fields,
so identity stays the `(class, id)` pair. One captures search, context and traversal, runs a full
sweep of discovery requests, and requires all three to answer identically afterwards, so a
projection that sorted its input in place would fail. The last cross-checks every item's title and
summary against the same record's search hit, so one record cannot be named two ways.

The HTTP tests pin the wire contract: the route serving all six start classes, the documented keys
and nothing more, omitted keys asserted on the bytes rather than the decoded value — no echoed scope
when none was asked for, no empty `additional_evidence`, no fabricated empty `summary`, and an empty
discovery serialising `[]` rather than `null`. Byte-identical bodies are required across repeated
requests for every start and every parameter shape. A percent-encoded identifier is required to be
the same request as its plain spelling, while an encoded separator is refused by the shared
identifier bound. The refusals are covered one at a time — unsupported start class, unknown record
under its own stable `related_start_not_found` code, a real ID under the wrong class, a Unicode
identifier the canonical ID contract cannot produce, every malformed scope spelling, a negative or
non-integer limit, and each of `depth`, `offset`, `q`, `type`, `relationship`, `target_type`,
`include_context` and `query_mode` refused as unknown parameters rather than ignored. Two are
disclosure guardrails: one requires no absolute path, repository-relative path, Go error or stack
frame in any response, success or failure; the other requires no record content — prose bodies,
future questions, practical applications, confidence bases — to appear in a suggestion list, since
canonical field *names* are legitimate explanations while their *contents* belong to the record's
own route. A cross-route test requires a malformed class list to be refused with the identical code
and message on this route and on `/api/v1/search`, and a cross-phase test captures every earlier
route, runs a discovery sweep, and requires each one byte for byte unchanged.

## Known limitations

- Repository changes require a process restart. There is no watcher, no background sync and
  no filesystem polling, so a running process always serves one consistent snapshot.
- Search is lexical substring matching, in both query modes. There is no stemming, no fuzzy
  matching, no semantic retrieval and no embedding, and Phase 1I did not add any: `relevance_score`
  ranks the records substring matching already found, and cannot make an unmatched record
  reachable. The per-layer `q` parameters do not rank at all.
- `relevance_score` is a fixed, hand-written weighting of explicit lexical signals, not a retrieval
  model. It has no term frequency, no field-length normalisation, no IDF, no popularity, usage,
  recency or user weighting, and no learned component. It is comparable only within one response:
  nothing calibrates it across queries, so a larger score on a different query does not mean a
  better answer. It orders results and never filters them — there is no relevance threshold and no
  "did you mean".
- Ranking sees only the query and each record's own searchable fields. It cannot use a record's
  relationships, its provenance, its resolved context or how often it is referenced, because
  context is resolved after ranking and paging and can never feed back into either.
- Search matches only the fields documented under "Cross-layer search". A record whose relevant
  text lives in an excluded field — source `notes`, an experiment's `procedure`, a node's markdown
  body — is not discoverable by that text.
- `q` composes in exactly two ways: one literal phrase, or `query_mode=all_terms` requiring every
  whitespace-separated term inside one record. There is no boolean, wildcard, regex, quoting or
  field-scoped query syntax, and the per-layer `q` parameters remain literal-only — composition
  exists on `/api/v1/search` alone.
- Result scope is a set of whole classes and nothing finer. `entity_types` restricts which of the
  six searchable classes may be returned; there is no field-scoped filter, no per-class field
  selection, no negation of a class, and no scope filter on the per-layer list endpoints, which
  keep their own single-value filters. `type` and `entity_types` may not be combined.
- Facets describe one axis: the searchable class of each hit. There is no facet over node
  `domain`, claim confidence, source type, experiment status or any other canonical field, and no
  caller-supplied facet field, because each would be a decision about which axes of the corpus are
  worth counting rather than a reading of one the model already fixes.
- `all_terms` splits on whitespace and nothing else, so punctuation stays part of a term and
  `room,` will not find a record that wrote `room`. It requires 2 to 8 distinct terms; one term is
  refused rather than treated as literal search under another name.
- `all_terms` intersects within one record only. Two terms held by two records that reference each
  other are not a hit, and `include_context` cannot supply a missing term — context is resolved
  after matching, for the returned page only.
- Experiment runs are not searchable at all, by design. See "Experiment runs are deliberately
  absent"; they remain addressable through their own structured routes.
- No query is stored. There is no search history, no query log, no analytics and no
  personalisation, so nothing improves with use. Context is resolved from the immutable startup
  index on every request; nothing about it is cached, materialised or persisted.
- Search-result context is deliberately one hop and bounded: 25 items per result, 2000 per
  response, no depth control and no caller-supplied shaping. It resolves what a record directly
  references and what directly references it, and nothing further; a neighbourhood is a traversal
  request.
- Context resolves provenance and never scores it. Nothing in the API rates a source's
  reliability, a claim's strength or a relation's importance, and a contradicting source is served
  exactly like a supporting one.
- Context names only the six searchable classes. An experiment run is never named, and a claim's
  `appears_in: document` reference resolves to nothing, because no layer addresses it.
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
- Related-knowledge discovery is one hop, like context and unlike traversal. It returns records
  the start directly references or that directly reference it, ranked by the canonical field each
  connection came from; there is no transitive discovery, no depth, no paging and no
  caller-supplied ranking, weighting or priority parameter. It is bounded to 25 items by default,
  100 at most, and 5 reported connections per item.
- Discovery ranks authored references and measures nothing. No similarity, embedding, vector,
  keyword overlap or co-occurrence contributes to it, and no confidence, relevance or probability
  appears in its output. Two records are related if and only if some canonical record wrote down a
  reference between them; AI- or embedding-based discovery is not implemented anywhere in this
  backend.
- Discovery reaches exactly what the context layer already resolves, so a canonical field that
  layer does not read is invisible to it too — node `experiments:` and `appears_in: document`
  among them. It starts from the six searchable classes and never from an experiment run.
- Discovery is stateless and identical for every caller. There is no reading history, no
  popularity, no click weighting and no personalisation, and nothing about a request is stored.
- No frontend, no graph visualization, and no LLM integration.

## Future work

Deferred, not implemented: graph traversal across the practice layer, richer diagnostics, query
syntax beyond the two documented composition modes, deeper or caller-shaped context, graph
visualization, semantic retrieval, and MLLM experimentation.

Deterministic multi-term composition is no longer deferred; `query_mode=all_terms` is the whole of
what shipped. Multi-class result scope is no longer deferred either; `entity_types` is the whole of
that one, and it filters rather than searching. Deterministic relevance ranking is no longer
deferred; `relevance_score` and `match_signals` are the whole of that one, and they rank and explain
the records lexical matching already found rather than widening what matching reaches. Richer
*syntax* — boolean operators, quoted phrases inside a composed query, negation, wildcards,
field-scoped terms — remains deferred, as do facets over any axis other than the searchable class,
and each would be its own contract rather than an extension of this one.

Ranking beyond explicit lexical signals stays deferred for the same reason semantic retrieval does.
Term frequency, field-length normalisation, IDF, popularity, usage or recency weighting, per-caller
ranking and any learned model would each require a judgment this service cannot source from the
corpus, and would trade a score a client can recompute for one it has to trust. A caller who
disagrees with the current ordering can read `match_signals` and see exactly which signal produced
it, which is the property a tuned model would give up first.

Semantic retrieval is deferred deliberately. The deterministic lexical surface exists first so that
the discovery contract — what is searchable, what a hit means, what order results arrive in and why
— is fixed and test-covered before anything harder to reason about is layered on top of it. Relevance
ranking was added inside that contract rather than alongside it, which is why it is a weighted list
of stated signals: the ordering stays as reproducible and as inspectable as the matching underneath
it.

Traversal over the practice layer is deferred deliberately, not incidentally. Vocabulary entries,
experiments and experiment runs are read surfaces adjacent to the graph, and their cross-references
are not canonical graph relationships; promoting them to traversal edges would assert a claim about
the corpus that the corpus does not make. See "Practice-layer representation" above.

Deterministic related-knowledge discovery is no longer deferred; `GET /api/v1/related/{entity_type}/{id}`
is the whole of what shipped, and it ranks references the corpus already authored rather than
widening what any layer reaches. Three extensions to it are worth recording as recommendations and
are explicitly **not** implemented: exposing the precedence table as a read-only contract endpoint,
so a client can render the classes without hard-coding them; a per-class breakdown of the eligible
set, in the shape search facets already use; and a bounded intersection answering "related to both
of these". Each would be its own phase with its own contract.

Multi-hop, similarity-ranked and model-generated discovery are not on that list. The first is what
the traversal routes are for; the second and third are deferred on exactly the grounds semantic
retrieval is, and a "what should I read next" surface is where the temptation to reach for an
embedding is strongest, which is why the refusal is stated rather than assumed. As the encyclopedia
grows, the cost of one discovery request stays a map lookup and a sort, because the references are
resolved once at startup; what grows is the eligible count a hub record reports, and that is a
number in the response rather than work in the request.
