# AudioMuse Experiments

An AudioMuse experiment is a bounded, repeatable exercise connecting existing knowledge to listening, observation, visualization, or recorded values. It is not an application, automated laboratory, DSP engine, or substitute for calibrated testing. Canonical records live in `records/`; `index.md` is a disposable navigation projection.

The purpose of this layer is `concept → listening task → observation or measurement → interpretation → related knowledge`. Experiments reference stable node, vocabulary, session, and source IDs, but they remain subordinate to the canonical graph and never create graph edges.

## Contract decisions

Every record identifies its purpose, references, equipment, safety conditions, setup, procedure, expected behavior, limitations, and repeatability information because those fields determine whether another reader can understand and reproduce the exercise. `observations` and `measurements` are separate: an observation is qualitative (for example, “bass appeared louder near the wall”), while a measurement is a quantitative value accompanied by its method and chain. Neither is allowed to impersonate the other.

`status` is limited to `proof` and `established`; current records are proofs of the repository contract, not claims of laboratory validation. `type` is limited to `listening`, `visualization`, and `hybrid`. A separate `measurement` type was rejected because measurement describes evidence collected, not the activity as a whole; any type may record a measurement. `comparative` was rejected because comparison is a procedure shared by all three types. Difficulty is intentionally only `introductory` or `intermediate` until real exercises justify more resolution.

All fields are required, but arrays may be empty when genuinely inapplicable. This prevents silent omission while avoiding speculative values. Free-form scientific hypotheses, result storage, ownership, timestamps, and completion tracking were not adopted: they would add runtime or laboratory semantics that this repository does not need.

## Safety and evidence

- Establish a conservative level before starting tones; avoid abrupt changes, prolonged pure-tone exposure, feedback, clipping, overload, unsafe wiring, and discomfort.
- Records distinguish speaker-dependent room exercises from exercises that may use headphones.
- Quantitative acoustic claims require a documented calibrated measurement chain. Uncalibrated displays and listening impressions remain observations.
- Reuse registered sources for supported claims. Add a source only when a new factual claim needs it; do not inflate provenance.

## Authoring and generated view

Records use exactly the JSON-compatible YAML fields in `schemas/experiment.schema.yaml`. Keep the proof set small; future sessions should add an experiment only when it tests a supported concept and provides a useful, safe procedure.

```powershell
pwsh -NoProfile -File tools/build-experiment-index.ps1
pwsh -NoProfile -File tools/validate-experiments.ps1
```

The index provides only A–Z, type, node, vocabulary, and session views. Equipment remains in canonical records because grouping prose equipment descriptions would create noisy, unstable navigation.
