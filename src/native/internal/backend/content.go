package backend

import "encoding/json"

// ContentPart is one part of a message whose content is a list, in the OpenAI chat format the route takes: text, or
// an image given as a data URL.
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL names the picture of an image part: a data URL the route decodes.
type ImageURL struct {
	URL string `json:"url"`
}

// TextPart is a text part of a message's content.
func TextPart(text string) ContentPart {
	return ContentPart{Type: "text", Text: text}
}

// ImagePart is an image part whose picture is the data URL url.
func ImagePart(url string) ContentPart {
	return ContentPart{Type: "image_url", ImageURL: &ImageURL{URL: url}}
}

// PartsContent returns the JSON list encoding of parts, for Message.Content.
func PartsContent(parts []ContentPart) json.RawMessage {
	data, err := json.Marshal(parts)
	if err != nil {
		return nil
	}
	return data
}
