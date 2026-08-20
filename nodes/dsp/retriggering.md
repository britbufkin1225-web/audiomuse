---
id: retriggering
title: Retriggering
domain: dsp
status: foundation
session_origin: []
definition: Restarting a sample or event before it has completed, so that the envelope is truncated and replaced by a new attack, changing the sound's apparent identity, motion, and rhythmic function without editing the source.
core_concepts:
  - the tail is never heard, so the sound is not the sound
  - attack density rises while event duration falls
  - a retrigger is an onset, and onsets are what rhythm is counted in
  - perceived causality changes when nothing is allowed to finish
  - the operation is invisible in the source file
relationships:
  - target: temporal-dsp
    type: used_in
  - target: amplitude-envelope
    type: influences
  - target: sampling
    type: used_in
  - target: rhythm
    type: contributes_to
sources:
  - grahn-brett-beat-perception-motor
  - head-acoustics-psychoacoustic-analyses
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - reading a fast hi-hat roll as an envelope decision rather than a velocity decision
  - understanding why a retriggered pad reads as percussive
project_connections:
  - AudioMuse temporal DSP foundation
  - AudioMuse Houston chopped-and-screwed foundation
future_questions:
  - At what retrigger rate does a sequence of attacks stop being heard as separate events?
  - Does truncating the decay of a familiar sound weaken its recognizability measurably?
---

# Retriggering

Retriggering restarts a sound before it finishes. That single description has more consequences than it looks like it should.

The first is that the sound heard is not the sound stored. A sample with a long decay, retriggered every 100 ms, never presents its decay at all — the listener hears a series of attacks and early bodies with the tails cut off. `amplitude-envelope` records that envelope contributes to timbre; retriggering therefore changes timbre without touching the spectrum of the underlying material. A sustained pad retriggered fast reads as percussive, and nothing about the pad changed.

The second is onset density. Rhythm is counted in onsets, and retriggering multiplies them. Grahn and Brett's work is about beat perception from temporal structure, and their observation that an onset not closely followed by another is heard as accented gives the corollary directly: a run of tightly spaced retriggers is a run of unaccented events with a single accent at whichever one is followed by space. The pattern of retrigger spacing is therefore an accent pattern.

The third is perceived causality. A sound that is allowed to complete is heard as a thing that happened; a sound repeatedly cut off is heard as something being done to it. That distinction has no acoustic definition AudioMuse can cite, and it is recorded here as an observation about musical practice rather than as a finding.

The fourth connects to the modulation continuum. Retriggering at increasing rate is `stutter` with the fragment defined by the retrigger interval rather than by an edit, and driven far enough it reaches the region where the HEAD acoustics note places roughness rather than separate events. The operations converge at high rate and are distinguishable only at low rate.

Existing AudioMuse material already contains the mechanical ancestor. `chopping` describes a phrase struck again during playback, performed by hand against a moving record, and `sampling` describes the random access that made the same gesture instantaneous and exact. Retriggering is that gesture once the machine holds the sound.
