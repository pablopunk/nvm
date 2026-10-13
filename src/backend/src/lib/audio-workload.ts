import { setImmediate } from 'node:timers/promises';
import {
  BufferSource,
  EncodedPacketSink,
  Input,
  InputAudioTrack,
  WEBM,
} from 'mediabunny';
import { OpusDecoder } from 'opus-decoder';

const configuredInputBytes = Number(
  process.env.AUDIO_TRANSCRIPTION_MAX_BYTES ?? 4 * 1024 * 1024,
);

export const TRANSCRIPTION_AUDIO_LIMITS = {
  inputBytes: Math.max(
    64 * 1024,
    Number.isSafeInteger(configuredInputBytes)
      ? configuredInputBytes
      : 4 * 1024 * 1024,
  ),
  decodedBytes: 64 * 1024 * 1024,
  packets: 100_000,
  processingMs: 10_000,
} as const;

type InvalidAudioReason = 'invalid_audio' | 'resource_limit';

export class InvalidTranscriptionAudioError extends Error {
  constructor(
    readonly reason: InvalidAudioReason,
    options?: ErrorOptions,
  ) {
    super(
      reason === 'resource_limit'
        ? 'Audio exceeds normalization resource limits.'
        : 'Audio must contain one decodable mono or stereo Opus track.',
      options,
    );
    this.name = 'InvalidTranscriptionAudioError';
  }
}

export interface NormalizedTranscriptionAudio {
  inputAudio: { data: string; format: 'wav' };
  durationSeconds: number;
  /** Admission estimate, not a guarantee about upstream minimums or settlement. */
  estimatedBillableSeconds: number;
}

function readOpusHeader(
  description: AllowSharedBufferSource | undefined,
  channels: number,
) {
  if (!description) throw new InvalidTranscriptionAudioError('invalid_audio');
  const header = ArrayBuffer.isView(description)
    ? new Uint8Array(
        description.buffer,
        description.byteOffset,
        description.byteLength,
      )
    : new Uint8Array(description);
  if (
    header.length !== 19 ||
    !Buffer.from(header.subarray(0, 8)).equals(Buffer.from('OpusHead')) ||
    header[8] !== 1 ||
    header[9] !== channels ||
    header[18] !== 0
  ) {
    throw new InvalidTranscriptionAudioError('invalid_audio');
  }
  const view = new DataView(
    header.buffer,
    header.byteOffset,
    header.byteLength,
  );
  return {
    preSkip: view.getUint16(10, true),
    gain: 10 ** (view.getInt16(16, true) / (256 * 20)),
  };
}

function encodePcmChunk(
  channelData: Float32Array[],
  samples: number,
  gain: number,
): Buffer {
  const channels = channelData.length;
  if (
    channelData.some(function hasWrongLength(channel) {
      return channel.length !== samples;
    })
  ) {
    throw new InvalidTranscriptionAudioError('invalid_audio');
  }
  const pcm = Buffer.allocUnsafe(samples * channels * 2);
  for (let sample = 0; sample < samples; sample++) {
    for (let channel = 0; channel < channels; channel++) {
      const value = channelData[channel][sample] * gain;
      if (!Number.isFinite(value))
        throw new InvalidTranscriptionAudioError('invalid_audio');
      const clipped = Math.max(-1, Math.min(1, value));
      pcm.writeInt16LE(
        Math.round(clipped * (clipped < 0 ? 32768 : 32767)),
        (sample * channels + channel) * 2,
      );
    }
  }
  return pcm;
}

function encodeWav(chunks: Buffer[], bytes: number, channels: number): Buffer {
  const wav = Buffer.allocUnsafe(44 + bytes);
  wav.write('RIFF', 0);
  wav.writeUInt32LE(36 + bytes, 4);
  wav.write('WAVEfmt ', 8);
  wav.writeUInt32LE(16, 16);
  wav.writeUInt16LE(1, 20);
  wav.writeUInt16LE(channels, 22);
  wav.writeUInt32LE(48_000, 24);
  wav.writeUInt32LE(48_000 * channels * 2, 28);
  wav.writeUInt16LE(channels * 2, 32);
  wav.writeUInt16LE(16, 34);
  wav.write('data', 36);
  wav.writeUInt32LE(bytes, 40);
  let offset = 44;
  for (const chunk of chunks) offset += chunk.copy(wav, offset);
  return wav;
}

/**
 * Decodes WebM into the exact PCM payload whose sample count defines the workload.
 * Packets are concatenated in decode order: untrusted timestamps never synthesize
 * silence or discard decoded content, and declared container durations are unused.
 * This validates the outgoing WAV, not every byte of the original container.
 *
 * Limits bound input, retained PCM, packet count and cooperative processing time.
 * The deadline cannot interrupt synchronous library code; it is not worker isolation.
 * Peak memory also includes demuxer state, a WAV copy and its base64 representation.
 */
export async function normalizeTranscriptionAudio(
  buffer: Uint8Array,
): Promise<NormalizedTranscriptionAudio> {
  if (!buffer.byteLength)
    throw new InvalidTranscriptionAudioError('invalid_audio');
  if (buffer.byteLength > TRANSCRIPTION_AUDIO_LIMITS.inputBytes) {
    throw new InvalidTranscriptionAudioError('resource_limit');
  }
  const input = new Input({
    source: new BufferSource(buffer),
    formats: [WEBM],
  });
  const started = performance.now();
  const limitError = new InvalidTranscriptionAudioError('resource_limit');
  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<never>(function startDeadline(_resolve, reject) {
    timer = setTimeout(function expireNormalization() {
      reject(limitError);
      input.dispose();
    }, TRANSCRIPTION_AUDIO_LIMITS.processingMs);
  });
  function checkDeadline() {
    if (performance.now() - started >= TRANSCRIPTION_AUDIO_LIMITS.processingMs)
      throw limitError;
  }
  async function bounded<T>(operation: Promise<T>): Promise<T> {
    checkDeadline();
    const result = await Promise.race([operation, deadline]);
    checkDeadline();
    return result;
  }
  let decoder: OpusDecoder<48000> | undefined;
  let decoderReady = false;
  try {
    const tracks = await bounded(input.getTracks());
    const track = tracks[0];
    if (
      tracks.length !== 1 ||
      !(track instanceof InputAudioTrack) ||
      (await bounded(track.getCodec())) !== 'opus'
    ) {
      throw new InvalidTranscriptionAudioError('invalid_audio');
    }
    const channels = await bounded(track.getNumberOfChannels());
    if (channels !== 1 && channels !== 2)
      throw new InvalidTranscriptionAudioError('invalid_audio');
    const config = await bounded(track.getDecoderConfig());
    const { preSkip, gain } = readOpusHeader(config?.description, channels);
    const activeDecoder = new OpusDecoder<48000>({
      channels,
      sampleRate: 48_000,
      preSkip,
    });
    decoder = activeDecoder;
    await bounded(activeDecoder.ready);
    decoderReady = true;
    const sink = new EncodedPacketSink(track);
    const chunks: Buffer[] = [];
    let bytes = 0;
    let packets = 0;
    // Fetch one packet at a time rather than using the iterator's preloading queue.
    let packet = await bounded(sink.getFirstPacket());
    while (packet) {
      checkDeadline();
      if (++packets > TRANSCRIPTION_AUDIO_LIMITS.packets) throw limitError;
      // The decoder's fixed input allocation is 3,840 bytes per channel.
      if (!packet.data.length || packet.data.length > 3840 * channels) {
        throw new InvalidTranscriptionAudioError('invalid_audio');
      }
      const decoded = activeDecoder.decodeFrame(packet.data);
      if (
        decoded.errors.length ||
        decoded.sampleRate !== 48_000 ||
        decoded.channelData.length !== channels ||
        !Number.isSafeInteger(decoded.samplesDecoded) ||
        decoded.samplesDecoded < 0 ||
        decoded.samplesDecoded > 5760
      ) {
        throw new InvalidTranscriptionAudioError('invalid_audio');
      }
      const chunkBytes = decoded.samplesDecoded * channels * 2;
      if (bytes + chunkBytes > TRANSCRIPTION_AUDIO_LIMITS.decodedBytes)
        throw limitError;
      if (chunkBytes) {
        chunks.push(
          encodePcmChunk(decoded.channelData, decoded.samplesDecoded, gain),
        );
        bytes += chunkBytes;
      }
      if (packets % 64 === 0) await bounded(setImmediate());
      packet = await bounded(sink.getNextPacket(packet));
    }
    if (!bytes) throw new InvalidTranscriptionAudioError('invalid_audio');
    checkDeadline();
    const wav = encodeWav(chunks, bytes, channels);
    const data = wav.toString('base64');
    checkDeadline();
    const durationSeconds = bytes / (channels * 2 * 48_000);
    return {
      inputAudio: { data, format: 'wav' },
      durationSeconds,
      estimatedBillableSeconds: Math.ceil(durationSeconds),
    };
  } catch (error) {
    if (error instanceof InvalidTranscriptionAudioError) throw error;
    throw new InvalidTranscriptionAudioError('invalid_audio', { cause: error });
  } finally {
    clearTimeout(timer);
    input.dispose();
    if (decoder) {
      const allocatedDecoder = decoder;
      if (decoderReady) allocatedDecoder.free();
      else
        void allocatedDecoder.ready.then(
          function freeLateDecoder() {
            allocatedDecoder.free();
          },
          function ignoreInitializationFailure() {},
        );
    }
  }
}
