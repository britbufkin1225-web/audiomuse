---
id: music-emotion-recognition
title: Music Emotion Recognition
domain: machine-audio
status: seed
session_origin: []
definition: The machine-learning task of predicting emotion labels for music from audio features and training data, which infers labels of a particular kind from human annotations and does not detect, measure, or experience feeling.
core_concepts:
  - expressed, perceived, and felt emotion are three different targets
  - the field mostly targets perceived emotion
  - ground truth is averaged subjective annotation
  - subjectivity makes evaluation difficult in principle, not just in practice
  - a predicted label is not a measured emotion
relationships:
  - target: music-evoked-emotion
    type: represents
  - target: emotion-measurement
    type: contributes_to
sources:
  - yang-chen-music-emotion-recognition-review
  - mauss-robinson-measures-of-emotion
experiments: []
practical_applications:
  - reading a mood-tagging feature in a music service for what it actually is
  - specifying what an AudioMuse analysis tool would and would not be claiming
project_connections:
  - AudioMuse machine audio foundation
future_questions:
  - What would an AudioMuse tool have to output to be useful without implying it detects feeling?
  - Do modern learned representations change the annotation problem, or only the modelling around it?
---

# Music Emotion Recognition

Yang and Chen's survey supplies the distinction that everything here rests on. The literature separates expressed emotion — what a performer intends to communicate — from perceived emotion — what a listener hears the music as expressing — from felt or induced emotion — what the listener actually experiences. The survey states that music information retrieval researchers tend to target perceived emotion, because it is less influenced by the situational factors of listening.

That choice determines what a trained system is. Ground truth is collected by subjective test, often by averaging ratings from a modest number of participants, or by scraping tags whose quality the survey describes as lower. Timbre, rhythm, and harmony features are extracted, and a model learns a mapping from those features to those labels. What comes out the other end is a prediction of how annotators of that kind would have labelled a track of that kind. It is a useful thing. It is not a measurement of a listener, and it is not an experience.

The survey is candid about why this is hard rather than merely unfinished. Emotion perception is subjective and different people perceive different emotions in the same song, which makes evaluation fundamentally difficult because agreement on the correct answer is hard to obtain. There is no consensus emotion taxonomy. And the authors state that it remains far from well understood what intrinsic element of music, if any, creates a specific emotional response.

Mauss and Robinson's conclusion applies here with full force: there is no gold-standard measure of emotional responding, and self-report, physiology, and behaviour are not interchangeable. A model trained on one of them inherits that limitation and cannot escape it by being accurate.

AudioMuse therefore records the following as a boundary rather than as an opinion. A machine listening system infers labels from data. It does not literally experience emotion, no output of such a system is evidence that any listener felt anything, and no AudioMuse material may describe one as detecting feeling. `audio-feature-extraction` holds the inputs; `emotion-measurement` holds what a real measurement would require.
