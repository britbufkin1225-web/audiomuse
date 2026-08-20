---
id: audio-feature-extraction
title: Audio Feature Extraction
domain: machine-audio
status: seed
session_origin: []
definition: The computation of numerical descriptors from an audio signal — spectral, temporal, rhythmic, harmonic, dynamic, and spatial — which describe the signal deterministically and describe no listener at all.
core_concepts:
  - descriptors are properties of the signal, full stop
  - spectral centroid, spectral flux, onset rate, inter-onset interval, modulation rate
  - a descriptor becomes perceptually relevant only through a stated relationship
  - the same descriptor can relate to more than one perceptual dimension
  - reproducibility requires recording the extraction method, not just the number
relationships:
  - target: digital-signal-processing
    type: used_in
  - target: music-emotion-recognition
    type: enables
  - target: emotion-measurement
    type: contributes_to
sources:
  - yang-chen-music-emotion-recognition-review
  - weineck-neural-synchronization-spectral-flux
  - mcadams-affective-qualities-instrument-sounds
  - itu-r-bs-1770-loudness
experiments: []
practical_applications:
  - describing a track with numbers that another person could reproduce
  - checking whether a perceptual claim about a mix has any measurable correlate at all
project_connections:
  - AudioMuse machine audio foundation
future_questions:
  - Which descriptors would a future AudioMuse tool need in order to visualize a temporal-DSP operation usefully?
  - How should AudioMuse record an extraction method so that two runs are comparable?
---

# Audio Feature Extraction

A descriptor is a number computed from a signal. That is the whole of what it is, and this node exists to keep it that way.

The families AudioMuse cares about follow the layers of the Phase 12E stack. Spectral descriptors — centroid, slope, spread, flux — describe distribution of energy across frequency and how it changes. Temporal descriptors — attack time, decay, envelope shape, onset rate, inter-onset intervals — describe the amplitude curve. Rhythmic descriptors — tempo, beat positions, syncopation measures, modulation rate — describe recurrence. Harmonic descriptors describe pitch content and harmonicity. Dynamic descriptors — RMS, peak, crest factor, programme loudness by a standardized algorithm such as ITU-R BS.1770 — describe level. Spatial descriptors describe inter-channel relationships.

Two findings from this phase show why the choice of descriptor is a real decision rather than a formality. Weineck and colleagues found neural synchronization strongest to spectral flux rather than to the amplitude envelope, which are both obvious candidates for describing the same rhythmic content. McAdams and colleagues found that valence, energy arousal, and tension arousal related to different descriptor combinations, with brightness appearing in two of them in different roles. Picking one descriptor and calling it the feature would have lost both results.

The boundary is where the node earns its place in this phase. A descriptor is not a perceptual quantity and it is certainly not an affective one. Spectral centroid is not brightness, it correlates with brightness. Onset rate is not busyness. Programme loudness is not loudness as heard, and AudioMuse has read only the ITU record page for BS.1770 rather than the algorithm text, so it does not describe how that measurement works.

`music-emotion-recognition` is the layer that tries to bridge descriptors to labels, and it is where the bridge's assumptions become visible.
