# Claims

A claim record says what one statement asserts, what kind of statement it is, what repository evidence stands behind it, and how strong that evidence is. It exists because `docs/houston-musical-cartography.md` had to carry all four of those in prose, and prose cannot be validated.

Canonical records live in `claims/records/*.yaml` and conform to `schemas/claim.schema.yaml`. `claims/index.md` is a generated read-only projection; do not edit it.

```powershell
pwsh -NoProfile -File tools/build-claim-index.ps1
pwsh -NoProfile -File tools/validate-claims.ps1
pwsh -NoProfile -File tools/test-claim-validator.ps1
```

## What the layer is for

Four things are stored separately and never combined into one number:

```text
CLAIM TYPE   what kind of statement this is
CONFIDENCE   how strongly repository evidence supports it
EVIDENCE     which registered sources support, contradict, or qualify it
DISPUTE      whether registered sources conflict
```

A `historical_claim` may be `high`. An `oral_history` claim may be `moderate`. An `established_fact` may not be `low`. Confidence grades the evidence, not the model's feeling about the statement, and there are no numeric scores.

## What is and is not a claim

Not every sentence in AudioMuse becomes a record. Write one when the statement is externally checkable and something depends on getting it right: a date, an attribution, a priority or origin assertion, a contested chronology, a technical statement a reader might act on, or a conclusion AudioMuse reached by combining sources. Ordinary explanatory prose, definitions, and navigation stay as prose.

A claim's `appears_in` list names where in the repository the statement is actually made. Canonical nodes and documents do not list their claims in return: the reverse view is derived in `claims/index.md`, following the same rule the graph uses for inverse edges.

## Records

Each file holds several `---`-separated documents, one per claim, in the same flat `key: <JSON value>` form the vocabulary and experiment layers use. Grouping is by subject, not by claim type.

## Reading the index

`## By Source` lists one row per source, claim, and relation, so a source that both supports and qualifies the same claim appears twice under it; the count in the heading is distinct claims. `## By Appearance Site` groups by `kind` first and then by reference, in ordinal order.

The full model, the semantics of every vocabulary value, and the migration plan are in `docs/claim-provenance-model.md`.
