package api

// MenuItem is a menu entry with a host-owned opaque handle.
type MenuItem struct {
	Handle   string   `json:"handle"`
	Title    string   `json:"title"`
	Path     []string `json:"path"`
	Shortcut string   `json:"shortcut"`
	Enabled  bool     `json:"enabled"`
	Checked  bool     `json:"checked"`
}

// MenuSnapshot is the immediately available cached menu data.
type MenuSnapshot struct {
	Items   []MenuItem `json:"items"`
	Loading bool       `json:"loading"`
}

// Menus accesses the menu of the host-bound source; it requires desktop.menus.
type Menus interface {
	Snapshot() (MenuSnapshot, error)
	Invoke(handle string) error
}
