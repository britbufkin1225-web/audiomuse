# Research Gap Triage and Knowledge Deepening Plan

Phase 11 is the human review layer above Phase 10. Phase 10 measures; Phase 11 decides. This document
records what was reviewed, what was concluded, and what Phase 12 is allowed to do. It creates no
canonical knowledge and changes no canonical data.

Every statement below is labelled by kind:

- **Fact** — a directly verifiable canonical repository property.
- **Measurement** — a number produced by the Phase 10 generator.
- **Observation** — a deterministic reading of facts and measurements.
- **Interpretation** — human judgement about what an observation means.
- **Recommendation** — proposed future work, not current knowledge.
- **Future work** — explicitly deferred or out of scope.

Nothing in the recommendation or future-work sections is canonical AudioMuse knowledge.

## Phase 11 purpose

Convert the bounded research-gap candidates produced by Phase 10 into a small, defensible research
plan. Phase 11 owns three stages of the knowledge pipeline and no others:

```text
canonical repository facts
        v
deterministic coverage measurement      <- Phase 10
        v
bounded research-gap candidate          <- Phase 10
        v
human review                            <- Phase 11
        v
accept / defer / dismiss / watch        <- Phase 11
        v
small prioritized research plan         <- Phase 11
        v
focused knowledge deepening             <- Phase 12
```

Phase 11 does not generate knowledge because a metric is low, does not adjust Phase 10 thresholds,
and does not treat a candidate as a defect.

## Phase 10 baseline

**Fact.** Phase 10 is implemented by `tools/build-knowledge-coverage.ps1` (generator),
`tools/validate-knowledge-coverage.ps1` (independent recomputation), and
`tools/test-knowledge-coverage.ps1` (adversarial tests). It is documented in `docs/knowledge-coverage.md`
and `indexes/README.md`.

**Fact.** Canonical inputs are node Markdown front matter under `nodes/`, `vocabulary/entries/*.yaml`,
`sources/source-registry.yaml`, `experiments/records/*.yaml`, `experiment-runs/records/*.yaml`,
`schemas/node.schema.yaml` (domain enum), and `schemas/relationship-types.yaml` (type enum). Generated
outputs are `indexes/knowledge-coverage.md` and `indexes/knowledge-coverage.json`; both are read-only
projections and neither is authoritative.

**Fact.** Coverage states are `covered`, `partial`, `unlinked`, and `not_applicable`.

- `covered` — at least one explicit canonical reference of that kind exists.
- `unlinked` — zero such references exist, and the dimension applies to the node.
- `not_applicable` — used only for experiment and run evidence on a node that declares no
  `practical_applications`; the applicability gate is the node's own declaration.
- `partial` — used only for completed-run evidence: a linked experiment definition exists and no run
  with canonical status `completed` does. It does not assert that any run record exists.

**Fact.** Six candidate rules exist in the generator. Each is a fixed zero-or-one-count test:

| Rule | Trigger | Fired now |
|---|---|---:|
| `source_coverage` | node references zero registered sources | 0 |
| `session_coverage` | node maps to zero sessions | 0 |
| `vocabulary_bridge` | zero vocabulary entries reference the node | 5 |
| `practical_evidence` | node declares practical applications and zero experiments reference it | 6 |
| `relationship_coverage` | node's inbound plus outbound typed edges total at most one | 0 |
| `domain_representation` | canonical domain contains at most one node | 11 |

**Measurement.** Repository overview at the Phase 11 baseline: 15 nodes, 45 relationships, 11 declared
relationship types, 5 registered sources, 3 sessions, 38 vocabulary entries, 3 experiments, 2
experiment runs, 22 research-gap candidates.

**Fact.** Phase 10 documents its own limitations. `relationship_type_count` reports declared types, not
types in use. Run evidence is composed only from an experiment record's `node_refs` and a run record's
`experiment_id`; no filename, directory, similarity, or shared-session inference contributes. Coverage
is explicitly not a quality score, and the documentation states that AudioMuse has no defensible
denominator for universal completeness.

**Interpretation.** The baseline is reproducible: the validator reparses canonical content and
independently recomputes every row and the full candidate set, so a Phase 11 reviewer can trust that
the 22 candidates are exactly what the canonical files imply, and can regenerate them at any time.

## Candidate inventory and decisions

**Measurement.** 22 candidates, in three types. Every candidate below was reviewed individually.

### Decision summary

| Decision | Count |
|---|---:|
| accept | 7 |
| defer | 5 |
| dismiss | 6 |
| watch | 4 |
| **total** | **22** |

### `vocabulary_bridge` — 5 candidates

**Observation.** All five flagged nodes (`midi`, `recording`, `rhythm`, `sequencing`, `synthesis`) share
one structural cause rather than five independent ones. All 38 vocabulary entries derive from Session 1
material: vocabulary domains present are `acoustics` (19), `psychoacoustics` (9), `digital-audio` (8),
and `dsp` (2). The `rhythm-time`, `synthesis`, and `recording` domains contain zero vocabulary entries.
Session coverage records `session-03-history-of-electronic-music` at vocabulary_count 0.

**Interpretation.** This is one gap — the vocabulary layer has not been extended past Session 1 — not
five node defects. Session 2 supplies the missing terminology directly and already organizes it under
explicit headings.

| Candidate | Decision | Gap type | Rationale |
|---|---|---|---|
| `node:midi` | accept | `cross_reference_gap` | Session 2 devotes a dedicated section to MIDI as control data rather than audio, and its Digital Audio term list names MIDI, sequencer, quantization, automation, and beatgrid. The terminology exists in source material and has never been promoted. |
| `node:rhythm` | accept | `cross_reference_gap` | The `rhythm-time` domain has zero vocabulary entries. Session 2 supplies tempo, meter, groove, swing, and beatgrid as discussed terms, not passing mentions. |
| `node:sequencing` | accept | `cross_reference_gap` | Same zero-vocabulary domain and same source block. Sequencing terminology (step, pattern, automation, loop) is the practical half of the rhythm-time vocabulary and should not be split from it. |
| `node:synthesis` | accept | `cross_reference_gap` | The `synthesis` domain has zero vocabulary entries, yet the node's own `core_concepts` (oscillator, spectrum, envelope, filter, modulation) are exactly the terms Session 2 develops. Only `filter` exists in vocabulary, scoped to `dsp`. |
| `node:recording` | defer | `cross_reference_gap` | Legitimate, but partially covered already: `transducer` exists as a vocabulary entry with empty `node_refs`. The cheaper correction is re-pointing and extending existing digital-audio entries, which is lower value than opening the two domains that have no vocabulary at all. |

### `practical_evidence` — 6 candidates

**Fact.** All three experiment definitions carry `status: proof`. Both run records
(`near-frequency-beating-planned-a`, `waveform-timbre-comparison-planned-a`) carry `status: planned`
with `run_date: null` and empty `observations`, `measurements`, `equipment`, and `software` arrays.
`frequency-room-position` has no run record at all.

**Measurement.** `completed_experiment_runs.count` is 0 for all 15 nodes.

**Observation.** No performed evidence exists anywhere in the repository. The six flagged nodes lack an
experiment *definition*; the nine unflagged nodes have definitions but still lack a completed run. The
`practical_evidence` rule therefore under-reports the actual evidence gap, which is repository-wide.

**Interpretation.** Adding six more experiment definitions would raise the coverage counts while leaving
the real gap untouched. Performed evidence is the scarce thing, not definitions.

| Candidate | Decision | Gap type | Rationale |
|---|---|---|---|
| `node:vibration` | accept | `practical_evidence_gap` | Graph root: it `produces` sound, is `characterized_by` frequency, and `influences` resonance. It has the fewest session origins of any node (1) and its declared future question — comparing airborne and structure-borne vibration — is directly executable with the equipment class the existing experiments already assume. Cheapest genuine evidence gap to close. |
| `node:rhythm` | defer | `practical_evidence_gap` | A real gap. Beat, groove, and microtiming work is most defensible alongside DJ and turntablism material rather than ahead of it, and the node's own future question warns against reducing groove to a grid. Sequence it after the DJ target. |
| `node:sampling` | defer | `practical_evidence_gap` | Best-connected node in the graph (6 inbound, 3 outbound, 5 type diversity, 4 sources, 3 vocabulary entries). Its practical applications are real, but a sampling experiment would overlap `waveform-timbre-comparison` and needs sampler tooling the repository has not established. |
| `node:midi` | dismiss | not a knowledge gap | MIDI is control data, not sound. An experiment here would document software behaviour rather than an observable acoustic phenomenon. The node's own definition states this distinction. The measurable part — timing resolution and its effect on expression — is its recorded future question and belongs to a later, better-equipped phase. |
| `node:sequencing` | dismiss | not a knowledge gap | Its declared practical applications ("drum and melodic programming", "arrangement and automation", "control of samplers and synthesizers") describe tool operation. Documenting a DAW workflow would not produce observation or measurement evidence under the Phase 7 and Phase 8 contracts. |
| `node:recording` | watch | `practical_evidence_gap` | Genuinely important and eventually measurable, since transduction is a physical process. Blocked on equipment: every `equipment` array in every run record is empty, so the repository has never recorded that a microphone or interface is available. Revisit once any completed run establishes real equipment. |

### `domain_representation` — 11 candidates

**Fact.** The rule tests the domain enum in `schemas/node.schema.yaml` against node counts. Eight of the
fifteen declared domains contain zero nodes; three contain exactly one.

**Interpretation.** A domain with zero or one node is a scope statement, not a defect. AudioMuse declared
a wide domain enum early so that future work has somewhere to land. The useful question is not "which
domains are small" but "which small domains already have unused source evidence".

| Candidate | Nodes | Decision | Gap type | Rationale |
|---|---:|---|---|---|
| `domain:dj-turntablism` | 0 | accept | `missing_knowledge` | Strongest evidence-to-representation mismatch in the repository. Session 1 has dedicated "Vinyl scratching" and "DJ monitoring" sections; Session 2 has a full section on scratching as real-time sample manipulation plus an explicit twelve-term "DJ / Turntablism" vocabulary block; Session 3 covers the growth of DJ culture. The directory `nodes/dj-turntablism/` exists and is empty. |
| `domain:history-culture` | 0 | accept | `session_depth_gap` | Session 3 is the largest source in the repository (32 KB) and is entirely historical. It maps to 7 nodes, contributes 0 vocabulary entries, and produces no node in its own domain. The largest single source has the thinnest durable representation. |
| `domain:music-theory` | 0 | defer | `undeveloped_territory` | Substantial Session 2 evidence: dedicated sections on melody, the octave, harmony, consonance and dissonance, texture, structure, and form. Deferred because of size, not weakness — it is the largest single unclaimed territory and would consume a bounded Phase 12 on its own, and `pitch`, `timbre`, and `rhythm` already carry its perceptual half. |
| `domain:mixing-mastering` | 0 | defer | `undeveloped_territory` | Session 2 lists eleven mixing and engineering terms, but tight matching across all three sessions finds roughly a dozen occurrences of mixing, mastering, or headroom in total. Legitimate territory that follows naturally from `recording`; thinner evidence than the accepted targets. |
| `domain:audio-hardware` | 0 | watch | `undeveloped_territory` | Loudspeakers, amplifiers, and interfaces appear only in passing (about seven lines across Sessions 1 and 3). The `transducer` vocabulary entry is the only durable trace. Becomes important when measurement equipment enters the project. |
| `domain:machine-audio` | 0 | watch | `undeveloped_territory` | Real but thin: one Session 2 section on machine learning and music, one Session 3 section on AI in production, and stem separation mentioned in both. AudioMuse has no measurement or DSP depth to anchor it yet, and premature work here would pull the project toward software the scope document rules out. |
| `domain:spatial-audio` | 0 | watch | `undeveloped_territory` | Session 2 has a dedicated section on music as spatial, Session 3 covers 2020s immersive formats, and `frequency-room-position` already exercises room behaviour. Currently carried by acoustics vocabulary (reflection, reverberation, standing wave, room mode, absorption, diffraction), which is adequate for now. |
| `domain:dsp` | 1 | dismiss | not a knowledge gap | `digital-signal-processing` is among the best-supported nodes: 3 sessions, 3 sources, 2 vocabulary entries, 1 linked experiment, 4 inbound and 3 outbound edges, type diversity 4. Node count 1 is a rule artifact. Its real gap is performed evidence, which the run-evidence dimension already records. |
| `domain:recording` | 1 | dismiss | not a knowledge gap | The `recording` node has 2 inbound and 4 outbound edges with type diversity 4, and participates in `captures`, `enables`, `processes`, and `influences`. Connectivity is healthy; the domain simply has one node. Its separate practical-evidence candidate carries the meaningful signal. |
| `domain:synthesis` | 1 | dismiss | not a knowledge gap | The `synthesis` node has 2 linked experiments, 3 inbound and 4 outbound edges, and type diversity 3. Its own future question already gates child nodes on hands-on comparison, so the correct trigger is performed evidence, not node count. |
| `domain:sound-design` | 0 | dismiss | not a knowledge gap | About four line-level mentions across all sessions, and the `synthesis` node already declares "sound design" as a practical application. A node here now would near-duplicate `synthesis` without adding a distinct claim. |

## Gap-type analysis

**Observation.** Applying only categories the repository justifies, the 22 candidates reduce to four
underlying gaps plus a set of non-gaps:

| Gap type | Candidates | Underlying gap |
|---|---:|---|
| `cross_reference_gap` | 5 | The vocabulary layer stops at Session 1. Sessions 2 and 3 have never been mined for terminology. |
| `practical_evidence_gap` | 4 | No completed experiment run exists anywhere; six nodes additionally lack a definition. |
| `missing_knowledge` | 1 | DJ and turntablism material is discussed across all three sessions with no durable node. |
| `session_depth_gap` | 1 | Session 3 is the largest source and the least converted into durable structure. |
| `undeveloped_territory` | 5 | Declared domains awaiting future sessions, correctly empty for now. |
| not a knowledge gap | 6 | Rule artifacts on well-supported nodes and domains. |

**Interpretation.** Two categories were considered and deliberately not used. `relationship_gap` was not
applied because the `relationship_coverage` rule fired zero times and manual review found no missing
edge that current node prose or sources support. `project_application_gap` was not applied because every
node already declares a `project_connections` entry, so no candidate evidences that gap.

**Observation, beyond Phase 10's reach.** All five registered sources are internal AudioMuse artifacts:
three session transcripts, one vocabulary atlas, and one brand guideline document. The repository has
zero external sources — no book, paper, standard, or manufacturer document. Phase 10 cannot flag this
because its source rule is count-based and every node references at least two sources.

**Interpretation.** This is a `source_depth_gap`, and it is arguably larger than any accepted candidate.
It is recorded here as an observation and folded into the cross-cutting evidence requirements below. It
is deliberately not a Phase 11 target, because targets must be drawn from accepted Phase 10 candidates,
and because acting on it hastily is the single most likely route to a fabricated citation.

## Relationship and domain observations

**Fact.** The `relationship_coverage` rule fired zero times: no node in the repository participates in
one or fewer typed relationships. The `source_coverage` and `session_coverage` rules also fired zero
times.

Five structurally distinctive cases were reviewed. This review was bounded on purpose; the graph was not
redesigned, no edge was created, and no node was optimized for numerical balance.

| Case | Structure | Finding | Action |
|---|---|---|---|
| `psychoacoustics` | 0 in, 5 out, type diversity 1 (all `studies`) | Sufficiently scoped node. `studies` is the semantically correct verb for a discipline acting on phenomena. Diversity 1 reflects correct verb choice, not weak modelling. | No action required |
| `timbre` | 7 in, 0 out, `relationships: []` | Sufficiently scoped node. It is a pure perceptual sink, and the knowledge model explicitly permits an empty outbound list rather than inventing an edge. | No action required |
| `midi` | 0 in, 5 out | Sufficiently scoped node. Nothing in the current graph legitimately acts on MIDI. A `sequencing -> midi` edge was considered and rejected: `midi --used_in--> sequencing` already carries the claim in its clearest direction, and the model forbids mirror edges for navigation. | No action required |
| `recording` domain | 1 node, 2 in, 4 out, diversity 4 | Future knowledge territory, not a missing relationship. The signal is node count; connectivity is healthy. | No action required |
| `dj-turntablism` domain | 0 nodes, empty directory | Genuine missing knowledge territory with real session evidence. | Cross-reference opportunity, deferred to Phase 12 |

**Interpretation.** Phase 10 exposed no structural relationship defect. Every low number reviewed was
either correct modelling or an absence of content rather than an absence of an edge. Where Phase 12
introduces new nodes, candidate edges to `sampling`, `rhythm`, and `recording` must be evidence-tested
against node prose and registered sources under the Phase 4 integrity rules, never assumed from topical
adjacency.

## Selected research-deepening targets

**Recommendation.** Four targets, drawn from the seven accepted candidates. These are proposals for
Phase 12. Nothing here has been implemented, and none of it is canonical AudioMuse knowledge.

### Target 1 — Extend vocabulary into Sessions 2 and 3

**Target.** Terminology coverage for the `rhythm-time` and `synthesis` domains and for musical-control
terminology in `digital-audio`.

**Why it matters.** Vocabulary is the documented bridge in the intended path
`Session -> Vocabulary -> Node -> Sources / Graph Relationships`. Two domains currently have no
vocabulary at all, so the bridge is missing for a third of the node set. This is the highest-value,
lowest-risk deepening available: it uses source material already in the repository and creates no new
conceptual claims.

**Current repository evidence.** 38 vocabulary entries, all Session-1 derived; `rhythm-time` and
`synthesis` vocabulary counts of 0; `session-03` vocabulary count of 0; the nodes `rhythm`, `sequencing`,
`synthesis`, and `midi` with zero vocabulary cross-references; Session 2's explicit Digital Audio and
Mixing/Engineering term blocks; the `core_concepts` already declared on each affected node.

**What is actually missing.** Vocabulary entries for terms the sessions already develop, with
`node_refs` and `session_refs` pointing at existing canonical IDs.

**Recommended deepening form.** Add source research, then add vocabulary entries and cross-references.
No new nodes. No new relationships.

**Evidence requirement.** Each entry must cite the session that develops the term, not merely lists it.
Terms appearing only inside Session 2's taxonomy blocks without discussion should be left out rather
than defined from general knowledge.

**Related AudioMuse territory.** Rhythm and time, synthesis, digital audio, DSP, and the existing
psychoacoustics vocabulary that already handles spectrum and timbre.

**Resolves candidates.** `vocabulary_bridge` for `midi`, `rhythm`, `sequencing`, `synthesis`.

### Target 2 — Establish the DJ and turntablism domain

**Target.** First durable node or nodes in `dj-turntablism`.

**Why it matters.** Turntablism is where AudioMuse's physics, perception, recording, sampling, and rhythm
layers meet a single practice. The scope document names DJing explicitly, and the domain is the clearest
case of substantial source evidence with zero durable representation. Adding it deepens the graph
naturally rather than widening it arbitrarily.

**Current repository evidence.** Session 1 sections on vinyl scratching and DJ monitoring, including the
statement that scratching manipulates time, direction, pitch, and transients simultaneously; Session 2's
section on scratching as real-time sample manipulation, its DJ beatgrid discussion, and its twelve-term
DJ/Turntablism vocabulary block; Session 3's coverage of DJ culture; the `rhythm` node's declared
practical application of beatmatching and beatgrids; the empty `nodes/dj-turntablism/` directory.

**What is actually missing.** A durable concept node for turntablism, and the vocabulary entries that
should precede it.

**Recommended deepening form.** Add source research, add vocabulary, then add one focused node. Review
candidate cross-references to `sampling`, `rhythm`, and `recording` only after the node prose exists.

**Evidence requirement.** Every typed edge must be supported by node prose or a registered source under
the Phase 4 rules. Turntablism practice claims that no session supports must be sourced externally or
omitted. Do not import the twelve-term vocabulary block wholesale; promote only terms the sessions
actually discuss.

**Related AudioMuse territory.** Sampling, rhythm, recording, DSP, and psychoacoustics.

**Resolves candidates.** `domain_representation` for `dj-turntablism`.

### Target 3 — Convert Session 3 into durable structure

**Target.** Representation of the historical and technological-lineage material in Session 3.

**Why it matters.** AudioMuse's stated model treats sessions as chronology and nodes as durable
structure. Session 3 is the largest source in the repository and has been converted the least, which
means the atlas currently under-represents how its concepts came to exist. The `history-culture` domain
was declared for exactly this.

**Current repository evidence.** A 32 KB Session 3 transcript registered as a source with
`relationship: historical`; a session-to-node map listing 7 contributed nodes; session coverage recording
0 vocabulary entries; 0 nodes in `history-culture`; Session 3 sections spanning early electrical
experiments, DJ culture, sampling, MIDI, DAWs, spatial audio, and machine learning.

**What is actually missing.** Either a durable node capturing the lineage claim, or an explicit,
documented decision that Session 3 stays chronology-only with deeper session mapping instead.

**Recommended deepening form.** Expand session mapping first. A focused node is justified only if Phase
12 can state a durable conceptual claim rather than a list of events.

**Evidence requirement.** Historical claims are factual claims and require source support beyond the
session transcript under the Phase 4 evidence rules. Dates, attributions, and firsts must not be
promoted from a single internal source. If external sourcing is unavailable, deepening the session map
without creating a node is the correct outcome.

**Related AudioMuse territory.** Recording, sampling, synthesis, sequencing, MIDI, DSP, and DJ culture.
Note the deliberate boundary with Target 2: Target 2 owns technique and concept, Target 3 owns
chronology and lineage.

**Open design question for Phase 12.** History nodes are a different shape from concept nodes. This must
be resolved before anything is created, and "no node created" is an acceptable Phase 12 outcome.

**Resolves candidates.** `domain_representation` for `history-culture`.

### Target 4 — Produce the first completed experiment run

**Target.** Performed evidence, anchored on `vibration`.

**Why it matters.** AudioMuse's experiment layer exists so the atlas connects theory to something a
reader can hear or observe. Right now it holds three definitions and two empty planned runs, so the
practical layer is entirely prospective. One completed run changes the repository from describing
experiments to having performed one, and it validates the Phase 8 evidence contract against real use.

**Current repository evidence.** `completed_experiment_runs.count` of 0 across all 15 nodes; both run
records at `status: planned` with `run_date: null` and empty evidence arrays; `frequency-room-position`
with no run record; the `vibration` node declaring three practical applications, zero linked experiments,
one session origin, and a future question about comparing airborne and structure-borne vibration.

**What is actually missing.** A performed execution with recorded environment, equipment, observations,
limitations, and interpretation.

**Recommended deepening form.** Add one listening or hybrid experiment definition referencing
`vibration`, then perform it and record a `completed` run. Alternatively, perform an existing definition
first if that is the more honest starting point.

**Evidence requirement.** This is the target most exposed to fabrication. The run record must reflect an
execution that actually happened. `run_date` must be real, `equipment` and `software` must name what was
used, observations must stay qualitative unless a calibrated chain is documented, and nominal generator
or DAW settings remain control settings rather than measured acoustic output. A planned run must not be
relabelled as completed.

**Related AudioMuse territory.** Acoustics, resonance, recording, transduction, and the measurement
conventions established in Phases 7 and 8.

**Resolves candidates.** `practical_evidence` for `vibration`, and addresses the repository-wide
zero-completed-run observation that Phase 10's rules cannot express.

### Cross-cutting evidence requirement

**Recommendation.** Any Phase 12 work that introduces a factual, historical, or practice claim not
developed by an existing session must register at least one external source in
`sources/source-registry.yaml` before the claim is written. Sources must be real and locatable. If a
suitable source cannot be obtained, the correct action is to narrow the claim or drop it, never to write
it unsourced or to invent a citation.

## Deferred items

**Future work.** Legitimate, not next.

| Item | Candidate | Revisit when |
|---|---|---|
| `music-theory` domain | `domain_representation` | A session or research effort is dedicated to it, so it does not consume a bounded phase by accident. |
| `mixing-mastering` domain | `domain_representation` | Recording gains depth or a session develops the material past a term list. |
| `recording` vocabulary bridge | `vocabulary_bridge` | Target 1 completes; then extend and re-point existing digital-audio entries. |
| `rhythm` practical evidence | `practical_evidence` | Target 2 establishes DJ and turntablism context. |
| `sampling` practical evidence | `practical_evidence` | Sampler tooling is available and the experiment would not duplicate `waveform-timbre-comparison`. |

## Watch items

**Future work.** Signals that may become important as AudioMuse expands. No work is justified now.

| Item | Candidate | Watch for |
|---|---|---|
| `spatial-audio` domain | `domain_representation` | A session developing stereo imaging, localization, or immersive formats past mention; or a room-behaviour experiment producing real evidence. |
| `machine-audio` domain | `domain_representation` | DSP depth sufficient to anchor it. Guard against this pulling AudioMuse toward being a software platform. |
| `audio-hardware` domain | `domain_representation` | Measurement or recording equipment entering the project and appearing in a completed run record. |
| `recording` practical evidence | `practical_evidence` | Any completed run establishing that microphone or interface equipment is available. |

## Phase 12 readiness criteria

**Recommendation.** Phase 12 is a focused knowledge-deepening phase. It is ready to begin when the
following are true, and it must stay inside these boundaries.

**Allowed targets.** Only Targets 1 through 4 above. Phase 12 may implement one or more of them and may
implement fewer than all four. It may not substitute a deferred or watch item without a new triage
decision recorded here.

**Recommended order.** Target 1, then Target 2, then Targets 3 and 4 in either order. Target 1 is
lowest-risk and supplies vocabulary that Target 2 builds on.

**Expand versus create.**

- Target 1 — extend existing vocabulary entries and add new ones. No node creation.
- Target 2 — a new node is justified; `nodes/dj-turntablism/` is empty and no existing node carries the
  claim. Vocabulary should precede it.
- Target 3 — node creation is conditional on a statable durable claim and external source support.
  Expanding `docs/session-node-map.md` is the default.
- Target 4 — no node creation. Experiment and experiment-run layers only.

**Relationships deserving review.** Only edges from a node Phase 12 actually creates, to `sampling`,
`rhythm`, or `recording`, and only where node prose or a registered source supports the claim. The five
structural cases reviewed above need no change. No mirror edges. No edge added to raise a count.

**Sources needed.** Session 2 and Session 3 are already registered and sufficient for Target 1. Targets 2
and 3 require at least one external source each before any factual, historical, or practice claim is
written.

**Experiments and listening work.** Only within Target 4, and only where an execution genuinely occurs.

**Explicitly out of scope for Phase 12.** Every deferred and watch item above; the `music-theory` and
`mixing-mastering` domains; new relationship types; threshold changes to Phase 10; and everything in the
guardrails below.

**Completion signal.** Phase 12 is complete when the targets it chose are implemented, the full
validation suite passes, and the Phase 10 coverage view regenerates cleanly with candidate changes that
follow from real content rather than from tuning.

## Non-goals and guardrails

Phase 11 created no canonical nodes, added or modified no canonical relationships, added no vocabulary,
registered no sources, wrote no sessions, and created no experiments or runs. It changed no Phase 10
threshold and altered no canonical count.

Phase 11 and the Phase 12 it prepares explicitly do not:

- treat a coverage candidate as a confirmed knowledge defect;
- auto-accept Phase 10 findings, or accept one solely because a count is low;
- generate knowledge because a metric indicates sparse coverage;
- adjust thresholds, rules, or enums to produce nicer results;
- introduce scoring, ranking, weighting, percentiles, or confidence values;
- introduce AI or LLM ranking, embeddings, semantic search, or recommendation algorithms;
- fabricate sources, citations, sessions, experiments, or results;
- treat a planned experiment run as performed;
- create relationships for symmetry, balance, or navigation;
- promote a vocabulary term to a node merely because the term exists;
- redesign the graph, migrate identifiers, or normalize canonical knowledge silently;
- add a web UI, frontend, API, backend service, database, dependency, graph visualization, or runtime
  infrastructure;
- expand AudioMuse into a large software platform.

A node may be intentionally narrow. A domain may be intentionally empty. A low count is a description of
current representation, not a defect to be corrected.
