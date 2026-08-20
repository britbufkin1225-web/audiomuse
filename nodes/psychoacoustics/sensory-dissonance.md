---
id: sensory-dissonance
title: Sensory Dissonance
domain: psychoacoustics
status: foundation
session_origin: []
definition: The component of perceived dissonance attributable to interaction between spectral components in the auditory periphery, principally roughness from unresolved beating within a critical band, considered separately from learned musical judgements of dissonance.
core_concepts:
  - a peripheral contribution, not the whole of dissonance
  - roughness from components too close to resolve
  - dependent on spectral content, not on interval names
  - orchestration and register change it while the notated interval stays fixed
  - separable in principle from tonal and cultural dissonance
relationships:
  - target: musical-dissonance
    type: contributes_to
  - target: musical-consonance
    type: contributes_to
sources:
  - head-acoustics-psychoacoustic-analyses
  - mcpherson-amazonian-interval-fusion
experiments: []
practical_applications:
  - explaining why the same chord voiced low sounds muddier than voiced high
  - predicting which layered parts will fight in a mix before hearing them together
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - How much of the dissonance a listener reports in real music is peripheral, and how much is learned?
  - Can AudioMuse construct stimuli that vary sensory dissonance while holding tonal function constant?
---

# Sensory Dissonance

Sensory dissonance is the part of the dissonance question that belongs to the ear rather than to a musical tradition. When two spectral components fall close enough together that the auditory periphery cannot resolve them separately, their interaction produces the envelope fluctuation that `roughness` describes. The HEAD acoustics note records that a roughness impression can arise from two tonal components occurring within a critical bandwidth, which is the mechanism stated in its simplest form.

Two things follow that matter more for production than for theory.

The first is that sensory dissonance is a property of spectra, not of intervals. A minor second between two sine tones and a minor second between two heavily distorted guitars are not the same event, because distortion has added partials and every added pair is another opportunity for close-spaced interaction. The same notated chord voiced in a low register will produce more peripheral interaction than the same chord voiced high, because the critical bands are proportionally wider at low frequencies relative to the interval spacing. Composers and arrangers have worked with this for a long time without the vocabulary; the vocabulary is what lets AudioMuse predict it.

The second is that sensory dissonance is not musical dissonance. McPherson and colleagues showed that the perceptual regularity most closely tied to consonance — fusion of simple-ratio intervals — did not predict pleasantness even in listeners with strong consonance preferences, and that listeners without exposure to Western music showed the perceptual pattern without the preference. Peripheral interaction is a real contributor and a poor explanation on its own.

AudioMuse therefore keeps two nodes. This one holds what the periphery does. `musical-dissonance` holds what a tradition does with it, and `musical-consonance` holds the same division from the other side. Experiment `harmonicity-versus-roughness` is designed to hold one and vary the other.
