package extview

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// cameraPageTemplate is the document of a camera view. It holds one video element and a small driver; the palette
// draws everything around it. The page has no bridge: it reports only by setting its title, and the title carries
// one JSON report, which the host reads with parseCameraTitle.
//
//go:embed camera_page.html
var cameraPageTemplate string

const (
	cameraConfigPlaceholder = "__CONFIG__"
	cameraTitlePrefix       = "nvm-camera:"
)

// cameraPageConfig is how the view starts its video: muted, with controls, mirrored, and cover or contain framing.
type cameraPageConfig struct {
	Muted    bool   `json:"muted"`
	Controls bool   `json:"controls"`
	Mirrored bool   `json:"mirrored"`
	Framing  string `json:"framing"`
}

// cameraPageHTML returns the camera document configured for a view.
func cameraPageHTML(config cameraPageConfig) string {
	encoded, _ := json.Marshal(config)
	return strings.Replace(cameraPageTemplate, cameraConfigPlaceholder, scriptSafeJSON(encoded), 1)
}

// scriptSafeJSON escapes the characters that could end a <script> element inside JSON text.
func scriptSafeJSON(encoded []byte) string {
	return strings.NewReplacer("<", "\\u003c", ">", "\\u003e", "&", "\\u0026").Replace(string(encoded))
}

// cameraCommandScript is the script that runs one command in the page.
func cameraCommandScript(command any) string {
	encoded, _ := json.Marshal(command)
	return "window.nvmCamera.command(" + scriptSafeJSON(encoded) + ")"
}

// cameraConfigureCommand is the command that updates the page's presentation without restarting the stream.
type cameraConfigureCommand struct {
	Op      string           `json:"op"`
	Options cameraPageConfig `json:"options"`
}

// parseCameraTitle reads the report of a camera page from its title. Any other title is not a report.
func parseCameraTitle(title string) (cameraEvent, bool) {
	body, ok := strings.CutPrefix(title, cameraTitlePrefix)
	if !ok {
		return cameraEvent{}, false
	}
	var report struct {
		Event    string         `json:"event"`
		ID       int            `json:"id"`
		DeviceID string         `json:"deviceId"`
		Label    string         `json:"label"`
		Name     string         `json:"name"`
		Message  string         `json:"message"`
		Devices  []cameraDevice `json:"devices"`
	}
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		return cameraEvent{}, false
	}
	return cameraEvent{
		Event: report.Event, ID: report.ID, DeviceID: report.DeviceID, Label: report.Label,
		Name: report.Name, Message: report.Message, Devices: report.Devices,
	}, true
}
