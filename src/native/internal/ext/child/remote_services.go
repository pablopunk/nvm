package child

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/ext/protocol"
)

// remoteSecrets is Secrets backed by the host's OS keychain. Values pass through the exchange only.
type remoteSecrets struct {
	link *hostLink
}

func (s remoteSecrets) Get(name string) (string, bool, error) {
	var result protocol.GetResult
	if err := s.link.Call(protocol.ServiceSecrets, protocol.OpGet, protocol.KeyArgs{Key: name}, &result); err != nil || !result.Found {
		return "", false, err
	}
	var value string
	if err := json.Unmarshal(result.Value, &value); err != nil {
		return "", false, api.ErrInvalidInput
	}
	return value, true, nil
}

func (s remoteSecrets) Set(name, value string) error {
	raw, _ := json.Marshal(value)
	return s.link.Call(protocol.ServiceSecrets, protocol.OpSet, protocol.KeyValueArgs{Key: name, Value: raw}, nil)
}

func (s remoteSecrets) Delete(name string) error {
	return s.link.Call(protocol.ServiceSecrets, protocol.OpDelete, protocol.KeyArgs{Key: name}, nil)
}

func (s remoteSecrets) Clear() error {
	return s.link.Call(protocol.ServiceSecrets, protocol.OpClear, nil, nil)
}

// remoteSettings is Settings backed by the host. Definitions come from the manifest the host reviewed.
type remoteSettings struct {
	link *hostLink
}

func (s remoteSettings) Definitions() []api.SettingDefinition {
	var definitions []api.SettingDefinition
	_ = s.link.Call(protocol.ServiceSettings, protocol.OpDefinitions, nil, &definitions)
	return definitions
}

func (s remoteSettings) Get(id string, into any) error {
	result, err := readEntry(s.link, protocol.ServiceSettings, id)
	if err != nil {
		return err
	}
	if !result.Found {
		return api.ErrNotFound
	}
	return decodeEntry(result.Value, into)
}

func (s remoteSettings) Set(id string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return api.ErrInvalidInput
	}
	return s.link.Call(protocol.ServiceSettings, protocol.OpSet, protocol.KeyValueArgs{Key: id, Value: raw}, nil)
}

func (s remoteSettings) Toggle(id string) (json.RawMessage, error) {
	var value json.RawMessage
	err := s.link.Call(protocol.ServiceSettings, protocol.OpToggle, protocol.KeyArgs{Key: id}, &value)
	return value, err
}

func (s remoteSettings) PublishDisplay(id string, text string) error {
	raw, err := json.Marshal(text)
	if err != nil {
		return api.ErrInvalidInput
	}
	return s.link.Call(protocol.ServiceSettings, protocol.OpPublishDisplay, protocol.KeyValueArgs{Key: id, Value: raw}, nil)
}

// remoteLogs is Logs backed by the host's file log. The host stamps every record with this extension's id.
type remoteLogs struct {
	link *hostLink
}

func (l remoteLogs) Log(level api.LogLevel, message string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		raw = nil
	}
	_ = l.link.Call(protocol.ServiceLogs, protocol.OpLog, protocol.LogArgs{Level: level, Message: message, Data: raw}, nil)
}

func (l remoteLogs) Recent(query api.LogQuery) ([]api.LogEntry, error) {
	var entries []api.LogEntry
	err := l.link.Call(protocol.ServiceLogs, protocol.OpRecent, query, &entries)
	return entries, err
}

// remoteText is Text backed by the host, which knows the clipboard and the standard variables.
type remoteText struct {
	link *hostLink
}

func (t remoteText) Template(input string, options api.TemplateOptions) (api.TemplateResult, error) {
	var result api.TemplateResult
	err := t.link.Call(protocol.ServiceText, protocol.OpTemplate, protocol.TemplateArgs{Input: input, Options: options}, &result)
	return result, err
}

// remoteNetwork is Network backed by the host's public-address HTTP port.
type remoteNetwork struct {
	link *hostLink
}

func (n remoteNetwork) Fetch(request api.FetchRequest) (api.FetchResponse, error) {
	var response api.FetchResponse
	err := n.link.Call(protocol.ServiceNetwork, protocol.OpFetch, request, &response)
	return response, err
}

func (n remoteNetwork) Download(request api.DownloadRequest) (api.DownloadResponse, error) {
	var response api.DownloadResponse
	err := n.link.Call(protocol.ServiceNetwork, protocol.OpDownload, request, &response)
	return response, downloadUnavailableOnOlderHosts(err)
}

// downloadUnavailableOnOlderHosts turns an older host's "network has no operation" refusal of network.download into
// the capability-unavailable error, so an extension built for this API can probe or fall back instead of failing as if
// it had sent bad input.
func downloadUnavailableOnOlderHosts(err error) error {
	if err != nil && errors.Is(err, api.ErrInvalidInput) && strings.Contains(err.Error(), "has no operation") {
		return fmt.Errorf("network.download: this host cannot stream downloads: %w", api.ErrNotAvailable)
	}
	return err
}
