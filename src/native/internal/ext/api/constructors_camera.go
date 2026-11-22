package api

// SelectCamera switches the open camera view to device. Use it in a camera view's own action panel.
func SelectCamera(device CameraDevice) Action {
	return Action{Kind: ActionCamera, Op: CameraSelectDevice, Title: device.Title, Icon: "camera", Target: device.ID}
}
