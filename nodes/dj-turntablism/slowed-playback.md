---
id: slowed-playback
title: Slowed Playback
domain: dj-turntablism
status: developed
session_origin: []
definition: Sustained reduction of the rate at which a recording is played back, which lengthens duration, lowers pitch, and slows tempo together because all three follow from the same rate change.
core_concepts:
  - playback rate as a single coupled control
  - duration and frequency scale inversely
  - transient spacing widens
  - vocal register and timbre shift downward
  - whole-performance treatment rather than a momentary gesture
relationships:
  - target: chopped-and-screwed
    type: used_in
  - target: pitch
    type: influences
  - target: rhythm
    type: influences
  - target: timbre
    type: influences
  - target: affective-timbre
    type: influences
sources:
  - session-01-what-is-sound
  - tsha-dj-screw
  - uh-news-dj-screw-exhibit
  - smith-spectral-audio-signal-processing
experiments: []
practical_applications:
  - predicting what a rate change does to pitch, tempo, and duration at once
  - distinguishing mechanical rate change from independent time or pitch manipulation
project_connections:
  - AudioMuse playback-rate and time-domain studies
future_questions:
  - At what rate reduction does a voice stop being recognizable as the same speaker?
  - How much of the perceived weight of slowed material comes from the playback system rather than from the signal?
---

# Slowed Playback

On a turntable or a tape machine, playback rate is one control with several consequences. Reducing it does not slow the music and separately lower its pitch; there is only one change, and everything else follows from it. Writing the rate as a ratio `r` against the original speed:

```text
new duration   = original duration / r
new frequency  = original frequency * r
```

Every partial moves by the same factor, so harmonic relationships are preserved while the whole spectrum translates downward. Session 1 already establishes the mechanism AudioMuse needs for the middle of that chain: frequency is set by how quickly the stored cycles pass, and the `frequency --influences--> pitch` edge carries the perceptual step. The chain is therefore:

```text
rate reduction -> longer duration and wider transient spacing -> lower fundamental and lower harmonics -> lower perceived pitch and altered timbre -> slower tempo
```

Three consequences deserve to be stated separately, because they are often merged. Tempo falls because events arrive further apart in time. Pitch falls because each stored cycle takes longer to traverse. Timbre changes because the entire spectral envelope moves down with the partials while the transients themselves lengthen, so attacks that were sharp become slower rises. A voice treated this way is not the same voice at a lower pitch; it is a different vocal object.

`scratching` in AudioMuse already models rate change as a momentary gesture. Slowed playback is the same physical parameter held constant across an entire performance, and that difference is what makes it a compositional decision rather than an articulation.

This is the property that separates the analog practice from modern software. Because the rate change is mechanical, time and pitch are coupled and cannot be separated: there is no setting that slows the record while leaving the voice where it was. Phase-vocoder methods, whose early applications include time-scale modification and frequency shifting, break that coupling — which is precisely why `time-stretching` is stored as a different node rather than as a digital synonym for this one.

No canonical rate is recorded. Accounts describe reductions of roughly half speed or less, and AudioMuse states the mechanism rather than a number, because no retrieved source establishes a single figure and tape-to-tape variation is likely.
