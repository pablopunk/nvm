package api

// ItemSection is a titled group of items inside a list or grid.
type ItemSection struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Items    []Item `json:"items"`
}

// DetailPane controls the inspector shown beside or below the selected item.
type DetailPane struct {
	// Placement is "side" (default) or "bottom".
	Placement string `json:"placement,omitempty"`
	// Visible is a pointer so that the zero value means "use the host default".
	Visible *bool `json:"visible,omitempty"`
}

// EmptyState is shown when an active result list has no items. Keep the subtitle short and do not
// tell users to open the action menu; the host shows the view's first action below it.
type EmptyState struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

// Pagination asks the host to offer more items. Next is run with the same invocation shape as
// any action and returns a ViewPatch with the next page.
type Pagination struct {
	HasMore  bool       `json:"hasMore"`
	PageSize int        `json:"pageSize,omitempty"`
	Next     *ActionRef `json:"next,omitempty"`
}

// ViewFilter narrows a view's rows by one choice, such as a category or a kind of file. The host shows it as a button in
// the footer bar, where the current option is named and a press opens a menu of Options with a check mark on Value, and
// in the action menu as Next and Previous actions on Command+] and Command+[. A view has no control in its search field:
// nothing an extension declares can place one there. A chosen option becomes Value and arrives as Invocation.Value
// through OnChange.
type ViewFilter struct {
	ID       string             `json:"id,omitempty"`
	Tooltip  string             `json:"tooltip,omitempty"`
	Value    string             `json:"value,omitempty"`
	Options  []ViewFilterOption `json:"options"`
	OnChange *ActionRef         `json:"onChange,omitempty"`
	// CycleOnTab makes Tab choose the next option, wrapping, and run OnChange.
	CycleOnTab bool `json:"cycleOnTab,omitempty"`
}

// ViewFilterOption is one value of a ViewFilter.
type ViewFilterOption struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

// Refresh asks the host to re-run a handler while the view is visible. Immediate paints the
// cached snapshot first and refreshes after paint. The host owns the timer and the cancellation.
// A refreshed list or grid may return a patch or a matching view; its rows update in place.
type Refresh struct {
	IntervalMs int64      `json:"intervalMs,omitempty"`
	Mode       PatchMode  `json:"mode,omitempty"`
	Immediate  bool       `json:"immediate,omitempty"`
	Ref        *ActionRef `json:"ref,omitempty"`
}

// DataSource replaces a view's items with a loader owned by the extension (see DataLoader).
// The host owns the loading, error and stale-while-revalidate lifecycle.
type DataSource struct {
	// Loader is the id that DataLoader.LoadItems receives.
	Loader string `json:"loader"`
	// CacheKey, when set, enables a persistent stale-while-revalidate snapshot.
	CacheKey   string `json:"cacheKey,omitempty"`
	TTLMs      int64  `json:"ttlMs,omitempty"`      // fresh window; default 60000
	StaleTTLMs int64  `json:"staleTtlMs,omitempty"` // stale window; default 300000
	Retry      bool   `json:"retry,omitempty"`
}
