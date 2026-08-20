---
id: perceived-space
title: Perceived Space
domain: spatial-audio
status: seed
session_origin:
  - session-01-what-is-sound
definition: The impression of location, distance, size, and enclosure that a listener infers from timing, level, spectral, and reverberant cues, which production can manipulate independently of the source material.
core_concepts:
  - interaural time and level differences for direction
  - direct-to-reverberant ratio and early reflections for distance
  - reverberation time and envelopment for enclosure
  - spatial impression is inferred, so it can be constructed
  - AudioMuse has no retrieved source linking spatial cues to affect
relationships:
  - target: sound
    type: characterized_by
  - target: recording
    type: influences
  - target: auditory-looming
    type: contributes_to
sources:
  - session-01-what-is-sound
  - purves-neuroscience-auditory-system
experiments: []
practical_applications:
  - treating reverb as a distance and size decision rather than as an effect amount
  - recognizing that a close, dry vocal is a spatial claim about intimacy that the listener will interpret
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - What retrievable evidence exists linking reverberation and distance cues to affective response?
  - Does the perceptual sense of enclosure behave differently on headphones and on speakers strongly enough to matter for a listening test?
---

# Perceived Space

This node is a seed, and the reason is worth stating rather than hiding. Phase 12E set out to cover space and emotion, and the source sweep produced no retrievable, citable work linking reverberation, distance cues, or stereo width to affective response at the standard the rest of this phase is held to. Rather than fill the gap with plausible sentences, the node records the mechanism it can support and marks the affective question open.

What is supported is inference. Direction comes largely from comparing the two ears: interaural time differences and interaural level differences, with the timing comparison depending on the phase-locked temporal coding the Purves text describes as available up to about 3 kHz and degrading above it. Distance and enclosure come from the relationship between direct sound and reflected sound — the ratio between them, the pattern and delay of early reflections, and the decay time of the later reverberant field.

The consequence that matters for production is that all of this is inferred rather than sensed directly. A listener does not detect a room; they reconstruct one from cues. That reconstruction can therefore be supplied artificially, which is what reverb, delay, filtering, and level automation do. A close dry vocal and the same vocal in a long hall are the same performance making two different claims about where the listener is standing.

`auditory-looming` holds the one spatial effect Phase 12E did find solid evidence for, and it is a perceptual bias rather than an emotional one. Experiment `apparent-space-and-affect` is written as a proposal precisely because AudioMuse has no result to report here: it specifies how to hold the source constant and vary apparent distance, reverberation, early reflections, and width, and it states its hypotheses as hypotheses.
