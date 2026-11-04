// Package analytics sends the app's technical diagnostics to the first-party ingest route, as PostHog-format batches:
// app launches, feature use by closed-set variant, failures as $exception events under their support reference, and
// problem reports. It follows docs/posthog-observability.md. Nothing is sent unless the consent switch is on and the
// build has an endpoint; development builds have none, so they never send.
//
// Every event goes through a typed method and a per-event key allow-list (schema.go), so content such as prompts,
// clipboard text, file names or screen text has no path into a property. Capture never blocks: it queues under a
// mutex, and the sender goroutine does the network work and retries what the endpoint could not take yet.
//
// Layer: rank 7, inherited from internal/diag. It imports diag, logx, platform and safego, never store.
package analytics
