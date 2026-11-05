package api

// Item is one row or tile. Items come from a view, a RootProvider or a SearchProvider. An item
// is never a durable command: durable, shortcut-able, aliasable actions are Commands (command.go).
type Item struct {
	// ID is stable within its view or provider. Ranking, recents, patches and focus use it.
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	Aliases  []string `json:"aliases,omitempty"`
	// Icon is a Lucide icon name such as "camera" or "volume-2".
	Icon string `json:"icon,omitempty"`
	// Glyph is one character drawn natively as a grid tile visual (emoji, symbols).
	Glyph string `json:"glyph,omitempty"`
	Image *Image `json:"image,omitempty"`
	Video string `json:"video,omitempty"`
	// File is the local file the row or grid cell drags out to other applications, as 1.x's path does. Without
	// DragImage, the drag shows the file itself when it is an image. Relative paths do not drag. A built-in row with no
	// Image or Icon shows the file's system icon.
	File *FileRef `json:"file,omitempty"`
	// DragImage is the local image file the drag shows instead of File, such as a thumbnail. It needs a File.
	DragImage   *FileRef    `json:"dragImage,omitempty"`
	Accessories []Accessory `json:"accessories,omitempty"`
	// Primary runs on Enter. Actions are secondary and listed in the action menu. The host removes
	// a Primary that repeats inside Actions.
	Primary               *Action         `json:"primary,omitempty"`
	Actions               []Action        `json:"actions,omitempty"`
	ActionPanel           *ActionPanel    `json:"actionPanel,omitempty"`
	ActionPanelVisibility PanelVisibility `json:"actionPanelVisibility,omitempty"`
	Appearance            *Appearance     `json:"appearance,omitempty"`
	// Disabled rows stay visible as informational context and cannot be selected.
	Disabled bool `json:"disabled,omitempty"`
	// Contextual, with Disabled, renders full-contrast guidance text that wraps. Use it for
	// messages the user must read, such as errors. Passive status rows leave it off.
	Contextual bool    `json:"contextual,omitempty"`
	Detail     *Detail `json:"detail,omitempty"`
	// CustomizeAs keys a built-in row's shortcut and alias, such as app:<path>. Folder extensions cannot set it.
	CustomizeAs string `json:"customizeAs,omitempty"`
	// Section and LearnKey are host-only presentation for built-in extensions: the root section a row is listed under
	// (for example "Commands") and the key the usage learner ranks it by. The host ignores both for folder extensions.
	Section  string `json:"section,omitempty"`
	LearnKey string `json:"learnKey,omitempty"`
	// TintTitle draws the title in the row's Appearance color, as the native clipboard rows did. Like Section, it is a
	// presentation the host offers built-in extensions.
	TintTitle bool `json:"tintTitle,omitempty"`
	// Shortcut is a local shortcut that is active only while this item is selected.
	Shortcut string `json:"shortcut,omitempty"`
	// QueryFallback marks an open-ended row that echoes the query (for example a web search row). The host offers it
	// for every typed query, below the rows whose words match the query and above loose letter matches. Its Score is
	// ignored. RootItems ignore the flag, so the row never shows at the empty root. Only SearchProvider rows may set it.
	// The host leaves it out while a row with AnswersQuery is shown.
	QueryFallback bool `json:"queryFallback,omitempty"`
	// AnswersQuery marks a row that answers the query outright, such as a calculation. The host then leaves out the
	// QueryFallback rows of the same search. Only SearchProvider rows may set it.
	AnswersQuery bool `json:"answersQuery,omitempty"`
	// Score is a ranking hint. The host caps it and combines it with usage signals. Query fallbacks ignore it.
	Score float64 `json:"score,omitempty"`
	// LastUsedMs is epoch milliseconds for the item's last use, an intrinsic recency signal.
	LastUsedMs int64 `json:"lastUsedMs,omitempty"`
}

// Accessory is a small trailing label or badge on an item row.
type Accessory struct {
	Text    string `json:"text,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Tone    Tone   `json:"tone,omitempty"`
	Tooltip string `json:"tooltip,omitempty"`
}

// Image is the only image shape. Use Src for a single image, or Light and Dark for theme variants.
// Local files go through FileRef.URL or the files service, never a raw path.
type Image struct {
	Src      string     `json:"src,omitempty"`
	Light    string     `json:"light,omitempty"`
	Dark     string     `json:"dark,omitempty"`
	Fallback string     `json:"fallback,omitempty"`
	Alt      string     `json:"alt,omitempty"`
	Fit      ImageFit   `json:"fit,omitempty"`
	Shape    ImageShape `json:"shape,omitempty"`
	Tint     Color      `json:"tint,omitempty"`
	Mask     ImageMask  `json:"mask,omitempty"`
}

// Appearance is the semantic visual treatment of a row. It never sets pixel colours.
type Appearance struct {
	Foreground Color `json:"foreground,omitempty"`
	// Background is "accent" for a subtle tinted row.
	Background string `json:"background,omitempty"`
	// Prominence is "primary" to elevate one consequential action. Use it on at most one action per
	// surface; the host owns focus and depth.
	Prominence string `json:"prominence,omitempty"`
}

// Detail is the inspector content for a side or bottom pane.
type Detail struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	// Facts are labelled values drawn large, one pair after another, below the other content. Use them for the
	// answer a row gives, such as the lines of a calculation.
	Facts    []Fact     `json:"facts,omitempty"`
	Metadata []Metadata `json:"metadata,omitempty"`
	Image    *Image     `json:"image,omitempty"`
	Video    string     `json:"video,omitempty"`
	Link     *LinkCard  `json:"link,omitempty"`
	Actions  []Action   `json:"actions,omitempty"`
}

// Fact is one labelled value in a detail pane: a small label above the value.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// MetadataKind selects the shape of a Metadata row.
type MetadataKind string

const (
	MetadataText      MetadataKind = "text"
	MetadataLink      MetadataKind = "link"
	MetadataTag       MetadataKind = "tag"
	MetadataSeparator MetadataKind = "separator"
)

// Metadata is one labelled row in a detail pane. Kind selects which fields are read.
type Metadata struct {
	Kind     MetadataKind `json:"kind,omitempty"`
	Label    string       `json:"label,omitempty"`
	Value    string       `json:"value,omitempty"`
	URL      string       `json:"url,omitempty"`
	Copyable bool         `json:"copyable,omitempty"`
	Tone     Tone         `json:"tone,omitempty"`
}

// LinkCard is a link preview. Hosts fill title, description and image from Open Graph metadata;
// the extension supplies the URL and the status.
type LinkCard struct {
	URL         string `json:"url"`
	Status      string `json:"status"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	SiteName    string `json:"siteName,omitempty"`
	// Image is a data URL inlined by the host, so the palette never contacts the page origin.
	Image string `json:"image,omitempty"`
}
