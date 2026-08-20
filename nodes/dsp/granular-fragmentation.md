---
id: granular-fragmentation
title: Granular Fragmentation
domain: dsp
status: foundation
session_origin: []
definition: Decomposition of a signal into very short windowed segments that can be repositioned, repeated, overlapped, and randomized independently, dissolving the boundary between event, rhythm, texture, and tone.
core_concepts:
  - grain size, density, overlap, and jitter as the parameter set
  - grain rate lands on the same modulation continuum as every other repeat rate
  - regular grain scheduling produces pitch; jittered scheduling produces noise
  - overlap determines whether the result is a stream or a sequence
  - a grain is short enough that its window shapes its spectrum
relationships:
  - target: temporal-dsp
    type: used_in
  - target: time-stretching
    type: used_in
  - target: micro-looping
    type: contributes_to
  - target: timbre
    type: influences
sources:
  - head-acoustics-psychoacoustic-analyses
  - smith-spectral-audio-signal-processing
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - reaching for jitter rather than for a filter when a granular texture sounds too tonal
  - understanding why extreme time-stretching produces a metallic periodicity
project_connections:
  - AudioMuse temporal DSP foundation
future_questions:
  - At what grain size does source identity become unrecoverable for typical musical material?
  - How much scheduling jitter is needed to suppress the perceived pitch of a regular grain stream?
---

# Granular Fragmentation

Granular processing chops a signal into windowed fragments short enough that they stop behaving like events and start behaving like samples of a texture. Four parameters describe most of it: **grain size**, **grain density** or rate, **overlap**, and **jitter** in position or timing.

The reason it belongs in Phase 12E rather than only in a synthesis chapter is that its parameters land on the same continuum every other temporal operation lands on. Grain rate is a repeat rate. Scheduled regularly at a few hertz it is heard as a sequence of small events; in the region the HEAD acoustics note associates with roughness it is heard as texture; regular and fast enough it produces a periodicity the ear reads as pitch, at the grain rate rather than at anything in the source. This is the same continuum `micro-looping` and `stutter` traverse, arrived at from a different direction.

Jitter is what separates granular processing from micro-looping perceptually. Regular grain scheduling produces periodicity and therefore pitch; randomizing the schedule destroys the periodicity and leaves broadband noise-like texture with the source's spectral colour. So one parameter moves the result between tone and noise without changing grain size or content at all.

Overlap decides whether grains are heard as separate or as a stream. With enough overlap the individual grains stop being resolvable and the result is continuous; with none, gaps between grains introduce their own periodic discontinuities, which `temporal-discontinuity` describes and which carry their own broadband content.

Grain size interacts with the window. At durations short enough, the window shape substantially determines the spectrum of each grain, so a short grain of a bass note is not a short bass note — it is a burst whose spectrum is dominated by its own envelope. That is why extreme granular time-stretching sounds metallic: the process has replaced the source's periodicity with the process's own.

The existing AudioMuse `time-stretching` node records short-time analysis and resynthesis, classically the phase vocoder, with its characteristic artifacts. Granular processing is the time-domain sibling of that idea, and `granular-fragmentation --used_in--> time-stretching` records that stretching is one of the things it is used for rather than the only one.
