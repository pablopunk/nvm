import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  BufferTarget,
  EncodedAudioPacketSource,
  EncodedPacket,
  EncodedPacketSink,
  EncodedVideoPacketSource,
  Input,
  BufferSource,
  Output,
  WEBM,
  WebMOutputFormat,
  type AudioCodec,
} from 'mediabunny';
import {
  InvalidTranscriptionAudioError,
  normalizeTranscriptionAudio,
  TRANSCRIPTION_AUDIO_LIMITS,
  type NormalizedTranscriptionAudio,
} from './audio-workload';

// Standard libopus 20ms silence packet. Fixtures use the maintained muxer, not an EBML builder.
const SILENCE = new Uint8Array([0xf8, 0xff, 0xfe]);

function opusHeader(channels: number, preSkip: number): Uint8Array {
  const header = Buffer.alloc(19);
  header.write('OpusHead');
  header[8] = 1;
  header[9] = channels;
  header.writeUInt16LE(preSkip, 10);
  header.writeUInt32LE(48_000, 12);
  return header;
}

interface FixtureOptions {
  codec?: AudioCodec;
  packets?: number;
  channels?: number;
  preSkip?: number;
  tracks?: number;
  video?: boolean;
  appendOnly?: boolean;
  description?: Uint8Array;
  lastPacket?: Uint8Array;
  lastTimestamp?: number;
  lastDuration?: number;
}

async function makeWebm(options: FixtureOptions = {}): Promise<Uint8Array> {
  const channels = options.channels ?? 1;
  const count = options.packets ?? 2;
  const target = new BufferTarget();
  const output = new Output({
    target,
    format: new WebMOutputFormat({ appendOnly: options.appendOnly ?? true }),
  });
  const sources: EncodedAudioPacketSource[] = [];
  for (let track = 0; track < (options.tracks ?? 1); track++) {
    const source = new EncodedAudioPacketSource(options.codec ?? 'opus');
    sources.push(source);
    output.addAudioTrack(source);
  }
  const video = options.video ? new EncodedVideoPacketSource('vp8') : undefined;
  if (video) output.addVideoTrack(video);
  await output.start();
  for (let packet = 0; packet < count; packet++) {
    for (const source of sources) {
      const last = packet === count - 1;
      await source.add(
        new EncodedPacket(
          last ? (options.lastPacket ?? SILENCE) : SILENCE,
          'key',
          last ? (options.lastTimestamp ?? packet * 0.02) : packet * 0.02,
          last ? (options.lastDuration ?? 0.02) : 0.02,
        ),
        packet === 0
          ? {
              decoderConfig: {
                codec: options.codec ?? 'opus',
                numberOfChannels: channels,
                sampleRate: 48_000,
                description:
                  options.description ??
                  opusHeader(channels, options.preSkip ?? 0),
              },
            }
          : undefined,
      );
    }
  }
  if (video) {
    await video.add(new EncodedPacket(new Uint8Array([0]), 'key', 0, 0.04), {
      decoderConfig: { codec: 'vp8', codedWidth: 16, codedHeight: 16 },
    });
  }
  await output.finalize();
  assert.ok(target.buffer);
  return new Uint8Array(target.buffer);
}

function assertCanonicalWav(
  result: NormalizedTranscriptionAudio,
  channels: number,
  samples: number,
) {
  assert.equal(result.inputAudio.format, 'wav');
  const wav = Buffer.from(result.inputAudio.data, 'base64');
  assert.equal(wav.toString('ascii', 0, 4), 'RIFF');
  assert.equal(wav.toString('ascii', 8, 16), 'WAVEfmt ');
  assert.equal(wav.readUInt32LE(4), wav.length - 8);
  assert.equal(wav.readUInt32LE(16), 16);
  assert.equal(wav.readUInt16LE(20), 1);
  assert.equal(wav.readUInt16LE(22), channels);
  assert.equal(wav.readUInt32LE(24), 48_000);
  assert.equal(wav.readUInt32LE(28), 48_000 * channels * 2);
  assert.equal(wav.readUInt16LE(32), channels * 2);
  assert.equal(wav.readUInt16LE(34), 16);
  assert.equal(wav.toString('ascii', 36, 40), 'data');
  assert.equal(wav.readUInt32LE(40), samples * channels * 2);
  assert.equal(wav.length, 44 + samples * channels * 2);
  assert.equal(result.durationSeconds, samples / 48_000);
  assert.equal(result.estimatedBillableSeconds, Math.ceil(samples / 48_000));
}

async function assertRejected(
  bytes: Uint8Array,
  reason: 'invalid_audio' | 'resource_limit',
) {
  await assert.rejects(
    normalizeTranscriptionAudio(bytes),
    function isTypedInvalidAudio(error: unknown) {
      return (
        error instanceof InvalidTranscriptionAudioError &&
        error.reason === reason
      );
    },
  );
}

test('canonical mono and stereo WAV workload equals outgoing sample count', async () => {
  for (const channels of [1, 2]) {
    assertCanonicalWav(
      await normalizeTranscriptionAudio(await makeWebm({ channels })),
      channels,
      1920,
    );
  }
});

test('Opus pre-skip is honored and zero-sample initial packets are not treated as decode errors', async () => {
  assertCanonicalWav(
    await normalizeTranscriptionAudio(await makeWebm({ preSkip: 1200 })),
    1,
    720,
  );
});

test('a final packet with zero container duration still contributes decoded samples', async () => {
  const bytes = await makeWebm({ packets: 1, lastDuration: 0 });
  const input = new Input({ source: new BufferSource(bytes), formats: [WEBM] });
  try {
    const track = (await input.getAudioTracks())[0];
    const packet = await new EncodedPacketSink(track).getFirstPacket();
    assert.ok(packet);
    assert.equal(packet.duration, 0);
  } finally {
    input.dispose();
  }
  assertCanonicalWav(await normalizeTranscriptionAudio(bytes), 1, 960);
});

test('large timestamp gaps do not synthesize billable silence', async () => {
  assertCanonicalWav(
    await normalizeTranscriptionAudio(await makeWebm({ lastTimestamp: 3600 })),
    1,
    1920,
  );
});

test('declared file duration does not override canonical sample duration', async () => {
  assertCanonicalWav(
    await normalizeTranscriptionAudio(
      await makeWebm({ appendOnly: false, lastTimestamp: 3600 }),
    ),
    1,
    1920,
  );
});

test('whole seconds are rounded up without adding a provider-specific minimum', async () => {
  assertCanonicalWav(
    await normalizeTranscriptionAudio(await makeWebm({ packets: 50 })),
    1,
    48_000,
  );
  assertCanonicalWav(
    await normalizeTranscriptionAudio(await makeWebm({ packets: 51 })),
    1,
    48_960,
  );
});

test('normalization does not impose a new sixty-second clip limit', async () => {
  assertCanonicalWav(
    await normalizeTranscriptionAudio(await makeWebm({ packets: 3001 })),
    1,
    2_880_960,
  );
});

test('multiple audio tracks and audio with video are rejected rather than silently dropped', async () => {
  await assertRejected(await makeWebm({ tracks: 2 }), 'invalid_audio');
  await assertRejected(await makeWebm({ video: true }), 'invalid_audio');
  await assertRejected(
    await makeWebm({ video: true, tracks: 0 }),
    'invalid_audio',
  );
});

test('a non-Opus audio track is rejected before decoding', async () => {
  await assertRejected(await makeWebm({ codec: 'vorbis' }), 'invalid_audio');
});

test('unsupported channel layouts, mismatched Opus headers and mapping families fail closed', async () => {
  await assertRejected(await makeWebm({ channels: 3 }), 'invalid_audio');
  await assertRejected(
    await makeWebm({ description: opusHeader(2, 0) }),
    'invalid_audio',
  );
  const mappedHeader = opusHeader(1, 0);
  mappedHeader[18] = 1;
  await assertRejected(
    await makeWebm({ description: mappedHeader }),
    'invalid_audio',
  );
});

test('empty, non-WebM and truncated headers fail with a typed error', async () => {
  for (const bytes of [
    new Uint8Array(),
    Buffer.from('not audio'),
    new Uint8Array([0x1a, 0x45, 0xdf, 0xa3]),
  ]) {
    await assertRejected(bytes, 'invalid_audio');
  }
});

test('a track with no samples after pre-skip is not forwarded', async () => {
  await assertRejected(await makeWebm({ preSkip: 65_535 }), 'invalid_audio');
});

test('the normalization deadline rejects before returning a payload', async (context) => {
  const bytes = await makeWebm();
  context.mock.timers.enable({ apis: ['setTimeout'] });
  const result = normalizeTranscriptionAudio(bytes);
  context.mock.timers.tick(TRANSCRIPTION_AUDIO_LIMITS.processingMs);
  await assert.rejects(result, function isResourceLimit(error: unknown) {
    return (
      error instanceof InvalidTranscriptionAudioError &&
      error.reason === 'resource_limit'
    );
  });
});

test('a decode error in the last packet rejects the entire clip, not a partial WAV', async () => {
  // TOC requests two equal-size frames, but the payload cannot be divided equally.
  await assertRejected(
    await makeWebm({ lastPacket: new Uint8Array([0xf9, 0]) }),
    'invalid_audio',
  );
});

test('oversized compressed packets are rejected before entering the fixed WASM input allocation', async () => {
  await assertRejected(
    await makeWebm({ lastPacket: new Uint8Array(3841) }),
    'invalid_audio',
  );
});

test('compressed input memory is limited before demuxing', async () => {
  await assertRejected(
    new Uint8Array(TRANSCRIPTION_AUDIO_LIMITS.inputBytes + 1),
    'resource_limit',
  );
});

test('highly compressible audio cannot produce an unbounded PCM payload', async () => {
  const packets =
    Math.floor(TRANSCRIPTION_AUDIO_LIMITS.decodedBytes / (960 * 2)) + 1;
  const bytes = await makeWebm({ packets });
  assert.ok(bytes.byteLength < TRANSCRIPTION_AUDIO_LIMITS.inputBytes);
  await assertRejected(bytes, 'resource_limit');
});
