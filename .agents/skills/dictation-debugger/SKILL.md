---
name: dictation-debugger
description: Use when dictation transcribes wrong, slow, or loses words (missing sentences, stray periods, cut endings), when comparing Nevermind's dictation with Hex, or when measuring dictation latency. Covers pulling a failing recording off the Mac, replaying it through the real pipeline with cmd/dictbench, benchmarking cloud and on-device, and checking how Hex does it.
---

# Dictation Debugger

Start from the audio, not the code. A transcript that lost words is either audio that never arrived or audio the model
dropped, and the saved recording tells them apart in minutes. Read `docs/solutions/dictation-*.md` first; the known
failures and their contracts are there.

## How dictation works

- Capture: the macOS Swift helper streams 16 kHz mono PCM; the Go service encodes 20 ms Opus frames, keeps the audio
  on disk from the first frame (`dictation-recordings`), and, after the text is in, moves it to `dictation-recent`
  (the last 5).
- Cloud (`engine: cloud`): live pieces cut at pauses (5 s minimum, 30 s maximum), each opening with the last 3 s of the
  one before, sent while the user speaks and stitched where the repeated words match (`stitch.go`).
- On device (`engine: device`): Hex's engine. A child process (`--nvm-speech-child`) loads transcribe.cpp
  (`resources/lib/libtranscribe.dylib`) and Hex's GGUF (`<data>/models/parakeet-tdt-0.6b-v3-gguf`) when the
  microphone opens, reads the raw samples whole at Stop, checks the end, and exits 30 s after the last use.
- Stop logs `dictation.capture-gap` when the audio is over a second shorter than the time the dictation was held.

## 1. Get the evidence from the Mac

The Mac is the Tailscale host `m4pro`, user `pablopunk` (plain `ssh m4pro` logs in as `pol`, who cannot reach the app's
data). Tailscale SSH often asks for an approval link: run ssh without BatchMode in the background, send the user the
link, and wait for it to connect.

```sh
data="Library/Application Support/Nevermind"
ssh pablopunk@m4pro "cd '$data' && ls -lt dictation-recent | head; grep -h 'capture-gap' logs/nvm.log | tail -3"
scp "pablopunk@m4pro:$data/dictation-recent/<id>.*" ./rec/
```

The history text is in `extension-data/nevermind.dictation/storage.json` (`data.values.history`, and
`data.values.settings.engine`). Compare it with what the user says they said, and note each segment's length.

## 2. Replay it

Go runs on the test box, never on the Mac (`src/native/scripts/boxgo.sh <slot> '<cmd>'`). The box needs the model and
the library once:

```sh
# on the box: Hex's GGUF (pinned in internal/dictation/devicemodel/parakeet-v3.json) and transcribe.cpp
mkdir -p /root/models/parakeet-tdt-0.6b-v3-gguf && cd /root/models/parakeet-tdt-0.6b-v3-gguf
curl -fLO <baseURL>/parakeet-tdt-0.6b-v3-Q8_0.gguf      # check its sha256 against the manifest
bash src/native/scripts/natives/build-transcribe.sh    # writes resources/linux-amd64/lib/libtranscribe.so
```

Then, from `src/native`, with the recording's files in a folder on the box:

```sh
go build -o /root/dictbench/tc ./cmd/dictbench
./tc -device /root/models -lib <libtranscribe.so> -recording <id> <folder>      # whole reading, then each segment alone
./tc -recording <id> -dump-wav out.wav <folder>                                 # the audio, for other tools
./tc -parakeet /root/models/parakeet-tdt-0.6b-v3-gguf -lib <lib> clip.wav       # the model alone, in process
```

If the segments alone hold the missing words and the whole reading drops them, the model dropped them; if every
reading lacks them, the audio did (check `capture-gap`). Vary one thing at a time: trailing silence, the window
length, the engine.

## 3. Compare with Hex

Hex (github.com/anomalyco/hex) is the reference for on-device behaviour. Clone it shallow and read
`src/transcription/models.rs` (model table, `streams`), `src/transcription/artifacts.rs` (pinned GGUFs),
`src/transcription/gguf_session.rs` (chunking, trimming) and `src/dictation.rs` (release tail, padding). Its engine is
transcribe.cpp at the commit pinned in Hex's `Cargo.toml`; its `transcribe-cli -m model.gguf audio.wav` reproduces
Hex's output on a WAV from `-dump-wav`.

## 4. Measure latency

```sh
./tc -token-file <file with an nvm_pat_ token> -runs 3 jfk.wav mm.wav jfk.wav   # cloud, against production
./tc -device /root/models -lib <lib> -runs 3 jfk.wav mm.wav jfk.wav             # on device, through the service
```

It prints release-to-text per run and each backend request's start and duration. Ask the user for a token with
`request_secret`; copy it to the box only for the run and delete it after. Public-domain clips work well:
`jfk.wav` from whisper.cpp's samples and OpenAI's `micro-machines.wav` (fast speech, few pauses).

## 5. Ship a build to the Mac

A PR's "Native packages" run uploads `native-macos-arm64`; install it on `m4pro` as `pablopunk`: quit the app,
`ditto` the new bundle beside `/Applications/Nevermind.app`, swap, reopen. To go back to the dev channel, install the
`darwin-arm64.tar.gz` of the newest `-dev` release (start `dev-build.yml` if it predates the fix) the same way.
