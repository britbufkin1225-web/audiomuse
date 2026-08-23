# AudioMuse Backend — Read-Only Knowledge API

A deterministic read-only HTTP projection of the canonical AudioMuse repository: nodes, sessions,
the typed relationship graph, the sources, claims and provenance that stand behind them, the
vocabulary, experiments and experiment runs that put them into practice, and one lexical search
surface spanning all of them that can compose several terms into one request and resolve the
canonical context around what it finds.

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
| GET | `/api/v1/search` | one lexical query across every searchable canonical layer, optionally composed from several terms and with the bounded canonical context of each hit |
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
| `query_mode` | optional; exactly `literal` or `all_terms`. How `q` is composed into a query. Default `literal` |
| `include_context` | optional; exactly `true` or `false`. Resolve the canonical context of each returned hit |
| `limit`, `offset` | page size and start; default 50, clamped to 200 |

`q` must be non-empty after trimming. An absent, empty or whitespace-only `q` is refused with
`400 invalid_query` rather than returning everything: each layer already has its own list endpoint,
and an empty search would be a second, slower whole-corpus dump that a caller who mistyped a
parameter name could not tell from a successful query. Values longer than 128 characters are
refused rather than truncated. An unknown or duplicated query parameter is refused exactly as it is
on every other route.

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
| total `q` length | 128 characters, as in every mode |

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
  "results": [
    {
      "entity_type": "node",
      "id": "room-mode",
      "title": "Room Mode",
      "summary": "A standing-wave resonance determined by the dimensions of the space.",
      "match_kind": "all_terms",
      "matched_fields": ["title", "definition"],
      "term_matches": [
        { "term": "room", "matched_fields": ["title"] },
        { "term": "resonance", "matched_fields": ["definition"] }
      ]
    }
  ]
}
```

`query` echoes the normalised term list joined by single spaces, so the response describes the
query that ran rather than the caller's spacing. `query_mode` is present only for `all_terms`;
`term_matches` is present only on composed results.

`term_matches` is the evidence for a composed hit, and it is bounded to exactly what the backend
knows: which of this record's canonical fields each term was found in. Terms follow the normalised
query order, fields follow the canonical field order, and there are no snippets, no offsets, no
highlighting, no occurrence counts and no rewritten prose. Every field named belongs to the hit's
own record — nothing is borrowed from a related one, including when `include_context=true`
resolved that record on the same response. `matched_fields` keeps its Phase 1E meaning and is the
union of the per-term lists in canonical field order.

##### Composed ordering

Composed results are ordered by the canonical class order, then by canonical ID. There is no
match-kind precedence, because every hit carries the one composed class `all_terms`: each returned
record satisfies every term, and inventing a tie-break between them would be a relevance judgement.
The same corpus and the same normalised query always produce the same bytes.

`type`, `limit` and `offset` compose unchanged, and the order of operations is fixed: normalise,
match, filter by class, order the **complete** match set, page it, and only then resolve context.
Paging never precedes matching, so `page.total` is always the size of the full composed result set
rather than of the page.

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

#### Ordering

Literal results are ordered by four mutually exclusive **categorical match classes**, then by the
canonical class order above, then by canonical ID. (A composed `all_terms` result set carries the
single class `all_terms` and is ordered by class and ID alone; see "Composed ordering".)

| `match_kind` | Meaning |
| --- | --- |
| `id_exact` | the query is exactly the record's canonical ID |
| `title_exact` | the query is exactly the record's display field |
| `title_substring` | the query appears inside the record's display field |
| `field_substring` | the query appears only in some other searchable field |
| `all_terms` | every term of a composed query was found in this record |

This is **not a relevance score**, and it is deliberately not rendered as a number. There is no
weighting, no field boosting, no term frequency and no ranking model; it is a fixed hand-written
precedence, and calling it anything else would dress a priority list as information retrieval. The
class order and canonical ID break every tie, so the ordering is total: the same corpus and the same
query always return byte-identical results. Two record classes may share an ID — a session and its
registry entry do — so a hit is identified by `entity_type` **and** `id`, never by `id` alone.

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
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=room%20resonance&query_mode=all_terms"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=frequency%20pitch&query_mode=all_terms&type=node"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=sampling%20audio&query_mode=all_terms&include_context=true"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&include_context=true"
```

```powershell
Invoke-RestMethod "http://127.0.0.1:8788/api/v1/search?q=resonance&type=claim&include_context=true"
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
cycle termination, deduplication and truncation bounds, the cross-layer discovery projection and
its context resolver, and the HTTP routes including 404, 400 and 405 behaviour. Determinism is tested directly: the loader and the index are each built twice from an
unchanged corpus and the results compared. Unit tests run against `testdata/corpus/`, a
small synthetic fixture, so a canonical content change cannot silently move a unit-test
expectation.

Six tests run against the real repository on purpose: one asserts it loads with no fatal issues,
one asserts the evidence layer parses and resolves, one asserts the practice layer does, one walks
every canonical entity as a traversal root at maximum depth and asserts no practice record appears
anywhere in the result, and two snapshot the size, modification time and content digest of every
canonical file — one across a load, one across a full index build plus one request to every read
surface, including three searches and two context resolutions, and a rejected request on each
mutating method. All six skip if the canonical repository is not found above the working
directory.

Search is covered at both layers: class coverage, case insensitivity, exact-ID and match-class
precedence, matched-field correctness, class filtering, empty and rejected queries, paging and
clamped bounds, defensive copying, and determinism across two indexes built from one corpus.
Query composition has its own suite at both layers. It pins the compatibility half first — that an
omitted mode and an explicit `literal` are the same response, that two words are still a phrase,
and that no literal result gains a `query_mode` or `term_matches` key — and then the composed half:
terms in one field and in separate fields of one record, a missing term rejecting the record, terms
held by two different records refusing to combine, case, whitespace and duplicate normalisation,
both term bounds and the character ceiling at the service rather than only over HTTP, per-term
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

## Known limitations

- Repository changes require a process restart. There is no watcher, no background sync and
  no filesystem polling, so a running process always serves one consistent snapshot.
- Search is lexical substring matching, in both query modes. `/api/v1/search` orders literal
  results by a fixed categorical match precedence and composed ones by class and ID, never by a
  relevance score; the per-layer `q` parameters do not reorder at all. There is no stemming, no
  fuzzy matching, no semantic retrieval and no embedding.
- Search matches only the fields documented under "Cross-layer search". A record whose relevant
  text lives in an excluded field — source `notes`, an experiment's `procedure`, a node's markdown
  body — is not discoverable by that text.
- `q` composes in exactly two ways: one literal phrase, or `query_mode=all_terms` requiring every
  whitespace-separated term inside one record. There is no boolean, wildcard, regex, quoting or
  field-scoped query syntax, `type` accepts one class rather than a set, and the per-layer `q`
  parameters remain literal-only — composition exists on `/api/v1/search` alone.
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
- No frontend, no graph visualization, and no LLM integration.

## Future work

Deferred, not implemented: graph traversal across the practice layer, richer diagnostics, query
syntax beyond the two documented composition modes, deeper or caller-shaped context, graph
visualization, semantic retrieval, and MLLM experimentation.

Deterministic multi-term composition is no longer deferred; `query_mode=all_terms` is the whole of
what shipped. Richer *syntax* — boolean operators, quoted phrases inside a composed query, negation,
wildcards, field-scoped terms, multi-class `type` — remains deferred, and each would be its own
contract rather than an extension of this one.

Semantic retrieval is deferred deliberately. The deterministic lexical surface exists first so that
the discovery contract — what is searchable, what a hit means, and what order results arrive in —
is fixed and test-covered before anything harder to reason about is layered on top of it.

Traversal over the practice layer is deferred deliberately, not incidentally. Vocabulary entries,
experiments and experiment runs are read surfaces adjacent to the graph, and their cross-references
are not canonical graph relationships; promoting them to traversal edges would assert a claim about
the corpus that the corpus does not make. See "Practice-layer representation" above.
