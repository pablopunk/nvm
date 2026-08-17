# Bluetooth Dictation Readiness

## Problem

macOS can expose a live Bluetooth microphone stream while it changes from the
playback profile to the headset profile. Chromium supplies silent audio frames
during this interval, so stream creation, track state, recorder start, and the
first Web Audio frame do not mean that the microphone is ready.

## Contract

Start recording before readiness checks so that all audio delivered during
preparation is retained. Do not show `Listening` until a regular microphone has
delivered a live frame or a Bluetooth microphone has delivered stable usable
input.

Bluetooth readiness requires a non-silent frame followed by two current frames.
Frame gaps longer than 250 ms reset readiness. A continuously delivered silent
stream becomes ready after five seconds because silence can be valid input and
the Web platform cannot distinguish device silence from operating-system
silence. Startup fails after seven seconds if neither condition is met.

Show the resolved microphone name while waiting. This makes device routing and
Bluetooth startup visible without adding a device-specific fixed delay.

## Evidence

TypeWhisper issue 938 and pull request 994 reproduce the same AirPods failure
and use stable non-silent buffers with a silent fallback. The project reports
AirPods readiness between about 1.6 and 4.2 seconds. The macOS mic-keepwarm
project documents the underlying microphone wake and Bluetooth SCO negotiation
delay, and confirms that permanent instant startup requires keeping capture
open, with a persistent privacy indicator and reduced Bluetooth playback
quality.
