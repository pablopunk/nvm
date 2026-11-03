package emoji

// AllCategory is the category that lists every entry. Empty and unknown category names normalize to it.
const AllCategory = "all"

// Catalog is the built-in character catalog. Its data is static, so every Catalog behaves the same and
// the value carries no state; the lookups are derived once and shared.
type Catalog struct{}

// NewCatalog returns the built-in catalog.
func NewCatalog() *Catalog {
	return &Catalog{}
}

// RecentSource supplies the ids of recently used entries, newest first, as Entry.ID formats them.
// Persistence belongs to the caller; the catalog only reads.
type RecentSource interface {
	RecentIDs() []string
}

// All returns every entry in catalog order: emoji first, then symbols.
func (c *Catalog) All() []Entry {
	out := make([]Entry, len(records))
	for index := range records {
		out[index] = entryAt(index)
	}
	return out
}

// Categories returns the group names in catalog order, without AllCategory.
func (c *Catalog) Categories() []string {
	out := make([]string, 0, len(categoryNames))
	for _, id := range categoryNames {
		out = append(out, stringAt(id))
	}
	return out
}

// NormalizeCategory returns value when it names a category, and AllCategory otherwise.
func (c *Catalog) NormalizeCategory(value string) string {
	if _, ok := categoryIDOf(value); ok {
		return value
	}
	return AllCategory
}

// Lookup returns the entry with the given id, such as "emoji:U+1F600".
func (c *Catalog) Lookup(id string) (Entry, bool) {
	index, ok := builtinLookups().byID[id]
	if !ok {
		return Entry{}, false
	}
	return entryAt(int(index)), true
}

// Recents resolves the source's ids to entries, newest first. Unknown and repeated ids are skipped, and
// a positive limit caps the result.
func (c *Catalog) Recents(source RecentSource, limit int) []Entry {
	if source == nil {
		return nil
	}
	byID := builtinLookups().byID
	seen := map[uint16]bool{}
	var out []Entry
	for _, id := range source.RecentIDs() {
		if limit > 0 && len(out) >= limit {
			break
		}
		index, ok := byID[id]
		if !ok || seen[index] {
			continue
		}
		seen[index] = true
		out = append(out, entryAt(int(index)))
	}
	return out
}

// categoryIDOf finds the string id of a group name. AllCategory is not a group, so it reports false.
func categoryIDOf(name string) (uint16, bool) {
	for _, id := range categoryNames {
		if stringAt(id) == name {
			return id, true
		}
	}
	return 0, false
}
