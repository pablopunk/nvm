// Package api defines the Nevermind extension API v1 as Go types: the data that crosses from
// extension code to the host, the author-side behaviour interfaces, and the context services.
//
// This is design-stage code. Nothing in the app imports it yet; the host runtime (WP3.2) will.
//
// Rules for every type in this package:
//   - Data types are JSON-serializable with stable field names and contain no func, chan or
//     interface fields. TestSerializableTypesAreCloneSafe enforces this by walking the type graph.
//   - Behaviour lives only in the interfaces (Extension, SearchProvider, ActionHandler, ...) and
//     in Context. Those never cross the host boundary.
//   - Actions cross the boundary as ActionRef values (stable ids plus string arguments), never as
//     code. The host mints opaque tokens for the palette and maps them back to ids.
//
// Rationale and the TypeScript mapping live in src/native/docs/extension-api.md.
package api
