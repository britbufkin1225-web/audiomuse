# Node Connections

Canonical relationships: 109
Outbound total: 109
Inbound total: 109

## Beatmatching (`beatmatching`)

Outbound: 1
Inbound: 1

### Outbound

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

Outbound: 3
Inbound: 0

### Outbound

- `rhythm` via `contributes_to`
- `chopped-and-screwed` via `used_in`
- `turntablism` via `used_in`

### Inbound

- None

## Digital Signal Processing (`digital-signal-processing`)

Outbound: 5
Inbound: 4

### Outbound

- `time-stretching` via `enables`
- `recording` via `processes`
- `sampling` via `processes`
- `djing` via `used_in`
- `synthesis` via `used_in`

### Inbound

- `frequency` via `influences`
- `phase` via `influences`
- `recording` via `enables`
- `sampling` via `enables`

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

## Pen and Pixel (`pen-and-pixel`)

Outbound: 1
Inbound: 0

### Outbound

- `music-distribution` via `contributes_to`

### Inbound

- None

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
Inbound: 5

### Outbound

- `timbre` via `influences`

### Inbound

- `frequency` via `influences`
- `midi` via `represents`
- `psychoacoustics` via `studies`
- `slowed-playback` via `influences`
- `synthesis` via `produces`

## Psychoacoustics (`psychoacoustics`)

Outbound: 5
Inbound: 0

### Outbound

- `frequency` via `studies`
- `pitch` via `studies`
- `rhythm` via `studies`
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
Inbound: 3

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
- `phase` via `influences`

## Repetition (`repetition`)

Outbound: 2
Inbound: 0

### Outbound

- `rhythm` via `contributes_to`
- `chopped-and-screwed` via `used_in`

### Inbound

- None

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

## Rhythm (`rhythm`)

Outbound: 3
Inbound: 7

### Outbound

- `beatmatching` via `influences`
- `sampling` via `influences`
- `sequencing` via `influences`

### Inbound

- `chopping` via `contributes_to`
- `midi` via `represents`
- `psychoacoustics` via `studies`
- `repetition` via `contributes_to`
- `scratching` via `contributes_to`
- `slowed-playback` via `influences`
- `turntablism` via `influences`

## Sampling (`sampling`)

Outbound: 4
Inbound: 9

### Outbound

- `digital-signal-processing` via `enables`
- `digital-vinyl-system` via `enables`
- `sound` via `represents`
- `timbre` via `represents`

### Inbound

- `digital-signal-processing` via `processes`
- `frequency` via `influences`
- `midi` via `controls`
- `recording` via `enables`
- `rhythm` via `influences`
- `scratching` via `controls`
- `sequencing` via `controls`
- `time-stretching` via `processes`
- `turntablism` via `contributes_to`

## Scratching (`scratching`)

Outbound: 4
Inbound: 0

### Outbound

- `rhythm` via `contributes_to`
- `sampling` via `controls`
- `frequency` via `influences`
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

## Sequencing (`sequencing`)

Outbound: 2
Inbound: 2

### Outbound

- `sampling` via `controls`
- `synthesis` via `controls`

### Inbound

- `midi` via `used_in`
- `rhythm` via `influences`

## Slowed Playback (`slowed-playback`)

Outbound: 4
Inbound: 0

### Outbound

- `pitch` via `influences`
- `rhythm` via `influences`
- `timbre` via `influences`
- `chopped-and-screwed` via `used_in`

### Inbound

- None

## Sound (`sound`)

Outbound: 3
Inbound: 6

### Outbound

- `frequency` via `characterized_by`
- `phase` via `characterized_by`
- `timbre` via `characterized_by`

### Inbound

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
Inbound: 8

### Outbound

- None

### Inbound

- `frequency` via `influences`
- `pitch` via `influences`
- `psychoacoustics` via `studies`
- `resonance` via `contributes_to`
- `sampling` via `represents`
- `slowed-playback` via `influences`
- `sound` via `characterized_by`
- `synthesis` via `produces`

## Time Stretching (`time-stretching`)

Outbound: 2
Inbound: 1

### Outbound

- `chopped-and-slowed` via `contributes_to`
- `sampling` via `processes`

### Inbound

- `digital-signal-processing` via `enables`

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
