package main

import (
	"github.com/pablopunk/nvm/native/internal/ext/api"
)

// acme.media is the dev fixture of the two page views: the camera, in its default and its contain-and-mirror forms,
// and the webview, with no permission, with the camera declared, and with a page that tries to reach out. Each
// command opens one view, the way a real extension does.

// staticWebviewPage is a document that reads the palette's theme variables and links out, which a click must not
// follow inside the view.
const staticWebviewPage = `<!doctype html><html><head><meta charset="utf-8"></head><body>
<h1 style="color: var(--nvm-text-strong); font-size: 15px;">Webview fixture</h1>
<p style="color: var(--nvm-text-secondary); font-size: 12px;">A sandboxed page with no Go bridge.</p>
<a href="https://example.test/">Open example</a>
</body></html>`

// cameraWebviewPage asks for the camera and reports the answer in its text, so a view shows what the page got.
const cameraWebviewPage = `<!doctype html><html><head><meta charset="utf-8"></head><body>
<p id="status" style="color: var(--nvm-text-strong);">Asking for the camera</p>
<video id="feed" autoplay playsinline style="width: 100%;"></video>
<script>
navigator.mediaDevices.getUserMedia({ video: true }).then(function (stream) {
  document.getElementById('feed').srcObject = stream;
  document.getElementById('status').textContent = 'Camera ready';
}, function (error) {
  document.getElementById('status').textContent = 'Camera refused: ' + error.name;
});
</script>
</body></html>`

// hostileWebviewPage tries what an untrusted page must not do: reach the host through the parent frame, navigate
// the top window, and read the clipboard. Only the link is a user action; nothing runs on load.
const hostileWebviewPage = `<!doctype html><html><head><meta charset="utf-8"></head><body>
<p id="status" style="color: var(--nvm-text-strong);">Untrusted page</p>
<a id="out" href="https://example.test/">Leave</a>
<button id="probe" type="button">Probe the host</button>
<script>
document.getElementById('probe').addEventListener('click', function () {
  var report = [];
  try { report.push(typeof window.mygo); } catch (error) { report.push('mygo blocked'); }
  try { report.push(typeof window.parent.nvm); } catch (error) { report.push('parent blocked'); }
  document.getElementById('status').textContent = report.join(', ');
});
</script>
</body></html>`

var (
	mutedFalse = false

	cameraView = api.View{
		ID: "camera-fixture", Kind: api.ViewCamera, Title: "Camera fixture",
		Camera: &api.CameraSpec{},
	}
	cameraContainView = api.View{
		ID: "camera-contain-fixture", Kind: api.ViewCamera, Title: "Camera contain fixture",
		Camera: &api.CameraSpec{Mirrored: true, Framing: "contain", Controls: true, Muted: &mutedFalse},
	}
	webviewView = api.View{
		ID: "webview-fixture", Kind: api.ViewWebview, Title: "Webview fixture",
		Webview: &api.WebviewSpec{HTML: staticWebviewPage},
	}
	webviewCameraView = api.View{
		ID: "webview-camera-fixture", Kind: api.ViewWebview, Title: "Webview camera fixture",
		Webview: &api.WebviewSpec{HTML: cameraWebviewPage, Permissions: []api.WebviewPermission{api.WebviewCamera}},
	}
	webviewHostileView = api.View{
		ID: "webview-hostile-fixture", Kind: api.ViewWebview, Title: "Webview hostile fixture",
		Webview: &api.WebviewSpec{HTML: hostileWebviewPage},
	}
)

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{
		{ID: "camera", Title: "Camera fixture"},
		{ID: "camera-contain", Title: "Camera contain fixture"},
		{ID: "webview", Title: "Webview fixture"},
		{ID: "webview-camera", Title: "Webview camera fixture"},
		{ID: "webview-hostile", Title: "Webview hostile fixture"},
	}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	switch in.Ref.ID {
	case "camera":
		return api.Result{View: &cameraView}, nil
	case "camera-contain":
		return api.Result{View: &cameraContainView}, nil
	case "webview":
		return api.Result{View: &webviewView}, nil
	case "webview-camera":
		return api.Result{View: &webviewCameraView}, nil
	case "webview-hostile":
		return api.Result{View: &webviewHostileView}, nil
	}
	return api.Result{}, nil
}
