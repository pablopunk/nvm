import React, {
  type ReactNode,
  type RefObject,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import type { CommandView } from '../model';
import {
  type CameraCaptureState,
  type CameraSelection,
  createCameraCapture,
  nextCameraDeviceId,
} from './camera-capture';

export function cameraStorageKey(view: Pick<CommandView, 'id' | 'title'>) {
  return `camera-device:${view.id || view.title}`;
}

function readSavedCameraDevice(storageKey: string) {
  if (typeof localStorage === 'undefined') return undefined;
  return localStorage.getItem(storageKey) || undefined;
}

export function initialCameraSelection(
  explicitDeviceId: string | undefined,
  storageKey: string,
): CameraSelection {
  if (explicitDeviceId)
    return { deviceId: explicitDeviceId, source: 'explicit' };
  const saved = readSavedCameraDevice(storageKey);
  return saved ? { deviceId: saved, source: 'saved' } : { source: 'default' };
}

export function cameraVideoClassName(
  view: Pick<CommandView, 'mirrored' | 'framing'>,
) {
  return [
    'cameraVideo',
    view.mirrored ? 'cameraVideoMirrored' : '',
    view.framing === 'contain' ? 'cameraVideoContain' : '',
  ]
    .filter(Boolean)
    .join(' ');
}

export function cameraStatusText(state: CameraCaptureState, playing: boolean) {
  if (state.status === 'starting') return 'Requesting camera…';
  if (state.status === 'live') return playing ? 'Live' : 'Starting…';
  if (state.status === 'error') return 'Camera unavailable';
  return 'Paused';
}

async function requestHostCameraAccess() {
  const access = await window.nvm.requestCameraAccess();
  if (
    access.status === 'denied' ||
    access.status === 'restricted' ||
    access.status === 'unsupported'
  )
    throw new Error(
      access.status === 'denied'
        ? 'Camera access is denied in system privacy settings.'
        : `Camera access is ${access.status}.`,
    );
}

async function listCameraDevices() {
  try {
    const devices = await navigator.mediaDevices.enumerateDevices();
    return devices.filter((device) => device.kind === 'videoinput');
  } catch {
    return [];
  }
}

function isPageVisible() {
  return (
    typeof document === 'undefined' || document.visibilityState === 'visible'
  );
}

function useCameraPresentation(rootRef: RefObject<HTMLElement | null>) {
  const [onscreen, setOnscreen] = useState(false);
  const [pageVisible, setPageVisible] = useState(isPageVisible);
  useEffect(() => {
    const element = rootRef.current;
    if (!element || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(([entry]) =>
      setOnscreen(Boolean(entry?.isIntersecting)),
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [rootRef]);
  useEffect(() => {
    const update = () => setPageVisible(isPageVisible());
    document.addEventListener('visibilitychange', update);
    return () => document.removeEventListener('visibilitychange', update);
  }, []);
  return onscreen && pageVisible;
}

export function CameraView({
  view,
  actions,
  active,
}: {
  view: CommandView;
  actions: ReactNode;
  active: boolean;
}) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const storageKey = cameraStorageKey(view);
  const [selection, setSelection] = useState(() =>
    initialCameraSelection(view.deviceId, storageKey),
  );
  const [captureState, setCaptureState] = useState<CameraCaptureState>({
    status: 'idle',
  });
  const [playing, setPlaying] = useState(false);
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
  const [muted, setMuted] = useState(view.muted !== false);
  const [controls, setControls] = useState(Boolean(view.controls));
  const presented = useCameraPresentation(rootRef);
  const captureEnabled = active && presented;
  const capture = useMemo(
    () =>
      createCameraCapture({
        requestAccess: requestHostCameraAccess,
        getUserMedia: (constraints) =>
          navigator.mediaDevices.getUserMedia(constraints),
        forgetSavedDevice: () => localStorage.removeItem(storageKey),
        onState: setCaptureState,
      }),
    [storageKey],
  );
  const { deviceId: selectedDeviceId, source } = selection;
  const liveStream =
    captureState.status === 'live' ? captureState.stream : null;
  const liveDeviceId =
    captureState.status === 'live' ? captureState.deviceId : undefined;

  useEffect(
    () => setSelection(initialCameraSelection(view.deviceId, storageKey)),
    [storageKey, view.deviceId],
  );
  useEffect(() => setMuted(view.muted !== false), [view.muted]);
  useEffect(() => setControls(Boolean(view.controls)), [view.controls]);

  useEffect(() => {
    if (!captureEnabled) return;
    void capture.start({ deviceId: selectedDeviceId, source });
    return () => capture.stop();
  }, [capture, captureEnabled, selectedDeviceId, source]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    video.srcObject = liveStream;
    if (!liveStream) return;
    setPlaying(false);
    video.play().catch((error: unknown) => {
      if (errorIsAbort(error)) return;
      capture.playbackFailed(liveStream);
    });
    return () => {
      if (video.srcObject === liveStream) video.srcObject = null;
    };
  }, [capture, liveStream]);

  useEffect(() => {
    const refresh = () => void listCameraDevices().then(setDevices);
    refresh();
    navigator.mediaDevices?.addEventListener('devicechange', refresh);
    return () =>
      navigator.mediaDevices?.removeEventListener('devicechange', refresh);
  }, []);

  useEffect(() => {
    if (liveStream) void listCameraDevices().then(setDevices);
  }, [liveStream]);

  function switchDevice(direction: 'next' | 'previous' = 'next') {
    const deviceId = nextCameraDeviceId(
      devices.map((device) => device.deviceId),
      liveDeviceId || selectedDeviceId,
      direction,
    );
    if (!deviceId) return;
    localStorage.setItem(storageKey, deviceId);
    setSelection({ deviceId, source: 'saved' });
  }

  useEffect(() => {
    function handleCameraAction(event: Event) {
      if (!captureEnabled) return;
      const kind = (event as CustomEvent<{ kind?: string }>).detail?.kind;
      if (kind === 'camera.switchDevice' || kind === 'camera.nextDevice')
        switchDevice('next');
      else if (kind === 'camera.previousDevice') switchDevice('previous');
      else if (kind === 'camera.toggleMuted') setMuted((value) => !value);
      else if (kind === 'camera.toggleControls') setControls((value) => !value);
    }
    window.addEventListener('nvm:camera-action', handleCameraAction);
    return () =>
      window.removeEventListener('nvm:camera-action', handleCameraAction);
  });

  const currentDevice =
    devices.find(
      (device) => device.deviceId === (liveDeviceId || selectedDeviceId),
    ) || devices[0];
  const activeLabel =
    captureState.status === 'live' ? captureState.label : currentDevice?.label;
  const switcher =
    view.showDeviceSwitcher === false || devices.length < 2 ? null : (
      <button
        className="cameraSwitchButton"
        type="button"
        onClick={() => switchDevice()}
      >
        Switch Camera{activeLabel ? ` · ${activeLabel}` : ''}
      </button>
    );

  return (
    <div
      ref={rootRef}
      className={`cameraSurface ${view.size === 'large' || view.presentation === 'preview' ? 'cameraLarge' : ''}`}
    >
      <div className="cameraFrame">
        <video
          ref={videoRef}
          className={cameraVideoClassName(view)}
          autoPlay={true}
          playsInline={true}
          muted={muted}
          controls={controls}
          onPlaying={() => setPlaying(true)}
          onError={() => {
            if (liveStream) capture.playbackFailed(liveStream);
          }}
        />
        {switcher}
        {captureState.status === 'error' ? (
          <div className="cameraError">
            <strong>Camera Issue</strong>
            <span>{captureState.message}</span>
          </div>
        ) : null}
        <div className="cameraStatus">
          {cameraStatusText(captureState, playing)}
        </div>
      </div>
      {actions}
    </div>
  );
}

function errorIsAbort(error: unknown) {
  return (error as { name?: unknown } | null)?.name === 'AbortError';
}
