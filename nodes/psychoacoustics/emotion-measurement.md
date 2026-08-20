---
id: emotion-measurement
title: Emotion Measurement
domain: psychoacoustics
status: foundation
session_origin: []
definition: The set of methods used to record affective response — self-report, behavioural, physiological, and neural — together with the constraint that each recovers something different and none of them is authoritative on its own.
core_concepts:
  - no gold-standard measure exists
  - self-report, physiology, and behaviour carry unique variance
  - dimensional recovery is stronger than categorical recovery
  - signal measurement and response measurement are different layers
  - a measure chosen after seeing the data is not a measure
relationships:
  - target: music-evoked-emotion
    type: studies
  - target: autonomic-response-to-music
    type: studies
sources:
  - mauss-robinson-measures-of-emotion
  - juslin-brecvema-unified-theory
  - itu-r-bs-1770-loudness
  - yang-chen-music-emotion-recognition-review
experiments: []
practical_applications:
  - designing an AudioMuse listening test that states its measure before collecting anything
  - keeping a measured signal property and a rated perceptual property in separate columns
project_connections:
  - AudioMuse affective psychoacoustics foundation
  - AudioMuse experiment and experiment-run layers
future_questions:
  - What is the smallest set of measures that makes an AudioMuse listening result interpretable?
  - How should AudioMuse record a rating scale so that two sessions are comparable at all?
---

# Emotion Measurement

AudioMuse already separates observation from measurement in its experiment layer. This node supplies the affective-science version of the same rule and the source behind it.

Mauss and Robinson conclude that there is no gold-standard measure of emotional responding: experiential, physiological, and behavioural measures each carry unique variance, converge only weakly, and cannot be assumed interchangeable. They also report that measures recover dimensions such as valence and arousal more readily than discrete categories. That is the single most consequential methodological finding in this phase, because it means an AudioMuse experiment must declare what it is measuring and may not silently upgrade the result.

Four families are available, and Phase 12E treats them as separate columns rather than as alternatives.

**Signal measurement** describes the stimulus, not the listener: RMS, peak, programme loudness by a standardized algorithm such as ITU-R BS.1770, spectral centroid, spectral flux, onset rate, inter-onset intervals, tempo, modulation rate, and dynamic range. These are the only quantities AudioMuse can produce deterministically, and they say nothing about anyone.

**Behavioural measurement** covers ratings — valence, arousal, tension, predictability, wanting-to-move, pleasantness — plus reaction time and tapping synchronization. Yang and Chen record how fragile ratings are as ground truth: emotion perception is subjective, people perceive different emotions in the same song, and averaging a modest number of raters is the usual and imperfect remedy.

**Physiological measurement** covers heart rate, skin conductance, respiration, pupil response, and movement, under the constraint `autonomic-response-to-music` states.

**Neural measurement** — EEG, MEG, fMRI, intracranial recording — is where most of the cited findings come from and is outside what AudioMuse will ever do. AudioMuse reads that literature; it does not become an instrumentation project.

The rule the node enforces: state the measure, state the layer, and never let a number from one column be reported as though it came from another.
