# AudioMuse Experiment Runs

Experiment definitions describe repeatable exercises; run records describe individual planned or performed executions. Keeping `experiment-runs/` separate prevents mutable result history from changing canonical definitions and lets one definition have zero, one, or many runs. Runs reference experiment IDs and optional registered sources, but do not add nodes or graph edges.

## Evidence and lifecycle contract

`status` uses four stable execution states: `planned` means no execution occurred and therefore requires `run_date: null` and no evidence or interpretation; `completed` means the procedure finished and requires at least one observation or measurement; `incomplete` means execution began but did not finish; `invalid` means execution occurred but its evidence must not support conclusions. Performed states require an ISO `YYYY-MM-DD` date. Incomplete or invalid records may preserve observations or measurements, but an invalid record cannot contain interpretation. A larger workflow enum and timestamps were rejected because this repository needs evidence integrity, not task management or invented precision.

An observation is a direct qualitative or perceptual statement plus the context in which it was noticed. Subjective listening notes are valid observations, not calibrated facts. A measurement is numeric and must identify its quantity, non-empty unit, method, tool, calibration knowledge, uncertainty (or explicit `null`), and limitations. Dimensionless quantities use unit `1`. Requiring this context prevents a number from masquerading as measured evidence.

Nominal oscillator frequencies, DAW values, sliders, and other configured inputs belong in `control_settings`, whose `context` must say what was configured. They are not measurements of physical acoustic output. Uncalibrated displays may be documented with `calibration: unknown` only when they actually produced a numeric reading and their limitation is recorded. An observation is never promoted automatically into scientific fact or canonical node status; interpretation remains a separate authored field.

All arrays are explicit, and nested objects reject unsupported fields. Calibration is limited to `known`, `unknown`, and `not-applicable`: a Boolean was rejected because it cannot distinguish absent calibration knowledge from a method that does not require calibration. Measurement values are numeric; estimates and ranges belong in uncertainty/limitations rather than fake decimal precision.

The committed records are intentionally evidence-free planned examples. Their null dates, planned status, and empty evidence fields prove references, multiplicity, and index behavior without asserting that a person performed a test. Do not fill them with hypothetical results; create or update a run only from an actual execution.

```text
session/source knowledge
        ↓
canonical nodes + vocabulary
        ↓
experiment definition
        ↓
experiment run
        ↓
observation / measurement
```

This is an authoring and evidence relationship, not a graph mutation path.

## Build and validation

Records use the JSON-compatible YAML line format in `schemas/experiment-run.schema.yaml`. The index is generated and must not be edited.

```powershell
pwsh -NoProfile -File tools/build-experiment-run-index.ps1
pwsh -NoProfile -File tools/validate-experiment-runs.ps1
```
