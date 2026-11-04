#!/usr/bin/env bash
# Starts the L4 rig desktop, runs the command given as arguments, then stops only what it started.
#
#   scripts/rig/entrypoint.sh bash -c 'echo $DISPLAY'
#
# Starts, in order: Xvfb on RIG_DISPLAY (default :99), a dbus session, openbox (window manager),
# picom (compositor), the fixed wallpaper, xfce4-panel as the status notifier (tray) host, and a
# PulseAudio server with a virtual microphone. Each process is recorded by PID and killed on exit;
# no other process is touched. Service logs go to RIG_ARTIFACTS when it is set.
#
# Environment:
#   RIG_DISPLAY     X display to start (default :99)
#   RIG_SCREEN      Xvfb screen (default 1280x800x24)
#   RIG_WALLPAPER   wallpaper image (default assets/wall.png next to this script)
#   RIG_ARTIFACTS   directory for service logs (optional)
#   RIG_STATE       state directory (default: a new temp directory, removed on exit unless kept)
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RIG_DISPLAY="${RIG_DISPLAY:-:99}"
RIG_SCREEN="${RIG_SCREEN:-1280x800x24}"
RIG_WALLPAPER="${RIG_WALLPAPER:-$script_dir/assets/wall.png}"
state="${RIG_STATE:-$(mktemp -d "${TMPDIR:-/tmp}/rig-state.XXXXXX")}"
logs="${RIG_ARTIFACTS:-$state}"
mkdir -p "$state" "$logs" "$state/xdg" "$state/pulse"
chmod 700 "$state/xdg" "$state/pulse"

pids=()
pid_names=()

# start_service NAME COMMAND... runs a background service, logs it, and records its PID.
start_service() {
  local name="$1"
  shift
  "$@" >"$logs/$name.log" 2>&1 </dev/null &
  pids+=("$!")
  pid_names+=("$name")
}

stop_services() {
  local i
  for ((i = ${#pids[@]} - 1; i >= 0; i--)); do
    kill "${pids[$i]}" 2>/dev/null || true
  done
  sleep 1
  for ((i = ${#pids[@]} - 1; i >= 0; i--)); do
    kill -9 "${pids[$i]}" 2>/dev/null || true
  done
}

wait_for() {
  local description="$1" seconds="$2"
  shift 2
  local deadline=$((SECONDS + seconds))
  until "$@" >/dev/null 2>&1; do
    if ((SECONDS >= deadline)); then
      echo "rig: timed out waiting for $description" >&2
      return 1
    fi
    sleep 0.2
  done
}

x_is_up() { DISPLAY="$RIG_DISPLAY" xdotool getdisplaygeometry; }

tray_host_registered() {
  dbus-send --session --print-reply --dest=org.freedesktop.DBus /org/freedesktop/DBus \
    org.freedesktop.DBus.NameHasOwner string:org.kde.StatusNotifierWatcher 2>/dev/null | grep -q 'boolean true'
}

pulse_is_up() { PULSE_SERVER="unix:$state/pulse/native" pactl info; }

start_desktop() {
  export DISPLAY="$RIG_DISPLAY"
  # The panel's settings live in the state directory, so a rig run never edits the user's xfce config.
  export XDG_CONFIG_HOME="$state/config"

  start_service xvfb Xvfb "$RIG_DISPLAY" -screen 0 "$RIG_SCREEN" -nolisten tcp +extension RANDR
  wait_for "the X display $RIG_DISPLAY" 15 x_is_up

  eval "$(dbus-launch --sh-syntax)"
  export DBUS_SESSION_BUS_ADDRESS
  pids+=("${DBUS_SESSION_BUS_PID}")
  pid_names+=("dbus-session")
  printf 'export DBUS_SESSION_BUS_ADDRESS=%q\nexport DISPLAY=%q\n' "$DBUS_SESSION_BUS_ADDRESS" "$DISPLAY" >"$state/env.sh"

  start_service openbox openbox
  start_service picom picom --backend xrender --no-fading-openclose

  if [ -f "$RIG_WALLPAPER" ]; then
    feh --no-fehbg --bg-fill "$RIG_WALLPAPER" >"$logs/wallpaper.log" 2>&1 </dev/null || echo "rig: wallpaper not set" >&2
  else
    echo "rig: wallpaper $RIG_WALLPAPER missing" >&2
  fi

  mkdir -p "$XDG_CONFIG_HOME/xfce4/xfconf/xfce-perchannel-xml"
  if [ -f /etc/xdg/xfce4/panel/default.xml ]; then
    cp /etc/xdg/xfce4/panel/default.xml "$XDG_CONFIG_HOME/xfce4/xfconf/xfce-perchannel-xml/xfce4-panel.xml"
  fi
  start_service xfce4-panel xfce4-panel --disable-wm-check
  if ! wait_for "a status notifier (tray) host" 20 tray_host_registered; then
    echo "rig: no tray host registered; tray items will not be available" >&2
  fi

  start_virtual_microphone
}

# start_virtual_microphone runs a private PulseAudio server with a null source as the default
# microphone, so dictation-style code paths have an input device without real audio hardware.
start_virtual_microphone() {
  start_service pulseaudio env PULSE_RUNTIME_PATH="$state/pulse" pulseaudio -n --daemonize=no --exit-idle-time=-1 \
    -L "module-native-protocol-unix auth-anonymous=1 socket=$state/pulse/native" \
    -L "module-null-source source_name=rig_mic" \
    -L "module-always-sink"
  wait_for "PulseAudio" 15 pulse_is_up
  PULSE_SERVER="unix:$state/pulse/native" pactl set-default-source rig_mic
  export PULSE_SERVER="unix:$state/pulse/native"
}

main() {
  trap stop_services EXIT
  start_desktop
  echo "rig: display $RIG_DISPLAY up (state $state, logs $logs)" >&2
  if [ "$#" -eq 0 ]; then
    echo "rig: no command given; exiting" >&2
    return 0
  fi
  set +e
  "$@"
  local status=$?
  set -e
  return "$status"
}

main "$@"
