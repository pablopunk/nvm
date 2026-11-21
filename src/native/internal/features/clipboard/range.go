package clipboard

import (
	"fmt"
	"time"
)

// Range names the entries a removal or a match applies to. The names are the 1.x action values.
type Range string

// Removal ranges. Item and IDs select by ID; the time windows select by copy time; OlderThan and All are
// the broad ones.
const (
	RangeItem         Range = "item"
	RangeIDs          Range = "ids"
	RangeLast5Minutes Range = "last-5-minutes"
	RangeLastHour     Range = "last-hour"
	RangeLastDay      Range = "last-day"
	RangeLastWeek     Range = "last-week"
	RangeOlderThan    Range = "older-than"
	RangeAll          Range = "all"
)

// rangeWindows are the look-back spans of the time ranges.
var rangeWindows = map[Range]time.Duration{
	RangeLast5Minutes: 5 * time.Minute,
	RangeLastHour:     time.Hour,
	RangeLastDay:      24 * time.Hour,
	RangeLastWeek:     7 * 24 * time.Hour,
}

// RangeQuery selects entries by range. ItemID is used by RangeItem, ItemIDs by RangeIDs, OlderThan by
// RangeOlderThan. Kinds, when set, narrows any range to those kinds.
type RangeQuery struct {
	Range     Range
	ItemID    string
	ItemIDs   []string
	Kinds     []Kind
	OlderThan time.Duration
}

// RemovedMessage is the confirmation shown after a removal of count entries.
func RemovedMessage(count int) string {
	if count == 1 {
		return "Removed 1 clipboard item"
	}
	return fmt.Sprintf("Removed %d clipboard items", count)
}
