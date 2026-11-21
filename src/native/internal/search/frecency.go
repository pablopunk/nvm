package search

import "math"

// The weights of a tiered search, in rank points. A row's rank inside its tier is its class base plus its frecency, so
// the base of a class is what a better class is worth and the frecency is what use is worth. Neither crosses a tier.
const (
	// ClassGap is the rank between two neighbouring classes.
	ClassGap = 100
	// FrecencyScale multiplies the log of the uses: n uses, the last just now, add FrecencyScale * ln(1+n). A week of daily
	// use (+124) passes a class; one use (+41), or an old one, does not.
	FrecencyScale = 60
	// FrecencyHalfLifeDays is how long it takes a row's frecency to halve when it is not used.
	FrecencyHalfLifeDays = 14
	// MaxFrecencyUses bounds the uses that count, so a huge count cannot grow without limit.
	MaxFrecencyUses = 1000
	// DirectBase lifts a Direct row above the class bases and the frecency of any other row.
	DirectBase = 100000

	millisPerDay = 24 * 60 * 60 * 1000
)

// classBase is the rank a class starts from.
func classBase(c Class) int {
	switch c {
	case ClassApplication:
		return 3 * ClassGap
	case ClassExtensionCommand:
		return 2 * ClassGap
	case ClassCommand:
		return ClassGap
	}
	return 0
}

// classOf resolves a row's class: the one it sets, else the one its Kind implies.
func classOf(it *Item) Class {
	if it.Class != ClassDefault {
		return it.Class
	}
	switch it.Kind {
	case KindApplications:
		return ClassApplication
	case KindCommands:
		return ClassCommand
	}
	return ClassOther
}

// Frecency is what use adds to a row's rank: the log of its uses, halved every FrecencyHalfLifeDays since its last use.
// It is zero for a row never used. nowMs and lastUsedMs are Unix milliseconds.
func Frecency(uses int, lastUsedMs, nowMs int64) int {
	if uses <= 0 {
		return 0
	}
	days := max(0, float64(nowMs-lastUsedMs)/millisPerDay)
	decay := math.Exp2(-days / FrecencyHalfLifeDays)
	return int(FrecencyScale * math.Log1p(float64(min(uses, MaxFrecencyUses))) * decay)
}
