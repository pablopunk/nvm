# AI code mode: conversations drive Nevermind through the Extension API

Status: phases 0 and 1 (the sandbox, the gate and notes) are built; later phases are still design. Written against the head of PR #280 (`src/native`). The section "As built (phases 0 and 1)" at the end records where the code differs from the text above it.

Idea (Pablo): instead of one hand-written tool per feature, a conversation gets one tool. The model writes a short program against the typed API extensions already use (`ctx.Notes()`, timers, clipboard history, commands). The host runs it in a sandbox and returns the result to the model. First wish list: notes, timers, clipboard, and running any command, including user extensions.

Evidence is cited as `path:line` relative to `src/native`. "Estimate" marks a number I did not measure.

## 0. Recommendation

Build it, as **one tool, `run_program`, that runs a short Go program in a throwaway Yaegi child of the existing extension runtime, behind a Go-side effect gate**. The gate is the single place where capability, effect class and the user's approval are decided. Do not hand-write notes, timer or command tools.

Three things in the current code make a naive version unsafe, and they decide the shape of the plan:

1. **The extension interpreter is not a sandbox.** `evalExtension` loads the whole Go stdlib table (`internal/ext/child/interpret.go:57`) and `docs/extension-api.md:23` lists "a sandbox" as a non-goal. I ran it: an interpreted source that imports `os` read `/etc/hostname`, and one that imports `net/http` loaded and ran (`os/exec` and `syscall` did not load). A model-written program must therefore never pass through that path as it is. The program gets a host-generated import list from a small allowlist, not the model's own imports (section 2.4).
2. **Approvals are per tool call today, not inside a call.** `Tool.Confirm` is decided once, before the tool runs (`internal/ai/agent/calls_run.go:75`). Code mode needs "the program reached `Notes.Delete`, ask now, then continue". That is a new agent primitive (section 3.3).
3. **Timers and commands are not on the API at all** (section 4). Notes and clipboard history are.

First slice: notes only (create, append, delete with approval). It forces all the hard parts (sandbox, gate, in-run approval, transcript rows) while touching an API that exists.

## 1. What exists, and what it means for the design

| Piece | Where | Relevance |
|---|---|---|
| Conversations run on the pi Node sidecar; the app owns every tool call, policy and approval | `internal/ai/pi/doc.go`, `internal/ai/pi/engine.go:290` (`answer` hands the call to `run.RunCall`) | A new tool needs **no sidecar change**: `definitions` forwards any `agent.Tool` (`engine.go:208`) and `hostTools` registers whatever arrives (`sidecar/pi-host/host.mjs:150`). |
| The agent package checks tool, schema, policy, approval, then runs under a 30 s timeout and a 16 KiB output cap | `internal/ai/agent/calls_run.go:61`, `run.go:14`, `events.go:85-86` | `run_program` is a normal `agent.Tool`. |
| Existing precedent: reading the clipboard asks the user each time, because the text goes to the model | `internal/ai/tools/clipboard.go:27` (`Confirm: true`) | Same rule for clipboard history reads in programs. |
| Conversation tool set is built in one place | `internal/app/ai_tools.go:14`, `internal/ai/tools/set.go:36` | Register `run_program` here. |
| pi session is rebuilt only when baseUrl, role, system or tool **names** change | `sidecar/pi-host/host.mjs:255` (`sessionKeyOf`) | A changed API text inside a tool description will not reach a live session. Put the API text in the system prompt or add its hash to the key (small sidecar edit). |
| Extension child: Yaegi in a supervised process, framed JSON over stdio, host-side kill at the deadline, `RLIMIT_AS` on Linux | `internal/ext/child/child.go`, `internal/ext/supervisor/runtime.go:184`, `child/limits_linux.go:8` | Reuse as the program runner. Spike numbers: spawn to ready 7 to 11 ms, kill about 1 ms (`docs/extension-runtime-spike.md` section 4.1). |
| `ProbeSource` already runs a staged source in a throwaway child with a time bound, with no services | `internal/ext/supervisor/probe.go:19`, `internal/ext/manager/control.go:160` (`Check`) | Direct precedent for a throwaway child; code mode is the same with a service handler attached. |
| Capabilities are enforced in the host on every service call | `internal/ext/services/access.go:12` (`require`) | Reuse, but the manifest is trusted for extensions; for programs the capability set is fixed in Go. |
| Destructive calls already ask the user from inside the host | `internal/ext/services/confirm.go:37`, `services/notes.go:85` (`noteDelete`: requires a `run` request, then confirms) | Right idea, wrong surface: it asks in the palette (`internal/app/extensions_host.go:272`) with only "Delete note". |
| Generated API reference from the api package, with a compact form and a size ratchet | `internal/ai/builder/apiref`, `builder/reference_gen.go`, `reference_test.go:61` (`maxCompactReferenceBytes = 39 << 10`) | Reuse for model guidance (section 5). |
| Native agent loop spike and its tool policy design | `docs/agent-loop-spike.md` | Policy, cancellation and "every call gets one answer" rules carry over. |
| 1.x prior art | `electron/ai/ai.ts:1868`, `pi-session-tool-policy.ts:69` | 1.x **disabled** pi's `codemode`, `mcp` and `tool-search` extensions and rejected any tool named like them. 1.x conversations could read files and use the web, never act on the app. There is no 1.x code mode to port. `host.mjs:181` keeps the same `disabledBuiltinExtensions`. |

A search of `docs/` and `src/native/docs` for "code mode" finds only the 1.x policy above. This document is the first design for it.

## 2. Architecture

```
chat turn (agent.Turn)  ──tool_call run_program {code}──►  pi sidecar (Node)  [no change]
        ▲                                                         │
        │ RunCall (policy, schema)                                │
        ▼                                                         │
 codemode.Tool.Run ──► source.Prepare (AST checks, host imports)  │
        │                                                         │
        ├─► supervisor.Runtime (throwaway Yaegi child, MethodProgram)
        │        │ ServiceCall (notes.delete …)
        │        ▼
        └──────► codemode.Gate ──► services.Handler (synthetic identity, fixed capabilities)
                    │  ▲
          approval  ▼  │ answer
                 agent Asker ──► chat transcript rows ──► user
```

### 2.1 Where the program runs

A throwaway child per run, started with `supervisor.New(cfg, identity, sourcePath, handler)` and `Start()`, the way `ProbeSource` does (`probe.go:19`), but with a service handler. It is not registered in the extension `Manager`: no enablement, no menu bar, no root rows, no commands cache. New pieces:

- `protocol.MethodProgram = "program"`, entry point `Main` in `protocol.EntryPoints` (`internal/ext/protocol/entry_points.go:4`). The child dispatch (`child/dispatch.go`) gets one case that calls `Main(ctx)` and JSON-encodes its single result.
- A supervisor budget for the method: hard 20 s of **active** time. The supervisor already restarts the budget after a call that waited for the user (`services/confirm.go:26-31`, `ServiceReply.WaitedForUser`), so approvals do not eat the budget.
- Source staged in a private temp directory under the data dir, removed on every path (as `Check` does, `control.go:160`).

Cost per run (spike numbers, same hardware class): about 7 to 11 ms spawn plus 2 to 5 ms to interpret a program of this size (`extension-runtime-spike.md` sections 2.1 and 4.1). Small against a model round trip. No pool is needed.

### 2.2 How pi hands code over and gets results back

No new channel. The model calls `run_program` like any host tool: the sidecar sends `{type:"tool_call", callId, name, arguments}` and waits (`host.mjs:150-170`); the engine routes it to `agent.ExternalRun.RunCall` (`engine.go:290`), which runs `decideAndRun` and sends back `{type:"tool_result", text, status}`. The text the model reads is the program's return value plus the effect journal (section 3.5), capped at 16 KiB by `capUTF8` (`agent/output.go`, `events.go:86`).

The native loop (used for the builder, and for conversations without the sidecar) gets the same tool because both go through `agent.Registry`. The builder keeps its own tools.

Constraint to keep in mind: the pi relay handles a tool call synchronously inside its read loop (`engine.go:217` calls `handle`, which calls `answer`). While a program runs, or waits for an approval, the relay cannot react to `ctx.Done()` itself. Cancellation therefore works only because `RunCall`'s context is the turn's context and both the runner and the approval wait return when it ends (`calls_run.go:90`, section 2.5). This must be a tested property.

### 2.3 Bounds

| Bound | Value | Enforced by |
|---|---|---|
| Program source | 8 KiB (estimate: enough for a dozen lines of API calls) | `source.Prepare` before anything is staged; cf. `maxCheckedBytes` at `control.go:181` |
| Active run time | 20 s, excluding approval waits | supervisor hard budget for `MethodProgram`; kill on breach |
| Each approval wait | 60 s, then "not confirmed in time" | existing `ConfirmWait` (`confirm.go:13`) |
| Service calls per run | 50 (estimate) | the gate counts, then refuses |
| Result returned to the model | 16 KiB | `capUTF8`, `events.go:86` |
| Memory | `RLIMIT_AS` 2 GiB on Linux (spike floor), resident watch `Config.ResidentLimit` everywhere | `child/limits_linux.go:8`; `limits_other.go` says no address-space cap exists off Linux, so macOS and Windows rely on the resident watch alone (1 s granularity, `supervisor/config.go` `memoryWatchInterval`). This is the weakest bound. |
| Goroutines and channels | refused by the source check (spike: `return <-ch` fails in Yaegi, blocked receives leak a goroutine, `extension-runtime-spike.md` 2.4 and 2.5) | `source.Prepare` AST check |
| Model retries | `DefaultMaxSteps = 8` (`events.go:84`) | agent loop; compile errors come back as the tool result so the model can fix them |

### 2.4 The program text and the import problem

The model sends **a function body**, not a file. The host wraps it:

```go
package main

import ( /* computed by the host */ )

func Main(ctx api.Context) (any, error) {
    // the model's statements
}
```

`source.Prepare` parses the wrapped file with `go/parser` and:

1. Rejects a body that contains `import`, `go`, `chan`, `select`, labels with `goto`, or a method or type declaration that shadows host names. It rejects dot or aliased imports by construction, since the model cannot write imports.
2. Collects the package qualifiers the body uses (`fmt.`, `strings.` …) and generates the import list from an **allowlist**: `fmt`, `strings`, `strconv`, `sort`, `time`, `errors`, `unicode`, `unicode/utf8`, `regexp`, `math`, `bytes`, `encoding/json`. A qualifier outside the list fails as an ordinary "undefined" compile error the model can read.
3. Loads the interpreter with a **restricted symbol table**: `stdlib.Symbols` filtered to those packages plus the `api` symbols (`child/api_symbols.go`). Defence in depth: even a parse bug cannot reach `os` or `net/http`. A test pins the probe result above: the same `os.ReadFile` program must fail.

Generating imports from the body also removes the most common model slip in Go, an unused import.

The child must be told it is a program child (a flag next to `ChildFlag`, `protocol.go`), so that `evalExtension` uses the restricted table. Extensions keep today's behaviour.

Not in scope here: whether extensions themselves should get an allowlist. That is a trust-model decision (`docs/extension-trust-model.md`), and this design does not depend on it.

### 2.5 Cancellation from the chat

Stop in the chat calls `Turn.Cancel` (`agent/turn.go`), which cancels the turn context. That context reaches the tool through `safego.Run` (`run.go:15-20`). `codemode.Tool.Run` must then:

1. call `Runtime.Stop()` (kills the child, about 1 ms per the spike) and return `ctx.Err()`;
2. have any pending approval return (`await` already selects on `r.ctx.Done()`, `calls_run.go:90-110`);
3. leave no process behind (test: process count before and after).

A service call already admitted to the host when the user stops may still finish; the host never replays it (`services/access.go:24`, `admitDesktopEffect`). The journal records it as "finished after Stop". This is honest and rare, since effects are short, but it is a real property: Stop is not a transaction rollback.

## 3. Safety

### 3.1 The trust boundary

The child is untrusted. Its only way to act is a `ServiceCall` over the protocol, answered by the host. If section 2.4 holds, the child has no other effect path (no `os`, no network, no exec). The gate in front of the host handler is therefore the security boundary, and it lives in Go, outside the model and outside the sidecar. Yaegi itself is not a boundary; OS-level confinement of the child is phase 5.

### 3.2 Default capabilities of a conversation program

A synthetic identity `nevermind.assistant.program` with a manifest fixed in Go (not model-selectable, not user-installable). Capabilities are the area names that `Handler.require` checks (`api/manifest.go:58-79`):

| Granted from phase | Capability | Notes |
|---|---|---|
| 1 | `notes` | read and write; delete gated |
| 2 | `timers` (new, section 4) | write |
| 3 | `clipboard.history`, system clipboard write | history reads gated |
| 4 | `commands` (new, section 4) | every run gated |
| never by default | `network`, `secrets`, `ai`, `desktop.files`, `desktop.shell`, `desktop.apps`, `desktop.selection`, `desktop.keyboard`, `system`, `settings.write`, `jobs`, `extensions.ownership`, `dictation`, `camera`, `ocr`, window management | Files, apps and links already have their own reviewed conversation tools (`tools/set.go:36`). Shell and file writes are out of the wish list. |

`require` only covers capability-gated services. Services without a capability (`storage`, `logs`, `text`, `drafts`, `views`, `scheduler`, `bundle`) are open to any extension (`services/handler.go`). So the gate is **default-deny on `(service, op)`**: a table of allowed pairs, each with an effect class, and anything else answers `permission` and is journaled as refused. `Secrets` is never in the table.

### 3.3 Effect classes and confirmation in the chat, enforced in Go

Every allowed `(service, op)` has a class, set in a Go table next to the gate (reviewed in code, not declared by the program):

| Class | Examples | Behaviour |
|---|---|---|
| `read` | notes list/get, commands list, timers list | runs, journaled |
| `write` | notes create/update, timers start/cancel, copy text to the clipboard | runs, journaled; no prompt. Additive or easily undone, and the user sees the row. |
| `sensitive-read` | clipboard history list/get, system clipboard read | prompt once per run with what is being read ("Read your clipboard history, 50 entries"); same rule as `read_clipboard` today |
| `destructive` | notes delete, clipboard history remove/clear | prompt **each time**, with a description the **gate builds from host data** (it looks the note up in the store: `Delete note “Packing list”, edited yesterday`), never from the model's words |
| `external` | run a command (section 4) | prompt each time, naming the extension and command from the host registry |

Mechanics, all new code in the agent and codemode packages:

- `agent` gains an in-run asker: `agent.AskerFrom(ctx).Ask(Effect{Label, Detail}) (allowed bool, err error)`. `turnRun` implements it with the existing `await` machinery (`calls_run.go:90`, `Turn.Decide`, `turn.go:80`) under sub-call IDs `<callID>:<n>`, emitting `EventToolStart` (a child row), `EventApproval`, `EventToolEnd`. Nothing about the answer channel changes; the sidecar never sees it.
- The tool timeout must not count approval waits. `safego.WithTimeout(r.setup.ToolTimeout)` (`run.go:20`, `WithTimeout`) is wall clock, so `run_program` opts out (`Tool.Timeout` override, or a negative value for "managed by the tool") and `codemode` enforces its own active-time budget via the supervisor. `turnRun.waited` already excludes waits from the turn deadline (`calls_run.go:76-79`).
- The decision binds to the exact decoded arguments. The gate decodes the call, builds the description, asks, and then executes **that decoded call**; the program cannot change the arguments between the question and the effect.
- Second line: the per-run `services.Handler` is built with `Deps.Confirm` set to a function that returns true only if the gate has just approved exactly this call, and false otherwise. A bug in the gate then fails closed at `confirmDestructive` (`confirm.go:37`), not open. `requireRun` (`services/desktop_limits.go:25`, used by `noteDelete`) accepts only `MethodRun`; it must also accept `MethodProgram`, otherwise delete is refused for the wrong reason.
- No persistent "always allow" for `destructive` or `external` in the first version (section 8, open question 3).
- If the chat has no asker (a build or path without UI), destructive and external effects are refused, not run. `confirmDestructive` already behaves this way for a nil port (`confirm.go:44`).

Prompt injection: text the program reads (notes, clipboard, and web content the same chat fetched with its native web tools) can steer the model. The defences are that destructive and external effects always ask, with host-built descriptions, that programs have no network, and that sensitive reads ask. A user who approves something they did not read is outside what code can fix.

### 3.4 How the user sees what ran

The transcript already has activity rows per call (`chat.ToolRun`, `internal/ai/chat/model.go:66`: call id, name, label, input, outcome). Code mode adds:

- One row for the program: label "Ran a program", `Input` the code (the row needs an expandable detail; a one-line summary is not enough here), `Outcome` such as "3 effects, 1 refused".
- One nested row per gated call (`ToolRun.ParentID`, new field), each with its label, host-built detail, status (ran, refused by you, refused by the gate, cancelled) and the awaiting state while the approval is pending (`StatusAwaiting`, set at `chat/execute.go:43`).
- The message text of the program row is the result the model read.

The effect journal is written only by the gate. The model receives it in the tool result (`created note n3 “Packing list”; deleted n1 (you approved)`), so it can report truthfully and cannot invent effects.

## 4. API coverage

| Wish | On the Extension API today | Missing |
|---|---|---|
| **Notes** | `Notes` with `List`, `Get`, `Create`, `Recent`, `Update(id, markdown, updatedAt)`, `Delete` (`internal/ext/api/services_notes.go:18`), served at `services/notes.go`; `delete` already asks the user. Child binding: `child/context.go:87`. | Ergonomics only. `Create` takes no content, so "create a note with X" is two calls and an `updatedAt` hand-off. `List` returns every note with full markdown, which is wasteful for a conversation. Add `Notes.Search(query string, limit int) ([]NoteMatch, error)` returning id, title line and a snippet, and `Notes.CreateWith(markdown string)`. Both are useful to normal extensions. |
| **Timers** | **Nothing a program can use.** The Timers built-in is a plain extension (`internal/ext/builtins/timers/timers.go:41`). It stores its one timer in the host `Scheduler` under a fixed key (`timers.go:32`), and `Scheduler.Once` runs only the calling extension's own handler (`api/scheduler.go:18`). | A first-class `Timers` service: `Start(d time.Duration, label string) (Timer, error)`, `List() []Timer`, `Cancel(id string) error`, with the finish notification owned by the host. Per AGENTS.md ("migrate an existing Nevermind host job first, then expose the extension API"), the first step is to **move the Timers built-in onto it** (it has a menu bar countdown and a notification, `timers.go:96-130`), then expose it. This also lifts the one-timer limit. A stopgap through `Scheduler` and action refs would be a backdoor and is rejected. |
| **Clipboard history** | `ClipboardHistory` with `List(ClipboardQuery{Text, Limit, Types})`, `Get`, `Remove`, `Clear` (`api/services_core.go:193`, `services/clipboard_history.go:30`); entries carry `CreatedAtMs`. Concealed copies never enter history. Copying text out: `Desktop().Clipboard().WriteText` (`api/desktop_clipboard.go`). | `ClipboardQuery` has no time window; "copied an hour ago" can be filtered inside the program from `CreatedAtMs`, but `SinceMs` and `UntilMs` on the query are cheap and keep the program short. **"Paste" does not exist as an API call**: paste is a result action (`api/action.go:25-26`, `ActionPaste`), and from a chat window there is no target app. The wish becomes "copy the link to the clipboard", and the model should say so. |
| **List commands** | Host only: `Manager.Commands` (`internal/ext/manager/commands.go:24`) for the palette. Nothing on `api.Context`. | `Commands.List(query string, limit int) ([]CommandInfo, error)` where `CommandInfo` has `ExtensionID`, `ID`, `Title`, `Subtitle`, `Mode`, `Source` (built-in or user). Ranked by the host's search so the model sends a query, not the whole catalogue. |
| **Run a command** | Host only: `Manager.RunCommand` (`internal/ext/manager/query.go:177`), called by the palette. Nothing on `api.Context`. | `Commands.Run(extensionID, commandID string, args map[string]string) (CommandOutcome, error)`. Semantics to decide in the API doc: it runs **headless**. A command that would push a palette view returns a summary (view title, first rows) and the host does not navigate; commands with `Mode` `noView` or `background` (`api/enums.go:57-58`) are the clean case. It carries `Launch.Reason` "assistant" so extensions can tell. The assistant's own commands (chats, AI commands) are excluded to prevent a program starting chats. Before it is exposed, migrate one existing host consumer of `RunCommand` onto it (candidates to confirm with Pablo: the shortcut or alias runner). |

Every new service gets: api types and TSDoc-equivalent comments (the reference is generated from them), a `protocol` service id and ops, host handler with `require`, child binding, a fixture under `internal/fixtures` where it renders UI (these do not), and parity entries (`docs/extension-api-parity-plan.md`).

## 5. Model guidance

The model learns the API from the same source of truth the builder uses: `apiref` renders the api package's own types and comments (`internal/ai/builder/apiref/apiref.go`), and a drift test fails if the generated copy is stale (`builder/reference_test.go:24`).

Add `apiref.RenderSubset(sources, accessors []string)` which renders only the listed `Context` accessors and the types they reach, in the compact form (names and signatures). The generator (`apiref/gen`) writes `internal/ai/codemode/reference_gen.go` with the same drift test.

Size, measured by splitting the generated compact reference by section: the full builder reference is 36,785 bytes (cap 39 KiB, `reference_test.go:61`) and its `Context services` section alone is 14,423 bytes. The pieces code mode needs today are tiny: `Notes` 218 B, `Note` 54 B, `ClipboardHistory` 254 B, `ClipboardQuery` 55 B, `ClipboardEntry` 190 B, `SystemClipboard` 594 B. With the new `Timers` and `Commands` services (estimate 300 B each) the API part is about 2 KB. Add the program contract and six worked examples (create, append, delete, timer, clipboard search, run a command), estimate 1.5 KB. Total about 3.5 KB, roughly 900 tokens at 4 bytes per token (an assumption, not measured).

For comparison, the nine existing conversation tools carry their JSON schemas in every request (`tools/set.go:36`); that is of the same order. Code mode's cost does not grow with each feature the way one-tool-per-feature does, and the text is static, so the provider's prompt cache applies.

Delivery: put the text in the system prompt, or in the tool description with its hash added to `sessionKeyOf` (section 1), so a changed API reaches live sessions. Do not preload the full builder reference; the subset is enough, and a `describe_api(service)` read tool is only needed if the surface grows past about 8 KB.

Examples are tested: each one runs in the interpreter against fake services in CI, as `internal/ext/api/examples_test.go` does for extensions, so the guidance cannot rot.

Known Yaegi traps go into the contract: no goroutines or channels, no generics, no `slices`/`maps` selectors (`extension-runtime-spike.md` 2.3 and 2.4). Compile errors return to the model as the tool result.

## 6. Code mode against plain tool calls

Plain tools are simpler to reason about per call, and each call is trivially reviewable, but they cost one model turn per step ("add milk to my shopping note" is search, read, write: three turns of an 8-step budget, `events.go:84`), put every list result through the model, need a Go tool plus schema, policy, label and summarizer per feature, and the schema list grows with the API. Code mode puts composition in one turn, keeps bulk data in the host (filter inside the program), derives its surface from the API (so a new service is available without writing a tool, which is the dogfooding goal), and makes the gate finer than per-tool: classes apply per operation. It costs a bigger security surface (arbitrary code instead of fixed schemas), a new in-run approval primitive, and some model failures writing Go. Recommendation: **code mode for everything that is an Extension API service; keep the existing file, app and link tools as they are**; do not add notes, timer or command tools.

## 7. Phased plan

Checks follow the repo rules: Go runs only through `src/native/scripts/boxgo.sh <slot> '<command>'`, nothing launches the app, and journeys run only in the rig or CI.

### Phase 0: sandbox and gate skeleton (no model)

Package `internal/ai/codemode` (layer `ai`; it imports `ext`, same rank, no cycle: `ext/services` imports only `ai/extai` and `ai/quota`).

Build `source.Prepare`, the restricted symbol table, the program-child flag, `MethodProgram`, the gate with an empty table, and a runner that executes a program against fake services.

Acceptance tests:
- a body using `os.ReadFile`, `net/http`, `syscall`, `unsafe`, `reflect` or `runtime` fails as undefined; a body containing `import`, `go`, `chan` or `select` is rejected with a readable message;
- `for {}` is killed at the active-time budget, the child is reaped, and the process count returns to its start value;
- a 64 MiB allocation does not take the host down (Linux: child dies under `RLIMIT_AS`; elsewhere: resident watch kills it);
- a source over the size cap is refused before staging; the staging directory is gone on success, failure and cancel;
- a call to a `(service, op)` not in the table returns a `permission` failure and is journaled;
- the layer and file-size checks (`internal/layers`) stay green.

### Phase 1: notes (first useful slice)

Build `run_program` in `tools.Standard`, the agent asker with nested rows, `ToolRun.ParentID`, the `Notes.Search` and `Notes.CreateWith` additions, the notes entries in the gate table, `requireRun` accepting `MethodProgram`, the system prompt line (the current prompt says tools cover "files, apps, links and the clipboard", `chat/transcript.go:18`), and the generated reference subset.

Acceptance tests (scripted model, scripted pi-host stand-in, fake notes store; no network):
1. "Create a note with my packing list": one model turn calls `run_program`; the note exists; the result lists the created id; the transcript has a program row and one effect row.
2. "Add milk to my shopping note": the program searches, gets, updates with the right `updatedAt`; a concurrent edit produces the conflict result and no write.
3. "Delete the note about X": an `EventApproval` is emitted **before** the store changes, with the host-built title; deny leaves the note and the model reads "The user did not allow this"; allow deletes; Stop while awaiting leaves the note and ends the row as cancelled.
4. Gate fail-closed: with the gate's approval token withheld, `Handler` still refuses the delete via `confirmDestructive`.
5. The program cannot reach `secrets`, `network` or `clipboard.history` (capability absent).
6. Timeout, output cap and service-call cap each produce a readable failure; approval waits do not consume the 20 s budget.
7. Transcript persistence: the program text, effects and outcomes survive a reload and are included in the next request's history as tool messages.
8. The pi path: a fake host script sends `tool_call run_program`; the engine returns the program result; no sidecar file changed.
9. Rig journey (CI): ask the conversation to create and then delete a note; assert the approval row is visible and keyboard operable.

Exit: the nine tests pass on the box; PLAN/work-package ledger updated.

### Phase 2: timers

Add the `Timers` service to the api; move the Timers built-in onto it first (its tests in `builtins/timers/timers_test.go` stay green, menu bar countdown unchanged); then grant `timers` to programs.

Acceptance: "start a 10 minute timer" starts one with no prompt and a row; "cancel it" works; two named timers coexist; the finish notification fires through the host; the built-in's existing behaviour (root row "timer 5m", cancel command only while running) is unchanged; a fixture for any new UI.

### Phase 3: clipboard

Add `SinceMs`/`UntilMs` to `ClipboardQuery`; add history reads (sensitive-read, once per run), history remove and clear (destructive), and system clipboard write.

Acceptance: "copy the link I copied an hour ago": the approval names what will be read before any text exists in the model's context; deny returns nothing; text-only results with a cap; image entries return type and size only; the link ends up on the clipboard; a test proves concealed copies never appear; "paste it" gets an honest answer that it was copied, not pasted.

### Phase 4: commands

Add `Commands.List` and `Commands.Run`; migrate one existing host consumer of `RunCommand` onto it; grant `commands`; every run is `external`.

Acceptance: "run Toggle Floating Notes" lists, resolves one match, asks with the host's extension and command title, runs after allow; a command from a fixture user extension runs the same way; an ambiguous title returns candidates and runs nothing; a view command returns a summary and does not navigate; the assistant's own commands are refused; deny and Stop behave as in phase 1; the 5 s `MethodRun` budget (`supervisor/config.go:42`) is reported as a clean failure.

### Phase 5: hardening and evaluation

- OS confinement of the program child (Linux seccomp or Landlock, macOS Seatbelt, Windows restricted token): no files, no sockets, only the stdio pipes. Measure startup cost.
- An evaluation set of about 40 prompts across the four areas run with a scripted and then a live model: first-try success, retries, tokens per request, approvals per task. Same method as the author-experience gate in `extension-runtime-spike.md` section 6.2. If JavaScript clearly wins there, swap the program runtime behind the same protocol.
- Optional remembered grants for `external` commands, listed and revocable in Settings, only if approval fatigue shows up in the evaluation.

## 8. Risks and open questions

Risks:

1. **Yaegi is not a security boundary.** Mitigated by body-only programs, host-generated imports, a restricted symbol table, default-deny gate and process isolation; fully addressed only by phase 5. Until then, programs must stay within the allowlist and no write-capable file or shell services are exposed.
2. **Off-Linux memory bound is weak.** No `RLIMIT_AS` (`limits_other.go`); the resident watch polls once per second.
3. **Approval design is the product risk.** Too many prompts and users stop reading; too few and a hijacked model deletes things. Classes above are a starting point to tune with the phase 5 evaluation.
4. **Model-written Go.** Yaegi traps and API misuse will cost retries; the 8-step budget bounds the damage.
5. **Stop is not rollback.** An admitted effect may finish after Stop (section 2.5).
6. **API growth pulls the prompt up.** The subset renderer and a size ratchet (like `maxCompactReferenceBytes`) keep it honest.
7. **New API surface is a commitment** (`Timers`, `Commands`) shared with user extensions; design it as an extension API first, not for the assistant.

Open questions for Pablo:

1. **Go or JavaScript for programs?** This plan reuses Yaegi because it is the extension runtime and needs no second API binding. JS (sobek) starts in under a millisecond in process, is interruptible and has no stdlib gap, but needs a generated binding of the Go api. It should follow whatever the extension runtime decision (D1) concludes. OK to keep Go for now?
2. **Which classes need a prompt?** Proposed: prompt for delete, clipboard history reads, clipboard remove/clear and every command run; no prompt for note create/update, timers and copy-to-clipboard. Do you want create/update of notes to ask too, at least at first?
3. **Remembered approvals.** None in v1 for delete or commands. Do you want "always allow this command" for user-chosen safe commands (Toggle Floating Notes)?
4. **Command scope.** Headless only, with view commands summarized, not opened. Or should the assistant be able to open a command's view in the palette ("show my notes")?
5. **Notes and clipboard text going to the model.** Notes are read without a prompt (files already are); clipboard history prompts. Is that the right line for notes?
6. **Where do chats live?** Builder chats run on the native loop. Should builder chats get `run_program` too, or conversations only?
7. **Which host consumer migrates onto `Commands.Run` first** (shortcut or alias runner, other)?

## 9. As built (phases 0 and 1)

Decisions Pablo approved: programs are Go, run in a throwaway child of the extension runtime through one `run_program` tool that conversations get (builder chats keep their own tools). Only deletes, and later running commands, ask in the chat. Creating, appending, editing and reading notes run without asking. There is no "always allow" yet.

Where the code is:

| Piece | Where |
|---|---|
| Program check, import allowlist, symbol table | `internal/ext/program` (`Prepare`, `Packages`, `Symbols`). Imports are generated from the qualifiers the body uses; `os`, `net`, `exec`, `syscall`, `unsafe`, `reflect`, `runtime` and every other package are refused by name before anything is staged. |
| Program child | `internal/ext/child/program.go`. The child gets a sixth argument (`protocol.ProgramArg`), loads only the allowed standard library packages plus the api, has an empty source tree (so the interpreter cannot find a package by its source) and serves `protocol.MethodProgram` only. An extension child refuses that method. |
| Throwaway runtime | `supervisor.NewProgram`, budget `protocol.MethodProgram` = 20 s of the program's own computing. The supervisor already adds back the time the host spends on service calls, so an approval wait does not count. |
| Gate | `internal/ai/codemode` (`gate.go`, `rules.go`, `class.go`, `journal.go`). A fixed table of `(service, op)` rules with a class; an unlisted pair is refused and journaled, including the services the extension handler leaves open to every extension (`storage`, `logs`, `text`). Fifty calls per run. |
| Second line | The program's `services.Handler` is built with a `Confirm` that answers yes only while the gate has allowed the call being served, once. `requireRun` accepts `MethodProgram`. A bug in the gate therefore fails closed at `confirmDestructive`. |
| In-run approval | `agent.SubCalls` (`internal/ai/agent/subcalls.go`): a running tool begins nested calls (`CallInfo.ParentID`), asks the user through the same approval channel as a tool's own confirmation, and ends them. `Tool.Timeout < 0` removes the wall-clock timeout for a tool that bounds its own active time. `Result.Failed` ends a call failed while the model still reads its text. |
| Transcript | `chat.ToolRun.ParentID`; the program row's `Input` is the code, its text the result the model read, its outcome "2 changes, 1 declined by you". The palette shows only the live activity row, as it does for every tool (completed tool entries are not conversation rows), so the nested approval shows there with its host-built question. |
| Guidance | `codemode.guidance()`: the contract, the generated reference subset (`apiref.RenderSubset`, `reference_gen.go`, drift test) and three examples that the tests run. About 2.2 KB. |
| Notes API | `Notes.Search` and `Notes.CreateWith`, added for this slice because `List` returns every note in full. |

Differences from the text above:

- The program's printing is discarded; only the returned value comes back (the plan did not decide this).
- A 64 KiB cap on the returned JSON (`protocol.MaxProgramResultBytes`) fails in the child with a message that tells the model to return less; the agent loop's 16 KiB cap still applies to what the model reads.
- The memory test allocates 6 GiB (the 2 GiB `RLIMIT_AS` makes it a fatal runtime error) and 250 MB against a 120 MB resident limit, not 64 MiB: a 64 MiB allocation is within both limits.
- The interpreter panics when a computed bool is returned directly (`return a < b, nil`). The guidance says to assign it first, and a failure of that kind carries the same hint.
- The pi session key still hashes tool names only. The guidance is static for a build and a pi host lives no longer than the app process, so a changed text reaches the next host.
