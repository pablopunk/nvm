# Trusted local extension model

Nevermind local extensions, including AI-generated extensions, are trusted code. The user’s explicit Enable or Apply action is the trust decision. Once enabled, an extension runs locally with full access to the user’s computer.

`capabilities` are review metadata. They communicate intended behavior in the Extensions surface and generated-source review, but they are not a sandbox, permission prompt, or runtime access-control mechanism. Enabled extensions receive the complete host extension context whether their capability list is populated, empty, or omitted.

The legacy `permissions` manifest field remains a read-only compatibility alias for existing local source. When `capabilities` is absent, Nevermind displays legacy values as declared capabilities. New built-in and generated extension source must use `capabilities`.

The only permissions enforced by the host are genuine platform boundaries, such as Chromium webview allowlists and operating-system privacy grants for camera, microphone, screen recording, accessibility, and similar OS services. Those concepts must not be described as extension capability enforcement.

Generated source is staged as a persisted proposal and is never imported merely because it was written. The Extensions review surface shows current and proposed source. Enable or Apply evaluates and prepares the candidate once, persists the intended source/state, then atomically swaps host-managed actions, jobs, file watchers, and shortcuts. A failed activation retains the proposal and restores the previous source, enabled state, and live runtime.
