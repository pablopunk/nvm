# Extension runtime spike (WP3.1): numbers and the D1 recommendation

Companion to `docs/extension-api.md`. Code: `src/native/internal/ext/spike` (its own Go module, every file behind `//go:build spike`, so the main module and CI never compile it). Every number below was produced by the commands in section 1 on the test box. Nothing was run on a maintainer machine.

## 0. Summary

- **Recommendation for D1 (2.0 beta):** run extensions in a **supervised child process** whatever the language, with a host-side deadline per call, a kill-and-restart on breach, and an address-space cap. **Keep Go (Yaegi) as the provisional authoring language.** Decide Yaegi versus sobek at the author-experience gate (section 6), not on these numbers. Details and criteria are in section 6.
- In-process Yaegi cannot be relied on for the B8 hard limit or for memory or goroutine-panic containment. Two of its three gaps are closed by using `EvalWithContext` for every call and by the child process; the third (no per-extension memory cap) needs the process boundary.
- Yaegi's stdlib coverage is the weakest number: 7 of 51 common imports fail, 39 of 193 GOROOT packages are absent, 9.4% of the exported names in 40 common packages are missing, and `return <-ch` fails at run time.
- Sobek (JavaScript) is interruptible on a held function, uses far less memory per extension, and has no stdlib to cover, but it is a different authoring language from the one the rewrite was built around.
- Both runtimes meet the B8 soft budget (20 ms) by two orders of magnitude on a realistic extension. Neither is the bottleneck.

## 1. How the numbers were produced

Machine: a shared 4-vCPU, 4 GB Linux LXC on a Proxmox host (`Linux 7.0`, x86_64), the "test box" used by other agents at the same time. Timings are noisy: a trivial Yaegi load measured 9.0 ms in the first run and 1.9 ms in a later one, and the realistic call p50 measured 410 µs and 250 µs. Tables show the last full run. Treat differences under about 2x as noise.

Toolchain: `go1.27.3 linux/amd64` (mise). Dependencies: `github.com/traefik/yaegi v0.16.1` (same version as the main module), `github.com/grafana/sobek v0.0.0-20261031082357-c1e3e36bfbc3` (latest when run; mixmusic's choice). `goja` was not measured (section 7).

Commands (run through `src/native/scripts/boxgo.sh wp31`, which copies the tree to the box; inside the box, in `internal/ext/spike`):

```sh
go get github.com/grafana/sobek@latest && go mod tidy       # once, to resolve sobek
go vet -tags spike ./...
go test -tags spike -count=1 -timeout 900s -v ./... | grep MEASURE   # the measurement suite (147 rows)
SPIKE_INTERRUPT=1 go test -tags spike -count=1 -run '^TestInterrupt...$' -v .   # one per process, see 4
go build -tags spike -trimpath -ldflags='-s -w' -o OUT ./cmd/hostbase        # binary sizes, see 6.x
staticcheck -tags spike ./...                                            # v0.8.2
```

Each interrupt probe runs in its own process because some of them deliberately leave interpreted code running. The `-run` names are listed in the tables.

Realistic extension: a generator (`gen.go`) produces a Go file of 461 lines (274 lines in JavaScript): a dataset built at init, 30 string helpers called through a table, JSON encoding, sorting and a filtered `Search`. It is a size and mix probe, not a product extension.

## 2. Yaegi (in-process, Go source)

### 2.1 Load and call

| Measure | Trivial (3 rows, 11 lines) | Realistic (461 lines, 20 rows) |
|---|---|---|
| Load: new interpreter, `Use(stdlib.Symbols)`, `Eval`, median of 10 | 2.1 ms | 5.4 ms |
| Call through `reflect.Value.Call`, p50 / p95 / p99 (2000 calls) | 0.8 / 4.3 / 6.9 µs | 250 / 651 / 861 µs |
| Call through `EvalWithContext(ctx, "Search(q)")`, p50 / p95 / p99 (1000 calls) | 15.3 / 47.5 / 104 µs | 305 / 921 / 3274 µs |
| Heap per loaded extension (50 kept alive, two GCs) | 1.27 MiB | 2.54 MiB |
| Heap for 50 extensions | 63 MiB | 127 MiB |

Reading: the per-interpreter cost is dominated by copying the stdlib symbol table. The `EvalWithContext` route adds about 15 µs per trivial call and is within noise for the realistic one. A realistic extension spends roughly a quarter of a millisecond per search, so the B8 soft budget (20 ms) has about 80x headroom. 50 realistic Yaegi extensions loaded at once would be 127 MB, above the B4 idle budget (100 MB with all built-ins), so lazy loading or the child-process model must bound how many are resident.

### 2.2 Stdlib

Import matrix (`import _ "pkg"` through `interp` with the default options), 51 packages tested:

- **Fail (7):** `iter`, `os/exec`, `unique`, `structs`, `weak`, `unsafe`, `testing/synctest`.
- `os/exec` is available only with `interp.Options{Unrestricted: true}` and the `stdlib/unrestricted` symbol set. Yaegi's own comment on that option says it "may cause the interpreter process to exit". Measured: `exec.LookPath("sh")` returned `/usr/bin/sh` under that option. Extensions must never get that option.
- `slices`, `maps`, `cmp` and others import without error through Yaegi's source fallback, but their names are not all callable (table 2.3).

Symbol coverage (exported top-level names of 40 common packages for linux/amd64, read from GOROOT, compared with the Yaegi symbol table):

| Packages | Coverage |
|---|---|
| 1612 exported names in total | 1460 present (90.6%) |
| Entirely absent: `cmp`, `iter`, `maps`, `os/exec`, `slices`, `unique`, `structs` | 0% |
| Partly absent: `encoding/json` (25 of 39), `bytes` (59/65), `strings` (55/61; includes `Lines`, `SplitSeq`, `FieldsSeq`), `sync/atomic` (36/47), `unicode` (291/309), `io/fs`, `os`, `reflect` (`TypeFor` missing), `runtime`, `log/slog`, `encoding/binary`, `math/rand/v2`, `sync` (`OnceValue`, `OnceValues`) | 85% to 100% |
| Fully covered | `bufio`, `context`, `encoding/base64`, `csv`, `hex`, `fmt`, `io`, `log`, `math`, `math/bits`, `math/rand`, `net/url`, `path`, `regexp`, `sort`, `strconv`, `text/template`, `time`, `unicode/utf8` | 100% |

GOROOT packages (193 importable for linux/amd64, excluding `internal`, `cmd`, `vendor`, `testdata`): 154 provided by the Yaegi table, **39 missing**, among them `encoding/json/v2`, `crypto/mlkem`, `crypto/hkdf`, `crypto/pbkdf2`, `crypto/sha3`, `embed`, `plugin`, `syscall`, `time/tzdata`, `uuid`.

The Yaegi table is generated per Go release. The v0.16.1 table stops at `go1_22`, but the toolchain is 1.27.3. That gap is the Go-version-bump risk in section 5.

### 2.3 Generics

| Program | Result |
|---|---|
| Generic function with two type parameters (`Map[T, U]`) | ok |
| Generic type with methods (`Stack[T]`) | ok |
| Union constraint (`Number interface{ ~int | ~float64 }`) | ok |
| Explicit instantiation of a generic struct and function (`Pair[K, V]`, `MakePair[string, int]`) | ok |
| Generic stdlib call through a selector (`slices.SortFunc` with a func literal) | **fail**: `undefined selector: slices.SortFunc` |

Selector probes: `slices.Index` ok, `cmp.Compare` ok, `maps.Keys` **undefined selector** (it returns an iterator).

### 2.4 Goroutines, channels, select, panics

- Goroutine that sends on a channel, received into a variable (`x := <-ch`): ok. Range over a channel: ok. Closure that mutates a captured variable: ok.
- **`return <-ch` fails** with `reflect: reflect.Value.Set using unaddressable value`, even on a buffered channel and even inside the function that receives. The same failure hit a goroutine that ranges and then reports with `return <-done`. Workaround that works: assign first (`x := <-ch; return x`). This is an authoring trap, and the builder's validation must catch it.
- `select` with `case v := <-ch:` works (`got=7`).
- Panics inside interpreted code reach the host as errors and the host survives: `panic("boom")` → error `boom`; index out of range → error; nil map write → error.
- Goroutine panics: in the child process the child died with exit status 2 about 23 ms after the panic, and the host was alive (section 3.3). In-process, an unrecovered panic on any goroutine terminates the whole app; that is Go's rule and was not run in-process.

### 2.5 Interruption and memory (each probe in its own process)

| Probe | Host returned at | Still running after the return |
|---|---|---|
| Pure loop, held function value, watchdog goroutine (`TestInterruptYaegiPureLoopWatchdog`) | 200.3 ms | **yes**: 502 ms of CPU in the next 500 ms |
| Pure loop, `EvalWithContext(ctx, "Run()")` (`TestInterruptYaegiPureLoopEvalWithContext`) | 200.4 ms | no: 0 ms of CPU in the next 500 ms |
| Loop with a host call, `EvalWithContext` (`TestInterruptYaegiHostCallLoopEvalWithContext`) | 200.1 ms | no: 2.80 M ticks at return, 0 in the next 500 ms |
| Blocked receive `x := <-ch`, `EvalWithContext` (`TestInterruptYaegiBlockedReceiveEvalWithContext`) | 200.2 ms | goroutines 2 → 3 after 200 ms; one goroutine stayed parked (not identified) |
| 10 s busy loop, held function value and watchdog (`TestInterruptYaegiBusyLoopTenSeconds`) | 200 ms (host gave up) | **yes**: the loop ended at 10.0 s |
| 10 s busy loop, `EvalWithContext` (`TestInterruptYaegiBusyLoopTenSecondsEvalWithContext`) | 200 ms | returned at the deadline; CPU was not sampled in this variant |
| 64 MiB allocation in an extension (`TestYaegiMemoryBombIsNotCapped`) | 2 ms to finish | no cap: 64.0 MiB allocated |

Reading: the host can stop an interpreted loop only when the code runs through `EvalWithContext`, which evaluates a source expression on each call. Calling a function value obtained earlier cannot be stopped. The watchdog pattern (goroutine plus context) returns on time but leaves the work running. Yaegi has no per-extension memory limit at all.

## 3. Sobek (in-process, JavaScript)

### 3.1 Load and call

| Measure | Trivial (3 rows, 7 lines) | Realistic (274 lines, 20 rows) |
|---|---|---|
| Load: `sobek.New`, `RunString`, median of 10 | under 0.1 ms | 0.8 ms |
| Call through a held callable, p50 / p95 / p99 (2000 calls) | 2.0 / 7.5 / 16.5 µs | 213 / 561 / 722 µs |
| Heap per loaded runtime (50 kept alive) | 0.00 MiB (below noise) | 0.35 MiB |
| Heap for 50 runtimes | 0.2 MiB | 17 MiB |

### 3.2 Interruption, memory, errors, globals

- `vm.Interrupt` from a 200 ms timer: `while (true) {}` returned at 200.3 ms with `InterruptedError`; a 10 s `Date.now()` busy loop also returned at 200.3 ms interrupted. Interruption is cooperative inside the interpreter loop and works on a held callable.
- Memory: a 64 MiB allocation of typed arrays inside a runtime completed (64.1 MiB allocated). Sobek has no memory cap either.
- JavaScript `throw` returns a Go error (`Error: js failure`). A Go panic inside a host callback propagated to the caller of `RunString`, where the host recovered it.
- Globals present without host help: `JSON`, `Date`, `Intl`, `Map`, `Set`, `Promise`, `Math`, `RegExp`, `fetch`, `process`, `require`, `setTimeout`. Only `fetch`, `process`, `require` and `setTimeout` are host-specific and would need review. Everything else is ECMAScript.

## 4. Subprocess isolation (Yaegi inside a child)

The child is the test binary re-executed (`os.Executable`) in serve mode. The protocol is one JSON request per line on stdin and one JSON response per line on stdout (`rpcRequest`, `rpcResponse`). The child holds the interpreter; the host holds the deadline and the process.

### 4.1 Startup, round trip, kill

| Measure | Value |
|---|---|
| Spawn to ready (trivial / realistic) | 7.2 ms / 10.8 ms |
| Round trip, trivial, 1000 calls, p50 / p95 / p99 | 9.9 / 15.0 / 18.6 µs |
| Round trip, realistic, 1000 calls, p50 / p95 / p99 | 274 / 675 / 905 µs |
| Kill and reap, one child, while a call is pending | 1.03 ms; the pending read returns EOF |
| Restart after kill (spawn to ready) | 6.9 ms |

Reading: the IPC cost is about 10 µs per call at p50 for the trivial case and about 25 µs for the realistic one. Startup is 7 to 11 ms, which can run in the background, but it must not sit on the palette's first-paint path (B1).

### 4.2 Crash and memory containment

| Scenario | Result |
|---|---|
| Extension panics on a goroutine it started (`goroutine-panic`) | child exit status 2, about 23 ms after the panic; the host was alive and its call had already returned |
| Allocation bomb with `RLIMIT_AS` 512 MiB | child died at start with SIGSEGV (the runtime could not reserve its arenas) |
| Same with 1024 MiB | child died at start: `fatal error: runtime: out of memory` |
| Same with 2048 MiB | child started in 7.4 ms; the bomb ended the child with exit status 2 within 9 ms of the call; host alive, pipe at EOF |

Reading: the address-space cap works, but for this test binary (the Go runtime plus Yaegi plus sobek) the floor is between 1 GiB and 2 GiB. The production child binary must be measured separately. `RLIMIT_AS` is Linux only; the macOS and Windows equivalents are unmeasured (section 7). Resident memory per child was not measured, only the address-space cap.

## 5. Binary size and Go-version risk

Stripped, `-trimpath`, linux/amd64, same toolchain:

| Binary | Size | Added over the previous row |
|---|---|---|
| Probe base (`encoding/json`, `net`, `fmt`) | 2.72 MB | |
| + Yaegi interpreter only (`cmd/hostyaegicore`) | 6.42 MB | +3.70 MB |
| + Yaegi with the full stdlib table (`cmd/hostyaegi`) | 20.42 MB | +14.00 MB (includes `net/http`, `crypto/*`, ...) |
| + sobek only (`cmd/hostsobek`) | 11.25 MB | +8.53 MB |

`net/http` is kept out of the probe base because the repository's port rule (`internal/platform/platform_grep_test.go`) forbids it outside platform code, so the Yaegi row includes the cost of `net/http` through its stdlib table. The real app binary (`cmd/nvm` at this commit, stripped, `-trimpath`) is 26.9 MB; section 6.4 gives the budget arithmetic.

Go-version risk, measured:

- Yaegi's tables top out at `go1_22`; the toolchain is 1.27.3. Packages and names added since then are missing (sections 2.2 and 2.3).
- Each Go bump needs a regenerated Yaegi table, or a published, shrinking list of the stdlib that extensions may use.
- Sobek has no stdlib table to regenerate, but its ECMAScript level moves with the library, so a bump still needs a conformance check. Not measured.

## 6. Recommendation for D1

### 6.1 Criteria

The brief named six criteria. Each is scored from the measured numbers above, with the unmeasured parts marked.

| Criterion | Yaegi in-process | Sobek in-process | Child process (either runtime) |
|---|---|---|---|
| B8 soft and hard budget | Passes: realistic p50 250 µs, p99 0.9 ms; the hard limit holds only through `EvalWithContext` | Passes: realistic p50 213 µs, p99 0.7 ms; the hard limit holds through `Interrupt` | Passes: +10 to 25 µs p50; hard limit enforced by kill, about 1 ms to reap |
| AI-authoring reliability | **Not measured.** Go has the built-in examples and the API types in `internal/ext/api`; known traps: `return <-ch`, generic selectors, post-1.22 stdlib names | **Not measured.** JS is the 1.x contract language (`nevermind-extension-api.d.ts`); the host would need its own stdlib | Runtime-neutral: the protocol carries wire types, not runtime types |
| Memory per extension | 1.3 MiB (trivial), 2.5 MiB (realistic); 127 MiB for 50 realistic | 0.0 MiB (trivial), 0.35 MiB (realistic); 17 MiB for 50 | Adds a process (not measured); bounded by the address-space cap |
| Isolation guarantees | None for memory; goroutine panics take the app down; interruption only through `EvalWithContext` | None for memory; no goroutines in JS; interruption on held callables | Crash contained (exit 2, host alive); kill in about 1 ms; memory capped (Linux, 2 GiB measured floor) |
| Binary size (B6 ≤ 45 MB total) | +17.7 MB (interpreter plus stdlib table) on a 2.7 MB probe base | +8.5 MB | Same binary via self-exec, or a helper binary (unmeasured) |
| Go-version-bump risk | High: tables stop at 1.22, 39 packages absent, 9.4% names missing | Medium: ECMAScript conformance, no table | Inherits the runtime's risk; isolates a bad bump to the children |

### 6.2 Decision

1. **Isolation (D1 for 2.0 beta): child process per extension group.** Use a self-exec helper (`nvm` in an extension-host mode), a framed RPC over stdio carrying only the wire types from `internal/ext/api`, and these host rules:
   - a host-side deadline per call: 20 ms soft (diagnostic) and 100 ms hard (kill the child, mark the extension failed with the reason, restart lazily with backoff, as PLAN section 4 requires);
   - an address-space cap per child (2 GiB on Linux as the measured floor for the probe binary; the production value and the macOS and Windows mechanisms are WP3.2 and WP5 work);
   - no `Unrestricted` option, ever;
   - children start in the background and never block the palette's first paint.
2. **Runtime: keep Go (Yaegi) provisionally, and decide at the author-experience gate.** The gate is PLAN WP3.1's UX test: a fresh agent, given only `docs/extension-api.md`, writes five extensions from prompts for each candidate runtime; record first-try success and fix loops. Keep Yaegi unless its first-try success is clearly lower. If sobek wins, swap the runtime behind the same child protocol; the wire types and the isolation design do not change.
3. **Validation at the builder and at load** (applies to either runtime): run the existing `api.Validate` functions; reject `return <-ch`-shaped receives and generic stdlib selectors that the spike shows failing; publish the supported stdlib list as the Yaegi table minus the failing names. This turns the measured traps into a lint.
4. **Pin and test the Go bump.** Pin `yaegi v0.16.1` in the spike and in the main module, add a CI check that `TestYaegiStdlibImportMatrix` and the symbol-coverage report do not regress, and make the Go bump a deliberate change with its own table regeneration.

### 6.3 What the recommendation is not

It is not a claim that Yaegi is good enough as a language for extensions. It is a claim that the process boundary is needed regardless of language, and that the language choice is open until the author experiment is run.

### 6.4 Binary size of the real app

`cmd/nvm` at this commit (the prototype, which links Yaegi with the full stdlib table) builds to **26.9 MB** stripped with `-trimpath -ldflags="-s -w"`, using the same toolchain. Against B6 (45 MB) that leaves about 18 MB. Adding sobek would cost about 8.5 MB by the probe delta, which still fits. The child-process model self-execs the same binary, so it adds no size; a separate helper binary would add its own size. The app's size without any runtime was not measured.

## 7. What could not be measured

- **AI authoring success.** No agent wrote extensions in this spike; there is no first-try rate for either runtime. The README says the builder was "verified end to end", and PLAN cites an earlier experiment with "Go via Yaegi worked first try for 3 of 3 extensions". I could not find a record of that experiment in the repository, so I did not use it as evidence.
- **Other Go versions.** Only go1.27.3 was installed. Behaviour under other releases, and the cost of regenerating the Yaegi table, are unmeasured.
- **Other runtimes.** `goja` was not measured. It is the other pure-Go JavaScript engine (Godwit's choice, with `Interrupt`).
- **Shared symbol tables.** Whether the per-interpreter stdlib copy (about 1.3 MiB per extension) can be shared was not explored.
- **Resident memory per child process.** Only the address-space cap was measured, not RSS.
- **macOS and Windows.** Nothing ran there: no `RLIMIT_AS` equivalent, no job objects, no Yaegi build on those systems.
- **In-process goroutine panic.** Measured in the child only; in-process it follows from Go's runtime rules and was not run.
- **Realistic extension size.** The generator produces 461 Go lines, not 500, and the content is synthetic. Real extensions with network I/O would be dominated by I/O, not by these CPU-bound numbers.
- **Noise.** One shared box, several agents, run-to-run variance up to about 2x. Single runs; no statistics beyond the percentiles shown.

## 8. Reproducing

```sh
src/native/scripts/boxgo.sh wp31 'cd /root/wp/wp31/internal/ext/spike && go vet -tags spike ./... && go test -tags spike -count=1 -v ./...'
src/native/scripts/boxgo.sh wp31 'cd /root/wp/wp31/internal/ext/spike && SPIKE_INTERRUPT=1 go test -tags spike -count=1 -run "^TestInterruptYaegiPureLoopWatchdog$" -v .'
```

Each `TestInterrupt...` and `TestYaegiMemoryBombIsNotCapped` runs in its own process; the list is in `yaegi_interrupt_test.go`.
