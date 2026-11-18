// Package search defines what the palette searches and how results are ranked: Item and Action, the
// Host a row's action runs against, the ranking bands, and (in the coordinator) the Provider interface
// that feeds results off the UI thread. Layer: search, above design.
//
// The package draws nothing: an Item carries plain data, and a Detail closure is the only place a row
// touches ui. The palette package decides how rows are shown.
//
// Typed root searches rank each available snapshot independently of provider arrival order. A provider may opt in to
// carrying safe, matching rows into a narrower query; its first authoritative batch replaces those rows. The palette
// follows the best ranked result until a user chooses one, and retains that choice by identity while it remains.
package search
