---
id: time-stretching
title: Time Stretching
domain: dsp
status: foundation
session_origin: []
definition: Digital modification of a signal's duration without a corresponding change in its pitch, which separates two properties that mechanical playback-rate change necessarily moves together.
core_concepts:
  - duration and pitch decoupled
  - short-time analysis and resynthesis
  - phase vocoder time-scale modification
  - pitch shifting as stretching plus resampling
  - artifacts as the cost of decoupling
relationships:
  - target: chopped-and-slowed
    type: contributes_to
  - target: sampling
    type: processes
  - target: temporal-dsp
    type: used_in
sources:
  - smith-spectral-audio-signal-processing
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - choosing deliberately between coupled and decoupled time manipulation
  - recognizing stretching artifacts by ear
project_connections:
  - AudioMuse time-domain and spectral processing studies
future_questions:
  - Which artifacts are diagnostic of phase-vocoder stretching versus of resampling?
  - At what stretch factors does the decoupled result stop sounding like the source material?
---

# Time Stretching

Mechanical rate change couples duration and pitch; digital processing can separate them. That is the whole distinction, and it is the reason AudioMuse stores this node apart from `slowed-playback`.

The classical route is the phase vocoder, introduced by Flanagan and Golden by interpreting a vocoder filter bank as a sliding short-time Fourier transform, with time-scale modification and frequency shifting among its early applications. The signal is analyzed in overlapping short frames and resynthesized with a different hop size, so more or fewer output samples are produced while the frequency content is held where it was. Pitch shifting is then available as a composition of two operations: stretch the signal, then resample it back to its original length, which moves the pitch and restores the duration.

Three consequences matter for the Houston material:

- **Resampling reproduces the analog behavior.** Changing sample rate on playback scales duration and frequency together, exactly as a turntable does. Digital tools can therefore imitate the mechanical result deliberately.
- **Decoupling is a choice with a cost.** Holding pitch while stretching time requires estimating and reassigning phase, and the estimate is imperfect; the characteristic smeared, phasey artifacts of aggressive stretching are the audible price of the separation.
- **The operations are no longer physically linked.** A slowed track made this way may have had its pitch moved back up, or not moved at all, which means "slowed" no longer describes one determinate transformation.

`time-stretching --processes--> sampling` records that the operation acts on the stored digital representation the `sampling` node defines, and the existing `digital-signal-processing --enables--> time-stretching` edge places it inside the discipline that makes it possible. `contributes_to --> chopped-and-slowed` records that this capability is one of the things that makes the later practice a different thing from the one it is named after.
