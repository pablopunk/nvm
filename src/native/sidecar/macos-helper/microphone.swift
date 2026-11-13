import AVFoundation
import Foundation

// The microphone subcommands of the helper. `devices`, `access` and `request` answer with the window envelope;
// `stream` writes signed 16-bit little-endian PCM to stdout until its parent kills it.

func microphoneStatusName(_ status: AVAuthorizationStatus) -> String {
  switch status {
  case .authorized: return "allowed"
  case .notDetermined: return "not-determined"
  case .denied, .restricted: return "denied"
  @unknown default: return "denied"
  }
}

func microphoneAccessStatus() -> String {
  microphoneStatusName(AVCaptureDevice.authorizationStatus(for: .audio))
}

// requestMicrophoneAccess shows the system's microphone prompt when the user has not answered it yet, and waits for
// the answer. The Go side bounds the wait with its own deadline.
func requestMicrophoneAccess() -> String {
  guard AVCaptureDevice.authorizationStatus(for: .audio) == .notDetermined else {
    return microphoneAccessStatus()
  }
  let answered = DispatchSemaphore(value: 0)
  AVCaptureDevice.requestAccess(for: .audio) { _ in answered.signal() }
  answered.wait()
  return microphoneAccessStatus()
}

func microphoneDevicesData() -> [[String: Any]] {
  let defaultID = AVCaptureDevice.default(for: .audio)?.uniqueID
  return AVCaptureDevice.devices(for: .audio).map { device in
    ["id": device.uniqueID, "title": device.localizedName, "default": device.uniqueID == defaultID]
  }
}

func microphoneDevice(id: String) -> AVCaptureDevice? {
  if id.isEmpty || id == "default" { return AVCaptureDevice.default(for: .audio) }
  return AVCaptureDevice(uniqueID: id)
}

func runMicrophoneCommand(_ arguments: [String]) -> Never {
  switch arguments.first ?? "" {
  case "devices": emitWindowResponse(WindowResponse(code: .ok, data: microphoneDevicesData()))
  case "access": emitWindowResponse(WindowResponse(code: .ok, data: ["status": microphoneAccessStatus()]))
  case "request": emitWindowResponse(WindowResponse(code: .ok, data: ["status": requestMicrophoneAccess()]))
  case "stream": runMicrophoneStream(Array(arguments.dropFirst()))
  default: fail("Usage: macos-selected-text microphone <devices|access|request|stream <device> <rate> <channels>>")
  }
}

// runMicrophoneStream prints the device's PCM until the process is killed. Refusals and missing devices go to stderr
// with a prefix the Go side maps to its errors.
func runMicrophoneStream(_ arguments: [String]) -> Never {
  guard arguments.count == 3, let rate = Double(arguments[1]), let channels = UInt32(arguments[2]) else {
    fail("error: usage: microphone stream <device> <rate> <channels>", code: 1)
  }
  guard requestMicrophoneAccess() == "allowed" else {
    fail("denied: Microphone access is denied", code: 2)
  }
  guard let device = microphoneDevice(id: arguments[0]) else {
    fail("not-found: No microphone was found", code: 3)
  }
  let streamer = MicrophoneStreamer(sampleRate: rate, channels: channels)
  do {
    try streamer.start(device: device)
  } catch {
    fail("error: \(error.localizedDescription)", code: 1)
  }
  dispatchMain()
}

final class MicrophoneStreamer: NSObject, AVCaptureAudioDataOutputSampleBufferDelegate {
  private let session = AVCaptureSession()
  private let output = AVCaptureAudioDataOutput()
  private let queue = DispatchQueue(label: "nvm.microphone.stream")
  private let target: AVAudioFormat
  private var converter: AVAudioConverter?
  private var converterInput: AVAudioFormat?

  init(sampleRate: Double, channels: UInt32) {
    target = AVAudioFormat(commonFormat: .pcmFormatInt16, sampleRate: sampleRate, channels: channels, interleaved: true)!
    super.init()
  }

  func start(device: AVCaptureDevice) throws {
    let input = try AVCaptureDeviceInput(device: device)
    session.beginConfiguration()
    guard session.canAddInput(input), session.canAddOutput(output) else {
      session.commitConfiguration()
      throw NSError(domain: "nvm.microphone", code: 1, userInfo: [NSLocalizedDescriptionKey: "The microphone cannot be opened"])
    }
    session.addInput(input)
    session.addOutput(output)
    output.setSampleBufferDelegate(self, queue: queue)
    session.commitConfiguration()
    session.startRunning()
  }

  func captureOutput(_ output: AVCaptureOutput, didOutput sampleBuffer: CMSampleBuffer, from connection: AVCaptureConnection) {
    guard let pcm = convertedPCM(sampleBuffer) else { return }
    FileHandle.standardOutput.write(pcm)
  }

  private func convertedPCM(_ sampleBuffer: CMSampleBuffer) -> Data? {
    guard
      let description = CMSampleBufferGetFormatDescription(sampleBuffer),
      let streamDescription = CMAudioFormatDescriptionGetStreamBasicDescription(description),
      let inputFormat = AVAudioFormat(streamDescription: streamDescription),
      let converter = converter(from: inputFormat)
    else { return nil }
    let frames = AVAudioFrameCount(CMSampleBufferGetNumSamples(sampleBuffer))
    guard frames > 0, let input = AVAudioPCMBuffer(pcmFormat: inputFormat, frameCapacity: frames) else { return nil }
    input.frameLength = frames
    guard
      CMSampleBufferCopyPCMDataIntoAudioBufferList(sampleBuffer, at: 0, frameCount: Int32(frames), into: input.mutableAudioBufferList) == noErr
    else { return nil }
    let ratio = target.sampleRate / inputFormat.sampleRate
    guard let output = AVAudioPCMBuffer(pcmFormat: target, frameCapacity: AVAudioFrameCount(Double(frames) * ratio) + 64) else {
      return nil
    }
    var handedOver = false
    var conversionError: NSError?
    converter.convert(to: output, error: &conversionError) { _, status in
      if handedOver {
        status.pointee = .noDataNow
        return nil
      }
      handedOver = true
      status.pointee = .haveData
      return input
    }
    guard conversionError == nil, let samples = output.int16ChannelData else { return nil }
    return Data(bytes: samples[0], count: Int(output.frameLength) * MemoryLayout<Int16>.size * Int(target.channelCount))
  }

  private func converter(from inputFormat: AVAudioFormat) -> AVAudioConverter? {
    if converterInput != inputFormat {
      converter = AVAudioConverter(from: inputFormat, to: target)
      converterInput = inputFormat
    }
    return converter
  }
}
