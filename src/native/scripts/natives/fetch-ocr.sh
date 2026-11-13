#!/usr/bin/env bash
# Downloads the pinned ONNX Runtime and PP-OCR models into resources/<goos>-<goarch>/, where mygo build ships them.
# usage: scripts/natives/fetch-ocr.sh <goos> <goarch>
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
goos=$1
goarch=$2
ORT_VERSION=1.23.2
MODELS=https://www.modelscope.cn/models/RapidAI/RapidOCR/resolve/v3.10.0

case "$goos-$goarch" in
  darwin-arm64) asset=onnxruntime-osx-arm64-$ORT_VERSION.tgz; library=libonnxruntime.$ORT_VERSION.dylib; output=libonnxruntime.dylib; sum=b4d513ab2b26f088c66891dbbc1408166708773d7cc4163de7bdca0e9bbb7856 ;;
  darwin-amd64) asset=onnxruntime-osx-x86_64-$ORT_VERSION.tgz; library=libonnxruntime.$ORT_VERSION.dylib; output=libonnxruntime.dylib; sum=d10359e16347b57d9959f7e80a225a5b4a66ed7d7e007274a15cae86836485a6 ;;
  linux-amd64) asset=onnxruntime-linux-x64-$ORT_VERSION.tgz; library=libonnxruntime.so.$ORT_VERSION; output=libonnxruntime.so; sum=1fa4dcaef22f6f7d5cd81b28c2800414350c10116f5fdd46a2160082551c5f9b ;;
  linux-arm64) asset=onnxruntime-linux-aarch64-$ORT_VERSION.tgz; library=libonnxruntime.so.$ORT_VERSION; output=libonnxruntime.so; sum=7c63c73560ed76b1fac6cff8204ffe34fe180e70d6582b5332ec094810241e5c ;;
  windows-amd64) asset=onnxruntime-win-x64-$ORT_VERSION.zip; library=onnxruntime.dll; output=onnxruntime.dll; sum=0b38df9af21834e41e73d602d90db5cb06dbd1ca618948b8f1d66d607ac9f3cd ;;
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
fetch "https://github.com/microsoft/onnxruntime/releases/download/v$ORT_VERSION/$asset" "$work/$asset" "$sum"
case "$asset" in
  *.zip) unzip -q "$work/$asset" -d "$work" ;;
  *) tar -xzf "$work/$asset" -C "$work" ;;
esac

out="$root/resources/$goos-$goarch"
mkdir -p "$out/lib" "$out/ocr"
cp -L "$work/${asset%.*}/lib/$library" "$out/lib/$output"
cp "$work/${asset%.*}/LICENSE" "$out/ocr/ONNX-RUNTIME-LICENSE"
cp "$work/${asset%.*}/ThirdPartyNotices.txt" "$out/ocr/ONNX-RUNTIME-NOTICES"
cat > "$out/ocr/MODELS-NOTICE" <<'NOTICE'
PP-OCRv5 mobile detection and Latin recognition models, distributed by RapidAI/RapidOCR v3.10.0.
PaddleOCR: https://github.com/PaddlePaddle/PaddleOCR (Apache-2.0).
Model source and checksums are pinned in scripts/natives/fetch-ocr.sh.
NOTICE
fetch "$MODELS/onnx/PP-OCRv5/det/ch_PP-OCRv5_det_mobile.onnx" "$out/ocr/det.onnx" 4d97c44a20d30a81aad087d6a396b08f786c4635742afc391f6621f5c6ae78ae
fetch "$MODELS/onnx/PP-OCRv5/rec/latin_PP-OCRv5_rec_mobile.onnx" "$out/ocr/rec.onnx" b20bd37c168a570f583afbc8cd7925603890efbcdc000a59e22c269d160b5f5a
fetch "$MODELS/paddle/PP-OCRv5/rec/latin_PP-OCRv5_rec_mobile/ppocrv5_latin_dict.txt" "$out/ocr/dict.txt" 3c0a8a79b612653c25f765271714f71281e4e955962c153e272b7b8c1d2b13ff
echo "OCR ($goos-$goarch): $out"
