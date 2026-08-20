---
id: affective-psychoacoustics
title: Affective Psychoacoustics
domain: psychoacoustics
status: foundation
session_origin: []
definition: The study of how measurable psychoacoustic properties of sound relate to affective response, treating the relationship as mediated by perceptual tendency, context, and interpretation rather than as a direct mapping from feature to emotion.
core_concepts:
  - psychoacoustic property, perceptual tendency, contextual interpretation, possible affective response
  - the mediation is where the discipline lives; collapsing it produces pseudo-science
  - the same property can support opposite readings in different contexts
  - properties interact rather than summing
  - a measurable feature is evidence about a signal, not about a listener
relationships:
  - target: music-evoked-emotion
    type: studies
  - target: roughness
    type: studies
  - target: harmonicity
    type: studies
  - target: affective-timbre
    type: studies
sources:
  - head-acoustics-psychoacoustic-analyses
  - mcadams-affective-qualities-instrument-sounds
  - mcpherson-amazonian-interval-fusion
  - juslin-brecvema-unified-theory
experiments: []
practical_applications:
  - stating a mix observation at the layer it belongs to instead of jumping to an emotion word
  - designing a listening test that varies one psychoacoustic property while holding the rest fixed
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Which psychoacoustic properties show the most context-stable affective tendencies, and which reverse entirely?
  - Can AudioMuse construct a stimulus set that isolates one property without changing any other?
---

# Affective Psychoacoustics

The whole of this node is one chain and the discipline of keeping its links apart:

```text
psychoacoustic property
  -> perceptual tendency
  -> contextual interpretation
  -> possible affective response
```

Every arrow is a place where the relationship can fail. Roughness is a measurable consequence of envelope variation within a critical band; that is the property. It reliably produces a distinctive sensation; that is the tendency. Whether the sensation is heard as aggression, as warmth, as grain, as a fault, or as nothing at all depends on the material, the genre, the listener, and what they are listening for; that is the interpretation. The HEAD acoustics note makes the point from the industrial side without meaning to: modulated sounds command more attention than unmodulated ones, but modulation is judged annoying only when the sound is unwanted, and is not experienced as annoying when the listener wants the information it carries. The same physical modulation, two opposite judgements.

McPherson and colleagues supply the sharpest available demonstration that a perceptual regularity need not carry an aesthetic one. Both Western and native Amazonian listeners fused simple-integer-ratio intervals more strongly than dissonant ones, and the octave fused most strongly in both groups. Yet fusion did not predict pleasantness in either group; in Westerners the most fused interval was not the most pleasant, and individual differences in fusion showed no correlation with consonance preference. The Amazonian listeners showed no consonance preference at all while showing the Western fusion pattern. Their conclusion is precisely the shape of the chain above: universal perceptual biases exist and could partially constrain musical systems, but they shape aesthetic responses only indirectly, and the aesthetic response tracks what is prevalent in the system a listener has been exposed to.

McAdams, Douglas, and Vempala add the other half of the discipline: properties do not sum into one affective quantity. Distinct combinations of audio descriptors carried valence, energy arousal, and tension arousal in their data, and register interacted with instrument identity rather than shifting everything uniformly. A single number describing how a sound feels would be discarding exactly the structure that is there.

What follows for AudioMuse is a working rule. A statement may be made at one layer at a time, and moving to the next layer requires either a source or an explicit label saying it is an inference. `docs/affective-mechanism-stack.md` applies the rule end to end.
