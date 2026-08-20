# AudioMuse Claim Provenance Model

Phase 12D. The architecture that lets AudioMuse say what kind of statement it is making, what evidence stands behind it, how strong that evidence is, and whether anything contradicts it — as stored data rather than as sentences.

## 1. Why this exists now

Through Phase 12B, AudioMuse provenance was a list of registered source IDs on a node. That is enough when the material is conceptual: Session 1 develops `frequency`, the node cites the session, and there is nothing further to grade.

Phase 12C changed the shape of the repository. It added twenty-two nodes built from external sources — archives, finding aids, signed reference entries, university-press books, journalism, and participant interviews — about events that happened in a particular city at a particular time. Those nodes carry dates, attributions, priority claims, and conflicts between sources, and a bare `sources: [tsha-dj-screw]` cannot distinguish any of them. So Phase 12C carried the distinctions in prose: `docs/houston-musical-cartography.md` marks every chronology entry `documented`, `reported`, `recollected`, `disputed`, or `approximate`, and it closes with an explicit list of the fields a later phase would need.

This is that phase. It implements the recorded requirement — evidence type, corroboration, chronology precision, disputed status, retrieval status — as a validated contract, and adds the three things that list did not name: an explicit claim taxonomy, a first-class place for AudioMuse's own inferences, and validation that can detect broken provenance.

It does not rewrite Phase 12C. The cartography's prose markers remain; ten claim records now stand beside them as the machine-readable form, and the migration in section 10 converts the rest by priority rather than in one sweep.

## 2. What counts as a claim

The single most dangerous design failure available here was requiring every sentence in AudioMuse to become a database record. That would produce thousands of records, none of them reviewed carefully, and would make the layer worthless.

A claim record is written when a statement is **externally checkable** and **something depends on getting it right**:

| Write a claim | Leave it as prose |
| --- | --- |
| A date, a place, a sequence of events | A definition |
| An attribution to a person or organization | An explanation of a mechanism already covered by a claim |
| A priority or origin assertion | Navigation, framing, and cross-references |
| A statement two registered sources disagree about | A restatement of something a claim already records |
| A technical statement a reader might act on | A question, a caveat, or an open research note |
| A conclusion AudioMuse reached by combining sources | Anything the repository does not actually assert |

The test is not importance. It is whether a reader could, in principle, check the statement against something outside AudioMuse, and whether being wrong would matter. `slowed-playback` contains many true sentences about rate change; one of them — the coupling of duration and frequency — is a claim, because the rest follow from it and a reader could act on it.

A claim's `statement` must stand alone. If it needs the surrounding paragraph to be intelligible, it is not yet a claim; it is a fragment of one.

## 3. Claim taxonomy

Nine types, defined in `schemas/claim.schema.yaml`. Each says what kind of statement is being made. None of them says how well evidenced it is.

| Type | What it means | Test |
| --- | --- | --- |
| `established_fact` | A non-technical statement documented well enough that it is not in question. | Would a competent reference work state it flatly? |
| `technical_fact` | A physical, mathematical, or engineering statement verifiable by derivation or measurement, independent of history. | Would it be true regardless of who did what, when? |
| `historical_claim` | A statement about what happened, when, or in what order. | Does it place something in time or sequence? |
| `attributed_claim` | A statement about what a named person, organization, or publication asserts. | Is the subject of the sentence somebody's assertion? |
| `oral_history` | A first-hand account given in interview, testimony, or participant memory. | Is the evidence somebody remembering? |
| `interpretation` | An organizing reading AudioMuse adopts: a frame, an ordering, a way of seeing the material. | Could a competent reader adopt a different frame without contradicting any source? |
| `audiomuse_synthesis` | A factual conclusion AudioMuse reached by combining evidence no single source combines. | Is the conclusion sourced in parts but not as a whole? |
| `hypothesis` | A statement AudioMuse proposes and does not assert, pending evidence. | Is it written to be tested? |
| `experiment_observation` | A statement grounded in a recorded experiment run. | Does a canonical run record support it? |

Two distinctions carry weight and are easy to blur.

**`interpretation` versus `audiomuse_synthesis`.** An interpretation is a frame; a synthesis is a factual conclusion. "Houston's rap economy is best read as a layered system" is a frame — a different frame would not make it false. "Sustained mechanical slowing necessarily altered vocal timbre" is a conclusion — it is either right or wrong, and the sources support each step without stating the step's combination.

**`established_fact` versus `technical_fact`.** A technical fact does not depend on the historical record at all; verifying it means deriving or measuring it. An established fact does depend on the record and happens to be documented beyond question. Keeping them apart matters because they fail differently: a technical fact fails to a better derivation, an established fact to a better document.

### `disputed_claim` was evaluated and rejected as a type

Phase 12D was asked to evaluate `disputed_claim` alongside the others. It is not implemented as a claim type, and the reason is the phase's own core principle: claim type, confidence, and dispute status are independent axes. A `disputed_claim` type would collapse two of them, and it produces immediate absurdities — a disputed historical claim would have to give up being historical, and a disputed technical claim would have no home at all.

Dispute is therefore a separate required field, `dispute_status`, which any claim of any type may carry. Phase 12C reached the same conclusion in prose: its dispute register is a separate table, not a separate category of claim.

## 4. Confidence

Four values. They grade **the strength of repository evidence for the statement as written** — not the probability that it is true, and not how certain a model feels.

| Value | Meaning |
| --- | --- |
| `high` | Strong direct evidence: an authoritative retrieved source, or several credible independent sources materially supporting the claim. |
| `moderate` | Credible evidence exists, but chronology, interpretation, attribution, or corroboration carries meaningful uncertainty. |
| `low` | Support is limited, indirect, anecdotal, weakly corroborated, or meaningfully contested. |
| `unknown` | The repository does not contain enough evidence to make a defensible determination. |

There is no numeric score, no percentage, and no model-generated grade. `confidence: 0.87` would be a fabrication: the repository has no denominator that could produce it.

`confidence` is validated, not merely declared:

- `high` requires either two distinct supporting sources, or one supporting source whose registered `evidence_class` is authoritative (`institutional_archive`, `reference_entry`, `technical_reference`) and whose `retrieval` is not `citation_only`.
- `moderate` and `low` require at least one evidence entry.
- `unknown` requires **zero** supporting sources. A claim with support is at least `low`; `unknown` means nothing bears on it.

The `high` rule is a floor, not an assignment. `houston-layered-infrastructure-reading` cites three supporting sources and is recorded `moderate`, because the frame it proposes is not what those sources state. Every record carries a required `confidence_basis` explaining why this level and not the next one up or down; the validator rejects a `confidence_basis` that merely restates the claim.

### Confidence is independent of type

| Claim | Type | Confidence |
| --- | --- | --- |
| `screw-master-tape-method` | `historical_claim` | `high` |
| `mike-dean-rap-a-lot-account` | `oral_history` | `moderate` |
| `screwed-up-records-1996-store-claim` | `historical_claim` | `low` |
| `phase-vocoder-attributed-to-flanagan-and-golden` | `attributed_claim` | `high` |

A historical claim is not weak because it is historical. An oral-history claim is not weak because it is oral history: a recorded, published, first-hand account is strong evidence of what the speaker says. What it is weak evidence *for* is a different question, which is why `mike-dean-rap-a-lot-account` states what he described rather than asserting the chronology directly.

## 5. Evidence relationships

Three relations connect a claim to a registered source. The vocabulary is small on purpose.

| Relation | Meaning |
| --- | --- |
| `supported_by` | The source materially supports the claim **as written**. |
| `contradicted_by` | The source states something incompatible with the claim as written. |
| `qualified_by` | The source narrows, bounds, dates, or limits the claim without contradicting it. |

The distinction the architecture exists to make is between a source that **supports a claim** and a source that **mentions a topic**. Node `sources:` lists are topical: they say this source is relevant to this concept. `supported_by` is not topical, and every entry carries a required `note` saying what that source does for that specific claim. A source that merely mentions the subject earns no evidence entry at all.

`qualified_by` is what Phase 12C's "does not settle" sections become. `screw-sales-copy-duplication-hypothesis` cites the University of Houston finding aid as `qualified_by`, because the finding aid's silence about sales-copy duplication is exactly what bounds the hypothesis.

Two further reference kinds are stored separately because their targets are not sources:

- **`attribution`** — `{actor, source_id}`. Who the statement is credited to, and the registered source that records the credit. This answers "who says so", which a source ID alone cannot.
- **`derived_from`** — `{kind, ref}` over `claim`, `node`, or `experiment_run`. What an AudioMuse-derived claim was built from. Registered sources never appear here; they belong in `evidence`.

`appears_in` — `{kind, ref}` over `node`, `vocabulary`, `session`, or `document` — records where the statement is actually made. At least one entry is required: a claim that appears nowhere in the repository is not a repository claim. Generated projections are rejected as appearance sites, because a derived view cannot be evidence that a claim exists.

Nodes do not list their claims. The reverse view is derived into `claims/index.md`, following the rule `docs/knowledge-model.md` already sets for inverse graph edges: store the statement once in its clearest direction and derive the other.

## 6. Dispute and uncertainty

Uncertainty comes in kinds, and flattening them into a `notes` field is what Phase 12D was built to stop. Three fields carry it.

**`dispute_status`** — whether registered sources conflict:

| Value | Meaning | Enforced |
| --- | --- | --- |
| `undisputed` | Nothing registered conflicts with the claim as written. | Zero `contradicted_by` entries. |
| `disputed` | Registered sources conflict. | At least one `contradicted_by` entry. |
| `unresolved` | A specific detail is not settled by available evidence, without two sources actively conflicting. | Confidence may not be `high`; at least one open question required. |

**`temporal_precision`** — how precisely the statement is placed in time: `exact_date`, `month`, `year`, `range`, `era`, or `not_temporal`. This is Phase 12C's `approximate` marker as data. It separates "we know the year and not the month" from "the year itself is contested", which the prose markers could not.

**`open_questions`** — what would have to be found to raise the confidence or settle the dispute. Required when `dispute_status` is `unresolved`.

Together these express the cases Phase 12C had to write out in sentences: sources disagree (`disputed`); the event is documented but the chronology is loose (`undisputed` with `temporal_precision: era`); the attribution rests on retrospective oral history (`oral_history` with an `attribution` entry); and AudioMuse is combining sources rather than repeating one (`audiomuse_synthesis`).

## 7. First, earliest, invented: origin claims

Priority language asserts exclusivity, and exclusivity is almost never what the evidence supports. `schemas/claim.schema.yaml` declares a bounded list of origin terms — `first`, `earliest`, `invented`, `inventor`, `originated`, `originator`, `pioneered`, `father of`, `birthplace of`, plus the contextual phrases `created the style`, `created the technique`, `created the sound`, and `created the scene` — and the validator enforces a rule on any statement containing one as a whole word:

- it must be typed `historical_claim`, `attributed_claim`, or `oral_history` — never `established_fact`, `technical_fact`, `interpretation`, or `audiomuse_synthesis`; and
- it must carry at least one `attribution` entry naming who credits it.

AudioMuse may record a priority claim. It may not make one in its own voice without saying whose claim it is.

Bare `created` is deliberately **not** on the list. It appears constantly in ordinary explanatory prose ("created a tape economy") and adding it would fire the guard on sentences that assert no priority at all, which trains authors to work around the rule instead of heeding it. The four bounded `created the ...` phrases catch direct creative-origin claims while leaving ordinary uses alone.

Where the evidence does not support exclusivity, the preferred formulations remain the ones Phase 12C used: *widely credited with*, *documented as an early example*, *one of the earliest documented examples*, *a central pioneering figure*, *commonly attributed to*. `kcoh-first-black-owned-texas-station-attribution` is the worked example: a single journalism source carries the superlative, so the claim records that the station *is described as* the first, attributes the description, grades it `low`, marks it `unresolved`, and names the record that would settle it.

Phase 12C's prose is not rewritten to match. The architecture now exists; the annotation pass is section 10.

## 8. AudioMuse synthesis

AudioMuse's value is largely in connections — physics to perception to musical practice to recording technology to DJ technique to DSP. Those connections are often correct and useful even when no single source states the conclusion. They must never be indistinguishable from sourced reporting.

`audiomuse_synthesis` is that distinction, and it is constrained:

- at least one `supported_by` source, and
- at least two grounding references counting supporting sources and `derived_from` entries together.

A synthesis is always grounded in identified evidence. It never means "the model generated something plausible": every component must already be a claim, a node, a run, or a retrieved source, and the `confidence_basis` must say which part is sourced and which part is the inference.

`screw-slowing-alters-vocal-timbre` is the worked example. It derives from two claims and a node, cites two sources, and is recorded `moderate` rather than `high` precisely because the combination is AudioMuse's and not the sources'.

## 9. Relationship to existing provenance

Phase 12D adds a layer; it replaces nothing.

| Layer | Question it answers | Changed by Phase 12D |
| --- | --- | --- |
| Sessions | What did AudioMuse explore, and when? | No |
| Nodes | What does AudioMuse know about this concept? | No |
| Node `sources:` | Which registered sources are relevant to this node? | No |
| Vocabulary | What does this term mean in practice? | No |
| Source registry | What is this source, and where is it? | Two optional fields added |
| Claims | What is asserted, on what evidence, how strongly? | New |
| Indexes | Deterministic read-only projections | One added, none altered |

Node `sources:` lists stay exactly as they are and keep their topical meaning. A node may cite five sources while one claim appearing in it cites two of them as evidence; both statements are true and neither replaces the other.

The source registry gained `evidence_class` and `retrieval`, both optional. They are required only of a source that a claim actually cites — annotation follows use rather than preceding it, so no repository-wide sweep is forced. Eleven of thirty-three sources carry them today, transcribed from the `Class:` and `Retrieval:` lines the Phase 12C research notes already recorded; nothing was invented and no source note was changed.

## 10. Migration strategy

Phase 12D is not the migration. Ten representative records prove the contract; the rest of the repository is annotated by priority, in this order:

1. **Historically sensitive claims** — deaths, violence, legal matters, and anything about a named individual's life.
2. **First, pioneer, and origin claims** — every statement the origin-term list would catch. `docs/houston-musical-cartography.md` and the `history-culture` nodes are the concentration.
3. **Regional and cultural chronology** — dates and sequences in the Houston material, starting with the entries the cartography already marks `disputed` or `approximate`.
4. **Technical claims needing stronger sourcing** — technical statements currently resting on session prose alone.
5. **General explanatory material** — mostly never. Definitions and mechanism explanations stay as prose.

Each step converts a prose marker into a record and cites the same sources, annotating each newly cited source with `evidence_class` and `retrieval` from its existing research note. No claim may be created for material the repository does not already assert; the migration is a change of representation, not of content.

There is deliberately **no requirement** that every sentence acquire claim metadata, and none that a node reach a claim quota. A node with zero claims is a normal node.

## 11. Validation guarantees

`tools/validate-claims.ps1` rejects, with exit status and a named reason:

- invalid `claim_type`, `confidence`, `dispute_status`, `temporal_precision`, evidence relation, `derived_from` kind, or `appears_in` kind, including case drift, compared with the exact ordinal semantics Phase 9 established;
- a missing, duplicated, unknown, or malformed field, at the record level and inside every evidence, attribution, derivation, and appearance object;
- a duplicate claim ID across all record files;
- an unresolved source, claim, node, vocabulary, session, experiment-run, or document reference;
- a generated projection or a non-repository-relative path cited as an appearance site;
- a claim with no appearance site at all;
- a source cited as evidence or attribution that does not declare `evidence_class` and `retrieval`;
- `high` confidence without two supporting sources or one retrieved authoritative source;
- `unknown` confidence alongside supporting evidence;
- a settled fact type that is disputed or weakly evidenced;
- an attributed or oral-history claim with no attribution;
- a synthesis that is not grounded in a source and a second reference;
- a hypothesis at `high` confidence, or with nothing cited at all;
- an observation claim not derived from an experiment run;
- a dispute status that contradicts the evidence actually cited;
- an origin-term statement made in AudioMuse's own voice or without attribution;
- one source recorded as both supporting and contradicting the same claim;
- a claim derivation cycle;
- a generated index that is stale, incomplete, reordered, or that asserts anything the canonical records do not.

Index reconciliation is independent: every expected line is re-derived from canonical records rather than by re-running the builder, and only then is the committed file byte-compared against a fresh build. A builder that formats every run the same wrong way is caught by the first check; a stale commit is caught by the second.

`tools/test-claim-validator.ps1` runs fifty-four adversarial cases, each of which declares the message it must fail with, so a fixture rejected for an unrelated reason is reported as a broken test rather than as a pass. It also asserts up front that the committed fixtures validate, since otherwise every negative below it would prove nothing — fifty-five checks in all. Invalid fixtures exist only inside a temporary directory and never in canonical content paths.

## 12. Representative fixtures

Ten records in `claims/records/`, all drawn from existing repository material and registered sources. No historical dispute was invented to fill a category.

| Claim | Type | Confidence | Dispute | Demonstrates |
| --- | --- | --- | --- | --- |
| `playback-rate-couples-time-and-pitch` | `technical_fact` | `high` | `undisputed` | A technical claim with two independent supports |
| `phase-vocoder-attributed-to-flanagan-and-golden` | `attributed_claim` | `high` | `undisputed` | A claim about what a source asserts, scoped so retrieval settles it |
| `screw-master-tape-method` | `historical_claim` | `high` | `undisputed` | History at high confidence on one institutional source |
| `screwed-up-records-1996-store-claim` | `historical_claim` | `low` | `disputed` | Two registered sources in conflict, neither adopted |
| `fat-pat-death-1998` | `established_fact` | `high` | `undisputed` | An exact date, with the unsettled detail qualified rather than asserted |
| `mike-dean-rap-a-lot-account` | `oral_history` | `moderate` | `undisputed` | First-hand testimony that is neither dismissed nor promoted |
| `screw-slowing-alters-vocal-timbre` | `audiomuse_synthesis` | `moderate` | `undisputed` | An AudioMuse inference, grounded and marked as one |
| `screw-sales-copy-duplication-hypothesis` | `hypothesis` | `unknown` | `undisputed` | A proposal with only qualifying evidence and nothing to grade |
| `houston-layered-infrastructure-reading` | `interpretation` | `moderate` | `undisputed` | A frame held below the confidence its source count would allow |
| `kcoh-first-black-owned-texas-station-attribution` | `attributed_claim` | `low` | `unresolved` | A priority claim attributed rather than asserted |

`experiment_observation` has no fixture. The repository contains two experiment runs and both are `planned` with no recorded observations, so any fixture of that type would have to invent evidence. The type and its rule are implemented and adversarially tested; the first completed run supplies the first record.

## 13. Deliberately deferred

Not attempted in Phase 12D, and not to be inferred from it:

- Repository-wide annotation. Section 10 is the plan, not this phase's work.
- Any rewrite of Phase 12C prose, nodes, chronology, or dispute register.
- Further Houston, Third Coast, turntablism, or DJ research; the Yolanda Adams and Choice research targets the cartography names remain open.
- Numeric, probabilistic, or model-generated confidence of any kind.
- A truth-scoring system, a knowledge database, a backend, a frontend, semantic search, embeddings, a vector store, or a retrieval system.
- Claim-to-claim relationship types beyond `derived_from`. Contradiction between two AudioMuse claims is not yet expressible and no current material needs it.
- Node front-matter claim references. The reverse view is derived; if traversal ever needs the forward direction, that is a separate decision with its own schema change.
- Source-registry annotation beyond cited sources, and any change to the twenty-two Phase 12C research notes.
