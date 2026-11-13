// Package budget lists the limits the host holds an extension to, in one place an author can read. Each limit is per
// extension and refills continuously over a minute, so a burst up to the budget is allowed at once. A call over its
// budget fails with a quota error that names the limit; it never fails silently.
package budget

const (
	// ShellRunsPerMinute is the budget Shell.Exec, Script, AppleScript and Which share. A scan of many items, such as
	// every app in /Applications, fits in it. Past it, run one Script that loops over the items instead of one Exec per
	// item.
	ShellRunsPerMinute = 600

	// DesktopActionsPerMinute is the budget opening paths and links, launching apps, window moves and clipboard writes
	// share.
	DesktopActionsPerMinute = 120

	// OCRReadsPerMinute is the budget of text reads from image files.
	OCRReadsPerMinute = 20

	// ScreenReadsPerMinute is the budget of screen and region reads. One read holds a worker for seconds, so it is
	// lower than the others.
	ScreenReadsPerMinute = 6

	// NotificationsPerMinute is the budget of system notifications.
	NotificationsPerMinute = 10
)
