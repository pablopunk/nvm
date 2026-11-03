package api

// ViewPatch updates the view that is currently visible, returned from an ActionRun handler.
//
// Go patches replace whole items: an item in Items with the same ID as a visible item replaces
// it. Partial field merges are not part of v1, because Go zero values cannot tell "unset" from
// "set to empty" without pointer-per-field types that would double the item model.
type ViewPatch struct {
	Mode          PatchMode `json:"mode,omitempty"`
	Items         []Item    `json:"items,omitempty"`
	RemoveItemIDs []string  `json:"removeItemIds,omitempty"`
	// IsLoading is a pointer so that nil means "leave loading unchanged".
	IsLoading      *bool  `json:"isLoading,omitempty"`
	SelectedItemID string `json:"selectedItemId,omitempty"`
}
