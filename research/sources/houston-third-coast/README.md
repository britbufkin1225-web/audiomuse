# Houston / Third Coast Source Notes

One file per external source registered for Phase 12C in `sources/source-registry.yaml`. The registry
holds the stable ID; these notes hold the citation, the stable external locator, what the source is
allowed to support, and what it does not settle.

Each note records a **Retrieval** line, because provenance strength depends on whether the text was
actually read:

- `full text retrieved` — the page or record was fetched and read while writing this phase.
- `citation verified, full text not retrieved` — the work's identity (title, publisher, author, date,
  stable locator) was confirmed through a search index, but automated retrieval was refused by the
  host. Claims resting only on such a source are marked as reported rather than verified, and are
  never the sole basis for a date, address, quotation, or attribution.

Source class follows the Phase 12C hierarchy: primary/participant, institutional archive, reference
encyclopedia, scholarly book, technical reference, journalism. Class is recorded here rather than in
the registry, which has a fixed `type` enum for format rather than evidentiary weight.
