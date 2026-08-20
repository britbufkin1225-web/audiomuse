---
id: affective-timbre
title: Affective Timbre
domain: psychoacoustics
status: foundation
session_origin: []
definition: The relationship between spectral and envelope properties of a sound and the affective qualities listeners attribute to it, in which distinct descriptor combinations relate to distinct affective dimensions rather than one quality of sound carrying one feeling.
core_concepts:
  - brightness, spectral slope, spectral variation, attack, and decay as separate variables
  - different descriptor combinations for valence, energy arousal, and tension arousal
  - register interacts with instrument identity rather than shifting everything uniformly
  - findings come from isolated tones, not from music
  - rated affective quality of a sound is closer to perceived than to felt emotion
relationships:
  - target: timbre
    type: characterized_by
  - target: music-evoked-emotion
    type: contributes_to
sources:
  - mcadams-affective-qualities-instrument-sounds
  - juslin-brecvema-unified-theory
  - head-acoustics-psychoacoustic-analyses
experiments: []
practical_applications:
  - choosing between changing brightness and changing attack when a part reads as too tense
  - understanding why the same patch an octave down does not simply sound like a lower version of itself
project_connections:
  - AudioMuse affective psychoacoustics foundation
future_questions:
  - Do the descriptor relationships from isolated orchestral tones survive in a dense electronic mix?
  - Where do distortion and saturation sit, given that the source corpus contained neither?
---

# Affective Timbre

McAdams, Douglas, and Vempala computed twenty-three audio descriptors for musical instrument sounds across pitch registers and modelled listeners' affective ratings against them. The result AudioMuse takes from it is structural: distinct combinations of descriptors relate to distinct affective dimensions, so the three dimensions are carried by different acoustic properties rather than by one quantity of expressiveness.

The reported relationships are worth stating precisely, because the precision is what stops them collapsing into folklore. Valence was rated more positive with lower spectral slopes, greater emergence of strong partials, and an amplitude envelope with sharper attack and earlier decay. Tension arousal was higher with brighter sounds, more spectral variation, and more gentle attacks. Energy arousal was higher with brighter sounds, higher spectral centroids, and slower decrease of the spectral slope.

Notice what that pattern rules out. Brightness relates to both tension arousal and energy arousal, so brightness alone does not identify an affective dimension. Attack relates to valence in one direction and to tension arousal in the other, so sharpening an attack is not simply making a sound more positive. Anyone who has tried to fix a part that reads as anxious by brightening it has met this in practice.

Register did not behave as a simple scaling either: valence ratings had a nonlinear concave relation to register, more positive in middle registers, apart from percussion, and register interacted with instrument identity. This is the finding that bears most directly on the existing AudioMuse Houston material, because sustained rate reduction moves register, envelope, and spectrum together. `slowed-playback` holds the mechanism; this node holds why the affective consequence is not derivable from any one of the three.

The limits are severe and must travel with the result. The stimuli were isolated tones, and the authors state that future work should validate the relationships in a musical context. The corpus was orchestral instruments, so distortion, saturation, and synthesized timbres are outside it. And what was collected was rated affective quality of a sound, which sits closer to perceived than to induced emotion. Experiment `timbre-emotion-separation` is the AudioMuse proposal for testing the distinction inside actual musical material.
