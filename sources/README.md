# Sources

Sources are AudioMuse provenance records: they answer, "Where did this knowledge come from?"

`source-registry.yaml` assigns stable IDs to repository materials and future external references. Nodes cite those IDs rather than duplicating paths or bibliographic details. Registry entries follow `schemas/source.schema.yaml`; fields that are unknown or inappropriate may be omitted. Relationships describe how a source supports AudioMuse knowledge, and provenance remains manually curated.

The existing `research/sources/` directory remains available for bibliographies and source-specific research notes. The root `sources/` directory is the canonical ID registry.

## External sources

The index builder resolves every registry `locator` as a repository-relative path, so an external work
is registered against a research note that holds its citation rather than against a bare URL. Phase 12C
established the convention in `research/sources/houston-third-coast/`: one note per source, carrying
the full citation, the stable external locator, what the source is allowed to support, what it does not
settle, and a retrieval status recording whether its text was actually read or only its identity
confirmed. The registry entry repeats the external locator and that retrieval status in `notes`.

Retrieval status is part of the provenance, not a formality. A claim resting only on a source whose
text could not be retrieved is recorded as reported rather than verified, and never becomes the sole
basis for a date, address, quotation, or attribution.
