---
id: temporal-dsp
title: Temporal DSP
domain: dsp
status: foundation
session_origin: []
definition: The family of operations that restructure a signal in time rather than in frequency — gating, chopping, stuttering, looping, retriggering, dropout, displacement, stretching, and granular fragmentation — treated as a layer that changes event structure and therefore prediction, not as a set of production tricks.
core_concepts:
  - operations on when rather than on what
  - event boundaries are the shared target
  - the missing middle layer between waveform manipulation and musical expectation
  - the same operation is rhythmic, textural, or tonal depending on its rate
  - performed and programmed versions of one operation are different stimuli
relationships:
  - target: digital-signal-processing
    type: used_in
  - target: amplitude-envelope
    type: influences
  - target: musical-expectation
    type: influences
  - target: temporal-discontinuity
    type: produces
sources:
  - head-acoustics-psychoacoustic-analyses
  - grahn-brett-beat-perception-motor
  - deutsch-speech-to-song-illusion
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - choosing a temporal operation by what it does to prediction rather than by what it is called
  - reading a production technique and a DJ gesture as instances of the same operation
project_connections:
  - AudioMuse temporal DSP foundation
  - AudioMuse Houston chopped-and-screwed foundation
future_questions:
  - Which of these operations can be described by a single parameter set, and which resist it?
  - Do listeners respond to a temporal violation the way they respond to a harmonic one?
---

# Temporal DSP

AudioMuse had a gap and this node is built to fill it. On one side the repository described signals — frequency, phase, envelope, spectrum, sampling, DSP. On the other it described musical expectation — repetition, pattern, prediction, tension. Between them sat a layer with no name: the operations that take continuous recorded material and cut, hold, repeat, remove, and displace it, which is most of what electronic music production and DJ performance actually consist of.

The unifying claim is that these operations share a target. Gating, chopping, stuttering, micro-looping, retriggering, dropout, silence insertion, temporal displacement, retriggering, and granular fragmentation all restructure **event boundaries**. They decide where things start, how long they last, whether they finish, and where the next one arrives. `amplitude-envelope` is the physical surface they work on; `temporal-discontinuity` is the perceptual consequence they produce.

Three properties recur across the family and are worth stating once rather than in every child node.

**Rate changes category, not just degree.** A repeat every two seconds is a musical event; the same repeat every 20 milliseconds is a texture; faster still it is a pitch. The HEAD acoustics modulation continuum gives the boundaries in the psychoacoustic case — pulsation below about 10 Hz, roughness between roughly 20 and 300 Hz, resolved sidebands above — and `stutter` and `micro-looping` are what a musician does when they drive a repeat across those thresholds deliberately.

**Removal creates accent.** Grahn and Brett report that perceptual accent arises from temporal structure alone, with an onset not closely followed by another heard as accented. So cutting material out redistributes emphasis onto what remains. This is why gating a pattern can make it feel harder without any level increase.

**Exact repetition reorganizes perception.** Deutsch, Henthorn, and Lapidis showed a spoken phrase transforming perceptually into song purely through repetition, and crucially that the effect depended on the repetition being exact — transposing slightly or jumbling the syllables prevented it. That is a strong result about what recurrence alone can do, and it sits directly under the chopping and looping practices AudioMuse already documents.

The historical thread is already in the repository. Session 3 records sequencing and sampling as the technologies that made machine-controlled repetition ordinary; the Houston material records the same operations performed by hand on turntables. The operations are the same; the timing signatures are not.
