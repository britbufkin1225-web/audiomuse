---
id: micro-looping
title: Micro-Looping
domain: dsp
status: foundation
session_origin: []
definition: Looping of a segment short enough that the loop period itself becomes a perceptual variable, carrying material along a continuum from rhythm through flutter and buzz to perceived pitch as the period shortens.
core_concepts:
  - loop period as a frequency, not only as a duration
  - the continuum from rhythm to texture to tone
  - the loop point becomes a periodic discontinuity, and that discontinuity has a spectrum
  - source identity survives long loops and dissolves in short ones
  - the same operation spans two domains AudioMuse has treated separately
relationships:
  - target: temporal-dsp
    type: used_in
  - target: repetition
    type: contributes_to
  - target: roughness
    type: influences
  - target: pitch
    type: influences
sources:
  - head-acoustics-psychoacoustic-analyses
  - deutsch-speech-to-song-illusion
  - session-01-what-is-sound
experiments: []
practical_applications:
  - using loop length as a timbre control rather than as an edit length
  - recognizing that a very short loop is generating a fundamental at its own period
project_connections:
  - AudioMuse temporal DSP foundation
future_questions:
  - How does source content change where the rhythm-to-pitch transition is heard, if at all?
  - Does a crossfaded loop point move the transition compared with a hard one?
---

# Micro-Looping

Micro-looping is the operation that shows AudioMuse's rhythm and pitch domains are the same domain seen at two scales.

The reasoning is arithmetic before it is perceptual. A loop of period T repeats at a rate of 1/T. At T = 500 ms that is 2 Hz and the result is a musical repeat. At T = 20 ms it is 50 Hz. At T = 5 ms it is 200 Hz. Nothing about the operation changed; only the number did. Session 1 already establishes that frequency is set by the rate at which cycles pass, and a loop is a cycle by construction.

The perceptual consequence is the continuum the HEAD acoustics note documents for envelope modulation, and AudioMuse maps the loop onto it directly. Below roughly 10 Hz the individual repeats are followed as pulsation. Between roughly 20 and 300 Hz the periodicity is heard as roughness rather than as separate events. Above that, the components separate out as audible tones. So the sequence a musician hears when shortening a loop — rhythm, then flutter, then buzz, then a pitch — is not a series of unrelated effects. It is one variable crossing perceptual boundaries.

AudioMuse marks the strength of that claim carefully. The modulation continuum is a psychoacoustic result for amplitude-modulated tones. Applying it to a loop of arbitrary recorded material is an inference: the loop point introduces a periodic discontinuity whose repetition rate is the loop rate, and that discontinuity has a spectrum whose partials are harmonics of the loop rate. Experiment `stutter-rate-continuum` is the proposal for checking where listeners actually place the boundaries with real material.

Two secondary effects are worth recording. Source identity does not survive uniformly: a long loop preserves what the material was, and a short one leaves only its spectral colour, so the same operation is a quotation at one length and a synthesis technique at another. And the loop point's discontinuity is itself a signal — a hard splice generates broadband content at every loop, which is why micro-looping tends to brighten material whether or not that was wanted.
