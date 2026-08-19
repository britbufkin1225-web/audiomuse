---
id: turntable
title: Turntable
domain: dj-turntablism
status: foundation
session_origin:
  - session-01-what-is-sound
  - session-03-history-of-electronic-music
definition: A mechanical playback system in which a stylus tracks the groove of a rotating record, and in DJ practice a hand-controllable performance instrument.
core_concepts:
  - platter
  - stylus
  - record groove
  - tonearm
  - mechanical isolation
relationships:
  - target: vibration
    type: produces
  - target: djing
    type: used_in
  - target: turntablism
    type: used_in
sources:
  - session-01-what-is-sound
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - record playback and transport control
  - turntablist performance
  - placement, decoupling, and isolation decisions
project_connections:
  - AudioMuse mechanical-playback and vibration-isolation studies
future_questions:
  - Which measurement would separate airborne excitation of a turntable from structure-borne excitation through its support?
  - How should AudioMuse describe slipmat, platter, and tonearm behavior once a source develops them beyond a component list?
---

# Turntable

Session 1 names the turntable twice, and the two mentions describe opposite directions of energy flow. In one, stylus, groove, tonearm, platter, and vibration isolation form a mechanical system, and turntable needles and records appear among the objects whose vibration becomes sound. In the other, a turntable can receive unwanted vibration through its furniture. AudioMuse keeps both.

The intended path runs one way:

```text
record groove -> stylus -> mechanical vibration -> electrical signal -> playback
```

The stored edge `turntable --produces--> vibration` records that step, and the existing `vibration --produces--> sound` edge continues it. Everything a turntablist does — changing platter speed, holding or reversing the record, lifting and dropping the needle at a cue point — acts on this path.

The unwanted path runs the other way. Energy from a loudspeaker, a floor, or a supporting surface reaches the same stylus that is meant to read only the groove. Near a system's natural frequency this coupling is strongest, which is why `resonance --influences--> turntable` is recorded on the `resonance` node and why decoupling, placement, and isolation are practical concerns rather than decoration. Session 1's more general principle applies here: a surface that moves more air, or a support that transmits vibration into another object, is doing so whether or not the result is wanted.

The turntable is therefore recorded as a component used in both `djing` and `turntablism`. The distinction matters: DJ practice uses it to reproduce and align recordings, while turntablism uses the same hardware as an instrument whose transport is played by hand.
