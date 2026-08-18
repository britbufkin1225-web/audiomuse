# Session-to-Node Map

This map shows how chronological Sessions 1–3 contribute to durable AudioMuse nodes. It is many-to-many provenance: a session can develop many nodes, and a node can be developed by multiple sessions. Inclusion does not assign ownership to a session or replace the canonical `session_origin` and `sources` fields in each node.

## Session 1 — What Is Sound?

- `sound`
- `vibration`
- `frequency`
- `phase`
- `resonance`
- `pitch`
- `timbre`
- `psychoacoustics`
- `recording`
- `sampling`
- `digital-signal-processing`

## Session 2 — What Is Music?

- `sound`
- `frequency`
- `phase`
- `resonance`
- `pitch`
- `timbre`
- `psychoacoustics`
- `rhythm`
- `sampling`
- `synthesis`
- `sequencing`
- `midi`
- `digital-signal-processing`

## Session 3 — History of Electronic Music

- `rhythm`
- `recording`
- `sampling`
- `synthesis`
- `sequencing`
- `midi`
- `digital-signal-processing`

## Emerging graph

This diagram is an explanatory view of selected typed relationships. Node files remain canonical, and each arrow label records the stored relationship type and direction.

```mermaid
graph TD
    VIBRATION[Vibration] -- produces --> SOUND[Sound]
    VIBRATION -- characterized_by --> FREQUENCY[Frequency]
    VIBRATION -- influences --> RESONANCE[Resonance]

    SOUND -- characterized_by --> PHASE[Phase]
    SOUND -- characterized_by --> FREQUENCY
    SOUND -- characterized_by --> TIMBRE[Timbre]

    FREQUENCY -- influences --> PITCH[Pitch]
    FREQUENCY -- influences --> TIMBRE
    FREQUENCY -- used_in --> SYNTHESIS[Synthesis]
    PHASE -- influences --> RECORDING[Recording]
    PHASE -- influences --> DSP[Digital Signal Processing]

    PSYCHO[Psychoacoustics] -- studies --> PITCH
    PSYCHO -- studies --> TIMBRE
    PSYCHO -- studies --> RHYTHM[Rhythm]

    RECORDING -- enables --> SAMPLING[Sampling]
    RECORDING -- enables --> DSP
    RHYTHM -- contributes_to --> SAMPLING
    RHYTHM -- influences --> SEQUENCING[Sequencing]
    SEQUENCING -- controls --> SAMPLING
    SEQUENCING -- controls --> SYNTHESIS
    MIDI[MIDI] -- used_in --> SEQUENCING
    MIDI -- controls --> SAMPLING
    MIDI -- controls --> SYNTHESIS
    DSP -- used_in --> SYNTHESIS
    SAMPLING -- enables --> DSP
```

## Curation boundary

A vocabulary entry is a concise terminology reference. A canonical node is a durable concept selected to accumulate relationships, provenance, sessions, experiments, projects, and deeper research. Terms do not become nodes automatically and may remain vocabulary-only indefinitely.
