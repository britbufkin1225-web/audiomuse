---
id: screw-tape
title: Screw Tape
domain: history-culture
status: foundation
session_origin: []
definition: A cassette of slowed, chopped, and often freestyle-carrying material recorded by DJ Screw, sold directly to listeners and circulated further by copying, which functioned simultaneously as recording, performance document, and commodity.
core_concepts:
  - cassette as the primary format
  - live DJ performance captured in one pass
  - custom and commissioned orders
  - handwritten and catalog identification
  - duplication and generational copies
  - bootlegs and later reissues
relationships:
  - target: chopped-and-screwed
    type: captures
  - target: music-distribution
    type: contributes_to
  - target: music-archiving
    type: contributes_to
sources:
  - uh-stories-dj-screw
  - uh-dj-screw-collection-finding-aid
  - popula-screw-tape-records
  - tsha-dj-screw
  - walker-dj-screw-biography
experiments: []
practical_applications:
  - distinguishing a performance, a master, a copy, and a reissue of the same material
  - reading a physical format as a distribution mechanism
project_connections:
  - AudioMuse circulation and format studies
future_questions:
  - How were masters actually dubbed, at what speed, and onto what stock?
  - What do the catalog numbers encode, and does a complete catalog exist in any archive?
---

# Screw Tape

A Screw tape is not one kind of object, and Phase 12C keeps the kinds apart because they carry different evidence:

```text
live DJ performance
  -> recorded pass onto tape (the master)
    -> custom or commissioned copy for a named buyer
      -> duplicated copies sold onward
        -> informal bootleg duplication
          -> later commercial reissue on CD
```

The performance feeds the master, but the finding aid describes a production chain inside that event: two slowed copies are switched with a crossfader to repeat material into a master cassette, then a four-track pitch control slows that master further to produce the final tape. Copies made for circulation are later generations. That is why AudioMuse treats format as historically significant rather than incidental: the recording is a document of a single unrepeatable pass, and most listeners heard a duplicate of it.

What the retrieved sources establish is the economy around those objects. Screw sold from a house on Greenstone Street; a shop later sold the catalog with titles listed on whiteboards and ordered by name or catalog number, with more than three hundred titles available; by 2019 that shop sold CDs rather than cassettes; and bootleg copies were a long-standing dispute, sold in places Screw had no arrangement with. Reported daily volumes are very high and come from participants rather than from measurement, so they are recorded as claims about scale, not as figures.

The finding aid establishes the master-production workflow but not the duplication mechanics for sales copies. It does not describe the duplication deck, dubbing speed, tape stock, or whether copies were run one at a time or in banks. `cassette-duplication` therefore holds what is generally true of the format and explicitly does not attribute a specific sales-copy method to Screw.

`screw-tape --captures--> chopped-and-screwed` states the relationship precisely: the practice happened, and the tape preserved a signal derived from it. The two `contributes_to` edges place the object in the circulation layer and, later, in the archival layer that now holds surviving copies.
