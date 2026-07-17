# Agent Guidelines

## Workflow

* Run pnpm through mise as `mise exec -- pnpm <command>`. Do not use npm, yarn, bun, or bare pnpm directly.
* Leverage existing primitives, formatters, and OS capabilities instead of creating one-off helpers.
* Respect manual file changes and stop the app before editing `userState` to prevent overwrites.
* Commit frequently and use history to guide changes.
* Do not fix symptoms, fix diseases. After two successive symptom patches in the same code path, stop patching and audit the architecture.

## Architecture

* **Dogfood the Extension API as the app's primary development model. Treat extension APIs as declarative host points, not backdoors; fix missing primitives in the API instead of bypassing it with bespoke native code, and keep commands and provider items distinct to avoid duplication in search. When adding background or automation primitives, migrate an existing Nevermind host job first, then expose the extension API.**
* Keep files small and focused. Refactor slow patterns when encountered.
* Model OS/desktop integrations as intent-named capabilities; keep platform checks in the capability layer and see `src/docs/os-architecture.md`.
* Maintain native behavioral contracts (shortcuts, icons, async lifecycle) when migrating features to extensions.
* Keep every action/search/view payload that crosses Electron IPC clone-safe and privilege-safe; strip handlers/functions after registering them, tokenize privileged actions or expose opaque host-owned handles, and add clone-safety checks for new payload shapes.
* In extension API host-layer routing (`extension-ui-api.ts`), discriminate on properties unique to one input shape before checking optional shared fields — the `kind` field is semantically overloaded across `ExtensionFileKind` and preview-item descriptors, and routing by it first causes misroutes when both shapes overlap.
* Keep desktop/backend API changes backward-compatible for supported released clients; see `src/docs/backend-api-compatibility.md`.
* Installed user extensions live in `app.getPath('userData')/extensions` (macOS: `~/Library/Application Support/nvm/extensions/`); read that directory directly to inspect an extension's source instead of searching the filesystem.
* When fixing bugs, evaluate how the system would look if built from scratch and propose improvements.

## Product and UX

* Use the Command-K palette for all interactions. Avoid `window.confirm`, `alert`, or dedicated settings windows.
* Show cached snapshots immediately and refresh in place. Do not disrupt navigation history with passive loading states.
* Do not block palette first paint, typing, or command execution on decoration/enrichment work such as running status, icons, thumbnails, metadata, or heavy refreshes.
* Reset filters and search during navigation unless inheritance is explicit.
* Reserve empty states for active result lists; do not show them in passive surfaces.
* Use canonical storage (`Command+Alt+K`) and symbol display (`⌘⌥K`) formats via `shortcutLabel`.

## Style and Documentation

* Prefer descriptive function naming over comments.
* Prefer named functions over anonymous arrow functions.
* Keep `AGENTS.md` concise. Move detailed guides to `src/docs/`. Document intention, NEVER implementation.
* Before debugging recurring architecture issues, search `src/docs/solutions/` for prior root causes and verification notes.
* Treat `src/resources/nevermind-extension-api.d.ts` as the self-documenting source of truth for the app's extension platform: runtime contract, validation types, author guidance, and AI builder context. Update its types/TSDoc with every API change and do not recreate API reference markdown.
* For selectable or multi-item extension and fixture UI, follow the list-first rule in `src/docs/list-first-extension-ui.md`; only use its documented exceptions.
* Every extension API endpoint that renders UI must have a dev-only fixture under `src/fixtures/`; use `.agents/skills/extensions-ui-test/SKILL.md` for dogfooding and see `src/docs/extension-api-ui-fixtures.md`.
* Use CSS variables from `src/styles.css` (e.g., `--accent-*`, `--radius-*`). Avoid hardcoded colors or radii.

## Verification

* **Bootstrap first.** Trust the checked-out `mise.toml` when prompted, then install with `mise exec -- pnpm install --frozen-lockfile` when dependencies or tools are missing. Treat absent dependencies, tools, or incomplete Electron setup as bootstrap/tooling failures before interpreting test failures; call a failure “pre-existing” only after it reproduces from this state or baseline/CI evidence corroborates it.
* **Match checks to scope.** Backend-only changes run `mise exec -- pnpm -C backend check` and `mise exec -- pnpm -C backend test`. Desktop/shared changes run the relevant root checks, including `mise exec -- pnpm typecheck` and `mise exec -- pnpm test`; use `mise exec -- pnpm verify` for cross-cutting or release-sensitive changes when practical. For applicable palette or interaction changes, also use `mise exec -- pnpm palette:debug` and verify navigation, action panels, shortcuts, and dismissal behavior.
* **Gate PR readiness.** Before pushing, run `mise exec -- pnpm check:changed <base-ref>`, where `<base-ref>` is the PR target/base SHA. Diagnostics in every touched file are blocking, including pre-existing lines surfaced by the changed-file policy; use behavior-preserving formatting or narrowly justified suppressions only when appropriate. This package command is the agent-facing default, not the exact CI changed-file implementation; claim CI parity only when using CI’s exact base-SHA diff, path filtering, and Biome options.
* **Keep completion state truthful.** A repository issue stays active while a linked PR is open, review has requested changes, required CI is pending or failing, or a required reported-user journey is failing or unverified; pushed code, local checks, and draft or open PRs are progress, not completion. When user testing fails, return the work to active implementation or review, record the terminal failure and narrow repair, and keep it open until the repair is independently reviewed and the journey is re-verified or explicitly deferred. Final handoffs must say `ready for review`, `approved awaiting merge`, `merged awaiting external/user validation`, or `complete`; use `done` only for `complete`, never because a PR was opened, superseded, or partially merged.
* **Bind evidence to its artifact and boundary.** Identify the branch or commit and automated checks that prove repository behavior separately from packaged-app, permission-bound, external, or user-reported journeys. A later repair or follow-up must earn its own evidence; a previous PR’s green result does not prove it. Name the strongest boundary actually validated and record anything beyond it as deferred.
* **Report external/deferred validation.** Final handoffs must list the exact commands actually run separately from credential, deployed-preview, packaged-app, or hardware checks that were deferred. Describe deferred coverage as outstanding validation, never as a full verification pass.
* **Operational work.** Before code or a PR, classify work as repository code, external configuration, or both in the first durable update. Restate the remaining acceptance criteria, target service/project/environment, and split safe agent work from confirmation-required changes and genuinely dashboard/human-only steps. Before calling vendor work inaccessible, read the relevant `production-debugger` playbook and use its safe read-only capability, authentication, and project-scope probes; an unlinked worktree or absent local config is not proof of missing access. Prefer test/preview targets, confirm destructive, security-sensitive, billing, release, or live changes, and verify intended public aliases after configuration or deployment changes. Do not open incidental code or documentation PRs that do not meet an acceptance criterion. Report code and external validation separately; completion requires evidence for every requested outcome and plainly lists anything remaining.
* **External CLIs and live journeys.** For code that drives an external CLI or provider, verify the supported command and arguments against the installed tool, add contract coverage, and run the closest safe authenticated smoke when practical. Cover material identity and permission variants, including upstream-owner/direct-write and contributor/fork paths; mocked success is not end-to-end proof. For a live journey, name its terminal outcome and target host/environment, then report the last boundary actually verified. Redirects, invalid or synthetic callbacks, state probes, healthy APIs, and Ready deployments are useful partial evidence—not proof of a successful user journey.
