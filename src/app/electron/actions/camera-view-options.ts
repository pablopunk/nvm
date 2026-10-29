export type CameraViewFraming = 'cover' | 'contain';

export type CameraViewOptions = {
  mirrored: boolean;
  framing: CameraViewFraming;
};

export function normalizeCameraViewOptions(view: {
  mirrored?: unknown;
  framing?: unknown;
}): CameraViewOptions {
  return {
    mirrored: view.mirrored === true,
    framing: view.framing === 'contain' ? 'contain' : 'cover',
  };
}
