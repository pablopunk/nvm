# Background pastes need current visibility, not a hide event

## Symptom and evidence

Dictation finished transcription without inserting the transcript. The installed Next build's log recorded successful
audio requests followed by `the paste action failed` with `context deadline exceeded`. Repeating the background paste
fixture on the Linux rig reproduced the same terminal failure: the first paste landed, but the second timed out.

## Root cause and contract

The paste gate treated a buffered native window-hide notification as a single-use permit. A background command does not
open the palette, so repeated invocations exhausted the permit while the palette remained hidden. A test that opened
and closed the palette before one paste could pass indefinitely without exercising this lifecycle.

A paste should depend on the palette's current visibility after dismissal and focus restoration, rather than a prior
notification. Check that state on the UI thread before sending the gesture. Already-hidden windows must allow repeated
background actions. A newly summoned palette must refuse the paste, and a stalled UI must remain cancellable. Preserve
the existing target-app guard and clipboard ownership/restore rules.

## Regression boundary

The background shortcut journey replaces a different note draft on each of three invocations without opening the
palette between them, and checks restoration of each prior clipboard. The actual Dictate journey uses the private
PulseAudio microphone, fixture transcription backend, and user-bound global shortcut through to note insertion.
Visibility tests cover repeated hidden checks, queued dismissal, a newer summon, and cancellation before UI response.

Run runtime verification only in CI or the isolated rig. Linux insertion and fixture transcription do not establish
macOS Accessibility permission, actual microphone hardware, or focus behavior in another installed application.
