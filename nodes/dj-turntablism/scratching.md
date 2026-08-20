---
id: scratching
title: Scratching
domain: dj-turntablism
status: foundation
session_origin:
  - session-01-what-is-sound
  - session-02-what-is-music
  - session-03-history-of-electronic-music
definition: Manual manipulation of a record's playback position, velocity, and direction by hand, often articulated through amplitude gating at the mixer.
core_concepts:
  - playback position
  - playback velocity
  - direction reversal
  - hand acceleration
  - amplitude gating
relationships:
  - target: turntablism
    type: used_in
  - target: sampling
    type: controls
  - target: frequency
    type: influences
  - target: rhythm
    type: contributes_to
  - target: temporal-dsp
    type: used_in
  - target: gating
    type: used_in
sources:
  - session-01-what-is-sound
  - session-02-what-is-music
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - rhythmic articulation of recorded material
  - shaping attack and envelope through hand acceleration
  - waveform, velocity, and gating analysis of a performed scratch
project_connections:
  - AudioMuse vinyl scratch analyzer concept
future_questions:
  - What measurement would let AudioMuse relate hand velocity directly to the resulting frequency shift?
  - How much of a scratch's perceived character comes from spectral change rather than from timing and gating?
---

# Scratching

Scratching is the clearest case in AudioMuse of a physical gesture acting as an audio process. Session 1 states the mechanism directly: moving vinyl faster raises pitch and compresses time, moving it backward reverses the waveform, hand acceleration shapes the envelope, and crossfader movement gates amplitude. Session 2 restates the same act in signal terms, describing scratching as real-time sample manipulation in which the performer controls playback position, velocity, direction, acceleration, and rhythm, and calling the hand a real-time transport controller for recorded audio.

Two chains run through the node. The first is the transport chain:

```text
hand motion -> platter and record motion -> playback velocity -> waveform traversal -> time and pitch transformation
```

AudioMuse stores the middle of that chain as `scratching --influences--> frequency` rather than jumping to pitch. Playback velocity changes how long each recorded cycle takes to traverse the stylus, which changes its period and therefore its frequency; the existing `frequency --influences--> pitch` edge carries the perceptual step. Reversing direction traverses the same stored waveform backwards, which is why a scratch can sound like a sound played inside out rather than merely transposed.

The second is the articulation chain:

```text
crossfader gesture -> amplitude gating -> rhythmic articulation
```

The crossfader decides whether the manipulated audio is audible at all. Cutting the fader against a moving record turns one continuous gesture into discrete rhythmic events, which is why the node contributes to `rhythm` rather than only changing frequency. Session 2 summarizes the combination as sample playback, manual time manipulation, rhythmic articulation, and amplitude gating occurring together, which is also why the edge into `sampling` is `controls`: the performer supplies transport parameters that direct playback of recorded material without generating or processing it computationally.

Session 3 records that scratching survived the move to digital playback: a `digital-vinyl-system` keeps the same gesture and the same interface while the audio being traversed is a digital file.
