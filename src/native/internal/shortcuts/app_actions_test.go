package shortcuts

import "testing"

func TestAppActionTitleIsTheAppNameFromItsPath(t *testing.T) {
	actionID := AppActionID("/Applications/Slack.app")
	if actionID != "app:/Applications/Slack.app" {
		t.Fatalf("AppActionID = %q", actionID)
	}
	title, ok := DescribeApp(actionID)
	if !ok || title != "Slack" {
		t.Fatalf("DescribeApp = %q, %v; want Slack", title, ok)
	}
	if _, ok := DescribeApp("extension-action:window-management:layout.left-half"); ok {
		t.Error("a window action is titled as an app")
	}
}
