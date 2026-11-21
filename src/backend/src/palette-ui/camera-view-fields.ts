export type CameraFraming = 'cover' | 'contain';

export type CameraViewFields = {
  deviceId?: string;
  showDeviceSwitcher?: boolean;
  muted?: boolean;
  controls?: boolean;
  mirrored?: boolean;
  framing?: CameraFraming;
};
