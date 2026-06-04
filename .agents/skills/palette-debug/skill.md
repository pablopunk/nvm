---
name: palette-debug
description: Use the palette debug CLI to test palette/search/action logic without opening the UI. Trigger on requests to debug provider output, ranking, root limits, cache invalidation, action return shapes, dismissal flags, or extension/native action wrapping.
---

# Palette Debug CLI

Use this skill to test the Nevermind palette/search/action logic headlessly, without opening the Electron UI.

## Usage

```bash
# Search for a query
mise exec pnpm -- pnpm palette:debug --query clipboard

# Execute a matching action by exact id or title
mise exec pnpm -- pnpm palette:debug --query "" --execute "Clipboard History"

# Skip the pre-build step (if already built)
mise exec pnpm -- pnpm palette:debug --no-build --query "settings"
```

The command runs Electron main in debug mode, loads the same user state/extensions as the app, indexes apps/files, calls the real search pipeline, and prints the final sorted action list as JSON. With `--execute`, it runs the matching action by exact `id` or `title` and includes the action result.

## When to use

This is for logic bugs where the UI is not the thing being tested:

- Provider output and ranking
- Root limits and deduplication
- Cache invalidation behavior
- Action return shapes and types
- Dismissal flags and behavior
- Extension/native action wrapping
- Search pipeline ordering

## Script

The companion script lives alongside this skill at `palette-debug.cjs`. It handles argument parsing (--query/-q, --execute/-x, --no-build), optionally runs `pnpm build`, then launches Electron with the `NVM_PALETTE_DEBUG` environment variable set.
