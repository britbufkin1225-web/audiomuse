---
id: music-and-memory
title: Music and Memory
domain: psychoacoustics
status: foundation
session_origin: []
definition: The relationship between music and remembering, in which a particular recording can carry a personal association strong enough to produce an affective response that none of its acoustic properties would predict.
core_concepts:
  - familiarity and autobiographical salience as separable variables
  - a recording as a retrieval cue rather than as a stimulus
  - acoustic causation and learned association are different explanations
  - the strongest response can attach to musically unremarkable material
  - retrospective report about one's own memories is itself unreliable evidence
relationships:
  - target: music-evoked-emotion
    type: contributes_to
  - target: musical-expectation
    type: influences
sources:
  - janata-music-evoked-autobiographical-memory
  - juslin-brecvema-unified-theory
  - harrison-loui-frisson-model
experiments: []
practical_applications:
  - recognizing that a reference track may be chosen for its history rather than its sound
  - separating "this works" from "this works on me" when evaluating a mix
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Can a listening test hold familiarity constant across participants without using unfamiliar material only?
  - Does the association attach to the specific recording, to the arrangement, or to the melodic content?
---

# Music and Memory

This node exists to block a specific error: attributing to a signal what belongs to a listener's history.

Juslin's framework names two separate mechanisms here. Evaluative conditioning is repeated pairing of a piece with something positive or negative until the piece alone carries the valence, and it is described as subconscious, unintentional, and effortless. Episodic memory is the piece evoking a specific remembered event, with the emotion of the event arriving alongside it. Neither requires the music to have any particular acoustic property. A jingle and a symphony are equally eligible.

Janata's imaging study gives the pairing a measurable correlate. Dorsal medial prefrontal cortex responded parametrically as autobiographical salience and familiarity increased, and the two were separable: salience produced effects beyond what familiarity alone predicted. The study also modelled tonality tracking independently of subjective salience and reports that the two operate on different timescales — the structure of the music and the personal meaning attached to it are not the same signal.

The consequence for AudioMuse is a permanent qualification on every affective claim in this phase. A listener may respond powerfully to a recording whose loudness, roughness, harmonicity, tempo, and timbre predict nothing of the kind, because the recording was playing during something that mattered. Harrison and Loui report the same variable turning up in the frisson literature: people are more likely to react physically to familiar music. Any listening test that does not control familiarity is measuring an unknown mixture.

The `music-and-memory --influences--> musical-expectation` edge records the other direction of the relationship. A remembered piece is a piece whose continuation the listener already knows, which changes what prediction and violation can even mean for it. Janata's own limitation is worth carrying too: he notes the postscan memory survey was probably more susceptible to false memory and distortion than immediate collection, so even the report of the association is evidence of a particular grade.
