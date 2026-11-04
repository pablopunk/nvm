#!/usr/bin/env bash
# X11 window management check (WP4.1b): a private Xvfb with openbox and a panel, a probe window, and the
# rig-tagged engine test that lays the probe out in halves and restores it. Run from src/native:
#   RIG_ARTIFACTS=/root/wp41b-logs bash scripts/rig/window-x11.sh
# feh is the probe because it publishes _NET_WM_PID; display and xmessage do not, and the backend ignores
# windows without a PID. Only the processes this script starts are stopped.
set -u

display="${RIG_DISPLAY:-:95}"
logs="${RIG_ARTIFACTS:?set RIG_ARTIFACTS to a directory for the logs and screenshots}"
mkdir -p "$logs"
pids=()

cleanup() {
  for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null; done
  sleep 1
  for pid in "${pids[@]}"; do kill -9 "$pid" 2>/dev/null; done
}
trap cleanup EXIT

start() {
  "$@" >>"$logs/services.log" 2>&1 &
  pids+=("$!")
}

start Xvfb "$display" -screen 0 1280x800x24 -nolisten tcp
sleep 2
export DISPLAY="$display"
start openbox
start xfce4-panel --disable-wm-check
sleep 3

convert -size 400x300 xc:steelblue "$logs/probe.png"
start feh --title nvm-probe -g 400x300+300+300 "$logs/probe.png"
sleep 3

import -window root "$logs/before.png"
go test -tags rig -count=1 -v -run TestRigMovesAProbeWindowOnX11 ./internal/windowmgmt/ >"$logs/test.txt" 2>&1
status=$?
import -window root "$logs/after.png"
tail -20 "$logs/test.txt"
echo "rig window-x11: exit $status (logs in $logs)"
exit "$status"
