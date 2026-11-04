package backend

import (
	"context"
	"strconv"
	"time"
)

// compatTimeout matches the desktop app's compatibility fetch budget.
const compatTimeout = 5 * time.Second

// BackendInfo describes the deployed backend.
type BackendInfo struct {
	// Version is the deployed backend version.
	Version string `json:"version"`
	// Environment is the deployment environment name.
	Environment string `json:"environment"`
}

// APIInfo lists the API majors the backend serves.
type APIInfo struct {
	// CurrentVersion is the API major the backend prefers.
	CurrentVersion int `json:"currentVersion"`
	// SupportedVersions lists every API major the backend still answers.
	SupportedVersions []int `json:"supportedVersions"`
}

// DesktopInfo carries the release policy for desktop clients. The manifest's latestVersion and updateUrl are not
// decoded: they describe the 1.x line until the 2.0 client name ships (src/native/docs/cutover.md, section 4).
type DesktopInfo struct {
	// MinimumSupportedVersion is the oldest client version the backend accepts.
	MinimumSupportedVersion string `json:"minimumSupportedVersion"`
	// SupportPolicy names the support window policy.
	SupportPolicy string `json:"supportPolicy"`
}

// ClientStatus is the backend's view of the request's client identity and whether it is accepted.
type ClientStatus struct {
	// Name is the X-Nevermind-Client value the backend read.
	Name string `json:"name"`
	// Version is the X-Nevermind-Client-Version value the backend read.
	Version string `json:"version"`
	// APIVersion is the X-Nevermind-API-Version value the backend read, or zero.
	APIVersion int `json:"apiVersion"`
	// Platform is the X-Nevermind-Platform value the backend read.
	Platform string `json:"platform"`
	// Arch is the X-Nevermind-Arch value the backend read.
	Arch string `json:"arch"`
	// Compatible reports whether the backend accepts this client.
	Compatible bool `json:"compatible"`
	// UnsupportedReason explains an unaccepted client, such as unsupported_desktop_version.
	UnsupportedReason string `json:"unsupportedReason"`
}

// Notice is a message the backend wants to show, with its severity.
type Notice struct {
	// Type is "info", "warning" or "force_update".
	Type string `json:"type"`
	// Message is the text to show.
	Message string `json:"message"`
}

// Manifest is the compatibility manifest the backend publishes at /api/compatibility.
type Manifest struct {
	// Backend describes the deployment.
	Backend BackendInfo `json:"backend"`
	// API lists the supported API majors.
	API APIInfo `json:"api"`
	// Desktop is the release policy.
	Desktop DesktopInfo `json:"desktop"`
	// Client is this client as the backend sees it.
	Client ClientStatus `json:"client"`
	// Features lists always-on feature flags kept for released clients.
	Features map[string]bool `json:"features"`
	// Notices are messages to show, such as a force update.
	Notices []Notice `json:"notices"`
}

// CompatReport is the manifest plus the state this client derives from it.
type CompatReport struct {
	// Manifest is the manifest the backend returned. Zero when State is CompatUnknown.
	Manifest Manifest
	// State says what the user should do, from the most urgent condition to none.
	State CompatState
}

// CompatState is the compatibility outcome shown to users.
type CompatState string

// The compatibility states, in increasing urgency order except that Unknown means no manifest was available.
const (
	// CompatUnknown means the backend did not publish a manifest, so no judgement is possible.
	CompatUnknown CompatState = "unknown"
	// CompatOK means this client is current enough and the API major is supported.
	CompatOK CompatState = "ok"
	// CompatUpdateRequired means the backend refuses this client version or has issued a force update.
	CompatUpdateRequired CompatState = "update_required"
	// CompatSunset means the backend retired this client's API major, so an update is the only way forward.
	CompatSunset CompatState = "sunset"
)

// compatWire mirrors the manifest with pointers on the fields the client requires, so a missing field is
// detected instead of read as zero.
type compatWire struct {
	Backend *struct {
		Version     *string `json:"version"`
		Environment *string `json:"environment"`
	} `json:"backend"`
	API *struct {
		CurrentVersion    *int  `json:"currentVersion"`
		SupportedVersions []int `json:"supportedVersions"`
	} `json:"api"`
	Desktop *struct {
		MinimumSupportedVersion *string `json:"minimumSupportedVersion"`
		SupportPolicy           string  `json:"supportPolicy"`
	} `json:"desktop"`
	Client *struct {
		Name              string  `json:"name"`
		Version           string  `json:"version"`
		APIVersion        int     `json:"apiVersion"`
		Platform          string  `json:"platform"`
		Arch              string  `json:"arch"`
		Compatible        *bool   `json:"compatible"`
		UnsupportedReason *string `json:"unsupportedReason"`
	} `json:"client"`
	Features map[string]bool `json:"features"`
	Notices  []Notice        `json:"notices"`
}

// Compatibility fetches the manifest and derives the state. A backend without the route (HTTP 404) yields
// CompatUnknown and no error, so an older deployment never blocks the app. The call is an idempotent GET.
func (c *Client) Compatibility(ctx context.Context) (CompatReport, error) {
	var wire compatWire
	err := c.callJSON(ctx, requestSpec{
		method:     "GET",
		path:       pathCompatibility,
		auth:       authOptional,
		timeout:    compatTimeout,
		idempotent: true,
	}, &wire)
	if statusOf(err) == statusNotFound {
		return CompatReport{State: CompatUnknown}, nil
	}
	if err != nil {
		return CompatReport{}, err
	}
	manifest, err := wire.manifest()
	if err != nil {
		return CompatReport{}, err
	}
	return CompatReport{Manifest: manifest, State: c.deriveState(manifest)}, nil
}

// manifest checks the required fields the way the desktop app does and converts the wire form.
func (w compatWire) manifest() (Manifest, error) {
	if w.Backend == nil || w.Backend.Version == nil || w.Backend.Environment == nil ||
		w.API == nil || w.API.CurrentVersion == nil || w.API.SupportedVersions == nil ||
		w.Desktop == nil || w.Desktop.MinimumSupportedVersion == nil ||
		w.Client == nil || w.Client.Compatible == nil || w.Features == nil {
		return Manifest{}, &Error{Kind: KindBadResponse, Status: 200, Message: "compatibility manifest is missing required fields"}
	}
	manifest := Manifest{
		Backend: BackendInfo{Version: *w.Backend.Version, Environment: *w.Backend.Environment},
		API:     APIInfo{CurrentVersion: *w.API.CurrentVersion, SupportedVersions: w.API.SupportedVersions},
		Desktop: DesktopInfo{
			MinimumSupportedVersion: *w.Desktop.MinimumSupportedVersion,
			SupportPolicy:           w.Desktop.SupportPolicy,
		},
		Client: ClientStatus{
			Name:       w.Client.Name,
			Version:    w.Client.Version,
			APIVersion: w.Client.APIVersion,
			Platform:   w.Client.Platform,
			Arch:       w.Client.Arch,
			Compatible: *w.Client.Compatible,
		},
		Features: w.Features,
		Notices:  w.Notices,
	}
	if w.Client.UnsupportedReason != nil {
		manifest.Client.UnsupportedReason = *w.Client.UnsupportedReason
	}
	return manifest, nil
}

// deriveState judges the manifest for this client. The order of the checks is the order of urgency.
func (c *Client) deriveState(manifest Manifest) CompatState {
	if !apiVersionSupported(manifest.API.SupportedVersions, APIVersion) || manifest.Client.UnsupportedReason == "unsupported_api_version" {
		return CompatSunset
	}
	if !manifest.Client.Compatible || hasForceUpdateNotice(manifest.Notices) {
		return CompatUpdateRequired
	}
	return CompatOK
}

// apiVersionSupported reports whether the backend still serves the API major this client speaks.
func apiVersionSupported(supported []int, apiVersion string) bool {
	version, err := strconv.Atoi(apiVersion)
	if err != nil {
		return false
	}
	for _, candidate := range supported {
		if candidate == version {
			return true
		}
	}
	return false
}

// hasForceUpdateNotice reports whether any notice demands an update.
func hasForceUpdateNotice(notices []Notice) bool {
	for _, notice := range notices {
		if notice.Type == "force_update" {
			return true
		}
	}
	return false
}

// statusOf returns the HTTP status of a backend error, or zero.
func statusOf(err error) int {
	if typed := asError(err); typed != nil {
		return typed.Status
	}
	return 0
}
