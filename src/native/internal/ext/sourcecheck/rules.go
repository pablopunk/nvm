// Package sourcecheck reads an extension's Go source without running it. It finds the host services a source calls,
// which capability each one needs, and the titles of the commands and root rows it declares. The builder uses it to
// refuse a reply before it is installed; the host uses the same rules to explain a refusal.
//
// Layer: ext. The package imports api and the standard library only.
package sourcecheck

import "github.com/pablopunk/nvm/native/internal/ext/api"

// Rule says that a call whose selector chain contains Path needs Capability. A chain is the names of the methods a
// source calls from its api.Context to the service, such as Desktop, Shell, AppleScript. The first matching rule of a
// call decides, so the more specific paths come first. These rules mirror the capability gates in package services.
type Rule struct {
	Path       []string
	Capability api.Capability
}

// rules lists the gated services of the host. Each entry names the gate it mirrors in package services.
var rules = []Rule{
	{Path: []string{"Desktop", "Shell", "OpenExternal"}, Capability: api.CapabilityDesktopFiles},
	{Path: []string{"Desktop", "Shell"}, Capability: api.CapabilitySystem},
	{Path: []string{"Desktop", "Files"}, Capability: api.CapabilityDesktopFiles},
	{Path: []string{"Desktop", "Apps"}, Capability: api.CapabilityDesktopApps},
	{Path: []string{"Desktop", "Clipboard"}, Capability: api.CapabilityDesktopClipboard},
	{Path: []string{"Desktop", "WindowManagement"}, Capability: api.CapabilityWindowManagement},
	{Path: []string{"Windows"}, Capability: api.CapabilityWindowManagement},
	{Path: []string{"AI", "Attachments", "Clipboard"}, Capability: api.CapabilityDesktopClipboard},
	{Path: []string{"AI", "Attachments", "OCRImage"}, Capability: api.CapabilityOCR},
	{Path: []string{"AI", "Attachments", "File"}, Capability: api.CapabilityDesktopFiles},
	{Path: []string{"AI", "Attachments", "SelectedFiles"}, Capability: api.CapabilityDesktopFiles},
	{Path: []string{"AI"}, Capability: api.CapabilityAI},
	{Path: []string{"Clipboard"}, Capability: api.CapabilityClipboardHistory},
	{Path: []string{"OCR"}, Capability: api.CapabilityOCR},
	{Path: []string{"Network"}, Capability: api.CapabilityNetwork},
	{Path: []string{"Secrets"}, Capability: api.CapabilitySecrets},
	{Path: []string{"Settings", "Set"}, Capability: api.CapabilitySettingsWrite},
	{Path: []string{"Settings", "Toggle"}, Capability: api.CapabilitySettingsWrite},
	{Path: []string{"Shortcuts"}, Capability: api.CapabilityShortcuts},
}

// Rules returns the capability rules, most specific first. The builder's prompt is written from this list.
func Rules() []Rule {
	return append([]Rule(nil), rules...)
}
