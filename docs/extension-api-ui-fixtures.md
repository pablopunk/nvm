# Extension API UI Fixtures

Nevermind's extension API is a product surface. Every API endpoint that renders host-owned UI must have a dev-only fixture before it is considered supported.

## Rule

When adding or changing an extension API method that renders UI:

- Add or update a dummy extension that exercises it.
- Exercise realistic states: empty, populated, action panel, navigation, loading and error where applicable, and form submission where relevant.
- Keep fixtures dev-only. They must not be packaged or registered as shipped built-in extensions.
- Exercise them through a real extension child, not only through unit tests of the host.

The conformance work that gives every view kind, action kind and service a fixture is tracked in `src/native/docs/extension-api-parity-plan.md` (WP3.16).
