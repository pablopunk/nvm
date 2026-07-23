# Trusted local extension model

Nevermind local extensions, including AI-generated extensions, are trusted code. AI-generated extensions validate and execute immediately when written; there is no approval, apply, or sandbox stage. Once written, an extension runs locally with full access to the user’s computer.

`capabilities` are review metadata. They communicate intended behavior in the Extensions surface and generated-source review, but they are not a sandbox, permission prompt, or runtime access-control mechanism. Enabled extensions receive the complete host extension context whether their capability list is populated, empty, or omitted.

The legacy `permissions` manifest field remains a read-only compatibility alias for existing local source. When `capabilities` is absent, Nevermind displays legacy values as declared capabilities. New built-in and generated extension source must use `capabilities`.

The only permissions enforced by the host are genuine platform boundaries, such as Chromium webview allowlists and operating-system privacy grants for camera, microphone, screen recording, accessibility, and similar OS services. Those concepts must not be described as extension capability enforcement.

Generated source is validated, persisted, and activated as one immediate trusted write. The host evaluates and prepares the candidate once, then atomically swaps host-managed actions, jobs, file watchers, and shortcuts. A failed activation restores the previous source, enabled state, and live runtime.
