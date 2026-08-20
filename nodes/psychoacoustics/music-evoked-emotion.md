---
id: music-evoked-emotion
title: Music-Evoked Emotion
domain: psychoacoustics
status: foundation
session_origin: []
definition: An emotional response arising in a listener during or after musical listening, produced by several distinguishable psychological mechanisms acting on a particular combination of music, listener, and situation rather than by any single musical property.
core_concepts:
  - multiple mechanisms rather than one channel from sound to feeling
  - perceiving an emotion in music is not the same as feeling one
  - the music affords a response and does not guarantee it
  - listener history and context are part of the cause, not noise around it
  - no musical feature has been shown to carry a universal emotional meaning
relationships: []
sources:
  - juslin-brecvema-unified-theory
  - zatorre-salimpoor-perception-to-pleasure
  - mauss-robinson-measures-of-emotion
experiments: []
practical_applications:
  - separating what a production choice does to a signal from what it might do to a listener
  - refusing feature-to-emotion shortcuts when describing a mix, a mastering decision, or a set
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Which mechanisms can AudioMuse actually manipulate in a listening test, and which can only be observed?
  - Do the mechanisms combine additively, or does one dominate once it engages?
---

# Music-Evoked Emotion

This node is the hub of the Phase 12E material, and its first job is to say what the phenomenon is not. It is not a property of a recording. Two listeners can hear the same file and one of them feels nothing; Juslin's review reports that music arouses emotion in roughly 55 to 65 percent of listening episodes on average, with wide individual differences. A model in which the waveform contains the emotion cannot accommodate that.

Juslin's BRECVEMA framework, which AudioMuse adopts as an organizing frame rather than as settled fact, names eight mechanisms: brain stem reflex, rhythmic entrainment, evaluative conditioning, contagion, visual imagery, episodic memory, musical expectancy, and aesthetic judgment. They are not stages of one process. They have different evolutionary ages, different speeds, different representations, and they can produce conflicting outputs at the same time, which the framework offers as an account of mixed feelings. The framework's own constraint is the one AudioMuse leans on hardest: each mechanism responds to a musical event comprising the music, the listener, and the context together, so the music affords a response without guaranteeing it.

A second distinction has to be held throughout. Perceiving an emotion in music and feeling one in response to it are different things, and the music-information-retrieval literature Yang and Chen review works mostly on the first. `music-emotion-recognition` holds that boundary; ignoring it is how a system that predicts a tag comes to be described as detecting a feeling.

A third distinction is measurement. Mauss and Robinson conclude that experiential, physiological, and behavioural measures carry unique variance, converge only weakly, and that no gold-standard measure exists. So a rise in skin conductance during a drop is evidence of something; it is not the emotion, and it is not interchangeable with a listener saying they felt something. `emotion-measurement` holds that contract.

What AudioMuse can trace is a path — vibration, cochlear encoding, psychoacoustic features, temporal organization, prediction, memory, culture, bodily response — where each layer constrains the next without determining it. `docs/affective-mechanism-stack.md` sets that path out in full. The purpose of the layering is not completeness. It is to make it obvious which layer any given claim belongs to, so that a statement about roughness is not quietly promoted into a statement about sadness.
