---
id: third-coast
title: Third Coast
domain: history-culture
status: seed
session_origin: []
definition: A contested regional term used for Texas, the Gulf Coast, or the wider South as a third center of American popular music alongside the East and West Coasts, whose boundaries differ by speaker and context.
core_concepts:
  - competing geographic definitions
  - Texas usage versus Gulf Coast usage
  - regional network rather than single scene
  - a term with no single point of origin
relationships: []
sources:
  - sarig-third-coast
  - museum-gulf-coast-ugk
  - tsha-pimp-c
experiments: []
practical_applications:
  - resisting the collapse of several regional scenes into one genre label
project_connections:
  - AudioMuse regional history model
future_questions:
  - When and where does the term first appear in print, and who was using it about whom?
  - Do the people described by the term use it about themselves, and with which boundaries?
---

# Third Coast

Third Coast is registered as a term under dispute rather than as a place. It is used for the Texas coast, for Texas generally, for the Gulf Coast region, and for Southern hip hop as a whole; a book-length work of journalism uses it as the organizing frame for the South's rise in hip hop. AudioMuse records that range of usage and does not adopt a definition, because adopting one would silently decide a question the sources leave open.

The practical consequence is a rule about modeling. Port Arthur is not Houston. The Museum of the Gulf Coast, a Port Arthur institution, documents UGK as a Port Arthur duo formed in 1987; the Handbook of Texas records Pimp C's birth in Port Arthur, the duo's 1988 cassette on the Houston label Bigtyme Recordz, and a 1992 deal with Jive, while also describing UGK as a Houston duo. Both framings are in the record. AudioMuse resolves this by modeling a network rather than a boundary:

```text
Port Arthur
   <-> Gulf Coast / Third Coast network
      <-> Houston studios, labels, distribution, radio, and audience
```

That structure lets Houston's infrastructure explain part of a Port Arthur group's career without annexing the place it came from. A full UGK and Port Arthur history is deferred; this node exists so that the deferral is explicit and so that later work has somewhere defensible to attach.

The node stores no outbound relationships. Nothing retrieved supports a directed claim from the term to a practice or a place, and inventing one to improve traversal is exactly the failure mode the relationship rules prohibit. Its inbound edges — from `houston-rap` and from the cartography model — carry the connections that are supported.
