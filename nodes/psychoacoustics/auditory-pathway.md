---
id: auditory-pathway
title: Auditory Pathway
domain: psychoacoustics
status: seed
session_origin: []
definition: The chain of neural structures carrying auditory information from the auditory nerve through brainstem nuclei and midbrain to thalamus and auditory cortex, preserving frequency organization while extracting timing, location, and pattern.
core_concepts:
  - auditory nerve to brainstem nuclei to inferior colliculus to medial geniculate to cortex
  - tonotopy preserved at every level
  - two frequency codes, temporal at low frequencies and place at high
  - phase locking and its upper limit
  - binaural comparison as an early operation, not a late one
relationships:
  - target: pitch
    type: contributes_to
  - target: auditory-reward
    type: enables
sources:
  - purves-neuroscience-auditory-system
  - nidcd-how-do-we-hear
experiments: []
practical_applications:
  - knowing which perceptual abilities are early and automatic rather than learned
  - reasoning about localization cues in terms of where they are computed
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - What does AudioMuse need above the auditory nerve, given that most production questions are answered below it?
  - Which cortical findings are stable enough to state, as opposed to being one study deep?
---

# Auditory Pathway

This node is deliberately a seed. AudioMuse has retrieved two textbook sections on the auditory nerve and the cochlea, and has not retrieved authoritative material on the brainstem nuclei, inferior colliculus, medial geniculate, or auditory cortex. Rather than write a confident chain from an unread source, the node records the shape of the pathway and states clearly what is and is not sourced here.

What is sourced is the coding scheme. The Purves text describes two mechanisms working over different ranges. At low frequencies individual nerve fibres track the waveform in time, firing preferentially at particular phases — phase locking — and the text states that hair cells and auditory nerve fibres can follow stimuli up to about 3 kHz in a one-to-one fashion. Above that, temporal tracking breaks down and the information is carried by which fibres are firing, using the tonotopic organization established mechanically in the cochlea. Every fibre has a tuning curve whose lowest-threshold point is its characteristic frequency, with apical fibres tuned low and basal fibres tuned high.

That division has consequences AudioMuse cares about. Fine timing information, including the interaural timing differences used for localization, is available at low frequencies in a way it is not at high ones. And the tonotopic map established at the cochlea is not discarded — the text records that it is retained at all levels of the central auditory system, so the frequency axis a producer manipulates with an equalizer is the same axis the nervous system is organized along.

Above the nerve, AudioMuse currently records structure without detail. `auditory-reward` picks the thread up at the point where auditory representations meet valuation, using a different and separately retrieved source, and the gap between the two is the honest state of this repository rather than a gap in the science.
