# Node Connections

Canonical relationships: 45
Outbound total: 45
Inbound total: 45

## Digital Signal Processing (`digital-signal-processing`)

Outbound: 3
Inbound: 4

### Outbound

- `recording` via `processes`
- `sampling` via `processes`
- `synthesis` via `used_in`

### Inbound

- `frequency` via `influences`
- `phase` via `influences`
- `recording` via `enables`
- `sampling` via `enables`

## Frequency (`frequency`)

Outbound: 6
Inbound: 4

### Outbound

- `digital-signal-processing` via `influences`
- `phase` via `influences`
- `pitch` via `influences`
- `resonance` via `influences`
- `sampling` via `influences`
- `timbre` via `influences`

### Inbound

- `psychoacoustics` via `studies`
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

Outbound: 4
Inbound: 2

### Outbound

- `sound` via `captures`
- `vibration` via `captures`
- `digital-signal-processing` via `enables`
- `sampling` via `enables`

### Inbound

- `digital-signal-processing` via `processes`
- `phase` via `influences`

## Resonance (`resonance`)

Outbound: 2
Inbound: 2

### Outbound

- `timbre` via `contributes_to`
- `sound` via `influences`

### Inbound

- `frequency` via `influences`
- `vibration` via `influences`

## Rhythm (`rhythm`)

Outbound: 2
Inbound: 2

### Outbound

- `sampling` via `influences`
- `sequencing` via `influences`

### Inbound

- `midi` via `represents`
- `psychoacoustics` via `studies`

## Sampling (`sampling`)

Outbound: 3
Inbound: 6

### Outbound

- `digital-signal-processing` via `enables`
- `sound` via `represents`
- `timbre` via `represents`

### Inbound

- `digital-signal-processing` via `processes`
- `frequency` via `influences`
- `midi` via `controls`
- `recording` via `enables`
- `rhythm` via `influences`
- `sequencing` via `controls`

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

## Vibration (`vibration`)

Outbound: 3
Inbound: 1

### Outbound

- `frequency` via `characterized_by`
- `resonance` via `influences`
- `sound` via `produces`

### Inbound

- `recording` via `captures`
