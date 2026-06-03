# OpenCode Zen Connector

Skill search for `opencode` did not reveal a clearly relevant OpenCode Zen operations skill; results were mostly unrelated OpenCode workflow skills. Treat this connector as a thin operational wrapper around Axiom evidence and verified OpenCode/OpenAI-compatible API behavior.

Use this connector when Nevermind's active provider is OpenCode Zen and AI proxy requests fail upstream.

## Operational flow

1. Start from Axiom logs for request id, model, status, upstream error text, and latency.
2. Verify active provider/model from production evidence or admin state before blaming OpenCode.
3. Check whether the upstream failure is auth, model not found, unsupported parameter, rate limit, provider outage, or response format/usage parsing.
4. If external docs/API calls are needed, verify current OpenCode Zen docs first; do not invent CLI commands.

## Safety

Never print OpenCode API keys, authorization headers, raw prompts, responses, or full request bodies.
