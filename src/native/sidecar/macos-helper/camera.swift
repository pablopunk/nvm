import AVFoundation
import CoreAudio
import Foundation

// The camera subcommands of the helper. `devices` answers with the window envelope. Listing reads names only: it never
// asks for camera permission and never turns a camera on.

func cameraDeviceTypes() -> [AVCaptureDevice.DeviceType] {
  var types: [AVCaptureDevice.DeviceType] = [.builtInWideAngleCamera]
  if #available(macOS 14.0, *) {
    types += [.external, .continuityCamera, .deskViewCamera]
  } else {
    types += [.externalUnknown]
  }
  return types
}

func isVirtualCamera(_ device: AVCaptureDevice) -> Bool {
  UInt32(bitPattern: device.transportType) == kAudioDeviceTransportTypeVirtual
}

func cameraKind(_ device: AVCaptureDevice) -> String {
  if isVirtualCamera(device) { return "virtual" }
  if device.deviceType == .builtInWideAngleCamera { return "built-in" }
  if #available(macOS 14.0, *) {
    if device.deviceType == .continuityCamera || device.deviceType == .deskViewCamera { return "continuity" }
  }
  if #available(macOS 13.0, *) {
    if device.isContinuityCamera { return "continuity" }
  }
  return "external"
}

func cameraDevicesData() -> [[String: Any]] {
  let session = AVCaptureDevice.DiscoverySession(
    deviceTypes: cameraDeviceTypes(), mediaType: .video, position: .unspecified)
  let defaultID = AVCaptureDevice.default(for: .video)?.uniqueID
  return session.devices.map { device in
    [
      "id": device.uniqueID, "title": device.localizedName, "default": device.uniqueID == defaultID,
      "kind": cameraKind(device),
    ]
  }
}

func runCameraCommand(_ arguments: [String]) -> Never {
  switch arguments.first ?? "" {
  case "devices": emitWindowResponse(WindowResponse(code: .ok, data: cameraDevicesData()))
  default: fail("Usage: macos-selected-text camera <devices>")
  }
}
