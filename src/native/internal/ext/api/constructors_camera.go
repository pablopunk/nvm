package api

// CameraView builds a live camera view of the camera with deviceID, a CameraDevice.ID; empty shows the camera the view
// last chose, else the system default.
func CameraView(title, deviceID string) View {
	return View{Kind: ViewCamera, Title: title, Camera: &CameraSpec{DeviceID: deviceID}}
}

// SelectCamera switches the open camera view to device. Use it in a camera view's own action panel.
func SelectCamera(device CameraDevice) Action {
	return Action{Kind: ActionCamera, Op: CameraSelectDevice, Title: device.Title, Icon: "camera", Target: device.ID}
}
