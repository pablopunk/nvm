# Dictation Never Loses Audio

## Problem

Dictation sometimes failed at once with "Dictation is not available on this
system yet." A second try worked right after, and the words of the failed
dictation were gone: they were not in Dictation History.

Three faults combined:

- `platform.ErrUnsupported` is the standard `errors.ErrUnsupported`, and
  `syscall.Errno` matches it for ENOTSUP, EOPNOTSUPP and ENOSYS. A passing OS
  error from one call (starting the microphone helper, a pipe, a socket under a
  backend error) therefore read as "this system cannot dictate". The failure
  copy also checked the microphone before the backend, so a lost connection
  that carried such an errno got the same words.
- Only segment transcription was retried, and only for three backend kinds.
  Opening the microphone, the readiness wait and a stream that died mid-way
  failed at once.
- Audio reached disk only after Stop, once the backend preparation finished.
  A capture error, an encode error, a cancel, a crash or a quit before that
  point dropped everything. The store and the history view also pruned
  unfinished recordings past a count.

No failure was logged, so the app log could not name the call.

## Contract

- The recording is on disk from its first frame (record first, then each
  segment replaced atomically about once a second) and stays until its text is
  in or the user deletes it. Only digital silence may be dropped.
- Every failure is retried unless it is known to be permanent: the account's
  refusals (sign-in, credits, spend cap, incompatible version, other
  rejections), a refused microphone, a missing codec, the port's own
  unsupported sentinel with no errno behind it, a cancellation.
- One policy (six tries, 1 to 15 s backoff, about 30 s) covers opening the
  microphone, the device list for the readiness wait, the readiness wait,
  reopening a stream that failed mid-recording, and each transcription.
- The user sees an error only after the retries, and it says the recording was
  saved only when every segment is on disk.
- Every user-visible failure is logged as `dictation.failed` with the raw
  error, and every retry as `dictation.*-retry`.

## Verification

`internal/dictation/reliability_test.go` covers a mid-recording capture error,
a transient start failure, transcription retries up to the cap, a permanent
failure, a cancel after speech, and a crash during recording and during
transcription. `internal/platform/real/pcm_process_test.go` covers a stream
read after Close ending with EOF instead of a closed-file error.
