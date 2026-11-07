package extai

import (
	"encoding/json"

	"github.com/pablopunk/nvm/native/internal/ai/chat"
	"github.com/pablopunk/nvm/native/internal/backend"
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// TierOf maps an extension's model option to a route tier. The empty option is smart, the backend's own default.
func TierOf(model api.ModelTier) (chat.Tier, error) {
	switch model {
	case "", api.ModelSmart:
		return chat.TierSmart, nil
	case api.ModelFast:
		return chat.TierFast, nil
	}
	return "", inputErrorf("Unsupported AI model role: %s. Use 'smart' or 'fast'.", model)
}

// promptWithContext joins the prompt and the text of its attachments. Context follows the prompt, after a blank line.
func promptWithContext(prompt, context string) string {
	if context == "" {
		return prompt
	}
	return prompt + "\n\n" + context
}

// contentPart is one part of a multi-part user message: text, or an image as a data URL.
type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// userMessage is the user turn of a prompt. Without images it is plain text; with images it lists the text and each
// image as a data URL, which is the form the chat route reads.
func userMessage(prompt string, images []backend.DecisionImage) backend.Message {
	if len(images) == 0 {
		return backend.Message{Role: backend.RoleUser, Content: backend.TextContent(prompt)}
	}
	parts := []contentPart{{Type: "text", Text: prompt}}
	for _, image := range images {
		parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: dataURL(image)}})
	}
	content, _ := json.Marshal(parts)
	return backend.Message{Role: backend.RoleUser, Content: content}
}
