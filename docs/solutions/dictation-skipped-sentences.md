# Dictation Skipped Sentences

**Status: Fixed and validated on the Mac** (2026-11-25, #321)

## Problem

Long on-device dictations lost words with no error: sometimes the last sentence or two, sometimes one sentence in the
middle with the words after it kept ("…shortcut to tile. optional tidiness." for "…shortcut to tile. So it feels like
freedom with optional tidiness."). Dictation History had the same cut text, so paste was not the cause.

## Root cause

The audio was complete; the model dropped the speech. Our ONNX export of Parakeet TDT 0.6B v3 runs full attention over
the whole input, and on real speech it silently skips sentences past about half a minute (a 60 s piece of one
dictation lost four sentences in the middle). Cutting the audio into 30 s pieces (#313) only made it rarer.

Hex runs the same model on transcribe.cpp with the Q8_0 GGUF. transcribe.cpp switches a full-attention Parakeet
checkpoint to NeMo's long-audio local attention (`rel_pos_local_attn`, 256 frames each side, about 20 s), the mode
NVIDIA documents for long audio, and Hex reads every dictation whole with no length limit.

Even there, the last sentence of a recording is unstable: on one recording, a few hundred milliseconds more or less of
trailing silence kept or dropped "End of communication."

## Contract

- On-device dictation runs Hex's engine and model: transcribe.cpp at the commit Hex pins
  (`scripts/natives/build-transcribe.sh`) and the GGUF Hex downloads, read whole from the raw microphone samples, never
  cut and never Opus-decoded first.
- A dictation longer than the end check reads its last stretch again from a pause and adds the words the whole
  reading left off. A reading that already has them is unchanged.
- Cloud pieces stay at most 30 s, since the hosted route reads them with full attention.
- The last few transcribed dictations keep their audio (`dictation-recent`), so a bad transcript can be replayed.

## Verification

- The two failing dictations, replayed through the service, the child process and transcribe.cpp, came back complete.
- `internal/dictation` covers the whole reading, the end check and the cloud piece cap.
- The macOS package job checks that `libtranscribe.dylib` is bundled, arm64, and exports `transcribe_run`.

## Debugging next time

Use the `dictation-debugger` skill: pull the failing recording, replay it, and compare engines before changing code.
