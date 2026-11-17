#!/usr/bin/env bash
set -uo pipefail
Xvfb :99 -screen 0 1280x800x24 -nolisten tcp > "$OUT/xvfb.log" 2>&1 &
openbox > "$OUT/openbox.log" 2>&1 &
sleep 3
xdotool getdisplaygeometry > "$OUT/display.txt" 2>&1
find "$OUT/v1" "$OUT/v2" -type f \( -name '*.deb' -o -name '*.AppImage' -o -executable \) -printf '%p\n' > "$OUT/executables.txt"
# This calibration records candidate package contents and visible desktop states. It does
# not calculate latency until a shared, reliable visible endpoint has been identified.
v1=$(find "$OUT/v1" -type f -name nevermind -executable | head -n 1)
v2deb=$(find "$OUT/v2" -type f -name '*.deb' | head -n 1)
if [ -n "$v2deb" ]; then
  mkdir -p "$OUT/v2-extracted"
  dpkg-deb -x "$v2deb" "$OUT/v2-extracted"
fi
v2=$(find "$OUT/v2-extracted" "$OUT/v2" -type f \( -name nevermind -o -name nvm \) -executable 2>/dev/null | head -n 1)
printf 'v1=%s\nv2=%s\n' "$v1" "$v2" > "$OUT/candidates.txt"
for version in v1 v2; do
  candidate_var=$version
  binary=${!candidate_var}
  if [ -z "$binary" ]; then printf '%s: executable not found\n' "$version" >> "$OUT/failures.txt"; continue; fi
  sha256sum "$binary" >> "$OUT/binary-sha256.txt"
  mkdir -p "$OUT/home-$version"
  HOME="$OUT/home-$version" XDG_CONFIG_HOME="$OUT/home-$version/.config" XDG_DATA_HOME="$OUT/home-$version/.local/share" "$binary" --no-sandbox > "$OUT/$version.log" 2>&1 &
  pid=$!
  sleep 15
  ps -eo pid,ppid,rss,args --forest > "$OUT/$version-processes.txt"
  xdotool search --onlyvisible --name . getwindowname %@ > "$OUT/$version-windows.txt" 2>&1 || true
  import -window root "$OUT/$version-start.png" || true
  xdotool key alt+shift+space || true
  sleep 3
  import -window root "$OUT/$version-hotkey.png" || true
  kill "$pid" 2>/dev/null || true
  pkill -P "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  sleep 2
done
printf '%s\n' 'Calibration only: no matched latency or PSS measurements performed.' > "$OUT/limitations.txt"
