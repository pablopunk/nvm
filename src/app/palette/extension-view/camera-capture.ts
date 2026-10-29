export type CameraDeviceSource = 'explicit' | 'saved' | 'default';

export type CameraSelection = {
  deviceId?: string;
  source: CameraDeviceSource;
};

export type CameraCaptureState =
  | { status: 'idle' }
  | { status: 'starting' }
  | { status: 'live'; stream: MediaStream; deviceId?: string; label: string }
  | { status: 'error'; message: string };

export type CameraCaptureDeps = {
  requestAccess: () => Promise<void>;
  getUserMedia: (constraints: MediaStreamConstraints) => Promise<MediaStream>;
  forgetSavedDevice: () => void;
  onState: (state: CameraCaptureState) => void;
};

const DISCONNECTED_MESSAGE =
  'Camera disconnected. Check the connection, then reopen this view.';
const PLAYBACK_MESSAGE = 'Camera feed stopped with a playback error.';

export function cameraVideoConstraints(
  deviceId?: string,
): MediaTrackConstraints {
  return {
    ...(deviceId ? { deviceId: { exact: deviceId } } : {}),
    width: { ideal: 1280 },
    height: { ideal: 720 },
  };
}

export function errorName(error: unknown) {
  const name = (error as { name?: unknown } | null)?.name;
  return typeof name === 'string' ? name : '';
}

export function isStaleCameraDeviceError(error: unknown) {
  const name = errorName(error);
  return name === 'NotFoundError' || name === 'OverconstrainedError';
}

export function cameraErrorMessage(error: unknown) {
  const name = errorName(error);
  if (name === 'NotAllowedError')
    return 'Camera access is denied. Allow Nevermind in system privacy settings.';
  if (name === 'NotReadableError')
    return 'The camera is busy. Close other apps using it, then reopen this view.';
  if (isStaleCameraDeviceError(error))
    return 'The camera is not available. Check that it is connected, or choose another camera.';
  return error instanceof Error && error.message
    ? error.message
    : 'Camera unavailable.';
}

export function nextCameraDeviceId(
  deviceIds: readonly string[],
  currentDeviceId: string | undefined,
  direction: 'next' | 'previous',
) {
  if (deviceIds.length < 2) return undefined;
  const index = currentDeviceId ? deviceIds.indexOf(currentDeviceId) : -1;
  const offset = direction === 'previous' ? -1 : 1;
  const base = index >= 0 ? index : offset > 0 ? -1 : deviceIds.length;
  return deviceIds[(base + offset + deviceIds.length) % deviceIds.length];
}

function stopStreamTracks(stream: MediaStream | null) {
  stream?.getTracks().forEach((track) => track.stop());
}

export function createCameraCapture(deps: CameraCaptureDeps) {
  let generation = 0;
  let liveStream: MediaStream | null = null;

  function releaseLiveStream() {
    const stream = liveStream;
    liveStream = null;
    stopStreamTracks(stream);
  }

  function reportError(message: string) {
    deps.onState({ status: 'error', message });
  }

  async function openStream(selection: CameraSelection, token: number) {
    try {
      return await deps.getUserMedia({
        audio: false,
        video: cameraVideoConstraints(selection.deviceId),
      });
    } catch (error) {
      if (selection.source !== 'saved' || !isStaleCameraDeviceError(error))
        throw error;
      if (token !== generation) throw error;
      deps.forgetSavedDevice();
      return deps.getUserMedia({
        audio: false,
        video: cameraVideoConstraints(),
      });
    }
  }

  function adoptStream(stream: MediaStream) {
    liveStream = stream;
    for (const track of stream.getVideoTracks())
      track.addEventListener('ended', () =>
        failStream(stream, DISCONNECTED_MESSAGE),
      );
    const [track] = stream.getVideoTracks();
    deps.onState({
      status: 'live',
      stream,
      deviceId: track?.getSettings().deviceId,
      label: track?.label ?? '',
    });
  }

  async function start(selection: CameraSelection) {
    const token = ++generation;
    releaseLiveStream();
    deps.onState({ status: 'starting' });
    try {
      await deps.requestAccess();
      if (token !== generation) return;
      const stream = await openStream(selection, token);
      if (token !== generation) {
        stopStreamTracks(stream);
        return;
      }
      adoptStream(stream);
    } catch (error) {
      if (token === generation) reportError(cameraErrorMessage(error));
    }
  }

  function failStream(stream: MediaStream, message: string) {
    if (liveStream !== stream) return;
    generation += 1;
    releaseLiveStream();
    reportError(message);
  }

  function stop() {
    generation += 1;
    releaseLiveStream();
    deps.onState({ status: 'idle' });
  }

  return {
    start,
    stop,
    failStream,
    playbackFailed: (stream: MediaStream) =>
      failStream(stream, PLAYBACK_MESSAGE),
  };
}
