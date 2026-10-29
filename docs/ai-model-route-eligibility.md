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

Admission prices come only from durable snapshots that a scheduled refresh
validated. The request path never fetches a catalog, an endpoint listing, or the
OpenRouter audio and decision catalogs; it restores the last validated copy. A
validated price is current for a day and a half. Past that and up to a week it is
stale: its reservations hold a 2x margin over the estimate, so a rate that moved
while the snapshot aged cannot settle above its hold. A stale price admits on
every tier only while the day's absorbed-overrun counter is under
`PRICING_OVERRUN_DAILY_CAP_CREDITS` (500 credits by default); the counter grows
by the list cost each settlement does not collect, whether that cost exceeded its
hold or the user's balance could not cover the debit, and resets at UTC midnight. At the
cap, stale prices are refused on chat, audio, and decision routes, and current
prices still admit. Free spend is held the same way inside the monthly free-spend
cap, which bounds how much stale free spend can be reserved. Past a week a price
is refused for every tier. A refresh that fails keeps the last validated copy serving until
its week ends, so an upstream outage does not block a route with a recent price.
Saving an admin route validates that route's price first, the only fetch outside
the scheduled refresh. A route with no validated price is refused as unverified
until a refresh succeeds. Deploys run that refresh and verify every configured
route before traffic moves, so a deploy cannot start unpriced.

Settlement always uses the actual cost of the request, whatever price admitted
it. Only the hold is scaled by the stale margin.

Capabilities and prices resolve through one precedence: a current catalog entry,
then the exact bundled entry, then a stale catalog entry. Listing, descriptors,
prices, and image bounds all use it, so they agree on the entry that serves a
model. Bundled entries share one verification date and age on the snapshot
windows, so past their hard bound they admit for no model.

OpenRouter routing may fail over between endpoints, so its admission price is the
most expensive live endpoint at each prompt size; the model-level price is never
used for admission. Capabilities and wire format still come from the models.dev
catalog. Each failover link is a candidate with its own price and its own hold,
so a stale link never inherits the margin of the primary. A provider's contract
declares whether its upstream-reported cost may settle a request; every
settlement path applies that same declaration, and any provider not declared
settles from the catalog estimate. Reservations are priced at the conservative
maximum across cache and tier rates, because settlement can charge any of them.
Pricing refusals keep the released `pricing_unavailable` type and add a reason
for newer desktop clients.
