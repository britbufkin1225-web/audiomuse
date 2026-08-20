# The AudioMuse Affective Mechanism Stack

Phase 12E. How physical sound becomes emotional experience, layer by layer, with the temporal-DSP layer that AudioMuse previously lacked put in its place between waveform manipulation and musical expectation.

## 0. What this document refuses to do

The question this phase set out to answer is how physical sound, auditory biology, signal processing, musical structure, expectation, memory, culture, and bodily response combine to produce emotion, and how those mechanisms can be measured, manipulated, performed, and tested.

The most common answer in circulation is a lookup table: major is happy, minor is sad, fast is exciting, distortion is angry, reverb is sad, minor seventh is jazzy. AudioMuse does not hold that view and this document exists partly to make it unavailable. The reasons are specific rather than a general preference for nuance:

- The perceptual regularities that such tables are built on have been shown, in retrieved cross-cultural work, to exist without producing the corresponding aesthetic preference.
- The measures used to establish affective response do not converge with one another, so a claim asserted in one currency cannot be cashed in another.
- The frameworks in this field are explicit that the music affords a response and does not guarantee it, because a mechanism responds to a musical event comprising the music, the listener, and the context together.

What replaces the lookup table is a layered model in which each layer constrains the next without determining it. The value of the layering is not completeness. It is that any given statement can be located, so that a fact about roughness is not silently promoted into a claim about sadness.

## 1. The stack

```text
SOUND EVENT
   │
   ├── spectrum
   ├── amplitude
   ├── envelope
   ├── spatial cues
   └── timing
        │
        ▼
AUDITORY ENCODING
   │  cochlear filtering, transduction, place and temporal codes
        │
        ▼
PERCEPTUAL FEATURES
   │
   ├── pitch
   ├── timbre
   ├── roughness
   ├── loudness
   ├── localization
   └── onset
        │
        ▼
TEMPORAL ORGANIZATION
   │
   ├── grouping
   ├── repetition
   ├── groove
   ├── interruption
   ├── gating
   ├── chopping
   └── stutter
        │
        ▼
PREDICTION
   │
   ├── expected
   ├── delayed
   ├── omitted
   └── violated
        │
        ▼
CONTEXTUAL INTERPRETATION
   │
   ├── memory
   ├── culture
   ├── genre
   ├── familiarity
   └── appraisal
        │
        ▼
RESPONSE
   │
   ├── attention
   ├── movement
   ├── autonomic arousal
   ├── reward
   └── subjective emotion
```

Two things about this diagram matter as much as its contents. It is not a pipeline with one direction: expectation shapes what is perceived, and memory changes what is expected. And no layer is sufficient. A complete description at the level of the signal is still a description of the signal.

## 2. Physical sound

The bottom layer is the one AudioMuse has held since Session 1: sound is a physical process, and its perceptual interpretation must stay distinct from the waveform. The properties that later layers act on are frequency content, spectral centroid and slope, harmonicity and inharmonicity, amplitude and dynamic range, onset strength and transient sharpness, envelope, modulation, temporal density, duration, silence, spatial position, reverberation, distance cues, distortion, noise, and bandwidth.

None of these encodes an emotion. Each is a variable that later layers can be sensitive to. `amplitude-envelope` is registered as its own node in Phase 12E precisely because it is the variable most of the temporal operations act on, and because it had been living inside `timbre` and `rhythm` without a name of its own.

## 3. Cochlear and auditory encoding

Pressure variation becomes neural activity through a chain AudioMuse can state from retrieved institutional and textbook sources: outer ear, canal, tympanic membrane, ossicles, cochlea, basilar membrane travelling wave, inner and outer hair cells, auditory nerve.

Two properties of that arrangement propagate all the way up.

**Frequency analysis is mechanical and comes first.** Position along the basilar membrane corresponds to frequency, and the resulting tonotopic organization is retained at every level of the central auditory system. The frequency axis a producer manipulates with an equalizer is the axis the nervous system is arranged along.

**Timing is encoded with extraordinary precision, up to a limit.** Transduction converts bundle displacement into an electrical potential in as little as 10 microseconds, and threshold movements are around 0.3 nanometres. Auditory nerve fibres phase-lock to low-frequency waveforms and can follow stimuli up to about 3 kHz in a one-to-one fashion; above that, frequency is carried by place rather than by timing.

Above the nerve, AudioMuse currently records structure without detail. `auditory-pathway` is deliberately a seed node: no authoritative source on the brainstem nuclei, inferior colliculus, medial geniculate, or auditory cortex was retrieved in this phase, and writing a confident chain from unread sources is exactly the failure mode this repository is built to avoid.

Nothing about emotion exists at this layer. What it supplies upward is a representation that is simultaneously frequency-resolved and time-resolved, carrying the resolution limits that later effects inherit.

## 4. Psychoacoustic feature extraction

Perceptual features are not signal properties. They are what the system above the nerve makes of them, and the mapping is many-to-many: `harmonicity` contributes to pitch and to timbre and to fusion; `amplitude-envelope` contributes to timbre and to rhythm.

The feature Phase 12E develops furthest is **roughness**, because it is measurable, bounded, and routinely misdescribed. Its relationship to envelope modulation rate is band-pass rather than monotonic. Taking a 1 kHz tone modulated at full depth and raising the modulation frequency:

| Modulation rate | What is heard |
| --- | --- |
| below about 10 Hz | pulsation, tracked as individual variation |
| near 4 Hz | maximum fluctuation strength (unit: vacil) |
| roughly 15–45 Hz | a slower R-roughness, peaking near 20 Hz |
| roughly 20–300 Hz | roughness proper |
| near 70 Hz at a 1 kHz carrier | maximum roughness (unit: asper) |
| above roughly 300 Hz | sidebands resolve into audible tones |

That table is the psychoacoustic backbone of the temporal-DSP material in section 6, because every repeat rate is a modulation rate.

And roughness carries no fixed valence. The same technical reference that supplies those numbers states that modulated sounds command more attention than unmodulated ones but are judged annoying only when the sound is unwanted, and are not experienced as annoying when the listener wants the information the modulation carries. Roughness in an overdriven guitar, a detuned pad, and a failing bearing is one sensation with three meanings.

The general form of the discipline, held by `affective-psychoacoustics`, is:

```text
psychoacoustic property
  -> perceptual tendency
  -> contextual interpretation
  -> possible affective response
```

Every arrow can fail, and each arrow requires either a source or an explicit label saying AudioMuse is inferring.

## 5. Temporal segmentation

Between features and structure sits the question of what counts as an event. Onsets in the amplitude envelope make event boundaries available; gaps between events make grouping boundaries; and both are perceptual rather than given.

The mechanism Phase 12E leans on hardest comes from a retrieved beat-perception study whose stimuli held pitch and loudness constant: perceptual accents arise from temporal structure alone, so an onset not closely followed by another onset is heard as accented, as is the final onset of a run of two or three. Read in reverse, that says removal creates emphasis. Cutting material out accents what remains, without touching a fader.

## 6. Temporal DSP — the missing middle layer

This is the defining contribution of Phase 12E. AudioMuse described signals on one side and musical expectation on the other, and had no name for the operations in between: the ones that take continuous recorded material and cut, hold, repeat, remove, and displace it, which is most of what electronic music production and DJ performance consist of.

The family shares a target. **Every one of these operations restructures event boundaries.**

| Operation | What it does to event structure | Node |
| --- | --- | --- |
| Gating | opens and closes audibility on a schedule, imposing articulation and silence the source does not contain | `gating` |
| Chopping | divides continuous material into fragments and reassembles them | `chopping` |
| Stutter | repeats a short fragment at a rate set independently of the material | `stutter` |
| Micro-looping | shortens the repeat until the loop period is itself a perceptual variable | `micro-looping` |
| Retriggering | restarts an event before completion, truncating the envelope and adding an onset | `retriggering` |
| Dropout | removes expected material, breaking a prediction rather than adding quiet | `temporal-discontinuity` |
| Silence insertion | places absence as a positioned element with a chosen length | `silence-as-musical-material` |
| Temporal displacement | moves events off the positions a listener predicts | `temporal-displacement` |
| Granular fragmentation | schedules windowed fragments shorter than events, with density, overlap, and jitter | `granular-fragmentation` |
| Time stretching | changes duration without proportional pitch change | `time-stretching` |
| Resampling | changes rate, coupling duration and frequency | `slowed-playback` |
| Compression | reshapes the envelope, with release acting as a rhythm | `dynamic-range-compression` |

Four properties recur across the family.

**Rate changes category, not degree.** A repeat every two seconds is a musical event; every 20 milliseconds it is a texture; shorter still it is a pitch. That is the section 4 table read as a musical control. AudioMuse marks the extension explicitly: the modulation continuum was measured for amplitude-modulated tones, and applying it to a loop of arbitrary recorded material is a hypothesis, recorded as `stutter-rate-crosses-perceptual-category-boundaries` and testable by experiment `stutter-rate-continuum`.

**Removal creates accent**, by the mechanism in section 5. This is why gating a pattern can make it feel harder with no level increase, and why an inserted silence emphasizes what surrounds it.

**A discontinuity is three events at once.** It is a spectral event, because an instantaneous amplitude step is broadband and is what a click actually is. It is a grouping boundary, because sounds separated by a gap belong to different groups. And it is a prediction failure, which is section 7.

**Exact repetition reorganizes perception.** A retrieved study showed a spoken phrase transforming perceptually into song through repetition alone, with no alteration to the signal — and, decisively for this phase, showed that the transformation did not occur when the repetitions were slightly transposed or the syllables jumbled. Exactness is part of the mechanism. That gives a principled reason to treat a digitally exact loop and a hand-performed repeat as different stimuli rather than as the same idea at different precisions.

## 7. Pattern learning, prediction, and surprise

Session 2 already gave AudioMuse the core: once a pattern repeats, the listener forms an expectation, which can be fulfilled, delayed, modified, interrupted, or violated, and composition is largely the management of that expectation.

Retrieved work adds the machinery and the quantities. Enculturation is proposed to rest on statistical learning of the regularities in the music a listener is exposed to, plus probabilistic prediction from the learned model. Two quantities fall out and are routinely conflated:

- **Information content** — how unexpected an event was, given the context. This is surprise.
- **Entropy** — how uncertain the prediction was before the event arrived. This is not knowing.

A passage can be highly uncertain and then deliver something unsurprising, or confidently predicted and then violated. Modelled information content accounted for up to 83 percent of the variance in listeners' pitch expectations across the reviewed studies, which is strong support for the prediction half.

The affective half is where care is required. High-information-content passages correlated with higher subjective and physiological arousal and lower valence. The widely repeated inverted-U proposal — too predictable is boring, too strange is unpleasant — is reported as having empirical support in some studies and not others, with the possibility that the reviewed result reflects only one side of such a curve. AudioMuse records that as unresolved rather than as a principle.

One domain does have a clean inverted U. Medium degrees of syncopation elicited the most desire to move and the most pleasure, with both extremes lower, and the proposed explanation is a balance between enough predictability for metre to be inferred and enough complexity for expectation to be violated. That is a specific result about syncopation, not a general law about surprise, and whether temporal-DSP operations sit on that same curve is an open AudioMuse question.

Two limits belong here permanently. The reviewed prediction results are correlational. And the model they rest on cannot process timbre, dynamics, or texture — which is most of what a producer manipulates, and precisely the region section 6 operates in.

## 8. Harmony, consonance, and dissonance

AudioMuse holds two nodes on each side of this question because the evidence will not support one.

**Consonance has several partly independent contributors.** Spectral fusion is one: simple-integer-ratio intervals fuse more strongly, and this was shown for both Western listeners and native Amazonian listeners with limited exposure to Western music, with the octave fusing most strongly in both groups. Peripheral interaction is another: components within a critical bandwidth produce roughness, which depends on the actual spectrum rather than on the interval's name. Learned tonal expectation is a third, and it is a property of the listener.

**And fusion does not predict pleasantness.** In the same study, the octave was the most fused interval but not the most pleasant among Western listeners who showed robust consonance preferences, and individual differences in fusion showed no correlation with consonance preference. The Amazonian listeners showed the same fusion pattern with no consonance preference at all. The authors' conclusion is the one AudioMuse adopts: universal perceptual biases exist and may partially constrain musical systems, but they shape aesthetic responses only indirectly, and aesthetic response tracks what is prevalent in the system a listener has experienced.

So the claim that a chord type carries an emotion is not merely unproven; the closest available perceptual candidate has been shown to come apart from the aesthetic judgement.

What replaces the lookup table is context. The same chord, at a different tempo, in a different register, with a different timbre, in a different orchestration, after a different preceding passage, is a different musical event. That is not a hedge, it is the structure the layers above and below this section describe: register interacts with instrument identity in affective ratings, spectral crowding changes with voicing, and a chord's function depends on the expectation the preceding bars established.

## 9. Melody, timbre, and dynamics

**Timbre.** Distinct combinations of audio descriptors relate to distinct affective dimensions rather than one quality of sound carrying one feeling. In a retrieved study of isolated instrument tones, valence was rated more positive with lower spectral slopes, greater emergence of strong partials, and sharper attack with earlier decay; tension arousal was higher with brighter sounds, more spectral variation, and gentler attacks; energy arousal was higher with brighter sounds and higher spectral centroids. Notice that brightness appears in two dimensions in different roles and attack appears in two with opposite signs. A single expressiveness dial would discard exactly the structure that is there. The stimuli were isolated tones and the authors say the relationships need validation in musical context.

**Dynamics.** Sudden dynamic change is the one purely dynamic property that appears repeatedly across this literature: it is named as a brain stem reflex trigger, described as sounds that are sudden or loud, and reported across nearly all reviewed frisson studies as a major catalyst. Section 6's discontinuities are dynamic changes made by removal rather than by addition, which is why a return after a dropout hits harder than its level accounts for.

**Melody and contour** are the thinnest part of this phase. AudioMuse retrieved no source specifically on melodic contour and affect, and this document records that as a gap rather than filling it. What the prediction material does supply is that melodic expectation is modelled well by learned statistics, which places contour effects under section 7 rather than under a separate rule.

## 10. Space

`perceived-space` is a seed node and the reason is stated in it: Phase 12E set out to cover space and emotion and retrieved no citable work linking reverberation, distance cues, or stereo width to affective response at the standard the rest of this phase is held to.

What is supported is the inference structure. Direction comes largely from interaural time and level differences, with the timing comparison depending on the phase-locking that operates up to about 3 kHz. Distance and enclosure come from the relationship between direct and reflected sound. All of it is inferred rather than sensed, which is why it can be constructed artificially — a close dry vocal and the same vocal in a long hall are one performance making two claims about where the listener is standing.

One spatial effect is solidly evidenced and is not an emotion. The auditory looming bias favours approaching over receding sources, with faster and more accurate responses, and it was demonstrated by manipulating spatial spectral cues while overall intensity was held constant, so it is not simply that louder gets attention. Risers, filter sweeps, and reverb pulled toward a dry close position are structurally approach cues; whether the musical gesture inherits the perceptual bias is untested, and experiment `apparent-space-and-affect` proposes how to find out.

## 11. Memory

A particular recording can produce a strong response when none of its low-level features predicts one, because of what the listener was doing when they heard it.

Two mechanisms are named in the framework. Evaluative conditioning is repeated pairing until the music alone carries the valence, described as subconscious and effortless. Episodic memory is the music cueing a specific remembered event, with that event's emotion arriving alongside it.

Retrieved imaging work gives the pairing a correlate and, more usefully, a separation: medial prefrontal cortex responded parametrically to both familiarity and autobiographical salience, with salience producing effects beyond what familiarity alone predicted, and musical structure and personal meaning were modelled independently and operated on different timescales.

The consequence is a standing qualification on everything else in this document, and a hard design constraint: **any listening test that does not control familiarity is measuring an unknown mixture.** Familiarity turns up as a moderator in the neural-tracking work and in the frisson literature as well.

## 12. Culture

AudioMuse takes neither of the two available slogans.

Against pure universality: aesthetic response tracks the musical system a listener has been exposed to, demonstrated by listeners who showed a perceptual regularity without the corresponding preference. And the reward literature itself states that the rewarding nature of aesthetic stimuli is not entirely universal, differing across cultures and between individuals within them.

Against pure culture: the fusion pattern was shared across groups with radically different musical exposure, and the encoding layer in section 3 is not culturally variable.

The layers coexist. Biology supplies constraints and tendencies; exposure supplies the statistics; aesthetic response sits with the statistics. An open question that AudioMuse should not paper over is what happens to listeners exposed to more than one musical system during enculturation — which is the ordinary condition of most listeners alive now, and which the reviewed work names as unresolved.

## 13. Appraisal, reward, arousal, and autonomic response

**Reward.** Auditory cortical regions showed increased functional interaction with the nucleus accumbens when musical sequences carried high rather than low reward value, which is a claim about a relationship between two levels of this stack rather than about a location. And anticipation and consummation are separable: dorsal striatal activity during the anticipation phase preceding chills, nucleus accumbens activity maximal during the chills. For a producer that is the formal counterpart of something obvious in practice — the bar before the drop is doing work, and it is not the same work as the drop.

**Frisson** is the marker most of this literature runs on, and its limits should be visible. Reported triggers include sudden dynamic leaps, unexpected harmony, modulation, and melody in the vocal register. Its terminology lacks consensus; authors disagree over whether piloerection is definitional; the samples are overwhelmingly Western classical and student participants; and the direction of causation has generally been assumed rather than tested — finding more frisson during sad music does not rule out the musical attributes of sad music eliciting both concurrently.

**Autonomic response is not emotion.** This is the single rule Phase 12E enforces most strictly. Experiential, physiological, and behavioural measures carry unique variance, converge only weakly, and no gold-standard measure exists; facial EMG is sensitive to valence, skin conductance to arousal. Even within a framework built to explain feeling, arousal is defined as autonomic activation that can occur without any emotion at all.

## 14. Subjective emotion

At the top of the stack sits what the listener would report, and the layers below constrain it without determining it. The frameworks are explicit that mechanisms respond to a musical event comprising music, listener, and context together; that music arouses emotion in only a majority of listening episodes with wide individual differences; and that perceiving an emotion in music and feeling one in response to it are different things.

The last distinction has teeth. Most machine-audio work targets perceived emotion because it is less influenced by situational factors, so an emotion label attached to a track by a system is a prediction about what annotators would say the music expresses — not a report of what anyone felt.

## 15. Measurement

Four columns, kept separate. Nothing from one may be reported as though it came from another.

| Column | Examples | What it describes |
| --- | --- | --- |
| **Signal** | waveform, RMS, peak, programme loudness by a standardized algorithm, spectral centroid, spectral flux, roughness estimates, harmonicity, onset rate, inter-onset intervals, tempo, dynamic range, modulation rate, reverberation metrics, spatial features | the stimulus |
| **Behavioural** | valence, arousal, tension, expectancy, predictability, wanting-to-move, pleasantness, similarity ratings; reaction time; tapping synchronization | what a listener did or said |
| **Physiological** | heart rate, heart-rate variability, respiration, skin conductance, pupil response, movement, facial EMG, piloerection | what a listener's body did |
| **Neural** | EEG, MEG, fMRI, intracranial recording | discussed conceptually only |

Signal measurement is the only column AudioMuse can produce deterministically, and it describes no listener. One retrieved result shows why the choice of descriptor is a real decision: neural synchronization tracked the spectral flux of music better than its amplitude envelope, and both are obvious candidates for describing the same rhythmic content.

Neural methods are how most of the findings cited in this document were produced. AudioMuse reads that literature and does not become an instrumentation project.

## 16. AudioMuse experiments

Seven specifications were added in Phase 12E. All are `proof` status: they prove the repository contract, and none has been performed.

| Experiment | Manipulates | Measures |
| --- | --- | --- |
| `temporal-gating-and-expectation` | gate rate, duty cycle, regularity, dropout placement | tension, predictability, wanting-to-move, arousal |
| `stutter-rate-continuum` | repeat period from 1000 ms to 2 ms | perceptual category at each rate |
| `harmonicity-versus-roughness` | partial spacing, modulation rate, register | fusion, roughness, pleasantness — separately |
| `expectation-violation-endings` | final event expected, delayed, omitted, harmonically unexpected, displaced | surprise, tension, expectedness, liking |
| `timbre-emotion-separation` | attack, decay, brightness, distortion, noise, notes held constant | valence, energy arousal, tension arousal |
| `apparent-space-and-affect` | distance, decay, early reflections, width, approach direction | distance, size, envelopment, then valence and arousal |
| `chopped-temporal-reconstruction` | regular and irregular chopping, retriggering, stutter, reversal, inserted silence | predictability, wanting-to-move, tension, recognition, liking |

Three design rules run through all of them. Level-match, because an uncontrolled loudness difference dominates every rating. Control familiarity, because it moderates results across several literatures. And write the rating scales before listening, because a scale chosen after hearing the material is not a measure.

## 17. DJ, turntablism, and production applications

Phase 12E connects backward into the DJ and Houston material AudioMuse already holds, because the operations are the same and the timing signatures are not.

**Scratching** manipulates playback speed and direction, which changes transient order, spectral content, and rhythmic placement at once. `scratching --used_in--> gating` records that the crossfader half of the gesture is amplitude gating, performed by hand.

**Crossfader gating** is `gating` with a hand as the control signal, and it produces the same duty-cycle and onset-density consequences a plugin does.

**Beat juggling** reorganizes recorded time and thereby manipulates prediction; it is `chopping` and `temporal-displacement` performed simultaneously against two moving records.

**Chopped and screwed practice** is where Phase 12E has to be most careful. The mechanisms it touches are real and now named — sustained rate reduction moving register, envelope timing, and spectral distribution together; repetition of beats, words, and phrases; retriggering; phrase interruption; altered groove; changed vocal identity. Section 6's whole argument applies to it directly, and the claim `sustained-rate-reduction-moves-several-affective-variables-together` is written specifically so that no single one of those changes can be credited with the result.

But the practice is not a DSP case study, and this document does not treat it as one. It has a history, a city, an economy, and people, held in `docs/houston-musical-cartography.md` and in the `history-culture` domain, and Phase 12E did not rewrite any of it. What Phase 12E adds is a vocabulary for the operations. What it deliberately does not do is explain the music by them.

**Compression** belongs here rather than only in mixing, because release time is a rhythm. The recovery after each gain reduction produces a periodic level contour at the rate of whatever triggers it, which places pumping in the same modulation territory as every other periodic envelope variation. AudioMuse has retrieved no source on listener response to compression settings, so that half is recorded as a hypothesis.

**Distortion** generates new partials, which raises roughness and sensory dissonance without changing a single note. It is not inherently aggressive; it is inherently spectrally denser, and the aggression is a contextual reading.

## 18. Machine audio

Future AudioMuse work could compute spectral, temporal, rhythmic, harmonic, dynamic, spatial, and structural features, and could build an emotion-feature inspector, a tension analyzer, a temporal-DSP visualizer, a stutter and gating analysis tool, an expectation-event annotation system, or an affective feature database. Phase 12E deliberately builds none of them.

What it does establish is a boundary. Music emotion recognition systems are trained on ground truth collected by subjective test, often averaged across a modest number of annotators, and target perceived emotion. What such a system outputs is a prediction of how annotators would have labelled a track. It does not detect feeling, it does not measure a listener, and no AudioMuse material may describe one as experiencing emotion.

## 19. What Phase 12E did not settle

Recorded plainly, because a foundation that hides its gaps is worse than a smaller one that does not.

- **Space and affect.** No retrievable source was found linking reverberation, distance, or width to affective response. `perceived-space` is a seed and experiment `apparent-space-and-affect` is a proposal with no prediction behind its affective half.
- **Melody and contour.** No source retrieved. The section above says so rather than improvising a rule.
- **Compression and distortion and listener response.** No retrieved source measures either. Both are carried as hypotheses with their signal-level reasoning stated and their affective half marked unsourced.
- **The central auditory pathway above the nerve.** Brainstem nuclei, inferior colliculus, medial geniculate, and cortex are named without detail, and `auditory-pathway` is a seed for that reason.
- **Classic monographs.** Meyer, Huron, Bregman, Zwicker and Fastl, Roads, and Blauert dominate this literature and are **not** registered as AudioMuse sources, because they were not read. Where their content matters it appears as an attributed statement about what a retrieved source says about them.
- **Neural entrainment terminology.** Whether the neural synchronization reported in this literature reflects oscillatory entrainment or repeated evoked responses is contested, the authors of the retrieved study decline to take a position, and AudioMuse does not resolve it on their behalf.
- **Whether temporal-DSP operations sit on the syncopation curve.** The inverted U was demonstrated for syncopation. Whether chopping, gating, and stutter fall on that curve, on a different one, or off it entirely is untested, and experiment `chopped-temporal-reconstruction` exists to start finding out.
- **Every hypothesis in the temporal-DSP claim set.** The stutter continuum, the micro-loop periodicity, the dropout proportionality, and the compression-release proposal are proposals. They are typed `hypothesis`, graded `low`, and carry open questions.

## 20. Cross-domain paths this phase makes traversable

```text
frequency -> pitch -> melody -> expectation -> emotion
phase -> interference -> timbre and spatial image -> interpretation
envelope -> articulation -> event segmentation -> rhythm -> expectation
repetition -> pattern learning -> prediction -> tension and release
sampling -> chopping -> temporal reconstruction -> expectation manipulation
scratching -> transport manipulation -> transient, pitch, and timing change -> articulation
compression -> envelope modification -> groove -> perceived intensity
reverb -> spatial inference -> distance and scale -> context
```

Each of those is now a chain of existing nodes joined by typed relationships, which is what makes it a knowledge graph rather than a reading list. The success condition Phase 12E set itself was to trace a defensible path from vibration through the cochlea, neural encoding, psychoacoustic features, temporal organization, gating and chopping and stutter and repetition, prediction, musical structure, memory and culture, and bodily response, to emotion — **without pretending that any one layer explains the whole.** That path exists. Most of it is marked with what it does not yet know.
