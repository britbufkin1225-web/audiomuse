---
id: dynamic-range-compression
title: Dynamic Range Compression
domain: dsp
status: foundation
session_origin: []
definition: Automatic level reduction above a threshold, whose attack and release times reshape the amplitude envelope, so that a device usually described as a loudness tool is in practice a temporal and rhythmic one.
core_concepts:
  - threshold and ratio decide how much, attack and release decide when
  - attack time governs how much transient survives
  - release time is a rhythm, and it interacts with the tempo whether or not it was set to
  - sidechain compression makes the level of one part a function of another part's timing
  - pumping is periodic modulation, so it lands on the modulation continuum
relationships:
  - target: amplitude-envelope
    type: influences
  - target: temporal-dsp
    type: used_in
  - target: groove
    type: influences
  - target: timbre
    type: influences
sources:
  - head-acoustics-psychoacoustic-analyses
  - mcadams-affective-qualities-instrument-sounds
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - setting release by tapping the tempo rather than by reading the gain reduction meter
  - recognizing that a compressor that sounds wrong may be creating a rhythm that fights the track
project_connections:
  - AudioMuse temporal DSP foundation
future_questions:
  - Is there a measurable difference in rated groove between release times that align with the tempo and ones that do not?
  - How much attack-time transient reduction is needed before rated energy arousal changes?
---

# Dynamic Range Compression

Compression is usually taught as a loudness tool and is more usefully understood as an envelope tool, which is why Phase 12E gives it a node in the temporal family rather than in mixing alone.

Threshold and ratio determine how much gain reduction occurs. Attack and release determine when, and the when is where the musical consequences live.

**Attack time** decides how much of a transient escapes before the gain reduction arrives. A slow attack preserves the initial spike and compresses the body, which increases the difference between attack and sustain. A fast attack removes the spike, which flattens it. `amplitude-envelope` records that envelope contributes to timbre, and McAdams and colleagues found sharper attack with earlier decay associated with more positive valence in their ratings while gentler attacks were associated with higher rated tension. AudioMuse does not turn that into a rule for compressor settings — the study used isolated orchestral tones — but it does establish that attack shape is an affectively relevant variable rather than a technical detail.

**Release time** is the parameter that turns a compressor into a rhythmic device. After each gain reduction the level recovers over the release period, so a compressor working on a repeating pattern produces a repeating level contour at the pattern's rate. If the release is short relative to the beat, the recovery finishes and is inaudible as a rhythm; if it is comparable to the beat, the recovery becomes an audible periodic swell. That is what pumping is, and it lands on the same modulation continuum as everything else in this family — the HEAD acoustics note places fluctuation strength peaking near 4 Hz, which for common dance tempi is roughly the range where an eighth-note-rate pumping cycle sits.

**Sidechain compression** makes this explicit by driving the reduction from a different signal. The level of one part becomes a function of another part's timing, which means the compressor is now generating rhythm rather than responding to it. `groove` is downstream of that, and `gating` is the limiting case: a gate is a compressor with an infinite ratio and an inverted sense.

The honest boundary: AudioMuse has no retrieved source measuring listener response to compression settings. The envelope facts are supported; the affective consequences are inference, and the node is written so the difference is visible.
