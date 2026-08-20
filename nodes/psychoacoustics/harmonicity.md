---
id: harmonicity
title: Harmonicity
domain: psychoacoustics
status: foundation
session_origin: []
definition: The degree to which a complex sound's partials approximate integer multiples of a common fundamental, which governs whether the components fuse into one perceived source or separate into several.
core_concepts:
  - integer-ratio spectra fuse; inharmonic spectra separate
  - fusion is a grouping outcome, not an aesthetic one
  - harmonicity supports pitch, inharmonicity weakens it
  - a shared universal tendency, demonstrated across very different musical cultures
  - fusion and pleasantness are experimentally dissociable
relationships:
  - target: pitch
    type: contributes_to
  - target: timbre
    type: contributes_to
  - target: musical-consonance
    type: contributes_to
sources:
  - mcpherson-amazonian-interval-fusion
  - session-01-what-is-sound
experiments: []
practical_applications:
  - predicting whether a layered stack will be heard as one instrument or two
  - understanding why inharmonic percussion resists being tuned
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - How much inharmonicity can be introduced before a layered sound splits perceptually?
  - Does fusion behave the same way for the short fragments that chopping and granular processing produce?
---

# Harmonicity

Harmonicity is a property of a spectrum: how closely the partials sit to integer multiples of a common fundamental. Its perceptual job is grouping. Components in simple integer relationships tend to fuse into one heard source; components that are not tend to be heard as separate things happening at once. That is why a bowed string reads as one instrument and a struck bell often reads as a cluster of pitches, and why layering two synthesizers at exactly an octave can make them disappear into each other.

The reason this node is registered in Phase 12E rather than left inside `timbre` is a specific piece of evidence. McPherson and colleagues tested fusion in Western listeners in Boston and in native Amazonian listeners with limited exposure to Western music. Both groups fused canonically consonant, simple-integer-ratio intervals more than dissonant ones, and the octave produced the strongest fusion in both. That is about as close to a shared perceptual tendency as this literature gets.

And then the same paper takes the aesthetic conclusion away. Fusion did not predict pleasantness. In the Western listeners, who did show robust consonance preferences, the octave was the most fused interval but not the most pleasant, and individual differences in fusion showed no correlation with consonance preference at all. The Amazonian listeners showed no preference for consonant over dissonant intervals while showing the Western fusion pattern. The authors' conclusion is the one AudioMuse adopts: universal perceptual biases exist and may partially constrain musical systems, but they shape aesthetic responses only indirectly, and those responses track what is prevalent in the musical system a listener has experienced.

So harmonicity is a strong candidate for a shared mechanism and a weak candidate for an explanation of why anyone likes anything. `musical-consonance` and `musical-dissonance` carry the learned and cultural layers that the gap in that finding demands.
