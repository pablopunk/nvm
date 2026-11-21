package extview

import (
	"strings"
	"testing"
)

// TestParseCameraTitleReadsOnlyReports proves a title with the camera prefix and a JSON body is a report, and any
// other title is ignored, so a page's own title can never drive the capture.
func TestParseCameraTitleReadsOnlyReports(t *testing.T) {
	event, ok := parseCameraTitle(`nvm-camera:{"seq":4,"event":"live","id":2,"deviceId":"cam","label":"FaceTime"}`)
	if !ok || event.Event != "live" || event.ID != 2 || event.DeviceID != "cam" || event.Label != "FaceTime" {
		t.Fatalf("live report = %+v, ok %v", event, ok)
	}
	devices, ok := parseCameraTitle(`nvm-camera:{"seq":5,"event":"devices","devices":[{"deviceId":"a","label":"A"}]}`)
	if !ok || len(devices.Devices) != 1 || devices.Devices[0].DeviceID != "a" {
		t.Fatalf("devices report = %+v, ok %v", devices, ok)
	}
	for _, title := range []string{"Gallery", "nvm-camera:not json", "", "NVM-CAMERA:{}"} {
		if _, ok := parseCameraTitle(title); ok {
			t.Errorf("title %q was read as a report", title)
		}
	}
}

// TestCameraPageHTMLCarriesTheConfigSafely proves the view's options reach the page, the placeholder is replaced, and
// a value cannot end the script element that holds it.
func TestCameraPageHTMLCarriesTheConfigSafely(t *testing.T) {
	html := cameraPageHTML(cameraPageConfig{Muted: true, Controls: true, Mirrored: true, Framing: "contain"})
	if strings.Contains(html, cameraConfigPlaceholder) {
		t.Fatal("the config placeholder was left in the document")
	}
	if !strings.Contains(html, `configure({"muted":true,"controls":true,"mirrored":true,"framing":"contain"})`) {
		t.Fatalf("config not in the page: %s", html[strings.Index(html, "configure(")-10:strings.Index(html, "configure(")+80])
	}
	hostile := scriptSafeJSON([]byte(`"</script><script>alert(1)"`))
	if strings.Contains(hostile, "</script>") || strings.Contains(hostile, "<") {
		t.Fatalf("script-unsafe JSON: %s", hostile)
	}
}

// TestCameraCommandScriptCallsThePageDriver proves each command reaches the one entry point the page defines.
func TestCameraCommandScriptCallsThePageDriver(t *testing.T) {
	script := cameraCommandScript(cameraCommand{Op: "start", ID: 3, DeviceID: "cam"})
	if want := `window.nvmCamera.command({"op":"start","id":3,"deviceId":"cam"})`; script != want {
		t.Fatalf("script = %s, want %s", script, want)
	}
	if want := `window.nvmCamera.command({"op":"stop"})`; cameraCommandScript(cameraCommand{Op: "stop"}) != want {
		t.Fatalf("stop script = %s", cameraCommandScript(cameraCommand{Op: "stop"}))
	}
}
