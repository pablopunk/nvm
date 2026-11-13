// Package emoji is the catalog behind the Emoji & Symbols picker: emoji, symbols and Unicode
// values, with search by name, keyword, character and code point, skin tone variants, categories,
// and Unicode details for the copy actions.
//
// The data comes from gen/emoji-symbol-data.ts through the generator in gen/, and
// catalog_data.go is checked in. Layer: features; the package imports only the standard library.
//
// Search matches and orders entries as 1.x's Emoji & Symbols grid does (score.go, search.go).
package emoji
