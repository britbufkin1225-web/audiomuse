---
id: roughness
title: Roughness
domain: psychoacoustics
status: foundation
session_origin: []
definition: A sensation produced by rapid envelope variation within a critical band, occupying a bounded range of modulation rates above the region where the ear can follow individual fluctuations and below the region where sidebands resolve into separate tones.
core_concepts:
  - a band-pass relationship to modulation rate, not a monotonic one
  - fluctuation below, roughness in the middle, resolved partials above
  - dependence on carrier frequency, modulation rate, and modulation depth
  - weak dependence on level
  - roughness is a sensation, not a judgement of unpleasantness
relationships:
  - target: sensory-dissonance
    type: contributes_to
  - target: timbre
    type: contributes_to
sources:
  - head-acoustics-psychoacoustic-analyses
  - mcpherson-amazonian-interval-fusion
experiments: []
practical_applications:
  - hearing distortion, detuning, and chorus as operations on the same underlying variable
  - choosing a modulation rate deliberately rather than by feel
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Where do the boundaries of the roughness region sit for the complex, broadband material producers actually work with?
  - Does a rhythmically meaningful modulation rate behave differently from an arbitrary one at the same frequency?
---

# Roughness

Roughness is the clearest case in Phase 12E of a psychoacoustic property that is genuinely measurable, genuinely bounded, and routinely misdescribed as an emotion.

The measurable part is a continuum in modulation rate. Taking a 1 kHz tone modulated at full depth and raising the modulation frequency, the HEAD acoustics technical note describes a series of distinct regions rather than a gradual intensification: below about 10 Hz the ear tracks the change and hears pulsation or beating; fluctuation strength peaks near 4 Hz and has its own unit, the vacil; a slow or R-roughness is described around 20 Hz, in a band from roughly 15 to 45 Hz; envelope variations between roughly 20 and 300 Hz are heard as rough; roughness itself peaks near 70 Hz for a 1 kHz carrier and shifts downward for lower carriers; and above that band the main spectral line and its sidebands become audible as separate tones. The unit of roughness, the asper, is defined by a 1 kHz tone at 60 dB modulated at 70 Hz with modulation depth 1.

Two consequences follow. Roughness has a band-pass rather than monotonic relationship to modulation rate, so more modulation does not mean more roughness. And it depends on carrier frequency, modulation frequency, and modulation depth, but only weakly on level — which means it is not a loudness effect wearing a different name.

The misdescription is the step from sensation to affect. The same technical note undercuts it directly: modulated sounds command more attention than unmodulated ones, but if the listener is interested in the information the sound conveys, modulation is not perceived as annoying, and the annoyance judgement appears only when the sound is unwanted. Roughness in a distorted guitar, an overdriven 808, and a failing bearing are the same sensation with three different meanings. `sensory-dissonance` holds roughness's contribution to the dissonance question; `musical-dissonance` holds why that contribution is not the whole of it.

This node is also where the stutter continuum lands. A repeat rate is a modulation rate, so `stutter` and `micro-looping` are not merely rhythmic operations — driven far enough, they enter this territory, and the perceptual result changes category rather than degree.
