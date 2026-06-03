# GitHub Connector

Installed local skill available: `github` (`gh` CLI). Skill search for GitHub Actions found mostly generic/low-signal skills, so prefer the installed GitHub skill and `gh` CLI.

Use this connector for GitHub Actions failures, desktop release artifacts, signing/notarization, missing latest YAML, broken release assets, and update/install incidents.

## Operational commands

```bash
gh run list --limit 10
gh run view "$RUN_ID" --log-failed
gh release view "$TAG"
```

Use release/update evidence only for packaged-client incidents. GitHub release failures do not explain backend Vercel runtime errors unless the failing client build is the only repro path.

Do not reveal signing/notarization secrets. Confirm before changing GitHub secrets, deleting releases, or overwriting release assets.
