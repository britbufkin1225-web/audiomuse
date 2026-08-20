---
id: frisson
title: Frisson
domain: psychoacoustics
status: foundation
session_origin: []
definition: A transient peak response during listening, variously reported as chills, shivers, tingling, or piloerection, accompanied by autonomic changes, and used in research as a marker of intense response without being identical to any named emotion.
core_concepts:
  - a marker used because it is time-locked and reportable
  - autonomic correlates: heart rate, skin conductance, respiration, piloerection
  - triggers reported around dynamic leaps, unexpected harmony, and modulation
  - terminology unsettled across the literature
  - direction of causation between musical feature and response generally assumed, not tested
relationships:
  - target: autonomic-response-to-music
    type: contributes_to
  - target: music-evoked-emotion
    type: contributes_to
sources:
  - harrison-loui-frisson-model
  - zatorre-salimpoor-perception-to-pleasure
experiments: []
practical_applications:
  - reading a build, a drop, and a sudden entry as candidate trigger structures rather than guaranteed ones
  - understanding why researchers use chills as a proxy and what that proxy costs
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Do the reported trigger structures transfer to genres outside the Western classical corpus most studies use?
  - Would a temporal-DSP manipulation such as a sudden dropout function as a trigger, and is that testable safely?
---

# Frisson

Frisson is the response that most of this literature is built on, which makes its unsettled status worth stating plainly.

Harrison and Loui's review records the physiological picture: increased heart rate, skin conductance responses, changes in respiratory depth, piloerection, and non-dermal responses including tears and muscle tension. It is used as a research marker because it is transient, time-locked to a musical moment, and reportable while listening — properties that most affective responses lack.

The structural triggers reported are the ones a producer would recognize. The review attributes to earlier work chord progressions descending the circle of fifths to the tonic, melodic appoggiaturas, the onset of unexpected harmonies, and melodic or harmonic sequences; and from later work, peaks in loudness, moments of modulation, and melody occupying the human vocal register. It states that sudden dynamic leaps appear as a major catalyst across nearly all studies reviewed. That is a list of expectation events and dynamic events, which is why `musical-expectation` and `amplitude-envelope` are both upstream of this node.

Three cautions travel with it. The terminology is not agreed: the review states that chills lacks operative consensus, that authors disagree over whether piloerection is definitional, and it argues for frisson as the clearer term while proposing to widen it beyond skin sensation. The samples are narrow: studies overwhelmingly use Western classical music and student participants, and the authors call for testing across as many genres as possible. And the causal direction has generally been assumed rather than demonstrated — the review notes that finding more frisson during sad music does not rule out the alternative that the musical attributes of sad music elicit the frisson and the sadness concurrently.

Zatorre and Salimpoor's work uses chills as the marker for the anticipation-versus-peak dissociation `auditory-reward` records. That makes frisson load-bearing for one of the better-known findings in the field, which is a reason to keep its limits visible rather than to quietly treat it as a solved measure.
