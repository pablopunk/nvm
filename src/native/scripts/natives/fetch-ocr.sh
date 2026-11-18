#!/usr/bin/env bash
# Downloads the pinned ONNX Runtime and PP-OCR models into resources/<goos>-<goarch>/, where mygo build ships them.
# usage: scripts/natives/fetch-ocr.sh <goos> <goarch>   (needs curl and unzip)
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
goos=$1
goarch=$2
ORT_VERSION=1.23.2
# Microsoft's NuGet package carries the official ONNX Runtime build for every desktop target in one archive; the
# per-platform GitHub release archives this script used before are no longer published.
ORT_PACKAGE=https://api.nuget.org/v3-flatcontainer/microsoft.ml.onnxruntime/$ORT_VERSION/microsoft.ml.onnxruntime.$ORT_VERSION.nupkg
ORT_PACKAGE_SUM=25fe172f7fcdf34f2b5b02c8b997f6b88abc282956849d4af4335af23d0c4e4a
MODELS=https://www.modelscope.cn/models/RapidAI/RapidOCR/resolve/v3.10.0

case "$goos-$goarch" in
  darwin-arm64) rid=osx-arm64; library=libonnxruntime.dylib ;;
  darwin-amd64) rid=osx-x64; library=libonnxruntime.dylib ;;
  linux-amd64) rid=linux-x64; library=libonnxruntime.so ;;
  linux-arm64) rid=linux-arm64; library=libonnxruntime.so ;;
  windows-amd64) rid=win-x64; library=onnxruntime.dll ;;
  *) echo "no OCR runtime for $goos-$goarch" >&2; exit 1 ;;
esac

sha256() {
  if command -v sha256sum >/dev/null; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

fetch() { # url, destination, sha256
  curl --fail --location --silent --show-error --retry 3 --output "$2" "$1"
  if [ "$(sha256 "$2")" != "$3" ]; then
    echo "checksum mismatch for $1" >&2
    exit 1
  fi
}

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fetch "$ORT_PACKAGE" "$work/onnxruntime.nupkg" "$ORT_PACKAGE_SUM"
unzip -q -o "$work/onnxruntime.nupkg" "runtimes/$rid/native/$library" LICENSE ThirdPartyNotices.txt -d "$work/package"

out="$root/resources/$goos-$goarch"
mkdir -p "$out/lib" "$out/ocr"
cp "$work/package/runtimes/$rid/native/$library" "$out/lib/$library"
cp "$work/package/LICENSE" "$out/ocr/ONNX-RUNTIME-LICENSE"
cp "$work/package/ThirdPartyNotices.txt" "$out/ocr/ONNX-RUNTIME-NOTICES"
cat > "$out/ocr/MODELS-NOTICE" <<'NOTICE'
PP-OCRv5 mobile detection and Latin recognition models, distributed by RapidAI/RapidOCR v3.10.0.
PaddleOCR: https://github.com/PaddlePaddle/PaddleOCR (Apache-2.0).
Model source and checksums are pinned in scripts/natives/fetch-ocr.sh.
NOTICE
fetch "$MODELS/onnx/PP-OCRv5/det/ch_PP-OCRv5_det_mobile.onnx" "$out/ocr/det.onnx" 4d97c44a20d30a81aad087d6a396b08f786c4635742afc391f6621f5c6ae78ae
fetch "$MODELS/onnx/PP-OCRv5/rec/latin_PP-OCRv5_rec_mobile.onnx" "$out/ocr/rec.onnx" b20bd37c168a570f583afbc8cd7925603890efbcdc000a59e22c269d160b5f5a
fetch "$MODELS/paddle/PP-OCRv5/rec/latin_PP-OCRv5_rec_mobile/ppocrv5_latin_dict.txt" "$out/ocr/dict.txt" 3c0a8a79b612653c25f765271714f71281e4e955962c153e272b7b8c1d2b13ff
echo "OCR ($goos-$goarch): $out"
