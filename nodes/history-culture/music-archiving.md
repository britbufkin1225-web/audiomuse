---
id: music-archiving
title: Music Archiving
domain: history-culture
status: foundation
session_origin: []
definition: The institutional preservation of recordings, documents, and objects from a music scene so that later claims about that scene can be checked against evidence rather than memory.
core_concepts:
  - scene to document to collection
  - finding aids and described holdings
  - format obsolescence and carrier decay
  - rediscovery and reissue
  - archives as the basis of historical confidence
relationships:
  - target: houston-musical-cartography
    type: enables
sources:
  - uh-houston-hip-hop-research-collection
  - uh-dj-screw-collection-finding-aid
  - uh-cia-records-finding-aid
  - hpl-african-american-history-research-center
  - ida-thunder-soul-kashmere
experiments: []
practical_applications:
  - checking a historical claim against a described holding instead of a repeated story
  - identifying which parts of a scene were never documented at all
project_connections:
  - AudioMuse provenance and claim-confidence work
future_questions:
  - What has already been lost because the carrier decayed before it was collected?
  - How should AudioMuse cite a described archival holding it has not physically consulted?
---

# Music Archiving

Archiving is the layer that makes the rest of this phase checkable:

```text
SCENE -> DOCUMENT -> COLLECTION -> ARCHIVE -> RESEARCH -> CULTURAL MEMORY
```

Houston supplies working examples at each step. The University of Houston Libraries holds a research collection documenting Houston hip hop, including DJ Screw's papers, photographs, equipment, sound recordings, and tapes, curated by a named archivist, together with material from the Geto Boys, the Screwed Up Click, Fat Pat, and HAWK. The same institution holds the records of C.I.A. Records, the Houston punk label formed by the band Really Red, which means the city's independent rock economy is described in the same building as its rap economy. The Houston Public Library maintains a municipal archive for African American history in the city's first public school for Black students. These are institutions with finding aids, not private collections, and that distinction is what allows a claim to be audited by someone else later.

Archives also recirculate. The Kashmere Stage Band recorded at a Houston high school under Conrad Johnson, who brought funk and soul arrangements into the program from 1968 and retired in 1978; a track was sampled decades later, a two-disc reissue followed in 2006, an NPR story about that reissue reached a filmmaker, alumni reunited in 2008, and a documentary reached theaters in 2011. Original recordings became a reissue, which became a film, which produced new listeners and new sampling material. AudioMuse records this loop as a demonstration that preservation is an active part of a city's music history and makes no claim of influence between the Kashmere material and chopped and screwed practice, because none of the retrieved sources asserts one.

The single outbound edge is the honest one: without described holdings, the cartography would be a set of stories. `music-archiving --enables--> houston-musical-cartography` states that dependency directly.
