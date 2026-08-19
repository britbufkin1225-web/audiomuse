---
id: digital-vinyl-system
title: Digital Vinyl System
domain: dj-turntablism
status: foundation
session_origin:
  - session-03-history-of-electronic-music
definition: A DJ system in which timecode records played on ordinary turntables control the playback of digital audio files.
core_concepts:
  - timecode vinyl
  - control signal
  - digital music library
  - turntable interface
  - latency
relationships:
  - target: djing
    type: used_in
  - target: turntablism
    type: used_in
sources:
  - session-03-history-of-electronic-music
experiments: []
practical_applications:
  - playing a digital library through a turntable interface
  - scratching digital files without abandoning vinyl mechanics
  - hybrid vinyl and digital DJ setups
project_connections:
  - AudioMuse analog-control to digital-playback signal-chain studies
future_questions:
  - How much delay between timecode motion and digital playback becomes perceptible, and what method would measure it?
  - How should AudioMuse represent real-time stem separation once a source develops it beyond a capability list?
---

# Digital Vinyl System

Session 3 describes the 2000s shift in which DJ software began replacing some traditional vinyl workflows, using digital music libraries, controllers, CDJs, and timecode vinyl. Digital vinyl systems are the part of that shift that kept the original interface. The session names Serato Scratch Live as an example of a system letting DJs manipulate digital files using physical turntables and timecode records, and summarizes the combination as vinyl mechanics plus digital audio.

The important structural point for AudioMuse is what the record now carries. On a timecode record the groove no longer holds the music; it holds a control signal from which the software derives position, speed, and direction. The mechanical chain described in `turntable` is unchanged — the stylus still reads a groove, the platter still turns, the hand still moves it — but what that motion controls is a digital file rather than the audio in the groove itself. This is why the system depends on digital audio representation, recorded as `sampling --enables--> digital-vinyl-system` on the `sampling` node.

Because the gesture survives intact, the same system is used in both `djing` and `turntablism`. Session 3 states the consequence explicitly: traditional scratching survives into the digital era without abandoning the turntable interface.

The session also records where this continues. Modern DJ platforms increasingly offer real-time stem separation, letting a DJ isolate vocals, drums, bass, or instrumental parts from a finished mix. The session notes this is particularly significant for turntablism and that it thins the border between DJ and producer, but it lists the capability rather than developing its method, so AudioMuse records it here without claiming how it works.
