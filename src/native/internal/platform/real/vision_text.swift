import Foundation
import ImageIO
import Vision

func fail(_ message: String) -> Never {
  FileHandle.standardError.write(Data(message.utf8))
  exit(1)
}

let arguments = CommandLine.arguments
guard arguments.count >= 2 else { fail("Missing image path") }
let imageURL = URL(fileURLWithPath: arguments[1])
let languages = Array(arguments.dropFirst(2)).filter { !$0.isEmpty }

guard let source = CGImageSourceCreateWithURL(imageURL as CFURL, nil),
      let image = CGImageSourceCreateImageAtIndex(source, 0, nil) else {
  fail("The image could not be read")
}
let width = Double(image.width)
let height = Double(image.height)

let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
request.usesLanguageCorrection = true
if !languages.isEmpty { request.recognitionLanguages = languages }

do {
  try VNImageRequestHandler(cgImage: image, options: [:]).perform([request])
} catch {
  fail(error.localizedDescription)
}

let observations = request.results ?? []
let blocks: [[String: Any]] = observations.compactMap { observation in
  guard let candidate = observation.topCandidates(1).first else { return nil }
  let box = observation.boundingBox
  let pixelBox: [String: Double] = [
    "x": Double(box.origin.x) * width,
    "y": (1 - Double(box.origin.y) - Double(box.size.height)) * height,
    "width": Double(box.size.width) * width,
    "height": Double(box.size.height) * height,
  ]
  return ["text": candidate.string, "confidence": Double(candidate.confidence), "box": pixelBox]
}
let confidences = blocks.compactMap { $0["confidence"] as? Double }
let meanConfidence: Any = confidences.isEmpty ? NSNull() : confidences.reduce(0, +) / Double(confidences.count)
let output: [String: Any] = [
  "text": blocks.compactMap { $0["text"] as? String }.joined(separator: "\n"),
  "confidence": meanConfidence,
  "language": languages.first ?? NSNull(),
  "blocks": blocks,
]
do {
  FileHandle.standardOutput.write(try JSONSerialization.data(withJSONObject: output, options: []))
} catch {
  fail(error.localizedDescription)
}
