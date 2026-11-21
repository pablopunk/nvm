package charsearch

// Record is one catalog character the funnel can choose: its key for the caller, the glyph and name the model reads,
// and the group (category) it belongs to.
type Record struct {
	Key   string
	Glyph string
	Name  string
	Group string
}

// label is the text a choice option or a match question names the character by.
func (r Record) label() string {
	return r.Glyph + " " + r.Name
}

// lexicalOnlyGroups are matched by name or code point only, never by meaning: a flag is found by its country name.
var lexicalOnlyGroups = map[string]bool{"Flags": true, "Technical": true, "Math": true}

// semanticGroupsOf returns the groups the model is asked about, in catalog order, and their characters in catalog
// order. Only the first maxGroupQuestions groups are asked, as one decision answers at most that many questions.
func semanticGroupsOf(records []Record) ([]string, map[string][]Record) {
	byGroup := map[string][]Record{}
	var groups []string
	for _, r := range records {
		if lexicalOnlyGroups[r.Group] {
			continue
		}
		if _, seen := byGroup[r.Group]; !seen {
			groups = append(groups, r.Group)
		}
		byGroup[r.Group] = append(byGroup[r.Group], r)
	}
	if len(groups) > maxGroupQuestions {
		groups = groups[:maxGroupQuestions]
	}
	return groups, byGroup
}
