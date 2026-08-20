# AudioMuse Roadmap

## Phase 0.5 — Canonical Naming + Scaffold Cleanup

Normalize the starter scaffold to the canonical **AudioMuse** identity and remove unintended project-level `AudioMuse` naming.

## Phase 1 — GitHub Repository Foundation + Content Import

Create the canonical GitHub repository foundation and import the existing AudioMuse session, vocabulary, brand, and research source material.

## Phase 2 — Node Schema + Source Provenance Foundation

Establish canonical node and source contracts, provenance conventions, and a small proof-node set.

## Phase 3 — Initial Node Expansion + Session Mapping

Promote a curated set of foundational Sessions 1–3 concepts, connect them through canonical node relationships, and expose the many-to-many session-to-node map.

## Phase 4 — Typed Relationship Graph + Edge Semantics Foundation

Replace untyped adjacency with a bounded directed relationship vocabulary, migrate the initial graph to explicit semantic edges, and add deterministic integrity validation.

## Phase 5 — Read-Only Derived Graph Views + Knowledge Index Foundation

Generate deterministic, rebuildable indexes for domains, typed relationships, inbound/outbound connections, session coverage, and source coverage while keeping canonical repository content authoritative.

## Phase 6 — Vocabulary Atlas Integration + Concept Cross-Reference Foundation

Formalize a bounded foundation vocabulary with practical digital-audio context, curated session and node cross-references, strict validation, and a deterministic read-only vocabulary index without creating a second graph.

## Phase 7 — Listening + Measurement Experiment Foundation

Establish a small canonical experiment contract, lightweight safety and observation-versus-measurement conventions, three source-supported proof exercises, strict cross-reference validation, and a deterministic read-only experiment index without mutating the knowledge graph.

## Phase 8 — Experiment Run Records + Observation Evidence Foundation

Separate individual experiment executions from canonical definitions, distinguish qualitative observations from instrument measurements and nominal controls, and provide strict validation plus a deterministic run index without mutating the knowledge graph.

## Phase 9 — Repository-Wide Reference Integrity

Enforce ordinal canonical identity and cohesive cross-reference validation across nodes, sources, sessions, vocabulary, experiments, and experiment runs.

## Phase 10 — Knowledge Coverage + Research Gap Analysis Foundation

Derive deterministic, read-only coverage observations and evidence-backed research-gap candidates from canonical repository facts. Keep coverage distinct from correctness and quality, preserve experiment-definition versus completed-run evidence, and leave all research decisions to humans.

## Phase 11 — Research Gap Triage + Knowledge Deepening Plan

Review every Phase 10 research-gap candidate by hand, classify each as accept, defer, dismiss, or watch
with repository-grounded rationale, and convert the accepted signals into a small prioritized
research-deepening plan. Documentation only; no canonical knowledge is created or modified. Complete.

Selected Phase 12 research-deepening targets:

1. Extend vocabulary into Sessions 2 and 3 for the `rhythm-time` and `synthesis` domains and
   musical-control terminology.
2. Establish the `dj-turntablism` domain with its first durable node.
3. Convert Session 3 into durable structure, or record why it stays chronology-only.
4. Produce the first completed experiment run, anchored on `vibration`.

Decisions, evidence, deferred and watch items, and the Phase 12 boundaries are recorded in
`docs/research-gap-triage.md`.

## Phase 12 — Focused Knowledge Deepening

Implement one or more of the four Phase 11 targets under the evidence requirements and guardrails that
document records. Phase 12 may not substitute a deferred or watch item without a new triage decision.

### Phase 12A — Focused Vocabulary Deepening. Complete.

Target 1 only. Extended the vocabulary layer from Sessions 2 and 3 with fourteen entries: seven in
`rhythm-time`, four in `synthesis`, and three musical-control entries in `digital-audio`. Each entry is
derived from session prose that develops the concept; terms appearing only inside Session 2's taxonomy
blocks were deliberately left out. This established vocabulary bridges to the existing `rhythm`,
`sequencing`, `synthesis`, and `midi` nodes. No node, relationship, source, session, or experiment was
created or modified.

### Phase 12B — DJ + Turntablism Domain Foundation. Complete.

Target 2 only. Established `dj-turntablism` as a populated domain with six nodes — `djing`,
`turntablism`, `scratching`, `turntable`, `beatmatching`, and `digital-vinyl-system` — plus twelve
vocabulary entries and eighteen typed relationships, five of which were added to existing nodes so the
domain connects into acoustics, recording, rhythm, sampling, and DSP rather than standing apart.

The domain exists to make cross-disciplinary traversal possible: hand motion through playback velocity
to frequency and pitch; crossfader gesture through amplitude gating to rhythm; and stylus, groove, and
platter through mechanical vibration to resonance and isolation. Every claim is drawn from Sessions 1,
2, and 3. No source, session, schema, relationship type, or validator was added or changed.

Targets 3 and 4 remain pending.

### Deferred — Third Coast, Houston, and chopped-and-screwed research

**Status: addressed by Phase 12C.** The section below is retained as the record of what was missing and
of the shape the work was expected to take. Phase 12C followed that shape: sources were registered
first, history was separated from interpretation, technique was covered alongside culture, and lineage
claims were required to carry citations.

Phase 12B deliberately did not attempt this material. The registered sessions document DJ practice
through New York breakbeat DJing, Chicago house, Detroit techno, UK breakbeat and dubstep, and later
digital systems. They contain nothing on regional Southern United States DJ traditions, so AudioMuse
currently has no source basis for Houston or Third Coast practice, chopped-and-screwed technique, or
later chopped-and-slowed derivatives. Writing it now would mean inventing history.

What Phase 12B established instead is the seam. `djing` and `turntablism` each carry an explicit
`future_questions` entry naming the gap, the `dj-turntablism` domain gives the material a place to
land, and `scratching` already models playback rate as a manipulable parameter — the same parameter a
sustained slowed-playback study would extend from a momentary gesture to a whole-track treatment. The
conceptual links a future phase would need to make are therefore playback rate, pitch, time, sampling,
DJ performance, and remix practice, all of which now exist as nodes.

This deserves a dedicated researched phase with its own registered sources, not a paragraph appended
to an existing one. The proposed shape is:

- Register primary or scholarly sources before writing any node.
- Separate source-supported history, conceptual interpretation, and open research question, as
  `docs/research-gap-triage.md` already requires.
- Cover technique — sustained rate reduction, its effect on pitch, time, and timbre, and its
  relationship to tape, sampler, and software-based slowing — alongside cultural history.
- Treat regional lineage claims as requiring citation, not inference.

Nothing in this section is canonical AudioMuse knowledge; it records what is absent and what a future
phase would need.

### Phase 12C — Houston / Third Coast + chopped-and-screwed foundation. Complete.

The first AudioMuse phase built on external sources rather than on session transcripts. Twenty-eight
sources were registered before any node was written — university archives and finding aids, signed
Handbook of Texas entries, university-press and trade books, a technical reference, and dated
journalism and participant interviews — each with a research note recording its citation, what it is
allowed to support, what it does not settle, and whether its full text was actually retrieved.

Twenty-two nodes were added: thirteen in the newly populated `history-culture` domain, three in
`recording`, four in `dj-turntablism`, and one each in `rhythm-time` and `dsp`. Five edges were added
to existing nodes so the material connects into DJ practice, recording, and DSP instead of forming an
island. Twenty-four vocabulary entries were added across five domains. `docs/houston-musical-cartography.md`
holds the sourced chronology, a confidence marker on every claim, and a dispute register.

The phase is deliberately as notable for what it refused to state. No founding year is asserted for
Rap-A-Lot or Southwest Wholesale; the Screw nickname, the store chronology, the South Park Coalition
and Pen & Pixel founding years, and the Swishahouse lineup are recorded as unresolved; no canonical
playback speed is given; and one proposed addition was excluded outright after research placed the
figure outside Houston. No schema, relationship type, or validator was changed.

## Near-term follow-up

- Independently review Phase 10 thresholds and candidate evidence against future repository growth
- Deepen high-value nodes only where human review confirms a documented research need
- Extend source and citation coverage as new evidence is introduced
- Add further experiments or vocabulary terms only as future sessions or research justify them

Keep later phases intentionally flexible so the research can determine what deserves implementation.
