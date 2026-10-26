# Keep Pi in charge of AI session recovery

Nevermind hosts Pi's agent loop and reports its settled result. An assistant error during a run is an attempt outcome: Pi may still compact the context or recover a transient provider failure. The host reports one final result after Pi settles, and user cancellation takes priority over recovery.

Overflow compaction is enabled for builder, conversation, and extension sessions. Builder sessions retain two transient provider retries; general sessions retain none. Pi owns both budgets. Short tool-free requests keep the direct streaming path so they do not pay the session setup cost.

Builder discovery and source reads return deterministic bounded pages with continuation details. Generated source must be fully read at its current revision before it can be replaced. Token-limit errors remain distinguishable from request-body-size errors, so a rejected image or oversized body is not presented as a recoverable context overflow.

The behavioral boundary is exercised by the real locked Pi SDK and Nevermind adapter against a local provider protocol fixture. Packaged import and session evidence is separate from authenticated provider use and the user's reopen-edit-follow-up journey; those remain external validation.

Protocol fixtures distinguish initial history summaries, updates to an existing summary, and split-turn summaries. They classify these requests before normal user turns, since compaction requests can contain earlier user text.

Pi 1.4 recognizes a leading HTTP 413 in OpenAI and Anthropic adapter errors. The Google adapter returns a JSON-only error, so an unmodified `Prompt exceeds` response remains terminal. The backend-compatible `context_length_exceeded` message recovers through all three adapters. CI requires that recovery and separately checks that the unrecognized Google error is preserved and reported once; it does not claim every raw provider 413 recovers.
