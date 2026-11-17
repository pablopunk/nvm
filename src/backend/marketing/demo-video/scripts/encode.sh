#!/bin/sh
# Re-encodes the HyperFrames master into the landing page files: a web-friendly H.264 MP4 and a poster JPG.
# Usage: sh scripts/encode.sh <master.mp4> <out-dir>
set -eu
master=$1
out=$2
mkdir -p "$out"
ffmpeg -hide_banner -loglevel warning -y -i "$master" -an \
  -c:v libx264 -preset veryslow -tune animation -crf 20 -maxrate 2500k -bufsize 5000k \
  -pix_fmt yuv420p -profile:v high -movflags +faststart "$out/nevermind-demo-v3.mp4"
# The poster is the builder's answer: the extension is written and installed.
ffmpeg -hide_banner -loglevel warning -y -ss 3.6 -i "$master" -frames:v 1 -update 1 -q:v 3 "$out/nevermind-demo-v3.jpg"
ls -l "$out/nevermind-demo-v3.mp4" "$out/nevermind-demo-v3.jpg"
