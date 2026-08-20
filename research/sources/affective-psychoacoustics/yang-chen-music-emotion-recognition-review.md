# Machine Recognition of Music Emotion: A Review

- Source ID: `yang-chen-music-emotion-recognition-review`
- Class: scholarly work (peer-reviewed survey)
- Authors: Yi-Hsuan Yang, Homer H. Chen
- Publication: ACM Transactions on Intelligent Systems and Technology 3 (3), Article 40, 2012
- Identifier: DOI 10.1145/2168752.2168754
- Stable locator: https://doi.org/10.1145/2168752.2168754
- Retrieval: full text retrieved

## Supports

- The distinction AudioMuse needs before it says anything about machine audio and emotion: the
  literature separates expressed emotion (what the performer intends to communicate), perceived
  emotion (what a listener hears the music as expressing), and felt or induced emotion (what the
  listener actually feels). The review states that music information retrieval researchers tend to
  focus on perceived emotion because it is less influenced by situational factors.
- A music emotion recognition system is trained on ground truth collected from subjective tests, and
  predicts labels of that kind. Timbre, rhythm, and harmony features are typically extracted to
  represent the acoustic properties of a piece.
- The stated challenges: emotion perception is subjective and different people perceive different
  emotions in the same song, which makes evaluation fundamentally difficult; annotations are hard to
  obtain with no consensus taxonomy; and it remains, in the authors' words, far from well understood
  what intrinsic element of music creates a specific emotional response.
- Ground truth is often produced by averaging the ratings of a modest number of subjects, or scraped
  from social tags whose quality the review describes as lower.

## Does not settle

- Whether any current model performs well. The review predates modern deep-learning systems.
- Anything about what a model represents internally. The review is about task formulation, features,
  and evaluation.
- Any suggestion that a model experiences emotion. Nothing in the source supports that reading, and
  AudioMuse registers this source partly to foreclose it.
