// Package search defines what the palette searches and how results are ranked: Item and Action, the
// Host a row's action runs against, the ranking bands, and (in the coordinator) the Provider interface
// that feeds results off the UI thread. Layer: search, above design.
//
// The package draws nothing: an Item carries plain data, and a Detail closure is the only place a row
// touches ui. The palette package decides how rows are shown.
//
// Typed root searches publish one complete provider result per query. Exact cached results are immediate and
// revalidation only updates future cache entries. A narrowing keeps the previously leading row first while it still
// matches by name and its provider still vouches for it, whether or not that provider answers in time. A cold query
// always publishes once each source has answered or reached its hard deadline (AggregateHardBudget for a self-budgeted
// aggregate): a source that missed it contributes nothing, and its late rows are dropped rather than reshuffling the
// published list. While a cold query runs, the palette retains the last coherent list with its old query
// attribution so it cannot run as the new query. Extension and resource authority is checked on action execution.
package search
