# Palette debug CLI

Use the palette debug CLI to test palette/search/action logic without opening the UI.

```sh
mise exec pnpm -- pnpm palette:debug --query clipboard
mise exec pnpm -- pnpm palette:debug --query "" --execute "Clipboard History"
mise exec pnpm -- pnpm palette:debug --no-build --query "settings"
```

The command runs Electron main in debug mode, loads the same user state/extensions as the app, indexes apps/files, calls the real search pipeline, and prints the final sorted action list as JSON. With `--execute`, it runs the matching action by exact `id` or `title` and includes the action result.

Use this for logic bugs where the UI is not the thing being tested: provider output, ranking, root limits, cache invalidation, action return shapes, dismissal flags, and extension/native action wrapping.
