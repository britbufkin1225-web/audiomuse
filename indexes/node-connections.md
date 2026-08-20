# Node Connections

Canonical relationships: 220
Outbound total: 220
Inbound total: 220

## Affective Psychoacoustics (`affective-psychoacoustics`)

Outbound: 4
Inbound: 0

### Outbound

- `affective-timbre` via `studies`
- `harmonicity` via `studies`
- `music-evoked-emotion` via `studies`
- `roughness` via `studies`

### Inbound

- None

## Affective Timbre (`affective-timbre`)

Outbound: 2
Inbound: 2

### Outbound

- `timbre` via `characterized_by`
- `music-evoked-emotion` via `contributes_to`

### Inbound

- `affective-psychoacoustics` via `studies`
- `slowed-playback` via `influences`

## Amplitude Envelope (`amplitude-envelope`)

Outbound: 4
Inbound: 6

### Outbound

- `sound` via `characterized_by`
- `rhythm` via `contributes_to`
- `timbre` via `contributes_to`
- `roughness` via `influences`

### Inbound

- `dynamic-range-compression` via `influences`
- `gating` via `influences`
- `retriggering` via `influences`
- `silence-as-musical-material` via `influences`
- `temporal-discontinuity` via `influences`
- `temporal-dsp` via `influences`

## Audio Feature Extraction (`audio-feature-extraction`)

Outbound: 3
Inbound: 0

### Outbound

- `emotion-measurement` via `contributes_to`
- `music-emotion-recognition` via `enables`
- `digital-signal-processing` via `used_in`

### Inbound

- None

## Auditory Looming (`auditory-looming`)

Outbound: 2
Inbound: 1

### Outbound

- `music-evoked-emotion` via `contributes_to`
- `perceived-space` via `influences`

### Inbound

- `perceived-space` via `contributes_to`

## Auditory Pathway (`auditory-pathway`)

Outbound: 2
Inbound: 1

### Outbound

- `pitch` via `contributes_to`
- `auditory-reward` via `enables`

### Inbound

- `auditory-transduction` via `enables`

## Auditory Reward (`auditory-reward`)

Outbound: 2
Inbound: 2

### Outbound

- `frisson` via `contributes_to`
- `music-evoked-emotion` via `contributes_to`

### Inbound

- `auditory-pathway` via `enables`
- `predictive-processing-music` via `contributes_to`

## Auditory Transduction (`auditory-transduction`)

Outbound: 4
Inbound: 1

### Outbound

- `pitch` via `contributes_to`
- `timbre` via `contributes_to`
- `auditory-pathway` via `enables`
- `sound` via `represents`

### Inbound

- `psychoacoustics` via `studies`

## Autonomic Response to Music (`autonomic-response-to-music`)

Outbound: 1
Inbound: 2

### Outbound

- `music-evoked-emotion` via `contributes_to`

### Inbound

- `emotion-measurement` via `studies`
- `frisson` via `contributes_to`

## Beatmatching (`beatmatching`)

Outbound: 2
Inbound: 1

### Outbound

- `rhythmic-entrainment` via `contributes_to`
- `djing` via `used_in`

### Inbound

- `rhythm` via `influences`

## Cassette Duplication (`cassette-duplication`)

Outbound: 2
Inbound: 1

### Outbound

- `music-distribution` via `enables`
- `screw-tape` via `used_in`

### Inbound

- `recording` via `enables`

## Chopped and Screwed (`chopped-and-screwed`)

Outbound: 1
Inbound: 9

### Outbound

- `chopped-and-slowed` via `influences`

### Inbound

- `chopping` via `used_in`
- `dj-screw` via `contributes_to`
- `houston-rap` via `enables`
- `repetition` via `used_in`
- `screw-tape` via `captures`
- `slowed-playback` via `used_in`
- `swishahouse` via `influences`
- `turntable` via `used_in`
- `turntablism` via `contributes_to`

## Chopped and Slowed (`chopped-and-slowed`)

Outbound: 0
Inbound: 2

### Outbound

- None

### Inbound

- `chopped-and-screwed` via `influences`
- `time-stretching` via `contributes_to`

## Chopping (`chopping`)

Outbound: 5
Inbound: 0

### Outbound

- `rhythm` via `contributes_to`
- `temporal-discontinuity` via `produces`
- `chopped-and-screwed` via `used_in`
- `temporal-dsp` via `used_in`
- `turntablism` via `used_in`

### Inbound

- None

## Digital Signal Processing (`digital-signal-processing`)

Outbound: 6
Inbound: 6

### Outbound

- `temporal-dsp` via `enables`
- `time-stretching` via `enables`
- `recording` via `processes`
- `sampling` via `processes`
- `djing` via `used_in`
- `synthesis` via `used_in`

### Inbound

- `audio-feature-extraction` via `used_in`
- `frequency` via `influences`
- `phase` via `influences`
- `recording` via `enables`
- `sampling` via `enables`
- `temporal-dsp` via `used_in`

## Digital Vinyl System (`digital-vinyl-system`)

Outbound: 2
Inbound: 1

### Outbound

- `djing` via `used_in`
- `turntablism` via `used_in`

### Inbound

- `sampling` via `enables`

## DJ Screw (`dj-screw`)

Outbound: 3
Inbound: 0

### Outbound

- `chopped-and-screwed` via `contributes_to`
- `screwed-up-click` via `enables`
- `screw-tape` via `produces`

### Inbound

- None

## DJing (`djing`)

Outbound: 2
Inbound: 5

### Outbound

- `screw-tape` via `contributes_to`
- `turntablism` via `enables`

### Inbound

- `beatmatching` via `used_in`
- `digital-signal-processing` via `used_in`
- `digital-vinyl-system` via `used_in`
- `recording` via `enables`
- `turntable` via `used_in`

## Dynamic Range Compression (`dynamic-range-compression`)

Outbound: 4
Inbound: 0

### Outbound

- `amplitude-envelope` via `influences`
- `groove` via `influences`
- `timbre` via `influences`
- `temporal-dsp` via `used_in`

### Inbound

- None

## Emotion Measurement (`emotion-measurement`)

Outbound: 2
Inbound: 2

### Outbound

- `autonomic-response-to-music` via `studies`
- `music-evoked-emotion` via `studies`

### Inbound

- `audio-feature-extraction` via `contributes_to`
- `music-emotion-recognition` via `contributes_to`

## Frequency (`frequency`)

Outbound: 6
Inbound: 5

### Outbound

- `digital-signal-processing` via `influences`
- `phase` via `influences`
- `pitch` via `influences`
- `resonance` via `influences`
- `sampling` via `influences`
- `timbre` via `influences`

### Inbound

- `psychoacoustics` via `studies`
- `scratching` via `influences`
- `sound` via `characterized_by`
- `synthesis` via `controls`
- `vibration` via `characterized_by`

## Frisson (`frisson`)

Outbound: 2
Inbound: 1

### Outbound

- `autonomic-response-to-music` via `contributes_to`
- `music-evoked-emotion` via `contributes_to`

### Inbound

- `auditory-reward` via `contributes_to`

## Gating (`gating`)

Outbound: 5
Inbound: 1

### Outbound

- `rhythm` via `contributes_to`
- `silence-as-musical-material` via `contributes_to`
- `amplitude-envelope` via `influences`
- `temporal-discontinuity` via `produces`
- `temporal-dsp` via `used_in`

### Inbound

- `scratching` via `used_in`

## Granular Fragmentation (`granular-fragmentation`)

Outbound: 4
Inbound: 1

### Outbound

- `micro-looping` via `contributes_to`
- `timbre` via `influences`
- `temporal-dsp` via `used_in`
- `time-stretching` via `used_in`

### Inbound

- `sampling` via `enables`

## Groove (`groove`)

Outbound: 2
Inbound: 3

### Outbound

- `rhythm` via `characterized_by`
- `music-evoked-emotion` via `contributes_to`

### Inbound

- `dynamic-range-compression` via `influences`
- `rhythmic-entrainment` via `contributes_to`
- `temporal-displacement` via `influences`

## Harmonicity (`harmonicity`)

Outbound: 3
Inbound: 1

### Outbound

- `musical-consonance` via `contributes_to`
- `pitch` via `contributes_to`
- `timbre` via `contributes_to`

### Inbound

- `affective-psychoacoustics` via `studies`

## Houston Musical Cartography (`houston-musical-cartography`)

Outbound: 4
Inbound: 1

### Outbound

- `houston-rap` via `studies`
- `houston-studio-infrastructure` via `studies`
- `music-distribution` via `studies`
- `third-coast` via `studies`

### Inbound

- `music-archiving` via `enables`

## Houston Rap (`houston-rap`)

Outbound: 2
Inbound: 8

### Outbound

- `third-coast` via `contributes_to`
- `chopped-and-screwed` via `enables`

### Inbound

- `houston-musical-cartography` via `studies`
- `houston-studio-infrastructure` via `captures`
- `mike-dean` via `contributes_to`
- `music-distribution` via `influences`
- `rap-a-lot-records` via `enables`
- `screwed-up-click` via `contributes_to`
- `south-park-coalition` via `contributes_to`
- `swishahouse` via `contributes_to`

## Houston Studio Infrastructure (`houston-studio-infrastructure`)

Outbound: 2
Inbound: 2

### Outbound

- `houston-rap` via `captures`
- `recording` via `used_in`

### Inbound

- `houston-musical-cartography` via `studies`
- `mike-dean` via `contributes_to`

## Micro-Looping (`micro-looping`)

Outbound: 4
Inbound: 3

### Outbound

- `repetition` via `contributes_to`
- `pitch` via `influences`
- `roughness` via `influences`
- `temporal-dsp` via `used_in`

### Inbound

- `granular-fragmentation` via `contributes_to`
- `repetition` via `contributes_to`
- `stutter` via `contributes_to`

## MIDI (`midi`)

Outbound: 5
Inbound: 0

### Outbound

- `sampling` via `controls`
- `synthesis` via `controls`
- `pitch` via `represents`
- `rhythm` via `represents`
- `sequencing` via `used_in`

### Inbound

- None

## Mike Dean (`mike-dean`)

Outbound: 2
Inbound: 0

### Outbound

- `houston-rap` via `contributes_to`
- `houston-studio-infrastructure` via `contributes_to`

### Inbound

- None

## Music and Memory (`music-and-memory`)

Outbound: 2
Inbound: 0

### Outbound

- `music-evoked-emotion` via `contributes_to`
- `musical-expectation` via `influences`

### Inbound

- None

## Music Archiving (`music-archiving`)

Outbound: 1
Inbound: 1

### Outbound

- `houston-musical-cartography` via `enables`

### Inbound

- `screw-tape` via `contributes_to`

## Music Distribution (`music-distribution`)

Outbound: 1
Inbound: 6

### Outbound

- `houston-rap` via `influences`

### Inbound

- `cassette-duplication` via `enables`
- `houston-musical-cartography` via `studies`
- `pen-and-pixel` via `contributes_to`
- `rap-a-lot-records` via `contributes_to`
- `screw-tape` via `contributes_to`
- `southwest-wholesale` via `contributes_to`

## Music Emotion Recognition (`music-emotion-recognition`)

Outbound: 2
Inbound: 1

### Outbound

- `emotion-measurement` via `contributes_to`
- `music-evoked-emotion` via `represents`

### Inbound

- `audio-feature-extraction` via `enables`

## Music-Evoked Emotion (`music-evoked-emotion`)

Outbound: 0
Inbound: 14

### Outbound

- None

### Inbound

- `affective-psychoacoustics` via `studies`
- `affective-timbre` via `contributes_to`
- `auditory-looming` via `contributes_to`
- `auditory-reward` via `contributes_to`
- `autonomic-response-to-music` via `contributes_to`
- `emotion-measurement` via `studies`
- `frisson` via `contributes_to`
- `groove` via `contributes_to`
- `music-and-memory` via `contributes_to`
- `music-emotion-recognition` via `represents`
- `musical-enculturation` via `contributes_to`
- `musical-expectation` via `contributes_to`
- `psychoacoustics` via `studies`
- `rhythmic-entrainment` via `contributes_to`

## Musical Consonance (`musical-consonance`)

Outbound: 1
Inbound: 5

### Outbound

- `musical-expectation` via `contributes_to`

### Inbound

- `harmonicity` via `contributes_to`
- `musical-dissonance` via `influences`
- `musical-enculturation` via `influences`
- `musical-expectation` via `influences`
- `sensory-dissonance` via `contributes_to`

## Musical Dissonance (`musical-dissonance`)

Outbound: 2
Inbound: 1

### Outbound

- `musical-expectation` via `contributes_to`
- `musical-consonance` via `influences`

### Inbound

- `sensory-dissonance` via `contributes_to`

## Musical Enculturation (`musical-enculturation`)

Outbound: 3
Inbound: 1

### Outbound

- `music-evoked-emotion` via `contributes_to`
- `musical-consonance` via `influences`
- `musical-expectation` via `influences`

### Inbound

- `predictive-processing-music` via `contributes_to`

## Musical Expectation (`musical-expectation`)

Outbound: 2
Inbound: 11

### Outbound

- `music-evoked-emotion` via `contributes_to`
- `musical-consonance` via `influences`

### Inbound

- `music-and-memory` via `influences`
- `musical-consonance` via `contributes_to`
- `musical-dissonance` via `contributes_to`
- `musical-enculturation` via `influences`
- `predictive-processing-music` via `contributes_to`
- `repetition` via `influences`
- `silence-as-musical-material` via `influences`
- `stutter` via `influences`
- `temporal-discontinuity` via `influences`
- `temporal-displacement` via `influences`
- `temporal-dsp` via `influences`

## Pen and Pixel (`pen-and-pixel`)

Outbound: 1
Inbound: 0

### Outbound

- `music-distribution` via `contributes_to`

### Inbound

- None

## Perceived Space (`perceived-space`)

Outbound: 3
Inbound: 1

### Outbound

- `sound` via `characterized_by`
- `auditory-looming` via `contributes_to`
- `recording` via `influences`

### Inbound

- `auditory-looming` via `influences`

## Phase (`phase`)

Outbound: 2
Inbound: 2

### Outbound

- `digital-signal-processing` via `influences`
- `recording` via `influences`

### Inbound

- `frequency` via `influences`
- `sound` via `characterized_by`

## Pitch (`pitch`)

Outbound: 1
Inbound: 9

### Outbound

- `timbre` via `influences`

### Inbound

- `auditory-pathway` via `contributes_to`
- `auditory-transduction` via `contributes_to`
- `frequency` via `influences`
- `harmonicity` via `contributes_to`
- `micro-looping` via `influences`
- `midi` via `represents`
- `psychoacoustics` via `studies`
- `slowed-playback` via `influences`
- `synthesis` via `produces`

## Predictive Processing in Music (`predictive-processing-music`)

Outbound: 3
Inbound: 0

### Outbound

- `auditory-reward` via `contributes_to`
- `musical-enculturation` via `contributes_to`
- `musical-expectation` via `contributes_to`

### Inbound

- None

## Psychoacoustics (`psychoacoustics`)

Outbound: 8
Inbound: 0

### Outbound

- `auditory-transduction` via `studies`
- `frequency` via `studies`
- `music-evoked-emotion` via `studies`
- `pitch` via `studies`
- `rhythm` via `studies`
- `roughness` via `studies`
- `sound` via `studies`
- `timbre` via `studies`

### Inbound

- None

## Rap-A-Lot Records (`rap-a-lot-records`)

Outbound: 2
Inbound: 0

### Outbound

- `music-distribution` via `contributes_to`
- `houston-rap` via `enables`

### Inbound

- None

## Recording (`recording`)

Outbound: 6
Inbound: 4

### Outbound

- `sound` via `captures`
- `vibration` via `captures`
- `cassette-duplication` via `enables`
- `digital-signal-processing` via `enables`
- `djing` via `enables`
- `sampling` via `enables`

### Inbound

- `digital-signal-processing` via `processes`
- `houston-studio-infrastructure` via `used_in`
- `perceived-space` via `influences`
- `phase` via `influences`

## Repetition (`repetition`)

Outbound: 4
Inbound: 2

### Outbound

- `micro-looping` via `contributes_to`
- `rhythm` via `contributes_to`
- `musical-expectation` via `influences`
- `chopped-and-screwed` via `used_in`

### Inbound

- `micro-looping` via `contributes_to`
- `stutter` via `contributes_to`

## Resonance (`resonance`)

Outbound: 3
Inbound: 2

### Outbound

- `timbre` via `contributes_to`
- `sound` via `influences`
- `turntable` via `influences`

### Inbound

- `frequency` via `influences`
- `vibration` via `influences`

## Retriggering (`retriggering`)

Outbound: 4
Inbound: 1

### Outbound

- `rhythm` via `contributes_to`
- `amplitude-envelope` via `influences`
- `sampling` via `used_in`
- `temporal-dsp` via `used_in`

### Inbound

- `sampling` via `enables`

## Rhythm (`rhythm`)

Outbound: 4
Inbound: 13

### Outbound

- `beatmatching` via `influences`
- `rhythmic-entrainment` via `influences`
- `sampling` via `influences`
- `sequencing` via `influences`

### Inbound

- `amplitude-envelope` via `contributes_to`
- `chopping` via `contributes_to`
- `gating` via `contributes_to`
- `groove` via `characterized_by`
- `midi` via `represents`
- `psychoacoustics` via `studies`
- `repetition` via `contributes_to`
- `retriggering` via `contributes_to`
- `scratching` via `contributes_to`
- `silence-as-musical-material` via `contributes_to`
- `slowed-playback` via `influences`
- `temporal-displacement` via `influences`
- `turntablism` via `influences`

## Rhythmic Entrainment (`rhythmic-entrainment`)

Outbound: 2
Inbound: 2

### Outbound

- `groove` via `contributes_to`
- `music-evoked-emotion` via `contributes_to`

### Inbound

- `beatmatching` via `contributes_to`
- `rhythm` via `influences`

## Roughness (`roughness`)

Outbound: 2
Inbound: 4

### Outbound

- `sensory-dissonance` via `contributes_to`
- `timbre` via `contributes_to`

### Inbound

- `affective-psychoacoustics` via `studies`
- `amplitude-envelope` via `influences`
- `micro-looping` via `influences`
- `psychoacoustics` via `studies`

## Sampling (`sampling`)

Outbound: 6
Inbound: 10

### Outbound

- `digital-signal-processing` via `enables`
- `digital-vinyl-system` via `enables`
- `granular-fragmentation` via `enables`
- `retriggering` via `enables`
- `sound` via `represents`
- `timbre` via `represents`

### Inbound

- `digital-signal-processing` via `processes`
- `frequency` via `influences`
- `midi` via `controls`
- `recording` via `enables`
- `retriggering` via `used_in`
- `rhythm` via `influences`
- `scratching` via `controls`
- `sequencing` via `controls`
- `time-stretching` via `processes`
- `turntablism` via `contributes_to`

## Scratching (`scratching`)

Outbound: 6
Inbound: 0

### Outbound

- `rhythm` via `contributes_to`
- `sampling` via `controls`
- `frequency` via `influences`
- `gating` via `used_in`
- `temporal-dsp` via `used_in`
- `turntablism` via `used_in`

### Inbound

- None

## Screw Tape (`screw-tape`)

Outbound: 3
Inbound: 4

### Outbound

- `chopped-and-screwed` via `captures`
- `music-archiving` via `contributes_to`
- `music-distribution` via `contributes_to`

### Inbound

- `cassette-duplication` via `used_in`
- `dj-screw` via `produces`
- `djing` via `contributes_to`
- `screwed-up-click` via `contributes_to`

## Screwed Up Click (`screwed-up-click`)

Outbound: 2
Inbound: 1

### Outbound

- `houston-rap` via `contributes_to`
- `screw-tape` via `contributes_to`

### Inbound

- `dj-screw` via `enables`

## Sensory Dissonance (`sensory-dissonance`)

Outbound: 2
Inbound: 1

### Outbound

- `musical-consonance` via `contributes_to`
- `musical-dissonance` via `contributes_to`

### Inbound

- `roughness` via `contributes_to`

## Sequencing (`sequencing`)

Outbound: 2
Inbound: 2

### Outbound

- `sampling` via `controls`
- `synthesis` via `controls`

### Inbound

- `midi` via `used_in`
- `rhythm` via `influences`

## Silence as Musical Material (`silence-as-musical-material`)

Outbound: 3
Inbound: 2

### Outbound

- `rhythm` via `contributes_to`
- `amplitude-envelope` via `influences`
- `musical-expectation` via `influences`

### Inbound

- `gating` via `contributes_to`
- `temporal-discontinuity` via `contributes_to`

## Slowed Playback (`slowed-playback`)

Outbound: 5
Inbound: 0

### Outbound

- `affective-timbre` via `influences`
- `pitch` via `influences`
- `rhythm` via `influences`
- `timbre` via `influences`
- `chopped-and-screwed` via `used_in`

### Inbound

- None

## Sound (`sound`)

Outbound: 3
Inbound: 9

### Outbound

- `frequency` via `characterized_by`
- `phase` via `characterized_by`
- `timbre` via `characterized_by`

### Inbound

- `amplitude-envelope` via `characterized_by`
- `auditory-transduction` via `represents`
- `perceived-space` via `characterized_by`
- `psychoacoustics` via `studies`
- `recording` via `captures`
- `resonance` via `influences`
- `sampling` via `represents`
- `synthesis` via `produces`
- `vibration` via `produces`

## South Park Coalition (`south-park-coalition`)

Outbound: 1
Inbound: 0

### Outbound

- `houston-rap` via `contributes_to`

### Inbound

- None

## Southwest Wholesale (`southwest-wholesale`)

Outbound: 1
Inbound: 0

### Outbound

- `music-distribution` via `contributes_to`

### Inbound

- None

## Stutter (`stutter`)

Outbound: 4
Inbound: 0

### Outbound

- `micro-looping` via `contributes_to`
- `repetition` via `contributes_to`
- `musical-expectation` via `influences`
- `temporal-dsp` via `used_in`

### Inbound

- None

## Swishahouse (`swishahouse`)

Outbound: 2
Inbound: 0

### Outbound

- `houston-rap` via `contributes_to`
- `chopped-and-screwed` via `influences`

### Inbound

- None

## Synthesis (`synthesis`)

Outbound: 4
Inbound: 3

### Outbound

- `frequency` via `controls`
- `pitch` via `produces`
- `sound` via `produces`
- `timbre` via `produces`

### Inbound

- `digital-signal-processing` via `used_in`
- `midi` via `controls`
- `sequencing` via `controls`

## Temporal Discontinuity (`temporal-discontinuity`)

Outbound: 3
Inbound: 3

### Outbound

- `silence-as-musical-material` via `contributes_to`
- `amplitude-envelope` via `influences`
- `musical-expectation` via `influences`

### Inbound

- `chopping` via `produces`
- `gating` via `produces`
- `temporal-dsp` via `produces`

## Temporal Displacement (`temporal-displacement`)

Outbound: 3
Inbound: 0

### Outbound

- `groove` via `influences`
- `musical-expectation` via `influences`
- `rhythm` via `influences`

### Inbound

- None

## Temporal DSP (`temporal-dsp`)

Outbound: 4
Inbound: 10

### Outbound

- `amplitude-envelope` via `influences`
- `musical-expectation` via `influences`
- `temporal-discontinuity` via `produces`
- `digital-signal-processing` via `used_in`

### Inbound

- `chopping` via `used_in`
- `digital-signal-processing` via `enables`
- `dynamic-range-compression` via `used_in`
- `gating` via `used_in`
- `granular-fragmentation` via `used_in`
- `micro-looping` via `used_in`
- `retriggering` via `used_in`
- `scratching` via `used_in`
- `stutter` via `used_in`
- `time-stretching` via `used_in`

## Third Coast (`third-coast`)

Outbound: 0
Inbound: 2

### Outbound

- None

### Inbound

- `houston-musical-cartography` via `studies`
- `houston-rap` via `contributes_to`

## Timbre (`timbre`)

Outbound: 0
Inbound: 15

### Outbound

- None

### Inbound

- `affective-timbre` via `characterized_by`
- `amplitude-envelope` via `contributes_to`
- `auditory-transduction` via `contributes_to`
- `dynamic-range-compression` via `influences`
- `frequency` via `influences`
- `granular-fragmentation` via `influences`
- `harmonicity` via `contributes_to`
- `pitch` via `influences`
- `psychoacoustics` via `studies`
- `resonance` via `contributes_to`
- `roughness` via `contributes_to`
- `sampling` via `represents`
- `slowed-playback` via `influences`
- `sound` via `characterized_by`
- `synthesis` via `produces`

## Time Stretching (`time-stretching`)

Outbound: 3
Inbound: 2

### Outbound

- `chopped-and-slowed` via `contributes_to`
- `sampling` via `processes`
- `temporal-dsp` via `used_in`

### Inbound

- `digital-signal-processing` via `enables`
- `granular-fragmentation` via `used_in`

## Turntable (`turntable`)

Outbound: 4
Inbound: 1

### Outbound

- `vibration` via `produces`
- `chopped-and-screwed` via `used_in`
- `djing` via `used_in`
- `turntablism` via `used_in`

### Inbound

- `resonance` via `influences`

## Turntablism (`turntablism`)

Outbound: 3
Inbound: 5

### Outbound

- `chopped-and-screwed` via `contributes_to`
- `sampling` via `contributes_to`
- `rhythm` via `influences`

### Inbound

- `chopping` via `used_in`
- `digital-vinyl-system` via `used_in`
- `djing` via `enables`
- `scratching` via `used_in`
- `turntable` via `used_in`

## Vibration (`vibration`)

Outbound: 3
Inbound: 2

### Outbound

- `frequency` via `characterized_by`
- `resonance` via `influences`
- `sound` via `produces`

### Inbound

- `recording` via `captures`
- `turntable` via `produces`
