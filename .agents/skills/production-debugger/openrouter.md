# OpenRouter Connector

Verified skill search results: `openrouterteam/skills@openrouter-models` and `openrouterteam/agent-skills@openrouter-typescript-sdk`.

Use this connector for OpenRouter upstream model availability, pricing/context/latency questions, provider routing issues, and upstream API failures when Nevermind's active provider is OpenRouter.

## Operational flow

1. Start from Axiom logs for the failed proxy request: status, model, provider, request id, upstream response shape.
2. Use the OpenRouter models skill for live model metadata: pricing, context length, provider latency/uptime, throughput, modalities, and supported parameters.
3. Confirm whether the issue is model unavailable, provider outage, unsupported parameter, rate/auth failure, or Nevermind proxy config.
4. Do not translate or strip provider-specific request fields unless the upstream evidence proves they are invalid.

## Safety

Never print OpenRouter API keys, authorization headers, raw prompts, responses, or full request bodies.
