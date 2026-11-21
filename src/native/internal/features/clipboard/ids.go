package clipboard

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashLength is the number of hex digits in a content hash. It matches the image file names the store
// creates and the garbage collector accepts.
const HashLength = 40

// hashHex returns the first HashLength hex digits of the SHA-256 of data.
func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:HashLength]
}

// TextID is the entry ID of plain text: the hash of its trimmed value. Blank text has no ID.
func TextID(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	return "text:" + hashHex([]byte(trimmed))
}

// HTMLID is the entry ID of HTML markup.
func HTMLID(html string) string {
	return "html:" + hashHex([]byte(html))
}

// RTFID is the entry ID of RTF markup.
func RTFID(rtf string) string {
	return "rtf:" + hashHex([]byte(rtf))
}

// FilesID is the entry ID of a list of file paths, in order.
func FilesID(paths []string) string {
	return "files:" + hashHex([]byte(strings.Join(paths, "\n")))
}

// ImageID is the entry ID of an image with the given content hash.
func ImageID(hash string) string {
	return "image:" + hash
}
