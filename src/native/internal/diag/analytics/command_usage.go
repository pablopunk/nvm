package analytics

import (
	"regexp"
	"strings"
)

const commandUsageDedupeLimit = 200

var commandVariantID = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9._:-]{0,63}$`)

type commandUsageState struct {
	keys  map[string]struct{}
	order []string
}

// CommandUsed records one canonical command.run fact per journey.
func (c *Client) CommandUsed(completion Completion, variant string) bool {
	if c == nil || c.cfg.Endpoint == "" || !validOperationOutcome(completion.Outcome) ||
		!diagnosticIDPattern.MatchString(completion.BootID) || !diagnosticIDPattern.MatchString(completion.JourneyID) ||
		!diagnosticIDPattern.MatchString(completion.ActionID) ||
		!validCommandVariant(variant) {
		return false
	}
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if !c.consent.Load() || completion.Outcome == OutcomeCancelled {
		return false
	}
	key := completion.JourneyID + ":" + FeatureCommandRun
	if c.commandUsage.keys == nil {
		c.commandUsage.keys = make(map[string]struct{})
	}
	if _, exists := c.commandUsage.keys[key]; exists {
		return false
	}
	c.commandUsage.keys[key] = struct{}{}
	c.commandUsage.order = append(c.commandUsage.order, key)
	if len(c.commandUsage.order) > commandUsageDedupeLimit {
		delete(c.commandUsage.keys, c.commandUsage.order[0])
		c.commandUsage.order = c.commandUsage.order[1:]
	}
	props := map[string]any{"feature": FeatureCommandRun, "outcome": string(completion.Outcome)}
	if variant != "" {
		props["variant"] = variant
	}
	if completion.Session.id != "" && completion.Session.epoch == c.session.epoch {
		props["$session_id"] = completion.Session.id
	}
	for key, value := range c.releaseProperties() {
		props[key] = value
	}
	return c.capture(EventFeatureUsed, props)
}

func validCommandVariant(variant string) bool {
	if variant == "" || variant == "user-extension" {
		return true
	}
	for _, value := range commandVariants {
		if variant == value {
			return true
		}
	}
	ids, ok := strings.CutPrefix(variant, "extension-command:")
	if !ok {
		return false
	}
	if commandVariantID.MatchString(ids) {
		return true
	}
	for i := range ids {
		if ids[i] == ':' && commandVariantID.MatchString(ids[:i]) && commandVariantID.MatchString(ids[i+1:]) {
			return true
		}
	}
	return false
}

// BuiltInCommandVariant formats validated host-owned source identifiers.
func BuiltInCommandVariant(kind, extensionID, commandID string) (string, bool) {
	if kind != "extension-command" || !commandVariantID.MatchString(extensionID) || commandID != "" && !commandVariantID.MatchString(commandID) {
		return "", false
	}
	variant := kind + ":" + extensionID
	if commandID != "" {
		variant += ":" + commandID
	}
	return variant, true
}
