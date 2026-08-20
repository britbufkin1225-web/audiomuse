---
id: temporal-discontinuity
title: Temporal Discontinuity
domain: dsp
status: foundation
session_origin: []
definition: An abrupt break in the continuity of a signal — a cut, a dropout, an omission, a splice — which functions simultaneously as a spectral event, a grouping boundary, and a failure of prediction.
core_concepts:
  - a discontinuity is three events at once: spectral, structural, and predictive
  - an abrupt cut has broadband content the source did not contain
  - a break is a grouping boundary, so it segments what surrounds it
  - an omission at a predicted position is not the same as ordinary quiet
  - the return after a break is an event in its own right
relationships:
  - target: musical-expectation
    type: influences
  - target: silence-as-musical-material
    type: contributes_to
  - target: amplitude-envelope
    type: influences
sources:
  - grahn-brett-beat-perception-motor
  - session-02-what-is-music
  - juslin-brecvema-unified-theory
experiments: []
practical_applications:
  - placing a dropout where the prediction is strongest rather than where the arrangement is thinnest
  - fading over a few milliseconds when the click is unwanted and cutting hard when it is
project_connections:
  - AudioMuse temporal DSP foundation
future_questions:
  - Does an omission at a strongly predicted position produce a measurably different response from one at a weakly predicted position?
  - How brief can a dropout be and still register as an event rather than as a fault?
---

# Temporal Discontinuity

This is the perceptual consequence that the whole temporal-DSP family produces, which is why it has a node of its own rather than living inside `gating`.

A hard break is three things at once, and separating them explains most of what practitioners already do by instinct.

**A spectral event.** An instantaneous amplitude change is broadband: the click at a bad splice is not a defect added to the signal, it is what a step function sounds like. Fading over a few milliseconds removes it. Cutting hard keeps it, and sometimes that is the point — the transient at the cut is an onset the material did not have.

**A grouping boundary.** Sounds separated by a gap are heard as belonging to different groups. Cutting a phrase in two makes two phrases, and where the cut lands determines what the listener takes as a unit. This is why chopping changes structure rather than just shortening things.

**A prediction failure.** Session 2 already records that a repeated pattern builds an expectation which can be interrupted, and Juslin's musical expectancy mechanism describes emotion induced by confirmation, delay, or violation of expectations built from previous experience of a style. An omission at a strongly predicted position is therefore not silence; it is a specific prediction failing, which is a different perceptual event from the same passage having been quiet from the start.

The three combine into the effect every producer knows: the return after a dropout hits harder than its level accounts for. Grahn and Brett's finding supplies part of the reason — an onset not closely preceded or followed by other onsets is perceived as accented — and dynamic contrast supplies the rest, with Juslin's brain stem reflex describing exactly the sudden dynamic change case.

The practical corollary is that a dropout's strength depends on the prediction it breaks, not on its own duration. A break in an established groove is loud; the same break in a passage with no established pattern is just a gap. `musical-expectation` is what a discontinuity acts on, and `gating` and `chopping` are the two most direct ways of producing one.
