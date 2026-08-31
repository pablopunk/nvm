# Background action feedback must outlive the palette

## Problem

Background actions dismissed the palette before execution, but lightweight feedback was returned as a palette-owned toast result. The result was invisible until the palette opened again, and selected-text actions could start before the palette had released focus.

## Root cause

Palette dismissal and action IPC ran concurrently. This made selected-text fallback reject the read while the palette still had focus. Toasts also depended on a mounted, visible palette renderer, so they could not report background success or failure.

## Durable rule

- Await palette dismissal before an immediate action crosses IPC.
- Hiding a palette window does not deactivate its macOS application. Relinquish app activation before selected-text work, then show the app without focusing it so passive indicators remain available.
- Treat Accessibility retries as transition tolerance, not as a substitute for restoring source-app focus.
- Reject selected-text accessibility reads while the palette owns focus.
- Use passive indicator windows for transient feedback because they do not activate the app and remain visible after palette dismissal.
- Let the indicator host own timed dismissal; renderer timers cannot reliably outlive a hidden or replaced surface.
- Keep selected text byte-for-byte except for transport delimiters added by the OS integration.

## Verification boundary

Static coverage must include palette dismissal ordering, focused-palette selection rejection, timed indicator replacement and dismissal, extension API fixture coverage, and the absence of a public toast API or palette toast component. Packaged macOS validation must still confirm focus restoration, Accessibility selection, clipboard-preserving fallback, replacement paste, and passive error visibility in another application.
