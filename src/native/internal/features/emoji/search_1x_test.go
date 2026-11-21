package emoji

import "testing"

// oneXGrid is the grid of 1.x's Emoji & Symbols picker for each query: total is how many entries its filter keeps, at
// the 50 minimum score of the grid, and head is the first of them in 1.x's order. The expected values come from running
// 1.x's own picker view (extensions/emoji-symbols.ts) through its grid filter (1.x/palette/filtering.ts),
// not from this port. "copy" matches every entry because 1.x searches the titles of the actions a row carries.
var oneXGrid = []struct {
	query string
	total int
	head  []string
}{
	{
		query: "cute",
		total: 5,
		head: []string{
			"Person Bowing",
			"Tiger Face",
			"Spouting Whale",
			"Diamond With A Dot",
			"Acute Accent",
		},
	},
	{
		query: "heart",
		total: 54,
		head: []string{
			"Smiling Face With Heart-Eyes",
			"Face Blowing A Kiss",
			"Smiling Cat With Heart-Eyes",
			"Love Letter",
			"Heart With Arrow",
			"Beating Heart",
			"Revolving Hearts",
			"Two Hearts",
			"Broken Heart",
			"Kiss Mark",
			"Hand With Index Finger And Thumb Crossed",
			"Kiss",
			"Kiss Woman, Man",
			"Kiss Man, Man",
			"Kiss Woman, Woman",
		},
	},
	{
		query: "smile",
		total: 174,
		head: []string{
			"Smile",
			"Grinning Face",
			"Grinning Face With Big Eyes",
			"Grinning Face With Smiling Eyes",
			"Beaming Face With Smiling Eyes",
			"Grinning Squinting Face",
			"Grinning Face With Sweat",
			"Slightly Smiling Face",
			"Upside-Down Face",
			"Winking Face",
			"Smiling Face With Smiling Eyes",
			"Smiling Face With Halo",
			"Smiling Face With Heart-Eyes",
			"Star-Struck",
			"Smiling Face",
		},
	},
	{
		query: "arrow",
		total: 357,
		head: []string{
			"Clockwise Vertical Arrows",
			"Counterclockwise Arrows Button",
			"BACK Arrow",
			"END Arrow",
			"ON! Arrow",
			"SOON Arrow",
			"Shuffle Tracks Button",
			"Repeat Button",
			"Repeat Single Button",
			"Play Button",
			"Fast-Forward Button",
			"Next Track Button",
			"Play Or Pause Button",
			"Reverse Button",
			"Fast Reverse Button",
		},
	},
	{
		query: "copy",
		total: 3870,
		head: []string{
			"Grinning Face",
			"Grinning Face With Big Eyes",
			"Grinning Face With Smiling Eyes",
		},
	},
	{
		query: "fire",
		total: 14,
		head: []string{
			"Fire",
			"Firefighter",
			"Man Firefighter",
			"Woman Firefighter",
			"Candle",
			"Name Badge",
			"Heart On Fire",
			"Fire Engine",
			"Fireworks",
			"Sparkler",
			"Firecracker",
			"Fire Extinguisher",
			"Sparkle",
			"Trigram For Fire",
		},
	},
	{
		query: "u+1f600",
		total: 1,
		head: []string{
			"Grinning Face",
		},
	},
	{
		query: "thumbs",
		total: 2,
		head: []string{
			"Thumbs Up",
			"Thumbs Down",
		},
	},
}

// TestSearchMatchesThe1xGridOnTheSameQueries compares the matches and their first rows with 1.x's grid.
func TestSearchMatchesThe1xGridOnTheSameQueries(t *testing.T) {
	catalog := NewCatalog()
	for _, tt := range oneXGrid {
		t.Run(tt.query, func(t *testing.T) {
			results := catalog.Search(tt.query, Options{})
			if len(results) != tt.total {
				t.Fatalf("Search(%q) keeps %d entries, 1.x keeps %d", tt.query, len(results), tt.total)
			}
			for index, want := range tt.head {
				if got := results[index].Title(); got != want {
					t.Errorf("Search(%q) row %d = %q, want %q as in 1.x", tt.query, index, got, want)
				}
			}
		})
	}
}
