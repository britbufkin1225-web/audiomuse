---
id: amplitude-envelope
title: Amplitude Envelope
domain: acoustics
status: foundation
session_origin:
  - session-01-what-is-sound
definition: The shape traced by a signal's amplitude over time, which determines where a listener hears an event begin, how it develops, and where it stops, independently of the spectrum it carries.
core_concepts:
  - attack, decay, sustain, release as a description of shape rather than a synthesizer control
  - onset as the perceptual start of an event
  - envelope as the carrier of event boundaries
  - modulation as periodic change of the envelope
  - the same spectrum with two envelopes is heard as two different sounds
relationships:
  - target: sound
    type: characterized_by
  - target: timbre
    type: contributes_to
  - target: rhythm
    type: contributes_to
  - target: roughness
    type: influences
sources:
  - session-01-what-is-sound
  - mcadams-affective-qualities-instrument-sounds
  - head-acoustics-psychoacoustic-analyses
experiments: []
practical_applications:
  - deciding whether a problem in a mix is spectral or temporal before reaching for an equalizer
  - reading a gate, a compressor, and a fade as three ways of drawing the same curve
project_connections:
  - AudioMuse temporal DSP foundation
future_questions:
  - How short can an envelope segment become before it stops being heard as an event and starts being heard as texture?
  - Which envelope descriptors are stable enough across instruments to be worth measuring in AudioMuse?
---

# Amplitude Envelope

The envelope is the amplitude of a signal considered as a function of time, with the waveform's individual cycles set aside. It is the layer at which most of Phase 12E's temporal operations actually work: a gate, a compressor, a fade, a tremolo, and a stutter all redraw this curve and leave the spectrum comparatively alone.

Its importance is that event boundaries live here. A listener does not receive a stream of samples and then decide where the notes are; the rises in the envelope are what make an onset available to be heard at all. Once an onset exists it can be counted, grouped, predicted, and displaced, which is how a physical amplitude curve becomes musical time. `temporal-discontinuity` holds what happens when the curve is broken; `rhythm` holds what happens when the breaks recur.

Envelope shape also contributes to timbre, and not only as a secondary cue. McAdams, Douglas, and Vempala found that affective ratings of isolated instrument tones related to envelope descriptors as well as spectral ones — sharper attack with earlier decay associated with more positive valence, gentler attacks with higher rated tension — while stressing that these were isolated tones rather than music and that different descriptor combinations carried different affective dimensions. AudioMuse takes from that the structural point rather than a lookup table: attack and decay are affectively relevant variables, and they are separable from brightness.

Repeated envelope change is modulation, and modulation rate is a perceptual variable in its own right rather than a faster version of the same thing. The HEAD acoustics technical note records the continuum AudioMuse builds on: below roughly 10 Hz the ear tracks the change as pulsation, fluctuation strength peaks near 4 Hz, envelope variation between roughly 20 and 300 Hz is heard as roughness, and above that band the sidebands separate out as individual tones. `micro-looping` and `stutter` are what happens when a musician drives a musical repeat up through those regions.
