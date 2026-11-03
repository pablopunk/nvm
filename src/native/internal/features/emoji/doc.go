// Package emoji is the catalog behind the Emoji & Symbols picker: emoji, symbols and Unicode
// values, with search by name, keyword, character and code point, skin tone variants, categories,
// recents through an injected source, and Unicode details for the copy actions.
//
// The data comes from src/app/extensions/emoji-symbol-data.ts through the generator in gen/, and
// catalog_data.go is checked in. Layer: features; the package imports only the standard library.
//
// Search uses the palette's score bands (score.go). WP1.1's search package replaces that scorer;
// keep the bands in step until then.
package emoji
