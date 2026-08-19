type AudioInputDevice = {
  deviceId: string;
  kind: string;
  label: string;
};

export function dictationDevices(devices: readonly AudioInputDevice[]) {
  const microphones = devices
    .filter((device) => device.kind === 'audioinput')
    .filter((device) => device.deviceId !== 'default')
    .map((device, index) => ({
      id: device.deviceId || `microphone-${index + 1}`,
      title:
        device.label.replace(/^(Default|Communications) - /, '') ||
        `Microphone ${index + 1}`,
      isDefault: false,
    }));

  return [{ id: 'default', title: 'Default', isDefault: true }, ...microphones];
}
