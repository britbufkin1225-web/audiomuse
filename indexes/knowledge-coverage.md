# AudioMuse Knowledge Coverage

> Generated, read-only, and non-authoritative. Rebuild with `tools/build-knowledge-coverage.ps1`; canonical repository content always wins.

Coverage measures explicit repository relationships. It is not correctness, quality, truth, or universal completeness. A candidate is evidence for human review, not a knowledge defect.

## Repository overview

- node_count: 21
- relationship_count: 63
- relationship_type_count: 11
- source_count: 5
- session_count: 3
- vocabulary_count: 64
- experiment_count: 3
- experiment_run_count: 2

## Node coverage

| Node | Domain | Sessions | Sources | Vocabulary | Experiments | Completed runs | In | Out | Type diversity |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `beatmatching` | dj-turntablism | 2 | 2 | 2 | 0 | 0 | 1 | 1 | 2 |
| `digital-signal-processing` | dsp | 3 | 3 | 2 | 1 | 0 | 4 | 4 | 4 |
| `digital-vinyl-system` | dj-turntablism | 1 | 1 | 3 | 0 | 0 | 1 | 2 | 2 |
| `djing` | dj-turntablism | 3 | 3 | 7 | 0 | 0 | 5 | 1 | 2 |
| `frequency` | acoustics | 2 | 3 | 2 | 3 | 0 | 5 | 6 | 4 |
| `midi` | digital-audio | 2 | 2 | 1 | 0 | 0 | 0 | 5 | 3 |
| `phase` | acoustics | 2 | 3 | 1 | 2 | 0 | 2 | 2 | 2 |
| `pitch` | psychoacoustics | 2 | 3 | 2 | 1 | 0 | 4 | 1 | 4 |
| `psychoacoustics` | psychoacoustics | 2 | 3 | 1 | 1 | 0 | 0 | 5 | 1 |
| `recording` | recording | 2 | 2 | 0 | 0 | 0 | 2 | 5 | 4 |
| `resonance` | acoustics | 2 | 3 | 1 | 1 | 0 | 2 | 3 | 2 |
| `rhythm` | rhythm-time | 2 | 2 | 9 | 0 | 0 | 4 | 3 | 4 |
| `sampling` | digital-audio | 3 | 4 | 4 | 0 | 0 | 8 | 4 | 6 |
| `scratching` | dj-turntablism | 3 | 3 | 2 | 0 | 0 | 0 | 4 | 4 |
| `sequencing` | rhythm-time | 2 | 2 | 2 | 0 | 0 | 2 | 2 | 3 |
| `sound` | acoustics | 2 | 3 | 1 | 1 | 0 | 6 | 3 | 6 |
| `synthesis` | synthesis | 2 | 2 | 4 | 2 | 0 | 3 | 4 | 3 |
| `timbre` | psychoacoustics | 2 | 3 | 3 | 1 | 0 | 7 | 0 | 6 |
| `turntable` | dj-turntablism | 2 | 2 | 5 | 0 | 0 | 1 | 3 | 3 |
| `turntablism` | dj-turntablism | 2 | 2 | 5 | 0 | 0 | 4 | 2 | 4 |
| `vibration` | acoustics | 1 | 2 | 1 | 0 | 0 | 2 | 3 | 4 |

## Domain coverage

| Domain | Nodes | Vocabulary | Experiments | Sources |
|---|---:|---:|---:|---:|
| `acoustics` | 5 | 19 | 3 | 3 |
| `audio-hardware` | 0 | 0 | 0 | 0 |
| `digital-audio` | 2 | 11 | 0 | 4 |
| `dj-turntablism` | 6 | 12 | 0 | 3 |
| `dsp` | 1 | 2 | 1 | 3 |
| `history-culture` | 0 | 0 | 0 | 0 |
| `machine-audio` | 0 | 0 | 0 | 0 |
| `mixing-mastering` | 0 | 0 | 0 | 0 |
| `music-theory` | 0 | 0 | 0 | 0 |
| `psychoacoustics` | 3 | 9 | 2 | 3 |
| `recording` | 1 | 0 | 0 | 2 |
| `rhythm-time` | 2 | 7 | 0 | 2 |
| `sound-design` | 0 | 0 | 0 | 0 |
| `spatial-audio` | 0 | 0 | 0 | 0 |
| `synthesis` | 1 | 4 | 2 | 2 |

## Session coverage

| Session | Nodes | Vocabulary | Experiments |
|---|---:|---:|---:|
| `session-01-what-is-sound` | 15 | 43 | 3 |
| `session-02-what-is-music` | 17 | 32 | 3 |
| `session-03-history-of-electronic-music` | 12 | 12 | 0 |

## Research-gap candidates

- **domain_representation** — `domain:audio-hardware` — evidence: node_count=0. The canonical domain audio-hardware currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:dsp` — evidence: node_count=1. The canonical domain dsp currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:history-culture` — evidence: node_count=0. The canonical domain history-culture currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:machine-audio` — evidence: node_count=0. The canonical domain machine-audio currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:mixing-mastering` — evidence: node_count=0. The canonical domain mixing-mastering currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:music-theory` — evidence: node_count=0. The canonical domain music-theory currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:recording` — evidence: node_count=1. The canonical domain recording currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:sound-design` — evidence: node_count=0. The canonical domain sound-design currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:spatial-audio` — evidence: node_count=0. The canonical domain spatial-audio currently contains at most one node; representation is sparse, not defective.

- **domain_representation** — `domain:synthesis` — evidence: node_count=1. The canonical domain synthesis currently contains at most one node; representation is sparse, not defective.

- **practical_evidence** — `node:beatmatching` — evidence: practical_application_count=3, experiment_count=0. The canonical beatmatching node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:digital-vinyl-system` — evidence: practical_application_count=3, experiment_count=0. The canonical digital-vinyl-system node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:djing` — evidence: practical_application_count=3, experiment_count=0. The canonical djing node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:midi` — evidence: practical_application_count=3, experiment_count=0. The canonical midi node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:recording` — evidence: practical_application_count=3, experiment_count=0. The canonical recording node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:rhythm` — evidence: practical_application_count=3, experiment_count=0. The canonical rhythm node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:sampling` — evidence: practical_application_count=3, experiment_count=0. The canonical sampling node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:scratching` — evidence: practical_application_count=3, experiment_count=0. The canonical scratching node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:sequencing` — evidence: practical_application_count=3, experiment_count=0. The canonical sequencing node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:turntable` — evidence: practical_application_count=3, experiment_count=0. The canonical turntable node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:turntablism` — evidence: practical_application_count=3, experiment_count=0. The canonical turntablism node declares practical applications but no experiment definition explicitly references it.

- **practical_evidence** — `node:vibration` — evidence: practical_application_count=3, experiment_count=0. The canonical vibration node declares practical applications but no experiment definition explicitly references it.

- **vocabulary_bridge** — `node:recording` — evidence: vocabulary_count=0. The canonical recording node currently has no explicit vocabulary entry cross-reference.
