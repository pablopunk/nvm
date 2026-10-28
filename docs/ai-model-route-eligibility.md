# AI model route eligibility

The admin picker offers every catalog model that fits a tier. Code never names
models: eligibility and wire API come from the models.dev catalog, so a new
compatible model is selectable as soon as the catalog lists it.

Every tier needs what each chat can carry: text and image input, text output,
function tools, catalog prices and a context limit. Smart also needs reasoning.
Fast runs with thinking off.

The upstream API follows the SDK the catalog declares for the model, falling
back to the provider's SDK: the OpenAI SDK or a `responses` shape uses
Responses, the Anthropic and Google SDKs use their native APIs, and anything
else uses Chat Completions.

Catalog metadata is best effort. It cannot prove that every provider accepts
every parameter combination, so a newly selected route still needs a real
image-and-tool journey before it is relied on.
