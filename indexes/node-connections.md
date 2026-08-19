# Node Connections

Canonical relationships: 63
Outbound total: 63
Inbound total: 63

## Beatmatching (`beatmatching`)

Outbound: 1
Inbound: 1

### Outbound

- `djing` via `used_in`

### Inbound

- `rhythm` via `influences`

## Digital Signal Processing (`digital-signal-processing`)

Outbound: 4
Inbound: 4

### Outbound

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

## DJing (`djing`)

Outbound: 1
Inbound: 5

### Outbound

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
Inbound: 4

### Outbound

- `timbre` via `influences`

### Inbound

- `frequency` via `influences`
- `midi` via `represents`
- `psychoacoustics` via `studies`
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

## Recording (`recording`)

Outbound: 5
Inbound: 2

### Outbound

- `sound` via `captures`
- `vibration` via `captures`
- `digital-signal-processing` via `enables`
- `djing` via `enables`
- `sampling` via `enables`

### Inbound

- `digital-signal-processing` via `processes`
- `phase` via `influences`

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
Inbound: 4

### Outbound

- `beatmatching` via `influences`
- `sampling` via `influences`
- `sequencing` via `influences`

### Inbound

- `midi` via `represents`
- `psychoacoustics` via `studies`
- `scratching` via `contributes_to`
- `turntablism` via `influences`

## Sampling (`sampling`)

Outbound: 4
Inbound: 8

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

## Sequencing (`sequencing`)

Outbound: 2
Inbound: 2

### Outbound

- `sampling` via `controls`
- `synthesis` via `controls`

### Inbound

- `midi` via `used_in`
- `rhythm` via `influences`

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

## Timbre (`timbre`)

Outbound: 0
Inbound: 7

### Outbound

- None

### Inbound

- `frequency` via `influences`
- `pitch` via `influences`
- `psychoacoustics` via `studies`
- `resonance` via `contributes_to`
- `sampling` via `represents`
- `sound` via `characterized_by`
- `synthesis` via `produces`

## Turntable (`turntable`)

Outbound: 3
Inbound: 1

### Outbound

- `vibration` via `produces`
- `djing` via `used_in`
- `turntablism` via `used_in`

### Inbound

- `resonance` via `influences`

## Turntablism (`turntablism`)

Outbound: 2
Inbound: 4

### Outbound

- `sampling` via `contributes_to`
- `rhythm` via `influences`

### Inbound

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
