package pi

import (
	"encoding/json"
	"strings"

	"github.com/pablopunk/nvm/native/internal/backend"
)

// contentText returns the text of a message and its pictures, whether its content is a string or a list of parts.
func contentText(message backend.Message) (string, []image) {
	if len(message.Content) == 0 {
		return "", nil
	}
	var text string
	if json.Unmarshal(message.Content, &text) == nil {
		return text, nil
	}
	var parts []backend.ContentPart
	if json.Unmarshal(message.Content, &parts) != nil {
		return "", nil
	}
	var texts []string
	var pictures []image
	for _, part := range parts {
		switch {
		case part.Type == "text" && part.Text != "":
			texts = append(texts, part.Text)
		case part.ImageURL != nil:
			if picture, ok := dataURLImage(part.ImageURL.URL); ok {
				pictures = append(pictures, picture)
			}
		}
	}
	return strings.Join(texts, "\n"), pictures
}

// userContent is the text and pictures of the user's message.
func userContent(message backend.Message) (string, []image) {
	return contentText(message)
}

// dataURLImage reads a base64 data URL such as data:image/png;base64,AAAA.
func dataURLImage(url string) (image, bool) {
	rest, ok := strings.CutPrefix(url, "data:")
	if !ok {
		return image{}, false
	}
	header, data, ok := strings.Cut(rest, ",")
	mime, encoding, _ := strings.Cut(header, ";")
	if !ok || encoding != "base64" || !strings.HasPrefix(mime, "image/") {
		return image{}, false
	}
	return image{MimeType: mime, Data: data}, true
}

// transcript writes the earlier conversation as text, newest last and bounded, for a host that has no context yet. Tool
// calls and their results are left out: the model needs what was said, not how the answers were found.
func transcript(history []backend.Message) string {
	var lines []string
	for _, message := range history {
		text, _ := contentText(message)
		text = strings.TrimSpace(text)
		switch {
		case text == "":
		case message.Role == backend.RoleUser:
			lines = append(lines, "User: "+text)
		case message.Role == backend.RoleAssistant:
			lines = append(lines, "Assistant: "+text)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	joined := strings.Join(lines, "\n\n")
	runes := []rune(joined)
	if len(runes) > maxHistoryRunes {
		joined = string(runes[len(runes)-maxHistoryRunes:])
	}
	return "Earlier in this conversation:\n\n" + joined
}
