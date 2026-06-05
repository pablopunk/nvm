## Summary


## Verification

- [ ] `mise exec -- pnpm test`

## Backend API compatibility

If this PR changes backend routes, proxy/auth/token/device flows, model descriptors, rate limits, billing errors, or desktop compatibility headers:

- [ ] Updated route-level contract tests.
- [ ] Updated `backend/src/fixtures/contracts/` fixtures when supported desktop clients depend on the changed shape.
- [ ] Confirmed the change is additive, feature-gated, versioned, or has explicit unsupported-client update UX.

