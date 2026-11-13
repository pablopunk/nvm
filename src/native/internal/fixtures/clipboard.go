package fixtures

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// clipboardEpoch is the time of the first generated clipboard entry; entries are a minute apart.
var clipboardEpoch = time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC)

// Clipboard generator seeds, fixed so the history is stable across runs.
const (
	clipboardSeedHigh = 0x636c6970
	clipboardSeedLow  = 0x68697374
)

// ClipboardEntry is one item of clipboard history as a copy produced it. Exactly one of Text and PNG is
// set. Concealed entries carry text a password manager marked as not to be recorded; history must drop them.
type ClipboardEntry struct {
	Text      string
	PNG       []byte
	Concealed bool
	At        time.Time
}

// ClipboardEntries returns n entries in copy order. Every 7th entry (offset 3) is a PNG image, every 11th
// (offset 5) is a concealed secret, and the rest are text: links, code, paths, mail addresses, sentences.
func ClipboardEntries(n int) []ClipboardEntry {
	rng := rand.New(rand.NewPCG(clipboardSeedHigh, clipboardSeedLow))
	entries := make([]ClipboardEntry, 0, max(n, 0))
	for i := range n {
		entry := ClipboardEntry{At: clipboardEpoch.Add(time.Duration(i) * time.Minute)}
		switch {
		case i%7 == 3:
			entry.PNG = iconPNG(fmt.Sprintf("clip-%d", i), 16)
		case i%11 == 5:
			entry.Text = generatedSecret(rng)
			entry.Concealed = true
		default:
			entry.Text = clipboardText(rng)
		}
		entries = append(entries, entry)
	}
	return entries
}

// clipboardText draws one plausible copied snippet.
func clipboardText(rng *rand.Rand) string {
	switch rng.IntN(8) {
	case 0:
		return fmt.Sprintf("https://github.com/example/repo-%d/pull/%d", rng.IntN(40), 100+rng.IntN(900))
	case 1:
		return fmt.Sprintf("func handle%d(ctx context.Context) error { return nil }", rng.IntN(90))
	case 2:
		return fmt.Sprintf("/home/user/Projects/atlas/src/module-%d/index.ts", rng.IntN(60))
	case 3:
		return fmt.Sprintf("team-%d@example.test", rng.IntN(30))
	case 4:
		return fmt.Sprintf("Meeting notes %d: agree on the release train and owners.", rng.IntN(50))
	case 5:
		return fmt.Sprintf("SELECT id, email FROM users WHERE plan = 'pro' LIMIT %d;", 10*(1+rng.IntN(9)))
	case 6:
		return fmt.Sprintf("  indented line %d with trailing spaces   ", rng.IntN(200))
	default:
		return fmt.Sprintf("Remember to water plant %d on Thursday", rng.IntN(12))
	}
}

// generatedSecret draws a password-like string. It is fixture data and is never a real credential.
func generatedSecret(rng *rand.Rand) string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!#$%"
	secret := make([]byte, 16)
	for i := range secret {
		secret[i] = alphabet[rng.IntN(len(alphabet))]
	}
	return string(secret)
}
