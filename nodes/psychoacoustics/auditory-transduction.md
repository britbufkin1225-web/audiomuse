---
id: auditory-transduction
title: Auditory Transduction
domain: psychoacoustics
status: foundation
session_origin: []
definition: The conversion of airborne pressure variation into neural activity by the outer, middle, and inner ear, in which the cochlea decomposes a signal by frequency along its length and hair cells turn mechanical displacement into electrical signals.
core_concepts:
  - outer ear and canal to eardrum to ossicles to cochlea
  - the basilar membrane travelling wave as a mechanical frequency analysis
  - place coding along the cochlea, high frequencies at the base
  - stereocilia deflection, tip links, and transduction channels
  - transduction speed and sensitivity at the physical limits of the possible
relationships:
  - target: sound
    type: represents
  - target: pitch
    type: contributes_to
  - target: timbre
    type: contributes_to
  - target: auditory-pathway
    type: enables
sources:
  - nidcd-how-do-we-hear
  - purves-neuroscience-auditory-system
  - session-01-what-is-sound
experiments: []
practical_applications:
  - understanding why frequency resolution and time resolution trade against one another in hearing as well as in analysis
  - reasoning about hearing protection in terms of what is actually being displaced
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Where does cochlear mechanics stop being a useful level of explanation for a production decision?
  - How should AudioMuse describe outer hair cell activity, which the retrieved sections cover only briefly?
---

# Auditory Transduction

The first layer of the Phase 12E stack, and the one where the physics stops and the biology starts. The NIDCD describes the chain: sound waves enter the outer ear and travel down the ear canal to the eardrum; the eardrum vibrates and passes the vibration to the malleus, incus, and stapes, which amplify it; the bones drive the fluid-filled cochlea, which the basilar membrane divides along its length. The vibration becomes a travelling wave on that membrane, and hair cells riding it bend their stereocilia, opening channels that admit ions and generate an electrical signal carried out by the auditory nerve.

Two properties of this arrangement matter to everything above it.

The first is that frequency analysis happens mechanically, before any neural computation. Position along the cochlea corresponds to frequency: the NIDCD states that hair cells near the wide end detect higher-pitched sounds and those nearer the centre detect lower ones, and the Purves textbook records that this tonotopic organization is retained at every level of the central auditory system. A complex sound is therefore already spread out across a spatial axis by the time anything examines it. `frequency` holds the physical property; this node holds the fact that the ear is a filter bank before it is anything else.

The second is speed and sensitivity. The Purves text states that hair cells can convert bundle displacement into an electrical potential in as little as 10 microseconds, and that bundle movements at the threshold of hearing are approximately 0.3 nanometres. The gating mechanism is direct: tip links between adjacent stereocilia open cation channels when stretched, so deflection toward the tallest stereocilia depolarizes the cell, deflection away hyperpolarizes it, and deflection perpendicular to the plane of symmetry does nothing at all. That directional asymmetry, at that speed, is what makes onset timing and interaural comparison possible.

Nothing about emotion is present at this layer, and AudioMuse says so deliberately. What the layer supplies upward is a representation that is simultaneously frequency-resolved and time-resolved, with the resolution limits that later perceptual effects inherit.
