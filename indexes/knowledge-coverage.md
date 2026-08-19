# AudioMuse Knowledge Coverage

> Generated, read-only, and non-authoritative. Rebuild with `tools/build-knowledge-coverage.ps1`; canonical repository content always wins.

Coverage measures explicit repository relationships. It is not correctness, quality, truth, or universal completeness. A candidate is evidence for human review, not a knowledge defect.

## Repository overview

- node_count: 15
- relationship_count: 45
- relationship_type_count: 11
- source_count: 5
- session_count: 3
- vocabulary_count: 38
- experiment_count: 3
- experiment_run_count: 2

## Node coverage

| Node | Domain | Sessions | Sources | Vocabulary | Experiments | Completed runs | In | Out | Type diversity |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `digital-signal-processing` | dsp | 3 | 3 | 2 | 1 | 0 | 4 | 3 | 4 |
| `frequency` | acoustics | 2 | 3 | 2 | 3 | 0 | 4 | 6 | 4 |
| `midi` | digital-audio | 2 | 2 | 0 | 0 | 0 | 0 | 5 | 3 |
| `phase` | acoustics | 2 | 3 | 1 | 2 | 0 | 2 | 2 | 2 |
| `pitch` | psychoacoustics | 2 | 3 | 1 | 1 | 0 | 4 | 1 | 4 |
| `psychoacoustics` | psychoacoustics | 2 | 3 | 1 | 1 | 0 | 0 | 5 | 1 |
| `recording` | recording | 2 | 2 | 0 | 0 | 0 | 2 | 4 | 4 |
| `resonance` | acoustics | 2 | 3 | 1 | 1 | 0 | 2 | 2 | 2 |
| `rhythm` | rhythm-time | 2 | 2 | 0 | 0 | 0 | 2 | 2 | 3 |
| `sampling` | digital-audio | 3 | 4 | 3 | 0 | 0 | 6 | 3 | 5 |
| `sequencing` | rhythm-time | 2 | 2 | 0 | 0 | 0 | 2 | 2 | 3 |
| `sound` | acoustics | 2 | 3 | 1 | 1 | 0 | 6 | 3 | 6 |
| `synthesis` | synthesis | 2 | 2 | 0 | 2 | 0 | 3 | 4 | 3 |
| `timbre` | psychoacoustics | 2 | 3 | 1 | 1 | 0 | 7 | 0 | 6 |
| `vibration` | acoustics | 1 | 2 | 1 | 0 | 0 | 1 | 3 | 4 |

## Domain coverage

| Domain | Nodes | Vocabulary | Experiments | Sources |
|---|---:|---:|---:|---:|
| `acoustics` | 5 | 19 | 3 | 3 |
| `audio-hardware` | 0 | 0 | 0 | 0 |
| `digital-audio` | 2 | 8 | 0 | 4 |
| `dj-turntablism` | 0 | 0 | 0 | 0 |
| `dsp` | 1 | 2 | 1 | 3 |
| `history-culture` | 0 | 0 | 0 | 0 |
| `machine-audio` | 0 | 0 | 0 | 0 |
| `mixing-mastering` | 0 | 0 | 0 | 0 |
| `music-theory` | 0 | 0 | 0 | 0 |
| `psychoacoustics` | 3 | 9 | 2 | 3 |
| `recording` | 1 | 0 | 0 | 2 |
| `rhythm-time` | 2 | 0 | 0 | 2 |
| `sound-design` | 0 | 0 | 0 | 0 |
| `spatial-audio` | 0 | 0 | 0 | 0 |
| `synthesis` | 1 | 0 | 2 | 2 |

## Session coverage

| Session | Nodes | Vocabulary | Experiments |
|---|---:|---:|---:|
| `session-01-what-is-sound` | 11 | 36 | 3 |
| `session-02-what-is-music` | 13 | 11 | 3 |
| `session-03-history-of-electronic-music` | 7 | 0 | 0 |

## Research-gap candidates

- **domain_representation** — `domain:audio-hardware` — evidence: node_count=0. The canonical domain audio-hardware currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:dj-turntablism` — evidence: node_count=0. The canonical domain dj-turntablism currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:dsp` — evidence: node_count=1. The canonical domain dsp currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:history-culture` — evidence: node_count=0. The canonical domain history-culture currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:machine-audio` — evidence: node_count=0. The canonical domain machine-audio currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:mixing-mastering` — evidence: node_count=0. The canonical domain mixing-mastering currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:music-theory` — evidence: node_count=0. The canonical domain music-theory currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:recording` — evidence: node_count=1. The canonical domain recording currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:sound-design` — evidence: node_count=0. The canonical domain sound-design currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:spatial-audio` — evidence: node_count=0. The canonical domain spatial-audio currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:synthesis` — evidence: node_count=1. The canonical domain synthesis currently contains at most one node; representation is sparse, not defective.

- **practical_evidence** — `node:midi` — evidence: practical_application_count=3, experiment_count=0. The canonical midi node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:recording` — evidence: practical_application_count=3, experiment_count=0. The canonical recording node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:rhythm` — evidence: practical_application_count=3, experiment_count=0. The canonical rhythm node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:sampling` — evidence: practical_application_count=3, experiment_count=0. The canonical sampling node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:sequencing` — evidence: practical_application_count=3, experiment_count=0. The canonical sequencing node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:vibration` — evidence: practical_application_count=3, experiment_count=0. The canonical vibration node declares practical applications but no experiment definition explicitly references it.

- **vocabulary_bridge** — `node:midi` — evidence: vocabulary_count=0. The canonical midi node currently has no explicit vocabulary entry cross-reference.

- **vocabulary_bridge** — `node:recording` — evidence: vocabulary_count=0. The canonical recording node currently has no explicit vocabulary entry cross-reference.

- **vocabulary_bridge** — `node:rhythm` — evidence: vocabulary_count=0. The canonical rhythm node currently has no explicit vocabulary entry cross-reference.

- **vocabulary_bridge** — `node:sequencing` — evidence: vocabulary_count=0. The canonical sequencing node currently has no explicit vocabulary entry cross-reference.

- **vocabulary_bridge** — `node:synthesis` — evidence: vocabulary_count=0. The canonical synthesis node currently has no explicit vocabulary entry cross-reference.
