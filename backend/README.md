# AudioMuse Backend — Read-Only Knowledge API

A deterministic read-only HTTP projection of the canonical AudioMuse repository: nodes, sessions,
the typed relationship graph, and the sources, claims and provenance that stand behind them.

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
│   ├── service/                immutable startup index, filtering, search, graph and evidence projections
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
`schemas/claim.schema.yaml` and `schemas/source.schema.yaml` are not markers but are still
required: the loader reads their bounded vocabularies, and an unreadable one is reported as a fatal
validation issue rather than as a bad root. `claims/records/` is optional — a corpus with no claim
records loads and serves an empty claim layer.

The default address binds loopback: a knowledge corpus should not become reachable from the
network by accident.

### Startup output

```text
level=INFO msg="AudioMuse API" mode=read-only repository=... nodes=78 sessions=3
  sources=51 edges=220 validation=WARN warnings=1 listen=127.0.0.1:8788
```

The corpus summary on `/api/v1/project` reports the claim count alongside the rest.

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
| GET | `/api/v1/graph` | the full read-only graph projection |
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

### Errors

```json
{ "error": { "code": "node_not_found", "message": "Node was not found." } }
```

Codes: `not_found`, `node_not_found`, `session_not_found`, `source_not_found`, `claim_not_found`,
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

The backend never repairs a record and never writes to the corpus. Canonical
inconsistencies are reported for a human to decide about.

## Tests

`go test ./...` covers the front-matter parser, the claim record stream parser, the filesystem
adapter and every validation defect, the service index, filtering, search, paging, the graph
projection and every evidence reverse index, and the HTTP routes including 404, 400 and 405
behaviour. Determinism is tested directly: the loader and the index are each built twice from an
unchanged corpus and the results compared. Unit tests run against `testdata/corpus/`, a
small synthetic fixture, so a canonical content change cannot silently move a unit-test
expectation.

Three tests run against the real repository on purpose: one asserts it loads with no fatal issues,
one asserts the evidence layer parses and resolves, and one snapshots the size, modification time
and content digest of every canonical file before and after a load to assert nothing was written.
All three skip if the canonical repository is not found above the working directory.

## Known limitations

- Repository changes require a process restart. There is no watcher, no background sync and
  no filesystem polling, so a running process always serves one consistent snapshot.
- Search is lexical substring matching with no ranking. There is no semantic retrieval and
  no embedding.
- No persistence and no database; the startup index is the only state.
- Experiments, experiment runs and vocabulary entries are canonical layers the backend does
  not parse. Node `experiments:` references and claim `appears_in: vocabulary` and
  `derived_from: experiment_run` references are checked for identifier shape only and are
  carried through unresolved.
- `appears_in: session` is a canonical reference kind no current claim record uses, so
  `?session_id=` on either evidence endpoint answers correctly and returns nothing against
  today's corpus.
- There is no graph traversal across the evidence layer; the relations resolve, walking them
  is a later decision.
- No frontend, no graph visualization, and no LLM integration.

## Future work

Deferred, not implemented: vocabulary and experiment-run parsing, evidence-layer graph traversal,
richer diagnostics, search hardening, graph visualization, semantic retrieval, and MLLM
experimentation.
