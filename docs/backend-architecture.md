# AudioMuse Backend Architecture

## Why AudioMuse has a backend

AudioMuse is a repository-first knowledge atlas. The canonical corpus — nodes, sessions, sources,
claims, vocabulary, experiments, experiment runs, schemas, and research notes — lives in files and
stays authoritative. Direct filesystem browsing has become limiting as the corpus grew: the typed
relationship graph, the provenance registry, and the cross-layer reference structure are all real
data that no text editor can traverse.

The backend draws a computational boundary across that corpus:

```text
CANONICAL KNOWLEDGE   →   SOFTWARE THAT INSPECTS THAT KNOWLEDGE
```

One repository load feeds one immutable projection, through one chain:

```text
repository  →  typed filesystem projection  →  validation  →  immutable service  →  read-only API
```

There is one adapter, one `service.Knowledge` constructor and one index. Everything below is a
view over that single load, not a store of its own:

| Read surface | What it serves |
| --- | --- |
| nodes / sessions / graph | the typed concept graph and the sessions that developed it |
| sources / claims / provenance | the registry and the checkable statements that cite it |
| bounded traversal | the knowledge and evidence layers walked as one graph, depth-capped |
| vocabulary | canonical terms, with reverse reads to the records that reference them |
| experiments | reusable exercise definitions, with derived run tallies |
| experiment runs | what was actually performed, kept apart from the definitions |

The first three are the graph. The last three sit beside it: they are read surfaces adjacent to
the graph rather than part of it, and the distinction is enforced rather than merely documented —
see "Why the practice layer did not extend graph traversal" below.

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
| the resolved context of one searchable record, ranked by the canonical field each relation came from | canonical field references | `domain.RelatedReason`, `domain.RelatedItem` | `Knowledge.searchContext` grouped by destination, plus `relatedSummary` | `GET /api/v1/related/{entity_type}/{id}` |
| the closed precedence classes those canonical fields fall into | ranking model, not corpus data | `domain.RelatedPriority`, `domain.RelatedPriorities` | one table: ranking, filter allowlist and explanation codes | `reason.priority`, `reason.priority_rank`, `reason.explanation`, `?relationship_types=` |
| `vocabulary/entries/*.yaml` YAML document streams (`schemas/vocabulary.schema.yaml`) | one mapping per entry | `domain.VocabularyEntry` | `vocabularyByID`, sorted `vocabulary` | `GET /api/v1/vocabulary`, `GET /api/v1/vocabulary/{id}` |
| vocabulary `domain`, reusing the enum in `schemas/node.schema.yaml` | bounded vocabulary | `domain.Vocabularies.VocabularyDomains` | startup and filter validation set | `?domain=`; `400 invalid_query` |
| `vocabulary.related_terms[]` | curated navigation ID list | `[]string` on the entry | none; deliberately not indexed as adjacency | `entry.related_terms` — never a graph edge |
| `experiments/records/*.yaml`, one record per file (`schemas/experiment.schema.yaml`) | one mapping per definition | `domain.Experiment` | `experimentsByID`, sorted `experiments` | `GET /api/v1/experiments`, `GET /api/v1/experiments/{id}` |
| `experiment-runs/records/*.yaml`, one record per file (`schemas/experiment-run.schema.yaml`) | one mapping per run | `domain.ExperimentRun` | `runsByID`, sorted `runs` | `GET /api/v1/experiment-runs`, `GET /api/v1/experiment-runs/{id}` |
| `run.observations[]` and `run.measurements[]` | two separately shaped object lists | `domain.RunObservation`, `domain.RunMeasurement` | per-run counts, never summed | `run.observations`, `run.measurements` |
| `run.status` across every run of one definition | lifecycle tally | `domain.ExperimentRunCounts` | `runCountsByExperiment` | `experiment.runs`; `project.experiment_runs` |
| `experiment.vocabulary_refs[]` and `claim.appears_in[kind: vocabulary]` | reverse reads | `[]string` | `experimentIDsByVocab`, `claimIDsByVocab` | `vocabulary/{id}.experiment_ids`, `.claim_ids` |
| `schemas/experiment.schema.yaml` and `schemas/experiment-run.schema.yaml` bounded enums | vocabulary lists | `domain.ExperimentVocabulary`, `domain.ExperimentRunVocabulary` | practice filter validation set | `project.vocabulary`; `400 invalid_query` |
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
  including the bounded breadth-first traversal over the relationship adjacency. One `New` builds
  every layer: `buildGraph`, `buildEvidence`, `buildTraversal` and `buildPractice` run in sequence
  over the same parsed corpus. They are independent by construction — `buildTraversal` reads nodes,
  claims and relationship types; `buildPractice` reads vocabulary, experiments, runs and claims —
  and neither reads the other's derived state, which is what keeps the practice layer out of the
  traversal adjacency structurally rather than by convention.
  `buildSearchContext` and then `buildRelated` run last, after every projection they read; the
  related-knowledge layer derives no connection of its own and only ranks, deduplicates and bounds
  the context projection.
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

**Why multi-term composition is opt-in and not a query language.** A reader who types two words
usually means "a record about both", and the corpus routinely carries those words in different
canonical fields of one record — a node whose title says one and whose definition says the other.
Phase 1G makes that reachable through `query_mode=all_terms` without changing what a query means by
default: reinterpreting whitespace as an implicit AND would silently alter the meaning of every
request already written against the literal contract, so the default stays one contiguous phrase and
the composed mode has to be named. The composition is bounded on purpose — whitespace splitting, 2
to 8 distinct terms, no operators, no quoting, no wildcards, no field selectors — because the moment
a query grows a parser it grows a precedence, and a precedence is a judgment about what the reader
meant. What ships is one deterministic retrieval fact: these terms all occur in this record. That is
a statement about text, not about meaning, and the response says so — `all_terms` is its own match
class, and evidence is per-term canonical field names with no counts or snippets. Phase 1G ordered
composed hits by the canonical class order and then ID, on the grounds that every hit satisfies
every term equally and any tie-break between them would be a relevance score by another name. That
was the right call while there was no score to name; Phase 1I supplies one explicitly, and composed
results are now ranked by it before falling back to the same class and ID keys.

**Why result scope is a separate parameter, and why facets are counts of one axis.** A reader who
searches across six layers at once needs two things the Phase 1E result page cannot give them:
a way to say "answer this from these layers", and a way to see the shape of the answer before
paging through it. Phase 1H adds both as projections of the result set that already existed, not
as retrieval. `entity_types` filters — same fields, same substring test, same match evidence, same
order, with other classes removed — so a scoped result set is provably the unscoped one minus
classes, and scope can be added to a request whose results a client has already reasoned about.
It is a new parameter rather than a widening of `type` because `type` is a single exact value, as
every other filter on this API is, and re-reading it as a list would give one parameter two
meanings and would silently turn a request that is refused today into one that succeeds; the two
are refused together because there is no reading of both at once that is not a guess. Facets are counted over the whole
filtered set before paging, which is the only version of the number worth serving — a breakdown of
the current page is something the caller can already count — and they cover exactly one axis, the
searchable class, because that axis is fixed by the model. A facet over node `domain`, claim
confidence or source type would be a decision about which axes of the corpus are worth counting,
and a caller-supplied facet field would make the response a query result rather than a fixed
projection. They are counts, not scores: nothing about them reorders a result set or rates a class.

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

**Why vocabulary cross-references are not graph edges.** `vocabulary/README.md` states the rule
outright: `related_terms` is human navigation, implies no equivalence, creates no edge, and must
not affect node degree. The backend resolves those references so a dangling one is caught, and then
stops. There is no related-term adjacency index, no vertex is created for a vocabulary entry, and
`buildGraph` still reads node relationships and nothing else. Resolving a reference and building an
edge are different acts, and only the second is a claim about the knowledge graph.

**Why the practice layer did not extend graph traversal.** The practice layer adds read surfaces
adjacent to the graph, not new graph semantics. Which entity types are traversable, and under which
typed relations, is a graph-contract decision; making vocabulary entries and experiments traversable
merely because the API now serves them would settle that decision by accident.

The two are easy to conflate because the backend does resolve the references that cross between
them: a claim's `appears_in: vocabulary` and `derived_from: experiment_run` are validated against
loaded records, and an unresolvable one is fatal. Resolution is not membership. `referenceEntity`
maps only session, node and claim kinds to an `EntityRef` and returns nothing for the rest, and
`domain.EntityTypes` is a closed set of four, so a practice record has no representable identity in
the traversal graph at all — the isolation is a property of the types, not a filter that could be
forgotten. It is checked directly against the canonical corpus, by walking every entity as a
traversal root at maximum depth and asserting no practice ID appears in any result.

**Why an experiment definition and its runs are served apart.** `experiment-runs/README.md` keeps
them in separate directories so mutable result history cannot change a canonical definition. The
projection mirrors that: `GET /api/v1/experiments/{id}` names its runs by ID and reports a
per-status tally, but does not embed them. Embedding would make results read as part of the
specification, which is the merge the directory split exists to prevent.

**Why every run status is counted separately.** A single "runs" number would let three planned runs
read as three performed experiments. `domain.ExperimentRunCounts` reports `planned`, `completed`,
`incomplete` and `invalid` alongside the total, and each run also carries a derived `performed`
boolean, so the distinction survives into a client that only reads the summary.

**Why measurement values are carried as the authored token.** A measurement is evidence, and
`72.50` is a claim about precision. Decoding it to a float64 and re-encoding would serve `72.5`,
silently weakening what was recorded. `domain.CanonicalNumber` keeps the authored token and
validates it against the JSON number grammar, which every canonical record already satisfies
because the repository validators parse each value with `ConvertFrom-Json`.

**Why the run lifecycle rules are enforced in the backend at all.** Everywhere else, canonical
semantic rules stay with the PowerShell validators. The lifecycle rules are enforced here for the
same reason those are not: the projection depends on them. The API serves a derived `performed`
flag and derived per-status counts, so a record claiming `planned` while carrying measurements
would make those derived values assert evidence the repository withholds. Vocabulary `domain`
membership is also enforced because it bounds a served filter, using the same node-schema enum as
`tools/validate-vocabulary.ps1`. Calendar validity and the future-date bound stay with
`tools/validate-experiment-runs.ps1`. The future-date rule
additionally depends on the wall clock, and a projection whose validity changed with the time of
day would not be deterministic.

**Why generated indexes are never read.** `vocabulary/index.md`, `experiments/index.md`,
`experiment-runs/index.md` and everything under `indexes/` are rebuildable views of canonical
records. The backend derives its own lookup structures from the records instead, so a disagreement
between the two is a real signal. Reading the projection the backend would be compared against
would destroy that signal.

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

Phase 1D adds, again at the same severity: a duplicate, blank or non-canonical vocabulary,
experiment or run ID; a vocabulary term reused in any casing; a record or nested control-setting,
observation or measurement item whose key set does not equal the contract's; an empty required
field or an empty value inside any list; a value repeated within one list; a vocabulary entry or
experiment that references itself; an experiment `status`, `type` or `difficulty`, a run `status`,
or a measurement `calibration` outside its schema enum, including case drift; an unresolved
`node_refs`, `session_refs`, `source_refs`, `vocabulary_refs`, `related_terms`,
`related_experiments` or run `experiment_id`; a `run_date` that is not ISO `YYYY-MM-DD`; a
measurement value that is not a JSON number; an experiment or run file holding more than one
record; a violation of the run lifecycle contract; and an unreadable or enum-less
`schemas/experiment.schema.yaml` or `schemas/experiment-run.schema.yaml`.

Phase 1D also upgrades two Phase 1B checks. Claim `appears_in: vocabulary` and
`derived_from: experiment_run` were shape-checked and carried through unresolved because the
backend did not read those layers. It reads them now, so both resolve, and a reference naming
nothing is fatal.

The run lifecycle contract, enforced at load: a planned run may not carry a `run_date`, an
observation, a measurement, an interpretation, or a procedure deviation; a performed run must carry
a `run_date`; a completed run must carry at least one observation or measurement; an invalid run
may not carry interpretation.

**Warning** — the projection is correct but the corpus has a gap, so startup succeeds and reports:
a registered source whose repository-relative locator does not exist, a registered session with no
`sessions/<id>/` directory, a session no node cites, a registered source that neither a node nor a
claim cites, and a claim appearance document that is safe and canonical but does not exist.

The backend never repairs a record and never writes to the corpus. Canonical inconsistencies are
reported for human decision.

`/api/v1/diagnostics` makes this boundary machine-readable: `validation_scope` is
`runtime_projection`, while `repository_semantic_validation` is `external_precondition`. Its
`valid` status must not be interpreted as an in-process execution of the PowerShell semantic rules.

**Why ranking is a weighted signal list and not a retrieval model.** Phase 1E ordered results by
four categorical match classes and Phase 1G ordered composed results by class and ID, which left
two questions a search response could not answer: why one `title_substring` hit sat above another,
and why one `all_terms` hit sat above another at all. The honest answer in both cases was "the
canonical ID sorted first", which is reproducible but is not relevance. Phase 1I closes that
without importing an information-retrieval system.

A signal is one named, yes-or-no fact about how the normalised query reached one record's own
canonical fields — the query *is* the ID, the display field *begins* with the query, every term
occurs in the display field — carrying a fixed integer weight. `relevance_score` is the sum of the
weights of the signals that are true, and `match_signals` lists exactly those. Defining the score
as the sum of its own explanation is the whole design: the two are one fact stated twice and cannot
drift apart, a client can recompute the number it was sent, and comparing two results' signal lists
shows precisely which signal separated them. An opaque score would allow none of that.

The weights are powers of two, each strictly greater than the sum of every weight below it, which
makes the integer sum behave exactly as a lexicographic comparison of the signal list: one strong
signal always beats any accumulation of weak ones. That property is what lets the score be read off
the table rather than trusted, and it is enforced by a test rather than asserted in a comment. It is
also what keeps Phase 1I additive rather than disruptive — for a literal search the score is a
strict refinement of the Phase 1E class precedence, so results still emerge in `id_exact`,
`title_exact`, `title_substring`, `field_substring` order and are merely sorted within each class
now. `match_kind` is derived from the signals rather than computed a second time, so the coarse
class a client reads and the score it is ordered by cannot disagree.

What is excluded is excluded on the same grounds every other exclusion in this backend is. There is
no term frequency, no field-length normalisation, no IDF, no popularity, click or usage weighting,
no recency and no learned component, because each is either an unsourced judgment about what a
reader meant or a measurement this service has no business taking. Ranking reads no clock, no
counter and no random source. It also cannot see a record's relationships, provenance or resolved
context: context is resolved after ranking and paging precisely so that it is structurally
impossible for a relationship count to reach the ordering. A record's score is a function of the
query and that record alone, which is why the scope filter, the page window and the context control
cannot move it.

The ranking stage sits inside the bounded pipeline that already existed, and the stage order is
unchanged: validate, match, restrict to scope, rank the complete set, count facets, page, resolve
context. Ranking the complete set before paging is what makes the first page the top of the ranking
rather than an arbitrary window of it, and counting facets before paging keeps them describing the
whole filtered set exactly as Phase 1H defined.

**Why related-knowledge discovery is a ranking of authored references and not a similarity model.**
Phase 1E answers "where in the corpus does this text appear" and Phase 1C answers "what is the
neighbourhood of this graph vertex". Neither answers the question a reader actually has while
holding one record and no query: which of the things this record is connected to is worth opening
first. Phase 2A adds that as one read-only route without adding a second definition of what
"related" means.

It derives nothing. The canonical references of every searchable record are already resolved once,
at startup, by the Phase 1F context layer — authored references and their documented reverse reads,
each carrying the canonical field it came from — and discovery reads that projection. Re-deriving
the connections would create two definitions of one repository fact that are free to disagree,
which is precisely the failure the traversal and context layers were each careful to avoid. What
the phase adds is precedence and bounds: the context layer returns everything a record references,
a discovery answer has to be short, and something therefore has to decide what is cut. "Whatever
the corpus listed last" is not a decision.

The rule that graph edges are explicit-only is not weakened here, and the reason it would have been
tempting to weaken it is worth naming. A "what should I read next" surface is exactly where an
embedding is conventionally reached for, and the conventional argument — that the corpus obviously
knows two records are about the same thing even where no field says so — is the same argument the
Phase 1A edge rule already refuses. A projection manufacturing edges from proximity inserts
unsourced claims into a corpus whose whole discipline is that claims carry provenance; a projection
that ranked *authored* references by their apparent similarity would do the same thing one step
later, presenting a machine's guess about meaning as though AudioMuse had said it. Every reason a
caller receives from this route is checkable against the canonical file it names.

**Why precedence is keyed on the canonical field rather than the relation name.** A related item's
rank has to come from something closed, or the ranking is only defined for the corpus that happens
to exist. Relation names are not closed: node-to-node edges carry the relationship-type IDs from
`schemas/relationship-types.yaml` and their declared inverses, so the vocabulary grows whenever the
corpus adds a type, and a table keyed by relation name would either have to be edited in lockstep
with a canonical contract or would silently drop new types into a fallback. Canonical field names
are closed, are already carried on every relation the context layer emits as `origin`, and name
exactly the fact the precedence is about — which field said so. The closed set is written down and
a test walks it, so a canonical field cannot reach the discovery contract without a deliberate
decision about where it ranks.

The seven classes are, strongest first: `conceptual` (`node.relationships`), `evidential`
(`claim.evidence`), `attributive` (`claim.attribution`), `assertional` (`claim.appears_in`,
`claim.derived_from`), `contextual` (`node.sources`, `node.session_origin`), `referential` (the
vocabulary and experiment reference lists) and `navigational` (`vocabulary.related_terms`,
`experiment.related_experiments`). The typed concept edge leads because it is the only connection
AudioMuse authors specifically as a knowledge relation. Evidence is kept ahead of attribution
because `docs/claim-provenance-model.md` treats what stands behind a statement and who says it as
different facts and the first is the one a reader checks — the same distinction that already keeps
`sourced_from` and `supported_by` apart. Curated navigation comes last because `vocabulary/README.md`
states that related terms are human navigation only and imply neither equivalence nor a graph edge,
which is the weakest thing any canonical field here says.

**Why the rank is an ordinal and not a weight.** Search ranking sums weights because one hit can
fire several independent signals at once and the score has to combine them, and the powers of two
exist to make that sum behave as a precedence order. A related item's precedence comes from exactly
one canonical field, so there is nothing to add up, and introducing a weight would invite exactly
the arithmetic this layer must not perform: three weak references outranking one strong one, which
is how a precedence order quietly becomes a popularity score. There is no confidence, similarity,
relevance percentage or probability anywhere in a discovery response, for the same reason there is
no measurement in a claim projection the corpus did not author.

**Why one item per destination, and why the strongest connection wins.** Two records are often
connected by more than one canonical field — a claim that both names a node in `appears_in` and
derives from it, a claim that both cites a source and credits it. The reader is being offered a
record to open, not a list of edges to read, so the destination appears once. The strongest
connection becomes the reason and decides the item's rank, and the rest are reported as bounded
further evidence in the same order, with the true total kept. Choosing the strongest rather than
the first is what makes deduplication independent of which loop ran first; reporting the others
rather than discarding them is what keeps the response from claiming a pair is connected in fewer
ways than it is. Evidence is a list inside a list, so it carries its own cap: without one the
payload would be the product of two corpus properties rather than of two constants.

**Why the ordering is what it is.** Items sort by the primary reason's precedence, then authored
before derived, then the destination's canonical class in model order, then its canonical ID. The
direction key sits second rather than first because the canonical field is the stronger statement
about what kind of connection this is and which way it was written is secondary to what it says.
The obvious alternative for the third key was the relation name, the way `sortContextRelations`
already groups a context list; it was rejected because the two lists are read differently. A
context list is read as a record's reference structure, where grouping by relation *is* the
structure. A discovery list is read as a set of records to choose between, where the class of
record is what a reader is choosing among and the relation name within one precedence class says
almost nothing to separate them. A destination appears exactly once, so class and ID are unique and
the order is total: no two items compare equal, which is what makes two runs and two independently
built indexes produce the same bytes rather than merely the same set.

**Why there is no depth, and why the traversal bounds are not reused.** Discovery is one hop by
construction. The references are resolved at startup, so a request is a map lookup, a group, a sort
and a slice — there is no frontier, no visited set, no recursion and no expansion, and a depth
parameter would have nothing to control. Reusing `MaxTraversalDepth` would suggest a walk that does
not happen. A caller who wants the neighbourhood of a graph record still uses
`/api/v1/graph/entities/{type}/{id}/traverse`, which is the route shaped for that question, and
this phase deliberately does not become a second, differently spelled traversal surface.

The bounds it does declare are its own: a default of 25 items, a hard maximum of 100, at most 5
canonical connections reported per item, and at most 2,000 canonical relations examined per
request. They are service constants rather than configuration for the reason the traversal and
context bounds are — API safety invariants, not deployment choices. The first three are the
statement that covers the response: no discovery request serialises more than 500 canonical
relations, a quarter of the 2,000 the API already states no single request exceeds. The fourth
covers the work behind it at that same 2,000, so one number now covers both halves of the API-wide
claim.

The scan bound is the one that had to be added rather than inherited. Without it the cost of a
request would be bounded by a property of the corpus — the widest context any single record happens
to have — rather than by anything the backend declares, which is a bound in practice and not in
contract; the two diverge exactly when the corpus outgrows the size the other bounds were reasoned
about at. Relations are counted as they are examined, before the destination scope and the
self-reference exclusion are applied, so a narrow class filter cannot buy a larger scan while
admitting almost nothing from it. A scan that stops early is reported rather than hidden, because
it is the one bound that changes how another field must be read: it makes the eligible count a
floor instead of an exact total, and a caller must not have to discover that by comparing counts
across requests. Every bound that shaped an answer is echoed in it for the same reason a truncated
list reports its true total — a bounded answer that does not say what bounded it cannot be told
apart from a complete one.

Against the corpus today the widest discovery result is 75 items, the widest single item carries 3
connections, and the widest scan examines 75 relations of the 2,000 allowed, so all three ceilings
have real headroom while the default does shorten the handful of genuine hub records, which is
what a default is for. As the encyclopedia grows the cost of one request stays a map lookup plus a
sort of the eligible set, because the projection is built once at startup and no request reads the
corpus; what grows is the eligible count a hub reports, and that is a number in the response rather
than work in the request, until a record's context reaches the scan ceiling, at which point the
response says so.

**Why `limit` is clamped where `depth` is refused.** The two contracts differ deliberately. A
silently reduced depth would let a caller believe they had seen a whole neighbourhood, so an
out-of-range depth is refused. A clamped limit returns the front of the same ordering the caller
asked for, and the response echoes both the applied limit and the exact eligible total, so the
clamp is visible rather than silent. Following the paging contract every other route already uses
also keeps one spelling of `limit` from behaving differently depending on which endpoint it was
sent to.

**Why the whole request is validated before the start is resolved.** A request that is both
malformed and names a record the corpus does not contain is refused as malformed rather than
reported as a miss. The two failures ask different things of the caller — "fix the query string you
wrote" and "that record is not in this corpus" — and a caller told only about the identifier would
fix it, resend, and be refused a second time for a mistake that was already visible in the request
they sent.

The traversal routes are a partial precedent rather than the model for this. An out-of-range depth
is already refused on a root that does not resolve, because that bound is checked at the edge, but
the `relationship` and `target_type` vocabularies are resolved after the root, so a mistyped filter
there is still reported as a missing entity. Phase 2A does not reach into a Phase 1C contract to
change that; it decides only what its own route does, and the other order would have added a second
surface that behaves that way rather than leaving one fewer.

**Why the starting classes are the search six and not the graph four.** A reader can be holding a
vocabulary entry or an experiment definition and want to know where to go next, and neither is a
graph vertex. Discovery therefore starts from the six searchable classes, and the boundary
`traversal.go` draws is untouched: a vocabulary entry named here is a navigation reference, never
an `EntityRef`, a graph vertex or an edge endpoint. `experiment_run` is refused as a start for the
reason it is refused as a search class — a run's prose is its observations and interpretation, and
a discovery surface over it would make a connection read as an evidence assertion. The route is
therefore `/api/v1/related/{entity_type}/{id}` rather than a fifth path under `/api/v1/graph/`,
because nesting it there would enrol two non-graph classes in the graph by URL alone.

**Why the destination scope reuses the Phase 1H class list unchanged.** It is the same restriction
over the same six classes, and a second spelling would be a second contract to keep in step: a
caller who moved a malformed list from one route to the other would be told two different things
about one mistake. The validation function is shared, and so is the renderer that turns its errors
into messages. Only the set spelling is offered — `/api/v1/search` carries a single-class `type`
alongside it only because `type` predates the list and an existing contract had to keep working, and
a new route has no such history to preserve.

**Why the relationship filter is the precedence class and not the relation name.** Phase 2B adds
`relationship_types` to the same route, restricting which canonical connections may explain and
rank an item. Its vocabulary is the seven precedence classes, for the reason the precedence itself
is keyed on the canonical field: relation names are not a closed set and grow whenever
`schemas/relationship-types.yaml` does, so a filter keyed on them would either drift from a
canonical contract or accept values with no defined rank. Canonical field names are closed but are
finer than a caller navigating records is choosing among, and a filter over fifteen field names
would expose the projection's internals as a request vocabulary. The precedence classes are closed,
already carried on every reason, already documented as the ranking model, and already the thing a
reader is choosing between — "show me what supports this" is a class-level question.

The consequence is that one list in `domain.related` is now four things at once: the ranking table,
the filter allowlist, the explanation codes, and the values an `invalid_query` message enumerates.
That is deliberate rather than incidental. A separate filter vocabulary over a seven-value closed
set would be a second spelling free to drift from the table that decides precedence, and the
drift would be invisible — a caller could name a scope the ranking has no rank for.

**Why the filter removes relations rather than filtering finished items.** A destination is often
reached by several canonical fields at once. Filtering after grouping would mean an item admitted
by its strongest connection kept reporting excluded connections as its evidence, so a filtered
response would explain an item by a class the caller removed; and an item whose strongest
connection was excluded would vanish even where a weaker admitted connection also reaches it, so a
filter would silently drop records that satisfy it. Removing the ineligible relations first makes
the eligible set exactly "the connections the caller asked about", and the winning reason, the
evidence, the count and the ranking are then computed over that set without any of them needing to
know a filter ran. Filtering therefore also happens before the limit: a caller asking for one class
and twenty-five items receives up to twenty-five items of that class, not what survives of the
first twenty-five of the unfiltered ranking.

The scan ceiling is counted before every exclusion, so neither filter can buy work: the relations
examined for one start are the same number whether a request names no class, one class or all
seven. A filter restricts what an answer contains and never enlarges what is behind it.

**Why the explanation is a lookup and not generated prose.** Phase 2A's reason is structured
because prose would have to be generated, and generated prose is the one kind of explanation this
backend cannot check. Phase 2B adds one human-readable field without weakening that: `explanation`
is one of eight fixed strings selected by the precedence class alone, so it is checkable against a
table, identical across calls, and adds no fact `priority` did not already carry. It exists because
a machine-readable vocabulary term is not something a person reading one response can expand.

The sentences say what a kind of canonical field asserts, never anything about the pair of records
they appear on. None names an endpoint, interpolates a record's text, or says a destination is
relevant, important or similar — none of which the corpus states. A template with a title
interpolated into it would have been the obvious alternative and is exactly what this refuses: it
would read as a claim about two specific records, which only the corpus may make.

**Why continuation is a cursor and not an offset, and why Phase 2A said no to paging at all.**
Phase 2A refused paging on the ground that a discovery list is a set of records to choose between
rather than a result set to work through, and that reasoning still holds for the reader it was
written about — the default answer is unchanged, and a caller who only wants to know what was left
still reads `counts.eligible` rather than walking anything. What it did not serve is the client
rendering a well-connected record's whole neighbourhood, which could raise `limit` to the ceiling
and then had no way to reach the remainder of an ordering the response told it existed.

An `offset` was refused a second time rather than reconsidered. An offset is a count into an
ordering the caller cannot see, and a count means something different the moment the ordering
changes: two requests either side of a corpus reload silently drop or repeat records, with nothing
in either response to mark it. A cursor naming the last item returned has the opposite property —
it is either found at the rank it claims, or it is refused. That is why the token carries the
destination's class, its canonical ID and its two ranking keys, and why a cursor the rebuilt
ordering does not contain is a `400` rather than a resume from the nearest survivor.

The pair (class, ID) is enough to name a position unambiguously because a destination is grouped
exactly once, so no two items of one result share it — including in the common case where dozens of
items tie on precedence, direction and class and are separated only by the canonical-ID tie-break.
The ranking was not changed to make paging easier, and the tie-break chain paging depends on is the
one Phase 2A already documented.

**Why the token is unsigned, and why that is not a gap.** The repository has no secret-management
contract, no key material and no deployment step that could supply one. An unkeyed digest carried
beside the payload it digests detects only the corruption base64 and a strict decode already
detect, while reading to a client as though it were tamper protection — which is the kind of
security theatre this backend's documentation is otherwise careful not to publish. Inventing a
secret, an environment variable or a persistence layer to sign a cursor would also have added a
deployment requirement to a read-only API that has none.

What replaces a signature is that no field in the token is trusted on its own. The start, both
normalised scopes and the effective limit must equal what the presenting request independently
resolves, and the cursor must name a record the rebuilt ordering actually contains at the rank the
token claims. An attacker editing a decoded payload can therefore produce only a token that is
refused, or one identical in effect to a query string they could have written anyway — on an API
that has no identities to distinguish and no records to withhold. The token is documented as a
continuation cursor and explicitly not as authentication or authorization, so no client builds a
capability on it.

**Why the token binds the limit.** A continuation whose window size differed from the issuing
page's would make "the next page" mean something the caller cannot compute, so `limit` is carried
in the token and compared after normalisation. Omitting it inherits the token's value, which makes
a traversal expressible as "the same URL plus a token"; supplying a different one is refused rather
than silently re-windowing the remainder. Comparing the *effective* limit rather than the caller's
literal is what keeps that from being pedantic: `limit=1000` normalises to the ceiling on every
page and continues, exactly as an omitted limit does.

**Why `has_more` is a new field rather than a reuse of `truncated`.** `truncated` has always meant
"this response does not carry the whole eligible set", and that stays true on every page of a paged
traversal including the last. Overloading it to also mean "you can fetch more" would have made the
final page of a long result unrepresentable: it is simultaneously incomplete and final. `has_more`
is always serialised, including when false, because a boolean that disappears cannot be told apart
from a server that does not implement it, and a client reading its absence as "keep going" would
loop. `next_continuation_token` is present exactly when `has_more` is true, so the two cannot
contradict each other, and no token is ever issued for a window that would come back empty.

**Why an unparseable query string is refused whole.** Every route on this API refuses a parameter it
does not accept, and refuses one supplied twice, on the stated ground that silently dropping a
filter a caller believed was applied returns a result set that does not mean what they think it
means. Phase 2C found the same drop reachable one layer earlier. Go's `r.URL.Query()` discards the
error from `url.ParseQuery` and returns whichever pairs it could read, so a query string carrying an
invalid percent-escape or a semicolon separator arrived with the malformed pairs simply absent — and
an absent filter is an unfiltered request. `?entity_types=%zz` answered `200` with the complete
unrestricted result set, no echo, and nothing in the body to mark the loss.

The refusal is placed on the shared parameter guard rather than on the related-knowledge handler,
because the defect is not this route's. It is a property of how every handler reads its query string,
and a fix applied to one of them would leave the same silent drop on the other twenty-one while
splitting one rule into two spellings — which is the drift the shared renderers on this API were each
written to avoid. Refusing the whole string rather than the pairs that failed follows from the same
reasoning: a request whose meaning cannot be established is refused rather than approximated.

The guard also decides *which* violation a multiply-malformed request is told about, by parameter
name rather than by Go map order. An error body is part of a response, and this API's contract is
that an identical request returns an identical response; a refusal that cited a different rule on
each run would be the one part of the surface where that stopped being true.

## Known limitations (through Phase 2D)

- Corpus changes require a process restart.
- Search is lexical substring matching only; there is no semantic retrieval, embedding or learned
  ranking model, and neither result scope, facets nor relevance ranking adds one. `relevance_score`
  is a fixed weighting of explicit lexical signals over the records substring matching already
  found; it orders results, never filters them, and cannot make an unmatched record reachable. `/api/v1/search?query_mode=all_terms`
  composes several literal terms into one record-level request and is bounded to whitespace
  splitting and 2 to 8 distinct terms: no boolean operators, no quoting, no negation, no wildcards,
  no regular expressions and no field-scoped terms. Terms must all occur inside one canonical
  record; two terms held by two records never combine, and search-result context is resolved after
  matching and can never supply a term.
- Query composition does not widen the search corpus. The searchable field set is unchanged in both
  modes, so node markdown bodies, source notes, experiment procedures and every experiment run stay
  outside discovery.
- Result scope filters whole classes and nothing finer. `/api/v1/search?entity_types=` restricts
  which of the six searchable classes may be returned; there is no field-scoped filter, no per-class
  field selection, no negation, and no scope filter on the per-layer list endpoints. It may not be
  combined with `type`, and it cannot make an unsearchable class reachable: `experiment_run` is
  refused in a class list exactly as it is refused as a `type`.
- Search facets describe one axis, the searchable class of each hit, counted over the complete
  filtered result set before paging. There is no facet over any other canonical field and no
  caller-supplied facet field, and a facet count is not a score: nothing in it orders or rates a
  result set.
- No persistence, no database, no cache beyond the startup index.
- No file watcher, background worker, or scheduled ingestion.
- No frontend and no graph visualization.
- No LLM or AI integration of any kind.
- Node `experiments:` is the one canonical reference field still carried unresolved. No repository
  validator treats it as a reference list and every current node leaves it empty, so resolving it
  would mean inventing a contract rather than reading one.
- Vocabulary entries, experiments, and experiment runs are read surfaces adjacent to the graph.
  None becomes a vertex or an edge, and `GET /api/v1/graph` is unchanged for an unchanged corpus.
- `/api/v1/experiment-runs` has no lexical search. A run has no authored prose identity, and
  searching its observation statements would make a text hit mean "this run observed that", which
  is an evidence assertion a list projection has no business making.
- The semantic confidence, dispute, attribution and origin-term rules in
  `schemas/claim.schema.yaml` are enforced by `tools/validate-claims.ps1`, not by the backend.
- `appears_in: session` is a canonical reference kind that no current claim record uses, so
  `GET /api/v1/claims?session_id=` and `GET /api/v1/sources?session_id=` answer correctly and
  return nothing against today's corpus.
- Graph traversal is deliberately bounded: depth 1 to 3, 500 entities and 2000 relationships per
  request, two GET routes, no query language, no caller-supplied traversal program, no paging and
  no mutation. The adjacency is derived at startup and never persisted.
- The traversal graph addresses only four record classes: session, node, claim and source. Phase 1D
  parses the practice layer and resolves claim `appears_in: vocabulary` and
  `derived_from: experiment_run` references, but resolution is not membership: neither reference
  produces a graph entity or an edge, and `appears_in: document` remains unresolved as before. A
  vocabulary entry, experiment or experiment run is never a traversal entity.
- Graph traversal across the practice layer is not implemented. Its references resolve
  deterministically; whether any of them is a typed graph relation is a graph-contract decision
  that has not been made.
- An experiment run connects to a claim only where a claim record says so through
  `derived_from: {kind: experiment_run}`. No such record exists today, the backend never
  synthesises the link from an observation, and such a reference would not become a traversal edge
  if one did.
- A registered session and its registry entry are addressed as two entities that share an ID, and
  no edge is emitted between them: they are one canonical record seen through two projections.
- Related-knowledge discovery is one hop and nothing else. `/api/v1/related/{entity_type}/{id}`
  returns records the start directly references, or that directly reference it, ranked by the
  canonical field each connection came from. There is no transitive discovery, no depth, and no
  caller-supplied ranking or weighting: the ordering is a property of the corpus rather than of the
  request. It is bounded to 25 items by default, 100 at most, and 5 reported connections per item.
  Phase 2D adds a continuation cursor over that same ordering and moves none of those bounds: a
  page is a window on the ranked eligible set, the scan ceiling is per request rather than per
  traversal, and paging cannot reach a record an unpaged request at the ceiling could not.
- Continuation creates no session and no server-side state. `continuation_token` is an opaque,
  versioned, bounded cursor naming the last item a page returned; each page is re-validated and
  rebuilt from the immutable startup index, and nothing is stored between requests — no result set,
  no cache, no cursor row, no per-caller record. It is not authentication and not authorization,
  and it is deliberately unsigned: the repository has no secret-management contract, and every
  field it carries is checked against what the presenting request independently resolves rather
  than trusted from the token. A token is refused, never approximated, when the request it is
  presented with differs in start, either scope or effective limit, or when the record it names is
  no longer at that position. There is still no `offset`, and the reason is unchanged: a count into
  a rebuilt ordering silently repeats or drops records whenever the ordering has changed.
- Discovery filters on two axes and nothing finer. `entity_types` restricts the destination class
  and `relationship_types` restricts the precedence class of the connection; both are closed sets,
  both refuse a blank, repeated, mis-cased or unknown member rather than repairing it, and neither
  falls back to an unfiltered discovery. There is no filter on the relation name, on the canonical
  field, or on direction, no negation, no per-class limit, and no way to name a class more than
  once. `relationship_types` cannot reorder anything: it removes ineligible connections before the
  limit is applied, and the precedence among whatever survives is the unchanged closed table, so
  the order the classes are written in has no effect on the response.
- A discovery explanation is a fixed sentence, not generated text. `explanation` is one of eight
  strings selected by the precedence class, identical across calls, and it names neither endpoint.
  Nothing in it is produced for the pair of records it appears on, and no model, template
  interpolation or corpus text contributes to it.
- Discovery ranks authored references and measures nothing. There is no similarity, embedding,
  vector, keyword-overlap or co-occurrence input anywhere in it, and no confidence, relevance or
  probability in its output. A record is related to another if and only if some canonical record
  wrote down a reference between them; AI- or embedding-based discovery is not implemented and is
  not part of this phase.
- Discovery starts from the six searchable classes and never from an experiment run, for the reason
  runs are not a search class. It resolves no connection the Phase 1F context layer does not already
  hold, so a canonical field that layer does not read — node `experiments:`, `appears_in: document` —
  is invisible to it as well.
- Discovery is stateless and per-caller state does not exist. There is no reading history, no
  popularity, no click weighting and no personalisation; two callers asking about one record are
  told the same thing, and two callers presenting one continuation token are told the same thing as
  each other and as the caller the token was issued to.

## Future work

Deferred, not implemented: graph traversal across the practice layer, richer diagnostics, query
syntax beyond the two documented composition modes, facets over any axis other than the searchable
class, graph visualization, semantic retrieval, and MLLM experimentation. Deterministic
related-knowledge discovery is no longer deferred — it shipped as
`GET /api/v1/related/{entity_type}/{id}`, with relationship-scope filtering and per-connection
explanations added to the same route — and two extensions to it are recommended without being
implemented or promised: a per-class breakdown of the eligible set in the shape search facets
already use, and a bounded intersection answering "related to both of these". A third, exposing the
precedence table as a read-only contract endpoint, is now more useful rather than less: those seven
class names are a request vocabulary as well as a response one, so a client filtering by them is
hard-coding a list it could be served. Each would be its own phase with its own contract. Multi-hop, similarity-ranked or model-generated discovery is not on
that list and remains refused on the grounds above. Deterministic multi-term
composition is no longer deferred — it shipped as `query_mode=all_terms` — nor is multi-class
result scope, which shipped as `entity_types` and filters rather than searching, while richer
syntax and semantic retrieval remain separately reviewed future phases. The Phase 1C contract
is shaped to be useful to a future read-only graph inspector, provenance-path view or
claim-confidence overlay without any of them being implemented here, and without the backend being
distorted around a hypothetical frontend.
