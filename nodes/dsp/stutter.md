---
id: stutter
title: Stutter
domain: dsp
status: foundation
session_origin: []
definition: Rapid repetition of a short fragment, in which repeat period and fragment length are set independently of the source material's own structure, so that the repeat rate becomes the dominant perceptual variable.
core_concepts:
  - repeat period and fragment length as separate parameters
  - accelerating and decelerating stutters build and release differently
  - a stutter suspends the material it interrupts
  - fast enough, the repeat rate becomes a modulation rate
  - exact repetition is what makes the effect work
relationships:
  - target: temporal-dsp
    type: used_in
  - target: repetition
    type: contributes_to
  - target: micro-looping
    type: contributes_to
  - target: musical-expectation
    type: influences
sources:
  - head-acoustics-psychoacoustic-analyses
  - deutsch-speech-to-song-illusion
  - session-02-what-is-music
experiments: []
practical_applications:
  - using an accelerating stutter as a build whose intensity comes from rate rather than from level
  - deciding fragment length by what it does to intelligibility, not by musical division alone
project_connections:
  - AudioMuse temporal DSP foundation
  - AudioMuse Houston chopped-and-screwed foundation
future_questions:
  - Where do listeners stop hearing separate repeats and start hearing a continuous buzz, and how variable is that boundary?
  - Does an accelerating stutter raise arousal independently of the level rise that usually accompanies it?
---

# Stutter

A stutter takes a fragment and repeats it fast. Its two parameters — how long the fragment is, and how often it recurs — are usually locked together in practice and are conceptually independent, which is where most of its expressive range lives.

The reason stutter belongs in a phase about emotion is that repetition rate is not a musical parameter alone. Drive a repeat upward and it crosses the boundaries the HEAD acoustics note describes for envelope modulation: below about 10 Hz the ear follows the individual repeats as pulsation, from roughly 20 Hz upward the modulation is heard as roughness rather than as separate events, and above roughly 300 Hz the sidebands resolve into audible tones. AudioMuse states this as a psychoacoustic mapping and marks the musical version as a hypothesis: a stutter driven up through those regions should stop being heard as repetition and start being heard as texture and then as pitch. Experiment `stutter-rate-continuum` exists to test it, and the transition regions are recorded there as hypotheses rather than as laws.

Two structural effects sit below the rate question.

A stutter **suspends** whatever it interrupts. The phrase that was running does not continue during the stutter; it is held. That makes the exit from a stutter an expectation event in its own right — the listener has been waiting through a repeat that carries no new information, and the resumption is what the prediction has been aimed at.

A stutter that **accelerates** does something a fixed-rate stutter does not. It supplies a trend, so the listener predicts not just the next repeat but the rate of the next repeat, and the arrival point becomes inferable. Juslin's brain stem reflex mechanism is described as responding to sounds that are sudden, loud, or dissonant, or that feature accelerating patterns — the only place in that framework where a purely temporal trend is named as an emotion-relevant acoustic property.

Deutsch, Henthorn, and Lapidis supply the constraint that exactness matters: their speech-to-song transformation occurred when repetitions were exact replicas and did not occur when the phrase was slightly transposed or the syllables jumbled. AudioMuse takes that as a reason to treat a digitally exact stutter and a hand-performed repeat as genuinely different stimuli rather than as the same idea at different precisions.
