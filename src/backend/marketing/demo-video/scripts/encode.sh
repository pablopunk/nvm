#!/bin/sh
# Re-encodes the HyperFrames master into the landing page files: a web-friendly H.264 MP4 under 4 MB and a poster JPG.
# Usage: sh scripts/encode.sh <master.mp4> <out-dir>
# Two passes at a fixed average bitrate keep the size predictable; with photo wallpapers, constant-quality encodes
# swing by megabytes.
set -eu
master=$1
out=$2
mkdir -p "$out"
x264="-c:v libx264 -preset veryslow -b:v 1450k -maxrate 2600k -bufsize 5200k -pix_fmt yuv420p -profile:v high"
ffmpeg -hide_banner -loglevel warning -y -i "$master" -an $x264 -pass 1 -passlogfile "$out/x264" -f mp4 /dev/null
ffmpeg -hide_banner -loglevel warning -y -i "$master" -an $x264 -pass 2 -passlogfile "$out/x264" -movflags +faststart "$out/nevermind-demo-v3.mp4"
rm -f "$out"/x264*
# The poster is the builder's answer: the extension is written and installed.
ffmpeg -hide_banner -loglevel warning -y -ss 3.6 -i "$master" -frames:v 1 -update 1 -q:v 3 "$out/nevermind-demo-v3.jpg"
ls -l "$out/nevermind-demo-v3.mp4" "$out/nevermind-demo-v3.jpg"
