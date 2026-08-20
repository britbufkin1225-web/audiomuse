---
id: temporal-displacement
title: Temporal Displacement
domain: rhythm-time
status: foundation
session_origin:
  - session-02-what-is-music
definition: Placement of an event earlier or later than the position a listener predicts, at scales from microtiming to whole-phrase anticipation, which changes felt rhythm without changing which events occur.
core_concepts:
  - the same notes at different offsets are a different rhythm
  - swing, push, drag, and syncopation as instances of one variable
  - displacement relative to a predicted grid, not to a printed one
  - a performed displacement carries a performer's timing; an edited one does not
  - displacement is an expectation event
relationships:
  - target: rhythm
    type: influences
  - target: musical-expectation
    type: influences
  - target: groove
    type: influences
sources:
  - session-02-what-is-music
  - witek-syncopation-groove-pleasure
  - grahn-brett-beat-perception-motor
experiments: []
practical_applications:
  - deciding whether to quantize by asking what the displacement is doing rather than whether it is accurate
  - hearing a hand-performed chop as carrying timing information a software slice does not
project_connections:
  - AudioMuse rhythm and expectation studies
  - AudioMuse Houston chopped-and-screwed foundation
future_questions:
  - How large a displacement can be applied before the event is heard as belonging to a different beat?
  - Is there a measurable difference between listener responses to performed and to programmed displacement?
---

# Temporal Displacement

Displacement is the variable that makes rhythm a performance rather than a list. The events are the same; only their positions relative to an expected grid change, and the result can be a different feel, a different metre, or a different genre entirely.

The unifying observation is that displacement is defined against a prediction, not against a printed position. A listener infers a pulse from what has already happened, and an event arriving early or late is early or late relative to that inference. This is why `musical-expectation` is downstream of this node and not merely adjacent to it: syncopation is a displacement large and systematic enough to contradict the inferred metre, and swing is a displacement small and systematic enough to be absorbed into it.

Witek and colleagues supply the affective consequence at one scale. Their inverted U — medium syncopation producing the most wanting-to-move and the most pleasure, with both extremes lower — is a result about how much contradiction of the inferred metre a listener rewards. `groove` holds it.

Grahn and Brett supply a mechanism at a smaller scale that Phase 12E uses repeatedly. Perceptual accent can arise from temporal position alone, with pitch and loudness held constant; an onset not closely followed by another onset is heard as accented, as is the final onset of a run of two or three. Displacement therefore redistributes accent even when nothing about the sounds changes, which is how a purely temporal edit produces an apparently dynamic result.

Existing AudioMuse material meets this node from the practical side. Session 2 already records temporal displacement and groove; `chopping` records that a chop performed against a moving record carries the performer's timing in a way a software slice does not, and `chopped-and-slowed` develops the comparison. Phase 12E adds the reason the difference could matter perceptually: a performed displacement is a distribution of small offsets, and an edited one is usually a single deliberate offset, and those are not the same stimulus.
