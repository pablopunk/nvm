<p align="center">
  <img src="src/backend/public/icon.png" height="100" />
</p>

<h1 align="center">Nevermind</h1>

<p align="center">
  <i align="center">The command palette that extends itself</i>
  <br/>
  <span><img src="https://cdn-icons-png.flaticon.com/512/9205/9205302.png" width="12" /> <a href="https://github.com/pablopunk/nvm/releases">Get the latest release</a></span>
</p>


<p align="center">
  <img src="src/backend/public/videos/nevermind-demo-v2.gif" alt="Nevermind command palette demo" width="640" />
</p>

> [!IMPORTANT]
> This app is in Alpha. Expect bugs.

Nevermind is a native command palette for your desktop, written in Go on [MyGo](https://mygo.egoist.dev). Summon it with a hotkey, search apps, files, clipboard history, emoji and your own extensions, and ask the AI to write a new extension when something is missing.

## What is in this repository

| Path | What |
|---|---|
| [`src/native`](src/native) | The app: Go source, the macOS helper, the Pi agent sidecar, packaging, the test rig and its docs. Start with [its README](src/native/README.md) and [`PLAN.md`](src/native/PLAN.md). |
| [`src/backend`](src/backend) | The server the app talks to (sign-in, AI proxy, billing) and the website, an Astro project deployed to Vercel. See [its README](src/backend/README.md). |
| [`docs`](docs) | Product, backend and operations guides that apply across both. |
| [`.github/workflows`](.github/workflows) | `native*.yml` build, test and package the app; `ci.yml`, `database-migration.yml` and `deployed-smoke.yml` check and ship the backend. |
| [`AGENTS.md`](AGENTS.md) | The rules for people and agents working in this repo. |

## Develop

The Go app is built and tested through the shared test box, not on a maintainer's desktop:

```sh
src/native/scripts/boxgo.sh <slot> 'go vet ./... && go test ./...'
```

The backend and the repo tooling use pnpm through mise:

```sh
mise exec -- pnpm install
mise exec -- pnpm -C src/backend check
mise exec -- pnpm -C src/backend test
mise exec -- pnpm check:changed origin/main
```

## License

AGPL-3.0-only. Third-party notices are in [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
