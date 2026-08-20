---
id: predictive-processing-music
title: Predictive Processing in Music
domain: psychoacoustics
status: foundation
session_origin: []
definition: An account of music listening in which the listener continuously predicts what comes next from a model learned by exposure, and in which the mismatch between prediction and event drives perceptual organization and affective response.
core_concepts:
  - statistical learning of a style, then probabilistic prediction from the learned model
  - information content as surprise, entropy as uncertainty, and the two are not the same
  - prediction error as the quantity that moves rather than the event itself
  - support is strong for the modelling and weaker for the causal story
  - an inverted-U between predictability and pleasure is proposed but not settled
relationships:
  - target: musical-expectation
    type: contributes_to
  - target: auditory-reward
    type: contributes_to
  - target: musical-enculturation
    type: contributes_to
sources:
  - pearce-statistical-learning-enculturation
  - zatorre-salimpoor-perception-to-pleasure
  - witek-syncopation-groove-pleasure
experiments: []
practical_applications:
  - describing an arrangement decision in terms of what the listener has been trained to expect by the preceding bars
  - distinguishing a passage that is surprising from one that is merely uncertain
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Can AudioMuse compute information content for the fragmented material temporal DSP produces, given that models of this kind work on note sequences?
  - Does repeated listening flatten prediction error enough to change the affective response, and over how many plays?
---

# Predictive Processing in Music

Pearce's review sets out the account AudioMuse uses. Musical enculturation is proposed to rest on two processes: statistical learning, in which listeners acquire internal models of the regularities in the music they are exposed to, and probabilistic prediction from those models, which organizes how the music is then processed. The claim is broad — expectation, emotion, memory, similarity, segmentation, and metre are all framed as consequences of the same predictive machinery.

The reason to take it seriously as more than a metaphor is that it produces numbers. Two quantities matter and they are routinely conflated. Information content measures how unexpected a particular event is in its context: high information content is surprise. Entropy measures the uncertainty of the prediction before the event arrives: high entropy is not knowing what is coming. A passage can be highly uncertain and then deliver something unsurprising, or be confidently predicted and then violated. Pearce reports that modelled information content accounted for up to 83 percent of the variance in listeners' pitch expectations across the studies reviewed, which is a strong result for the prediction half of the account.

Affect is where the account gets more careful, and AudioMuse follows the source rather than the popular version. High-information-content passages correlated with higher subjective and physiological arousal and lower valence. On the widely repeated proposal that pleasure follows an inverted U in predictability — too obvious is boring, too strange is unpleasant — Pearce states that it has received empirical support in some but not all studies, and suggests his own reviewed result may reflect only one side of such a curve. Witek and colleagues found an inverted U for syncopation, movement, and pleasure in groove, which is a genuine instance rather than a general law.

Two limitations belong on the node. Pearce states the reviewed results are correlational rather than causal, with one artificial-system exception. And the model the review is built on cannot process timbre, dynamics, or texture — which is a serious boundary for AudioMuse specifically, because that is most of what a producer manipulates. `temporal-dsp` operates almost entirely in the region this model does not reach, and Phase 12E treats the extension as an open question rather than an assumption.
