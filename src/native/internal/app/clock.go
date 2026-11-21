package app

import "time"

// clock is the instant relative times are measured from. Golden tests replace it with a fixed one.
var clock = time.Now
