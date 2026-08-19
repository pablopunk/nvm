# Nevermind

> [!IMPORTANT]  
> This app is not even in Alpha, expect bugs.

<p align="center">
  <img src="https://github.com/pablopunk/nvm/blob/main/build/screenshot.jpg?raw=true"  />
  <br /><br />
  <i>An AI-native desktop command palette that extends itself.</i>
  <br /><br />
  <span><img src="https://cdn-icons-png.flaticon.com/512/9205/9205302.png" width="12" /> <a href="https://github.com/pablopunk/nvm/releases">Get the latest release</a></span>
</p>

_*In the screenshot above, Screenshots and Open Camera were extensions done by our AI. Not builtin. That's all you need to know._

## Development

Trust the checked-out `mise.toml`, then install dependencies with `mise exec -- pnpm install --frozen-lockfile`. Installation configures the tracked pre-commit and pre-push hooks in `.githooks`.

Use `mise exec -- pnpm format:staged` to format staged frontend files, re-stage them, and commit. The pre-commit hook runs `mise exec -- pnpm check:staged`, and the pre-push hook runs `mise exec -- pnpm check:changed` from the target remote's main branch.
