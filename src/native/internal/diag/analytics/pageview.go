package analytics

// PageViewed records one host-defined visible view and its session atomically.
func (c *Client) PageViewed(view PageView) bool {
	if c == nil || !validPageView(view) {
		return false
	}
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	snapshot, ok := c.touchSessionLocked()
	if !ok {
		return false
	}
	return c.capture(EventPageview, map[string]any{"view": string(view), "$session_id": snapshot.SessionID()})
}

func validPageView(view PageView) bool {
	switch view {
	case PageViewPalette, PageViewExtension, PageViewAIChat:
		return true
	default:
		return false
	}
}
