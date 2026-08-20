---
id: houston-musical-cartography
title: Houston Musical Cartography
domain: history-culture
status: foundation
session_origin: []
definition: A layered method for describing a city's music history in which place, institution, people, practice, technology, circulation, and influence are recorded as separate connected layers rather than collapsed into a list of artists and release years.
core_concepts:
  - place before practice
  - institutions as infrastructure
  - technique as a technical claim
  - circulation as part of the history
  - evidence recorded with the claim
  - uncertainty stated rather than smoothed
relationships:
  - target: houston-rap
    type: studies
  - target: houston-studio-infrastructure
    type: studies
  - target: music-distribution
    type: studies
  - target: third-coast
    type: studies
sources:
  - uh-houston-hip-hop-research-collection
  - tsha-duke-peacock-records
  - tsha-sugarhill-recording-studios
  - black-enterprise-kcoh
  - wilkins-welcome-2-houston
experiments: []
practical_applications:
  - reading a regional scene as infrastructure rather than as a discography
  - locating where a technique became possible, not only when it appeared
  - separating what a source establishes from what it merely repeats
project_connections:
  - AudioMuse regional history model
  - AudioMuse provenance and claim-confidence work
future_questions:
  - Does the same layering hold for a city whose music economy was never independent of major labels?
  - What would AudioMuse need to store to compare two cities' infrastructure layers directly?
---

# Houston Musical Cartography

Cartography is the method, not the subject. A conventional music timeline moves year by year through artists and records, which makes a scene look like a sequence of releases. This node holds a different ordering, and the rest of the Phase 12C material is arranged to obey it:

```text
PLACE
  -> INSTITUTION / INFRASTRUCTURE
    -> PEOPLE
      -> MUSICAL PRACTICE / TECHNIQUE
        -> TECHNOLOGY / PLAYBACK / PRODUCTION
          -> CIRCULATION
            -> CULTURAL INFLUENCE
```

Read downward, the layering answers a question a discography cannot: why a technique was possible in one city at one time. Houston had Black-owned label and catalog infrastructure from 1949, when Don Robey established Peacock Records and named it after his Fifth Ward club, and a commercial studio economy from 1941, when Bill Quinn opened the room that became Gold Star and later SugarHill. Robey recorded at Bill Holford's ACA and sent work to Gold Star for mastering. Broadcast infrastructure belongs to the same layer: KCOH began broadcasting in 1948 and, after its 1953 purchase, programmed to Houston's African American community for decades. Those are institutions, rooms, and engineers — the layer a later rap economy could inherit. When Houston rap became a national presence in the 1990s, it did so through an infrastructure that already existed, and the chronology in `docs/houston-musical-cartography.md` is written to make that inheritance visible rather than implied.

The four `studies` edges name what this model is currently applied to. They are directed claims about inquiry, not about causation: the cartography examines Houston rap, the city's studio infrastructure, its distribution layer, and the contested regional term. Causal and technical claims live on the nodes themselves, where they can carry their own sources.

Two commitments make this more than a narrative frame. First, technique is treated as a technical claim: what a playback device physically did to a signal belongs in the record alongside who did it. Second, circulation is treated as history rather than commerce trivia — a duplicated cassette, a distributor's warehouse, and a recognizable cover are the mechanisms by which music reached a listener at all.

The third commitment is evidentiary. Houston's rap history in particular reaches the present through oral history, retrospective interviews, and community memory, and those accounts disagree with each other and occasionally with institutional records. AudioMuse does not resolve such disagreements by choosing the more repeated version. The chronology document marks each unresolved point where it occurs, and the nodes below repeat those marks rather than quietly inheriting a clean date. `music-archiving` records the other half of that discipline: the archives that make any of this checkable.
