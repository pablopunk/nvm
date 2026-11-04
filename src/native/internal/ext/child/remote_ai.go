package child

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteAI makes the model calls on the host, which holds the account and the routes. Streams are pulled; see
// remote_ai_stream.go.
type remoteAI struct {
	link *hostLink
}

// defaultSessionID is the session of a call that names none, as 1.x names it.
const defaultSessionID = "default"

func (a remoteAI) IsAvailable() (bool, error) {
	var result protocol.AIAvailability
	err := a.link.Call(protocol.ServiceAI, protocol.OpIsAvailable, nil, &result)
	return result.Available, err
}

func (a remoteAI) Decide(request api.DecisionRequest) (api.DecisionResult, error) {
	var result api.DecisionResult
	err := a.link.Call(protocol.ServiceAI, protocol.OpDecide, protocol.AIDecideArgs{Request: request}, &result)
	return result, err
}

func (a remoteAI) Ask(prompt string, options api.AIOptions) (string, error) {
	var text string
	err := a.link.Call(protocol.ServiceAI, protocol.OpAsk, protocol.AIAskArgs{Prompt: prompt, Options: options}, &text)
	return text, err
}

func (a remoteAI) Stream(prompt string, options api.AIOptions) (api.AIStream, error) {
	return a.open(protocol.AIAskArgs{Prompt: prompt, Options: options})
}

// open starts a stream on the host and returns the pull side of it.
func (a remoteAI) open(args protocol.AIAskArgs) (api.AIStream, error) {
	var opened protocol.AIStreamOpened
	if err := a.link.Call(protocol.ServiceAI, protocol.OpStreamOpen, args, &opened); err != nil {
		return nil, err
	}
	return newRemoteStream(a.link, opened.ID), nil
}

func (a remoteAI) Prepare(options api.AIOptions) error {
	return a.link.Call(protocol.ServiceAI, protocol.OpPrepare, protocol.AIPrepareArgs{Options: options}, nil)
}

func (a remoteAI) Attachments() api.AttachmentBuilder {
	return remoteAttachments{link: a.link}
}

func (a remoteAI) Session(id string, options api.AIOptions) (api.AISession, error) {
	if id == "" {
		id = defaultSessionID
	}
	return remoteSession{link: a.link, id: id, options: options}, nil
}

// remoteSession is a conversation the host keeps under id. The options it was made with apply to every call that
// leaves them empty.
type remoteSession struct {
	link    *hostLink
	id      string
	options api.AIOptions
}

func (s remoteSession) Ask(prompt string, options api.AIOptions) (string, error) {
	var text string
	args := protocol.AIAskArgs{Prompt: prompt, Options: mergeAIOptions(s.options, options), Session: s.id}
	err := s.link.Call(protocol.ServiceAI, protocol.OpAsk, args, &text)
	return text, err
}

func (s remoteSession) Stream(prompt string, options api.AIOptions) (api.AIStream, error) {
	args := protocol.AIAskArgs{Prompt: prompt, Options: mergeAIOptions(s.options, options), Session: s.id}
	return remoteAI{link: s.link}.open(args)
}

func (s remoteSession) Reset() error {
	return s.link.Call(protocol.ServiceAI, protocol.OpSessionReset, protocol.KeyArgs{Key: s.id}, nil)
}

// mergeAIOptions returns base with the fields of call that are set.
func mergeAIOptions(base, call api.AIOptions) api.AIOptions {
	if call.Model != "" {
		base.Model = call.Model
	}
	if call.System != "" {
		base.System = call.System
	}
	base.Attachments = call.Attachments
	return base
}

// remoteAttachments builds attachments. Text, images and files are plain data; the clipboard and OCR are read on the
// host. Selected text and selected files have no host port yet, so they answer unavailable.
type remoteAttachments struct {
	link *hostLink
}

func (a remoteAttachments) Text(text, title string) api.Attachment {
	return api.Attachment{Kind: "text", Text: text, Title: title}
}

func (a remoteAttachments) Image(pathOrDataURL, title string, ocr bool) api.Attachment {
	attachment := api.Attachment{Kind: "image", Title: title, OCR: ocr}
	if isDataURL(pathOrDataURL) {
		attachment.DataURL = pathOrDataURL
	} else {
		attachment.Path = pathOrDataURL
	}
	return attachment
}

func (a remoteAttachments) File(path, as, title string) api.Attachment {
	return api.Attachment{Kind: "file", Path: path, As: as, Title: title}
}

func (a remoteAttachments) SelectedText(string) (api.Attachment, error) {
	return api.Attachment{}, unsupported("ctx.ai.attachments.selectedText")
}

func (a remoteAttachments) SelectedFiles(string) ([]api.Attachment, error) {
	return nil, unsupported("ctx.ai.attachments.selectedFiles")
}

func (a remoteAttachments) Clipboard(as string) ([]api.Attachment, error) {
	var result protocol.AIAttachments
	err := a.link.Call(protocol.ServiceAI, protocol.OpAttachClipboard, protocol.AIClipboardArgs{As: as}, &result)
	return result.Attachments, err
}

func (a remoteAttachments) OCRImage(path, title string) (api.Attachment, error) {
	var result protocol.AIAttachments
	args := protocol.AIOCRImageArgs{Path: path, Title: title}
	if err := a.link.Call(protocol.ServiceAI, protocol.OpAttachOCR, args, &result); err != nil {
		return api.Attachment{}, err
	}
	if len(result.Attachments) == 0 {
		return api.Attachment{}, fmt.Errorf("ctx.ai.attachments.ocrImage: the host returned no text: %w", api.ErrNotAvailable)
	}
	return result.Attachments[0], nil
}

// isDataURL reports whether s is an inline image rather than a path.
func isDataURL(s string) bool {
	return strings.HasPrefix(s, "data:")
}
