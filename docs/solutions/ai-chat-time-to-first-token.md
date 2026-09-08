# AI Chat Time to First Token

**Status: Implemented and user-validated** (2026-09-08)

## Problem

A short Fast AI conversation felt much slower than comparable palette assistants. Renderer work was initially a suspect, but the delay occurred before the first streamed token reached the UI.

## Measurement method

Record the line count in `.tmp/dev.log`, run one real chat through the visible Electron UI, and correlate only the new lines across these boundaries:

- `ai-conversation.start-chat`
- `ai.general-session.create`
- `/api/v1/active-model`
- `credit_reserved` and `proxy_preflight`
- `upstream_first_chunk`
- `ai.ask.time-to-first-delta`
- `ai.first-painted-delta`
- `credit_reservation_settled` and `chat_completion`
- `ai.send-message`

Use the backend `request_id`, renderer `traceId`, and chat `sessionId` to avoid mixing background jobs or search traces into the request timeline.

## Baseline

For a new Fast chat with the prompt `hey`, the measured path was:

| Boundary | Duration |
| --- | ---: |
| Start-chat IPC | 0.9 ms |
| Session and model setup | 1.247 s |
| `/api/v1/active-model` | 1.208 s |
| Backend request to credit reservation | 1.151 s |
| Credit reservation to first upstream chunk | 4.452 s |
| Backend request to first upstream chunk | 5.603 s |
| First upstream chunk to Electron delta | 6 ms |
| Electron delta to painted token | 12 ms |
| Start to first painted token | 6.874 s |
| Start to completed UI request | 7.879 s |

The request used `openrouter/~deepseek/deepseek-v4-flash-latest` and reported 2,778 input tokens for a three-character user message. Provider and backend work dominated; renderer paint was not the problem.

## Root causes

### Warm-up was a one-use startup optimization

Startup created one prepared conversation session. Taking it removed it from the prepared-session map, and a prepared session with a chat ID was not registered in the general-session cache. A later chat therefore repeated active-model resolution and session creation on the blocking path.

### Active-model resolution was synchronous with the first prompt

Creating a cold session awaited `/api/v1/active-model`. That route performs authenticated account, balance, routing, pricing, and descriptor work. In the measured request it accounted for almost all session setup time.

### Backend admission repeated independent and stable work

Monthly free-credit setup ran on every request in the process, and the minute and daily Upstash checks ran serially. The proxy did not emit enough phase data to distinguish routing, request preparation, deduplication, rate limiting, and reservation work.

### Stream closure waited for billing persistence

After the upstream stream ended, the response stayed open while usage, ledger, reservation, and deduplication writes completed. This did not affect the first token, but it delayed visible request completion by about 0.67 seconds.

### Provider health was not a latency benchmark

The admin Probe action requested the provider model catalog. A healthy catalog endpoint did not prove that the selected model could run or return its first byte quickly.

## Fix

### Warm on palette open

`src/app/electron/palette-window.ts` now exposes a non-blocking palette-open callback. `src/app/electron/main.ts` uses it to prepare the Fast conversation configuration, with a 30-second throttle, without delaying palette paint.

`src/app/electron/ai.ts` now lets `generalSession()` consume prepared fallback sessions and stores a prepared session under its chat-specific cache key. Later messages reuse the same session instead of treating the warm session as an uncached one-shot object.

### Reduce and trace proxy preflight work

`src/backend/src/lib/users.ts` remembers a successfully completed monthly grant for each user, period, and database runtime. Correctness remains in the database, while repeated requests in one warm process avoid the same transaction.

`src/backend/src/lib/ratelimit.ts` runs independent minute and daily Upstash checks concurrently.

`src/backend/src/lib/proxy.ts` emits `proxy_preflight` with routing, request preparation, deduplication, rate-limit, admission, payload-size, message-size, system-size, tool-size, tool-count, and estimated-token measurements. These fields identify the next blocking edge without logging prompt content.

### Close normal streams before settlement completes

Normal streaming completion starts billing finalization through Vercel `waitUntil` and closes the client stream immediately. Cancellation remains synchronous because its release-or-settle transition is part of safe disconnect handling.

The response status, headers, framing, and body are unchanged, so supported desktop clients remain compatible.

### Benchmark the selected model

The admin primary and fallback Probe actions now send a one-token streaming request in the selected model's OpenAI, Anthropic, or Google API format and report time to first response byte. The provider-table Probe keeps its catalog-connectivity behavior.

## What not to do

- Do not optimize renderer token paint before measuring it; it was approximately 12 ms here.
- Do not call a catalog health check a model latency test.
- Do not use a startup-only warm slot without defining ownership after consumption.
- Do not move cancellation billing into an untracked background task.
- Do not infer prompt bloat from user-message length alone; compare `message_bytes`, `system_bytes`, `tool_bytes`, and provider-reported input tokens.

## Verification

The user reported that AI response speed was “way better” after the changes.

`git diff --check` and the applicable desktop Biome check passed during implementation. Runtime tests, typechecks, and builds were intentionally left to CI under the repository verification policy. A post-change trace with exact timings was not captured, so the qualitative user result is the strongest validated boundary.

## Notes for future searches

Search for: Fast AI latency, AI chat TTFT, time to first token, `active-model`, `ai.general-session.create`, prepared fallback session, palette warm, `proxy_preflight`, `upstream_first_chunk`, prompt tool bytes, stream billing settlement, `waitUntil`, model Probe, OpenRouter latency.
