---
id: gating
title: Gating
domain: dsp
status: foundation
session_origin: []
definition: Opening and closing a signal path according to a control signal, so that material present in the source is heard only during the open intervals, producing articulation, silence, and rhythm that the source did not contain.
core_concepts:
  - a control signal decides audibility, the source decides content
  - gate rate and duty cycle as the two primary parameters
  - hard gating produces discontinuity; slewed gating produces tremolo
  - crossfader gating and threshold gating are the same operation with different controls
  - the rhythm produced belongs to the gate, not to the material
relationships:
  - target: temporal-dsp
    type: used_in
  - target: amplitude-envelope
    type: influences
  - target: temporal-discontinuity
    type: produces
  - target: rhythm
    type: contributes_to
  - target: silence-as-musical-material
    type: contributes_to
sources:
  - head-acoustics-psychoacoustic-analyses
  - grahn-brett-beat-perception-motor
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - imposing a rhythm on sustained material without editing it
  - hearing a crossfader gesture, a noise gate, and a sidechain duck as one operation
project_connections:
  - AudioMuse temporal DSP foundation
  - AudioMuse DJ and turntablism domain
future_questions:
  - At what duty cycle does a gated sustained sound stop being heard as continuous with holes and start being heard as a new rhythmic part?
  - Does an irregular gate pattern reduce entrainment the way high syncopation does?
---

# Gating

A gate decides when a signal is audible. Everything else about it follows from the fact that this decision is made by a separate control signal, so the rhythm a gate produces is a property of the control and not of the material.

Two parameters carry most of the perceptual weight. **Gate rate** is how often the gate opens, and it sits on the same continuum every rate in Phase 12E sits on: slow enough and it is phrasing, faster and it is rhythm, faster still and the HEAD acoustics continuum applies — pulsation below about 10 Hz, roughness between roughly 20 and 300 Hz, resolved sidebands above that. A gate driven into the tens of hertz stops being a rhythm device and becomes a timbre device, and the transition is not gradual in category even though the knob turns smoothly. **Duty cycle** is the proportion of each period during which the gate is open, and it controls how much of the source survives. At high duty cycle the material sounds continuous with holes in it; at low duty cycle the holes become the material and what survives reads as a series of separate events.

Gating produces accents it never applies. Grahn and Brett report that an onset not closely followed by another onset is perceived as accented. A gate that leaves a longer gap after one hit has accented that hit, using only removal.

The edge cases matter for how AudioMuse connects this to existing material. A gate with instantaneous transitions produces `temporal-discontinuity`, with clicks if the transitions land off zero crossings. A gate with slewed transitions is tremolo — the same operation, made continuous. A threshold-triggered noise gate, a sidechain compressor ducking on a kick, an LFO-driven volume element, and a DJ cutting with a crossfader are all this operation with different control sources. `scratching` and `chopping` already describe the crossfader form in the AudioMuse DJ material, and Phase 12E's contribution is naming what is shared: amplitude opened and closed on a schedule, against material that knows nothing about it.

Experiment `temporal-gating-and-expectation` varies gate rate, duty cycle, regularity, and dropout placement against the same source, which is the smallest design that separates these parameters.
