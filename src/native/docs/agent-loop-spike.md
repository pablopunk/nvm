# WP2.4a agent loop spike

Status: report for review. Experimental prototype in `src/native/internal/ai/spike`. Nothing in the app imports it.
Scope: PLAN.md decision D2 (AI agent loop), section 0 table, WP2.4a and WP3.9.

## Decision summary

- **Recommended D2: native Go loop on the backend's chat completions route, behind `ai.Agent`** (the default already in PLAN.md). The prototype shows the loop shape, the tool policy hook and a port of the 1.x credential path guard (with two deliberate divergences, section 4.3), all testable without a network.
- **Two conditions before WP2.4b starts.** (1) The owner confirms which upstream API the production active model resolves to for each tier. Chat completions serves only `openai-completions` routes; an `openai-responses` route gets a 409 (section 3). If any tier needs Responses, WP2.4b-1 must add that wire format on the Go side. (2) The owner accepts the gaps listed in section 3.4, especially compaction and route coverage.
- **Keep a Node `pi` sidecar as a builder-only fallback, decided at WP3.9 with eval data.** Do not start it now: it adds a Node runtime to the bundle and moves tool policy into JavaScript.
- **Do not pursue pi in moejs for WP2.4.** Godwit runs only `pi-ai` and `pi-agent-core` there. The 1.x chat and builder depend on `pi-coding-agent` (sessions, compaction, retry, extension hooks, web access), which is the part no evidence shows running in a pure-Go JS engine.
- **Do not adopt the thin hybrid as the default.** It keeps two agent implementations and two policy implementations, one of them in Node.

## 1. Inventory of the 1.x agent

Source: `src/app/electron/ai/`. "pi" is `@earendil-works/pi-coding-agent` 1.4.0 with `pi-ai` 1.4.0 and `pi-web-access` 0.24.0 (`package.json`).

### 1.1 Tools

| Surface | Tools | Implemented by |
|---|---|---|
| Conversation chat (`toolMode: 'conversation'`) | `read`, `grep`, `find`, `ls` (pi built-ins, cwd = AI workspace), `web_search`, `fetch_content` (pi-web-access) | `ai.ts` `createGeneralSession`; allowlist `ai-tool-policy.ts` (`CONVERSATION_AI_TOOLS`); web names `ai.ts` `CONVERSATION_PI_WEB_ACCESS_TOOLS` |
| Builder chat (extension builder) | Read: `read_extension_api`, `list_capabilities`, `list_shortcuts`, `read_app_logs`, `list_extensions`, `read_extension`, `read_current_extension`. Write: `write_extension`, `read_preview_run`, `remove_extension`, `validate_extension`, `install_extension` (no-op kept for compatibility). Web: `web_search`, `source_check`, `fetch_content`, `get_search_content` | `ai-builder-tools.ts`, `ai-builder-read-tools.ts`, `ai-builder-write-tools.ts`, `ai-builder-tool-shared.ts`, `ai.ts` `BUILDER_PI_WEB_ACCESS_TOOLS`, `createBuilderTools` |
| General session (no tools) | none (`noTools: 'all'`) | `ai.ts` `createGeneralSession` |
| One-shot prompt | none; direct model stream when no session and no conversation mode | `ai-tool-policy.ts` `aiPromptUsesDirectModel`, `ai-one-shot-result.ts` |

Builder write tools stage through `stageAiGeneratedExtension` (atomic promotion), validate with `validateTypeScriptExtension`, and may only touch files the chat owns (`ai-extension-ownership.ts`, `canWriteExtension`). `write_extension` requires the file to have been fully read at its current revision (`readFiles` in `ai-builder-tools.ts`). `read_preview_run` waits up to 10 s for the user's preview (`ai-preview-run-observer.ts`, `PREVIEW_RUN_WAIT_MS`).

### 1.2 Session lifecycle

- Chats are records in user state with `kind` `builder` or `conversation` (`ai-chat-records.ts`). Draft chats are promoted on first send (`ai-chat-send.ts`).
- `sendAiChatMessage` rejects empty input, a chat whose model is changing, and a chat that is already responding (`pendingAiChatSends`, `activeConversationRequests`). It sets the active chat.
- Conversation: `ai-chat-send.ts` `sendConversationMessage` opens `getNevermindAi().session(chatId, {toolMode: 'conversation'})` and streams. On an error or abort with no response activity, it resets the session. Session state is in memory (`SessionManager.inMemory`); the transcript is in user state.
- Builder: `ai.ts` `sendBuilderMessage` serializes behind the previous run, checks the cached credit notice, and emits `start`. The pi session is created once per chat (`createBuilderSession`) and re-resolves the model before each prompt by character count (`sessionModelPreparations`).
- Shutdown: `ai-chat-session-teardown.ts` `disposeAiSessions` aborts pending sends, aborts conversations, waits for them to settle, then disposes all sessions.
- Learning review runs when a builder chat exits (`ai-chat-session-teardown.ts` `noteAiChatExited`), see 1.7.

### 1.3 Events the UI consumes

Broadcast as `ai:chat:event` to the palette and extension windows (`ai-chat-events.ts`). Emitted from `ai.ts` and `ai-chat-send.ts`:

| Event | Payload | Notes |
|---|---|---|
| `start` | `chatId`, `traceId` | Builder emits it before the first model call; conversation maps it to status `running`. |
| `user_message` | rendered message | Emitted by `ai-chat-send.ts` after the user message is stored. |
| `delta` | `text` | Only `text_delta` assistant events pass (`pi-session-events.ts` `isMessageUpdateEvent`). |
| `tool_start` / `tool_end` | `name`, `isError` | Emitted by conversation, one-shot and builder sessions from pi's tool execution events; builder write tools also emit their own events through `observeBuilderTool`. |
| `extension_activated` | `filename`, `preview` | From `write_extension`. |
| `credit_warning` | `message` | Builder only, when the cached notice is `low`. |
| `error` | `message`, `data` (limit notice) | `data` is an `AiLimitNotice`, see 1.5. |
| `done` | `data: {durationMs, timeToFirstDeltaMs}` | Terminal. |
| `aborted` | none | Terminal. |
| `ready` | none | Builder session created. |
| `debug` | `label`, `data` | Labels include `model`, `tools`, `pi-tool-policy`, `pi-session-budget`, `model-source`. |

Terminal events are deduplicated (`ai-chat-send.ts` `terminalEvent`). The terminal outcome is decided once by `ai-session-outcome.ts` `resolveAiSessionOutcome`, which checks in order: user abort, pi abort, recovery abort, then an assistant `aborted` stop, a thrown error, a pending recovery error, an unsettled session, a missing assistant message, an assistant `error`, and a `toolUse` stop left pending.

### 1.4 Cancellation

- Abort before start: `pending.aborted` is set and the send returns without calling the model (`ai-chat-send.ts`).
- Abort during a run: `activeRun.abortRequested = true` and `session.abort()` (`ai.ts` `abort`, around lines 630 and 773). Bound to the caller's `AbortSignal` by `bindAbortSignal`.
- User cancellation takes priority over pi's own recovery (`ai-session-outcome.ts`; learning recorded in `docs/solutions/pi-agent-session-recovery.md`).

### 1.5 Credit and limit handling

- Active model: `GET /api/v1/active-model` returns `credits {paid, free, total}`, `notice` (`ok`, `low`, `blocked`) and `noticeReason` (`free_spend_cap_reached`). `ai.ts` `resolveAiModelAndAuth` stores this as `creditInfoRef`.
- Builder only: a `blocked` notice ends the send locally with an `error` event and no model call. A `low` notice emits `credit_warning`. Conversation chats do not pre-check; they rely on error mapping.
- Error mapping: `ai-limit-notice.ts` `aiLimitNoticeFromError` matches the error text, not a code. Kinds: `insufficient_credits` (402), `free_spend_cap_reached`, `rate_limited` (with retry-after parsed from text), `provider_busy` (429), `prompt_too_large` (413), `request_too_large`, `unsupported_request_shape`, `pricing_unavailable` (`ai-pricing-notice.ts`), `unsupported_client`. The Go client has a structured `Code` field instead (section 3.3).
- Retries: pi owns transient retries. Builder sessions keep 2 (`retry: {enabled: true, maxRetries: 2}`); general sessions keep none (`ai.ts`). Overflow compaction is on for all sessions with a 16,384-token reserve (`pi-runtime.ts`, `PI_AI_COMPACTION_RESERVE_TOKENS`).

### 1.6 Retention

- Conversation chats expire 24 hours after their last update (`ai-chat-retention.ts`, `AI_CONVERSATION_RETENTION_MS`). Builder chats are not expired by this module.
- Chat images are capped at 8 per message, 2048 px, 2 MB each (`src/app/shared/ai-chat-images.ts`). Removing a chat removes its image directory (`ai-chat-lifecycle.ts`).

### 1.7 Learnings

- After a builder chat exits with tool calls, more than two messages, or extension events, a 60-second host job asks the model for a small JSON rule set and replaces the local learnings (`ai-learning-review.ts`, `ai-chat-session-teardown.ts`).
- Up to four relevant learnings are appended to future builder prompts (`relevantLearningContext`).

### 1.8 Tool policy and credential path guard

- **Allowlist.** `pi-session-tool-policy.ts` `ensurePiToolAllowlist` requires the active tools to equal the allowed names, every allowed tool to be `direct`, and no discovery tools (`tool_search`, `codemode*`, `mcp__*`). It disposes the session and throws on a violation. Conversation sessions also require every `CONVERSATION_AI_TOOLS` entry to be present (`ai-tool-policy.ts` `missingConversationAiTools`).
- **Guard.** `ai-credential-path-guard.ts` runs as a pi extension. On `tool_call` it blocks `read`, `grep`, `find` and `ls` when the requested path is an env file (`.env`, `.env.*` except templates), a credential location under the home directory (`.ssh`, `.aws`, `.gnupg`, `.config/gh`, browser profiles, keychains and others), or an extra denied path. Allowed paths override only extra denied paths. Symlinks are resolved, so a link into a credential location is blocked. On `tool_result` it removes grep, find and ls output lines whose path is a credential location.
- **Production configuration.** `ai.ts` (around line 1924) passes `homeDir: os.homedir()` and `extraDeniedPaths: nevermindPrivatePaths(userData, agentDir)`. **It passes no allowed paths and no root.** Conversation and builder reads can therefore reach any non-credential file on disk, not just the AI workspace.
- **Fail-open.** Both handlers wrap their work in `failOpenOnInternalErrorAsync`, and the test suite asserts that garbage events are allowed. An internal error or malformed input means the call proceeds.

### 1.9 Test surface

- `ai-credential-path-guard.test.ts` (13 cases, ported in full in `pathguard_test.go`), `ai-tool-policy.test.ts`, `pi-session-integration.test.ts` (the real pi SDK against a local provider protocol fixture, see `packaged-pi-session-fixture.ts`), `ai-session-outcome.test.ts`, `ai-one-shot-result.test.ts`, builder tool tests.

## 2. Options compared

### 2.1 The four options

| | Option | What it is | Evidence in this spike |
|---|---|---|---|
| A | Native Go loop on `POST /api/v1/chat/completions` | Go streams the chat, runs tools in Go, appends results, repeats. | Built and tested (`internal/ai/spike`, 41 test functions). |
| B | Node `pi` sidecar over stdio | A Node child process runs `pi-coding-agent` in its RPC mode (`dist/modes/rpc/rpc-mode.d.ts`: JSON commands on stdin, `AgentSessionEvent` objects and responses on stdout, extension UI requests). | Read the RPC types. Not run. |
| C | pi in a pure-Go JS runtime (moejs), as Godwit does | `pi-ai` and `pi-agent-core` compiled to a bundle and run in moejs; Go keeps HTTP, storage and approvals. | Not installed (rule). Source: `src/native/docs/mygo-practices.md` G1 (Godwit ADR 0024), not the ADR itself. |
| D | Thin hybrid | A for conversation; B for the builder only. | Combination of A and B. |

### 2.2 Criteria

| Criterion | A: native Go loop | B: Node pi sidecar | C: pi in moejs | D: hybrid |
|---|---|---|---|---|
| Tool and event parity | Conversation tools and all builder host tools are implementable in Go. Events cover start, delta, tool start and end, error, aborted and done. Missing: compaction, automatic retry, JSON Schema argument validation, credit warnings and debug events. | Full by construction; same pi version and hooks as 1.x. | Only `pi-ai` and `pi-agent-core` are shown to run (Godwit). `pi-coding-agent` sessions, compaction, retry and hooks are unproven. | Conversation as A; builder as B. |
| Failure isolation | In-process. Needs `safego` around each turn and tool. A hung tool is bounded only if it honours its context. | Process boundary. A sidecar crash is contained and restartable. | In-process JS engine; interrupt-based pre-emption (Godwit ADR 0008/0024, inferred). | Mixed. |
| Install size and runtime | No runtime beyond Go. | Node must be bundled and code-signed for macOS hardened runtime. Node itself not measured here. On disk in this lockfile, as installed: `pi-coding-agent` 26 MB (includes its nested dependencies) and `pi-ai` 6.6 MB. | No Node. A moejs dependency that is alpha and single-maintainer (per mygo-practices G1). Bundle built with bun and committed. Size not measured. | Node bundled, as B. |
| Update path | Go modules (`go.mod`) and the backend compatibility manifest. | `.github/workflows/pi-updates.yml` (below). Reusable as is, since the Go app would ship the same Node bundle. | Godwit pins an alpha moejs tag and commits a compiled bundle that CI checks for currency. | As B for the builder. |
| Test determinism | High. `MockModel` scripts every turn; 41 tests, race-clean, no sleeps, cancellation by channel handshake. | Medium. Process spawn and JSON framing; 1.x uses a local provider fixture. | Unknown. | Mixed. |
| Security: where tool policy is enforced | Go. The guard is ported and fails closed on unreadable arguments (test `TestMalformedToolArgumentsAreRefusedNotAllowed`). | JavaScript (pi hooks). The Go host sees events only, unless tools are delegated back over RPC. Keeps the 1.x fail-open behaviour unless fixed. | JavaScript for tools; Go for HTTP and approvals (Godwit model). | Go for conversation; JavaScript for the builder. |
| Latency and memory | Lowest: no second runtime and no session setup. | Extra Node process resident memory and session start. Not measured. `docs/solutions/ai-chat-time-to-first-token.md` records that a cold pi session awaited `/api/v1/active-model`, which dominated setup time, and that prepared sessions were added to hide it. | Fresh runtime per run (ADR 0024): per-run cost, not measured. | Mixed. |
| Effort (rough estimate, not measured) | Conversation tools and loop: 3 to 4 weeks. Responses wire: 1 to 2 weeks. Messages and Google wires: 1 to 2 weeks each if needed. Compaction: 1 to 2 weeks. Builder tools: 2 to 3 weeks. | Process supervision, RPC client, Node bundling and signing, packaging: 3 to 4 weeks, then ongoing pin maintenance. | Spike 4 to 6 weeks at high uncertainty, before any product work. | Sum of A and B for the builder. |
| AI builder fit | Builder host tools are Go code. Validation today is TypeScript-based (`validateTypeScriptExtension`), so WP3.9 must decide what the builder generates (Yaegi-era Go per D1). | Builder works as in 1.x, validator included. | Same as B, if pi-coding-agent runs. | Builder as in 1.x. |
| Route coverage | Chat completions only (section 3). Responses routes get 409; other formats need Go wire adapters. | All four upstream formats through pi-ai adapters. | All formats if moejs runs the adapters. | Builder all; conversation completions only. |

### 2.3 The pi update pipeline (`.github/workflows/pi-updates.yml`)

- Runs weekly (Monday 07:23 UTC) and on demand.
- `resolve-candidate` checks out the default branch, installs with the frozen lockfile, and runs `resolve-pi-update.cjs`. That script reads npm registry metadata for `@earendil-works/pi-ai` and `@earendil-works/pi-coding-agent`, selects the newest coherent stable release, and applies a minimum release age (`DEFAULT_MINIMUM_RELEASE_AGE_MINUTES` = 1440, 24 hours; `pi-update-utils.cjs`). It then installs the candidate graph and uploads only `package.json`, `pnpm-lock.yaml` and metadata.
- `publish-pr` runs only on the default branch. It validates artifact provenance and allowed data changes (`publish-pi-update.cjs --validate`), requires a repository GitHub App (`PI_UPDATE_APP_ID`, `PI_UPDATE_APP_PRIVATE_KEY`), and opens or updates one bot-owned PR.
- Other scripts in `.github/scripts/` cover change classification, CI gating and evidence checks.

For Options A and C this pipeline is not used. For B and D it is directly reusable.

## 3. Backend facts

### 3.1 What the proxy accepts on `POST /api/v1/chat/completions`

Source: `src/backend/src/pages/api/v1/chat/completions.ts` and `src/backend/src/lib/proxy.ts` (`proxyAndBill`), `limits.ts`, `upstream.ts`.

- **No request-shape validation on this route.** It has no `validateRequestBody`. The body is parsed as JSON and forwarded with a few rewrites. Unknown fields are forwarded.
- **Rewrites.** `model` is replaced with the active model id. `service_tier` is deleted. `max_tokens` (or `max_completion_tokens` when the client sent it) is set to the server's output cap. For streams, `stream_options.include_usage` is forced to true. OpenRouter routes also drop `models`, `route`, `provider`, `plugins`, `web_search_options`, `reasoning_effort`, and set `reasoning`.
- **Tools.** `tools` is forwarded unchanged when every entry is a client-executed definition. `containsUnpricedProviderTool` (`limits.ts`) rejects any entry whose `type` is not `function` or `custom`, and any body with `mcp_servers`, with a 400 `unsupported_request_shape`. The message text says "OpenRouter web search", but the rule is generic. An entry with no string `type` passes only when it carries a `name`, `input_schema` or `function` key; any other typeless entry is rejected.
- **`tool_choice`, `parallel_tool_calls`, `functions`, `function_call`, `response_format`.** Forwarded without checks.
- **Tool messages.** `role: "tool"` messages and assistant `tool_calls` are forwarded unchanged (contract test `contract-routes.test.ts` around line 2950 covers a non-streamed `tool_calls` response and tool forwarding).
- **Streaming.** The SSE bytes are forwarded verbatim (`teeStreamAndBill` enqueues each upstream chunk). Usage is sniffed from the stream without changing it. Tool-call argument text is counted for billing when usage is missing.
- **Limits.** Body up to 4 MiB (`MAX_PROXY_REQUEST_BODY_BYTES`). `n` must be 1 to 128. The estimated input, which counts all text including tool schemas (`estimateInputTokensFromValue` walks the whole body), must not exceed `MAX_INPUT_TOKENS`, else 413 `prompt_too_large`. Multimodal payloads are limited to small inline images.
- **Route format.** The proxy compares the requested wire API with the route's API only for responses: a responses-shaped route on this endpoint gets 409 `model_route_changed` (`proxy.ts` around line 1511). Routes are chosen by `selectApiForModel`, which maps the catalog SDK (`@ai-sdk/openai` to responses, `@ai-sdk/anthropic` to messages, `@ai-sdk/google` to Google, anything else to completions; `upstream.ts`). Provider format lists: `opencode_zen` supports all four formats; `openrouter` and `openai` support completions and responses; `anthropic` supports messages only; `google` supports Google only (`upstream.ts`).

### 3.2 `/api/v1/responses` and `/api/v1/messages`

- **Responses** (`responses.ts`, `lib/responses-request.ts`): strict. Only the listed top-level fields; `input` must be text or a list of `message`, `function_call`, `function_call_output` or `reasoning` items; `tools` must all be `type: "function"`; `tool_choice` is `auto`, `none`, `required` or a function; `include` is only `reasoning.encrypted_content`; `store` is forced to false.
- **Messages** (`messages.ts`): no request validation. Anthropic client tools carry `name` and `input_schema` and pass the priced-tool check.
- **Google** (`models/[...path].ts`): `generateContent`, `streamGenerateContent` and `countTokens` only; `alt` is `sse` or `json`; requires the Google route.

### 3.3 The Go client (`src/native/internal/backend`)

- `ChatStream` sends tools and tool_choice unchanged, and message roles include `tool` and assistant `tool_calls` (`chat.go`). It is never retried (a retried stream could bill twice).
- `sse_chunks.go` decodes `delta.tool_calls` into `EventToolCallDelta` pieces keyed by index, with id and name on the first piece (`events.go`).
- `SpeaksChatCompletions()` checks the descriptor's `api` before a chat call (`models.go`).
- `Error.Code` carries the backend's machine-readable `error.type` (`errors.go`), so 409 `model_route_changed`, 413 `prompt_too_large` and 402 can be keyed on Code. The client does not send an `Idempotency-Key` on chat; the proxy accepts one (`completions.ts` line 123).

### 3.4 Does the loop work with the existing proxy?

- **Openai-completions routes: yes.** Tool definitions, assistant tool calls, tool results, streamed tool-call deltas and usage all pass through, and the Go client already speaks this wire. The prototype's `BackendModel` adapter maps `backend.Event` to model chunks. Its mapping is unit tested; its stream loop needs a live client, so it is not exercised end to end here.
- **Responses routes: no, not through chat completions.** The request gets 409. The Go side needs a Responses wire adapter (function-call items and argument deltas). The proxy's Responses route accepts that shape.
- **Anthropic-messages and Google routes: no, not through chat completions.** Use the messages or Google route with a Go wire adapter for each.
- **Gaps that need a backend change (out of scope, recorded for follow-up):**
  - Chat completions and messages do not compare the requested wire format with the route's format. Only responses is checked. A client that ignores `api` can therefore reach a mismatched upstream path (for example an Anthropic-format route on `/chat/completions`). Read from `proxy.ts`; not exercised against live providers.
  - The 1.x guard, the chat limit mapping and the builder limit mapping all depend on message text. Whether the backend always sends a structured `error.type` for every limit error was not verified for each path.
- **Not verified.** Streaming tool-call behaviour against a live provider. The only contract test seen covers a non-streamed tool call. The per-tier `api` in production is catalog-driven and was not read (no backend call was made).

### 3.5 Required contract checks before WP2.4b

- Owner runs a read-only `GET /api/v1/active-model` for the smart and fast tiers and records `api`.
- Add recorded SSE fixtures for tool-call streams (split arguments, parallel calls, malformed JSON in arguments, stream drop) derived from the backend's contract tests.

## 4. Prototype: `ai.Agent` and what it covers

Package `src/native/internal/ai/spike` (experimental; layer `ai`; imports only the standard library and `internal/backend`).

### 4.1 Interface

```go
type Agent interface {
    Run(ctx context.Context, session *Session, message UserMessage) <-chan Event
}
```

- `Session` owns the Go transcript (`Messages`) and refuses a second concurrent turn (`session_busy`). A sidecar implementation would either mirror `Messages` from its events or treat the session as an opaque ID. This is the one place the interface reflects the choice; WP2.4b should hide `Messages` behind accessors.
- `Event` kinds: `start`, `delta`, `tool_start`, `tool_end`, `error` (with a code), `aborted`, `done` (with usage). Exactly one terminal event, last, then the channel closes.
- `Model` streams `ModelChunk` values (text, tool-call delta, usage, finish reason). `BackendModel` is the adapter over the backend client.

### 4.2 Loop behaviour

- Model turn, then tool calls in call order, then tool results appended, then the next model turn. The loop stops when the model asks for no tools (`done`), when `MaxModelTurns` is reached (`error:iteration_limit`), or on cancellation (`aborted`).
- Each call goes through the policy (`Decide`), then argument validation (a JSON object), then the tool under a per-tool timeout, then `FilterResult` and an output cap at a rune boundary.
- A refused, unknown, malformed or failing call returns an error outcome to the model; the loop continues.
- Cancellation answers every call that did not run with a fixed text, so the transcript stays valid for the next turn. This matters because the openai-completions wire requires an answer for every tool call id.

### 4.3 Path guard port

- `pathguard.go`: credential locations, env files and templates, symlink canonicalization through a `Resolver` port, production-style extra denied paths, and an allowed carve-out. Case-insensitive matching.
- `pathpolicy.go`: guards `read`, `grep`, `find` and `ls`; redacts search output with the same line-number boundary rule as the TS regex, written out because Go regexp has no lookahead.
- Divergences from 1.x, all deliberate:
  - Fails closed on unreadable arguments (1.x fails open).
  - Optional roots confine every resolved path (1.x has none; production sets none). A link inside a root that points outside is refused.
  - The guard resolves through an injected `Resolver`, so tests run against an in-memory tree. The real-file-system tests use `t.TempDir` and skip where symlinks are refused.

### 4.4 Mock model

`MockModel` replays scripted turns, records every request, counts closes, and can stall until the context ends. `NewRepeatingMockModel` gives an endless tool-call loop.

## 5. Test results (run in the box, slot `wp24`)

Commands actually run:

- `gofmt -l .`: no output (clean). One file was formatted before the final run.
- `go vet ./...`: clean.
- `go test -race -count=1 ./...`: all packages `ok`, including `internal/ai/spike` (41 test functions), `internal/layers` (layer and cycle rules), `internal/platform` and its grep test (no `time.Now`, no `os/exec`, no `net/http` outside the allowlist), `internal/backend`, `internal/app`, `internal/design` and goldens.
- `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...`: clean after removing one unused test helper (U1000).
- Mutation checks, run in the box copy only, then reverted:
  1. Removing the canonical credential roots from the extra denied list: `TestASymlinkedCredentialDirectoryIsStillDenied` and `TestSymlinksThatResolveIntoSSHAreDenied` fail.
  2. Making unreadable arguments fail open: `TestMalformedToolArgumentsAreRefusedNotAllowed` fails.
  3. Dropping the synthetic answers for cancelled calls: `TestCancelDuringAToolAnswersEveryCallAndEndsAborted` fails.

Not run, intentionally: any live backend call, any Electron or GUI launch, `pnpm` (not allowed locally), the packaged app, the live smoke test, and anything that needs credentials.

## 6. Learnings from 1.x that the Go design must keep

- Pi owns compaction and transient retry budgets. The Go loop must own both explicitly (`docs/solutions/pi-agent-session-recovery.md`).
- User cancellation beats recovery; the host reports one terminal result after the session settles.
- A 413 with a leading status is a request-size problem, not a context overflow; a token-limit message is recoverable by compaction. Keep the two distinct.
- Builder reads are bounded pages with continuation; a file must be fully read at its current revision before it can be replaced.
- Pi's default truncation is 2000 lines or 50 KB per tool output (`pi-coding-agent` `DEFAULT_MAX_LINES` and `DEFAULT_MAX_BYTES`). The spike uses a 50 KB cap.

## 7. First three work items for WP2.4b

1. **Wire and route gate.** In `internal/backend`, check `ActiveModel.API` before each turn and refetch once on 409 `model_route_changed`. If the owner's check finds a Responses tier, add a Responses adapter behind `Model` (function items and argument deltas, strict field set from `responses-request.ts`). Record SSE fixtures from the backend contract tests for split arguments, parallel calls, malformed arguments and stream drop. Map 402, 409, 413 and 429 on `Error.Code`. Acceptance: contract tests green, and no string matching on error messages.
2. **Tool runtime on the platform ports.** Move guard file access behind the `internal/platform` file-system port (the spike uses `os` through `OSResolver`). Keep fail-closed behaviour. Decide the workspace root and set it in production (1.x has none). Add JSON Schema argument validation with a small validator and golden tests. Use process-group kill for any subprocess tool, and route web tools through an egress approval that names the hosts (Godwit's rule, `mygo-practices.md` G4 and A11). Acceptance: the spike's vectors, the traversal and symlink-out-of-root cases, and a test that a cancelled tool leaves no process.
3. **Session lifecycle and budgets.** Persist the Go-owned transcript through the WP0.6 store (versioned, atomic). Keep single-flight per session. Implement stop and resume (J13). Replace pi compaction with deterministic truncation of old tool output first; decide with the owner whether summarization is needed. Retry only before the first delta, with an `Idempotency-Key` per turn. Port the 24-hour conversation retention (`ai-chat-retention.ts`). Acceptance: scripted SSE transcripts from WP2.3, L2 tests for stop and resume, retention tests.

## 8. What I could not determine

- The production `api` for each tier. It depends on the catalog and admin routes; needs the owner's read-only check (section 3.5).
- Whether pi-coding-agent runs in moejs. Not installed; Godwit's ADR 0024 was not read directly.
- Node runtime size, resident memory, and per-turn latency for each option. Not measured.
- Streaming tool-call behaviour against live providers.
- Whether every limit error path sends a structured `error.type`.
- Whether the chat-completions and messages format mismatch (section 3.4) is reachable from a shipping client. Read from code only.
