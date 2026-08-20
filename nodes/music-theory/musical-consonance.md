---
id: musical-consonance
title: Musical Consonance
domain: music-theory
status: foundation
session_origin: []
definition: A judgement that simultaneous tones belong together, arising from several partly independent contributions — spectral fusion, peripheral interaction, and learned exposure to a tonal system — rather than from any one of them alone.
core_concepts:
  - several contributions, none sufficient
  - fusion and preference dissociate experimentally
  - peripheral roughness is a contributor, not the explanation
  - preference tracks the system a listener has been exposed to
  - consonance is a judgement, not an emotion
relationships:
  - target: musical-expectation
    type: contributes_to
sources:
  - mcpherson-amazonian-interval-fusion
  - head-acoustics-psychoacoustic-analyses
  - session-02-what-is-music
experiments: []
practical_applications:
  - voicing a chord for a specific register instead of assuming the interval carries its meaning with it
  - explaining why a harmonically simple part can still sound wrong in a dense arrangement
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - How much of the consonance judgement survives when timbre is made deliberately inharmonic?
  - Is there a measurable difference between finding a chord consonant and finding it pleasant?
---

# Musical Consonance

Consonance is where AudioMuse refuses a shortcut it could easily take. There is a long tradition of explaining it by one mechanism — simple frequency ratios, or absence of roughness, or cultural convention — and the evidence supports none of those as sufficient.

McPherson and colleagues provide the decisive separation. Testing Western listeners and native Amazonian listeners with limited exposure to Western music, both groups fused simple-integer-ratio intervals more than dissonant ones, with the octave strongest in both. So a perceptual regularity commonly offered as the basis of consonance is present in both groups. And yet fusion did not predict pleasantness. In the Western group, which did show robust consonance preferences, the octave was the most fused but not the most pleasant, and individual differences in fusion showed no correlation with preference. The Amazonian listeners showed no consonance preference at all. Their conclusion: universal perceptual biases may partially constrain musical systems but shape aesthetic responses only indirectly, and preference tracks what is prevalent in the system a listener has experienced.

Peripheral interaction is a second contributor and has its own limits. The HEAD acoustics note records that a roughness impression can arise from two tonal components within a critical bandwidth — real, measurable, and dependent on spectrum rather than on interval names. `sensory-dissonance` holds it. But roughness is a sensation with no fixed valence, as the same note's own annoyance discussion shows.

A third contributor is tonal context, and it is the one this node's domain exists for. A chord's function depends on what came before it; a dominant seventh is unstable in a key and inert outside one. That is expectation, not acoustics, which is why `musical-consonance --contributes_to--> musical-expectation` and `musical-enculturation --influences--> musical-consonance` both exist.

AudioMuse therefore states no equivalence between consonance and pleasantness, and no equivalence between mode and mood. `musical-dissonance` carries the same argument from the other side, and experiment `harmonicity-versus-roughness` is designed to vary the acoustic contributors while holding the musical frame still.
