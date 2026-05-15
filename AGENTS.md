# Agent Guidelines

* Always use `mise exec pnpm` for any package manager commands. Never use npm, yarn, or bun directly — the project pins Node v22 and pnpm v10 via mise.toml.
* Try to keep files small and focused.
* When changing/adding code, always explore the repo to understand conventions and similar use cases.
* Comments are a smell. 3 long named functions is better than 1 function with a comment.
* If you suddenly see changes you have not done, it might be the user in the background, do not mess it up.
* If you spend a lot of iterations with the user to finally find a solution for something, document your learnings in docs/. But you shouldn't touch it if all goes smooth.
