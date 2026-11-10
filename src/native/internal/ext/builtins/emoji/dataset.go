package emoji

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
)

// Dataset is the picker's bundled dataset in catalog order: every character with the fields the picker reads from the
// dataset. The app ships it as internal/ext/bundled/datasets/emoji-symbols.json, which the update test writes.
func Dataset() []api.BundleRecord {
	entries := characterLibrary.All()
	records := make([]api.BundleRecord, 0, len(entries))
	for _, entry := range entries {
		records = append(records, api.BundleRecord{
			ID: entry.ID(), Title: entry.Title(), Subtitle: entry.Codepoints(),
			Keywords: curatedKeywords(entry),
			Fields: map[string]string{
				"glyph": entry.Glyph(), "group": entry.Category(), "kind": kindField(entry),
				"skinTone": boolField(entry.SupportsSkinTone()),
			},
		})
	}
	return records
}

// curatedKeywords are the keywords the catalog lists after the glyph, name, group and Unicode values.
func curatedKeywords(entry emoji.Entry) []string {
	keywords := entry.Keywords()
	const identityFields = 4
	if len(keywords) <= identityFields {
		return nil
	}
	return keywords[identityFields:]
}

func kindField(entry emoji.Entry) string {
	if entry.Kind() == emoji.KindEmoji {
		return "emoji"
	}
	return "symbol"
}

func boolField(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
