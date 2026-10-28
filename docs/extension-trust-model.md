# Trusted local extension model

Nevermind local extensions, including AI-generated extensions, are trusted code. AI-generated extensions validate and execute immediately when written; there is no approval, apply, or sandbox stage. Once written, an extension runs locally with full access to the user’s computer.

`capabilities` are review metadata. They communicate intended behavior in the Extensions surface and generated-source review, but they are not a sandbox, permission prompt, or runtime access-control mechanism. Enabled extensions receive the complete host extension context whether their capability list is populated, empty, or omitted.

The legacy `permissions` manifest field remains a read-only compatibility alias for existing local source. When `capabilities` is absent, Nevermind displays legacy values as declared capabilities. New built-in and generated extension source must use `capabilities`.

The only permissions enforced by the host are genuine platform boundaries, such as Chromium webview allowlists and operating-system privacy grants for camera, microphone, screen recording, accessibility, and similar OS services. Those concepts must not be described as extension capability enforcement.

Generated source is validated, persisted, and activated as one immediate trusted write. The host evaluates and prepares the candidate once, then atomically swaps host-managed actions, jobs, file watchers, and shortcuts. A failed activation restores the previous source, enabled state, and live runtime.

## Accepted risks

Owners have accepted the following risks as consequences of this model. They are not bugs to be fixed by adding enforcement at the current stage.

- **Web content steering the AI builder.** Pages fetched by the builder's `web_search` and `fetch_content` tools during a build are untrusted and could steer the code the builder writes. Generated extensions are trusted code that validate and activate immediately, by design, because the user asked the builder to automate something. This prompt-injection path is an accepted consequence of that model. An approval step would change the builder UX and is a deliberate non-goal for now.
- **Extension webview permissions.** Webview HTML runs in a sandboxed `srcDoc` iframe without `allow-same-origin`, popups, or top navigation. Camera, microphone, display capture, and clipboard are reachable only when the extension delegates them through `webviewPermissions` (Permissions Policy `allow`). The host permission handler gates OS-level access for the palette page only. Per-frame host checks are intentionally not added.
