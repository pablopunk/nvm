// Package calc answers calculator queries for the palette: arithmetic with percentages and functions, unit
// conversions, calendar and clock questions, and currency conversions priced from cached exchange rates.
//
// The package is a pure library with two entry points on Calculator. Evaluate answers offline and never reads
// rates. Query answers everything Evaluate does, then prices currency questions from the cache and queues any
// missing pair; the composition root calls Refresh from a goroutine afterwards to fetch queued pairs. Neither
// entry point waits on the network, and time comes only from the platform Clock port.
//
// Results carry Display (what the palette shows), Copy (what Copy Result and Paste Result put on the clipboard),
// Raw (unformatted, for Copy Unformatted Result and Continue Calculation), Detail lines, and for currency answers
// Stale plus CacheAge, which the view uses to label quotes older than a day.
//
// Wiring for WP1.1 (search.Provider): the provider keeps one Calculator, and its Search(ctx, q) calls
// Query(q). A returned Result becomes one item whose primary action copies Copy, whose secondary actions are
// Continue Calculation (Raw), Swap Units (SwapQuery, when set) and Copy Unformatted Result (Raw, when it differs
// from Display), and whose subtitle is Subtitle. After a currency query returns no answer the provider calls
// go calc.Refresh(ctx) in a background goroutine with the search context, so a later keystroke finds the quote.
// Query and Refresh never block on the network, so typing and command execution stay off the critical path.
//
// The behavior contract is 1.x/electron/search/calculator.ts and its tests, ported case for case.
package calc
