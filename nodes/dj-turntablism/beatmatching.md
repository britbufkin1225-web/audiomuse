---
id: beatmatching
title: Beatmatching
domain: dj-turntablism
status: foundation
session_origin:
  - session-01-what-is-sound
  - session-02-what-is-music
definition: Aligning the tempo and beat positions of two recordings so that they can play together in time.
core_concepts:
  - tempo alignment
  - beat position
  - beatgrid
  - phrase matching
  - drift
relationships:
  - target: djing
    type: used_in
sources:
  - session-01-what-is-sound
  - session-02-what-is-music
experiments: []
practical_applications:
  - playing two recordings together in time
  - checking and correcting library beatgrids
  - planning transitions at section boundaries
project_connections:
  - AudioMuse timing, beatgrid, and transition studies
future_questions:
  - At what point does grid-based alignment stop describing performed timing and groove?
  - How should AudioMuse represent the difference between matched tempo and matched phase between two records?
---

# Beatmatching

Beatmatching is the alignment step that makes two recordings playable at once. Session 1 lists beat matching alongside cue monitoring, speaker placement, and subwoofer alignment among the things wavelength and phase help explain — that is, alignment is treated as a timing relationship the DJ has to hear correctly before adjusting it.

Session 2 supplies the digital half. In a DAW or DJ library, musical time becomes measurable data: BPM, bars, beats, subdivisions, ticks, samples, milliseconds, and MIDI timestamps. Against that background the session describes a DJ beatgrid as a digital coordinate system for musical time, and observes that an incorrect beatgrid can wreck synchronization even when the audio itself is fine. Beatmatching therefore depends on a correct reading of a track's time, not only on a steady hand: this is why `rhythm --influences--> beatmatching` is stored on the `rhythm` node.

Tempo alignment is not the whole task. Session 2 lists a track's sections — intro, verse, build, drop, breakdown, outro — and notes that a DJ reads them as navigational landmarks, so that mixing tracks is often mixing structures. Phrase matching is the name the session gives to aligning those larger groupings rather than only the beats inside them. AudioMuse keeps both under this node because they are the same alignment problem measured at two scales.

The `beatgrid` vocabulary entry records the digital representation; this node records the practice that uses it.
