# Reviewed AI model routes

The admin picker offers only provider, model and role combinations whose text,
image, reasoning and function-tool behavior has been reviewed together. Catalog
metadata supplies current pricing and limits; it does not establish that those
capabilities work together. Existing configured routes continue to run even if
they are no longer selectable.

Before adding a combination, verify the actual provider API with a bounded request
that uses an image and a function tool, execute the returned call, and send its
result back for a final response. Verify streaming usage and every selectable
thinking level. Record the provider, model, role, API, date and outcome without
credentials, prompts or user data. Update the reviewed route policy and its tests.
A direct provider probe does not replace a Nevermind proxy or packaged-app journey.

On 2026-10-28 the following direct provider checks succeeded:

- OpenRouter Chat Completions: Gemini 2.5 Flash, DeepSeek V4.1 Flash 1017 and
  GPT-6.1 Terra for Smart and Fast; GPT-6 Sol for Fast. Each returned the correct
  color from an image through a function call and used its result in a follow-up.
  Streaming returned tool arguments and usage, and all selectable Smart
  thinking levels were accepted for the first three.
- OpenAI and OpenRouter Responses: GPT-6.1 Sol for Smart. Both returned the correct
  color from an image through a function call, completed a tool-result follow-up,
  streamed a function call with usage, and accepted low through max reasoning.
  OpenAI rejected reasoning off, so this model is not offered for Fast.

These are point-in-time checks. Provider availability, routing and pricing can
change; the model catalog remains necessary for current priced limits. A complete
Nevermind proxy and released-desktop journey requires separate verification.

The GPT-6.1 Sol fallback prices reflect OpenAI standard rates and the highest
standard OpenRouter endpoint rates observed on 2026-10-28, including the
272,000-token long-context tier. Fresh catalog data takes precedence for
descriptors and pricing, while Responses reservations also use the reviewed
OpenRouter ceiling when it is higher.
