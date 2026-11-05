package files

import "github.com/pablopunk/nvm/native/internal/ext/api"

// askedFor reports whether the user asked the decision about query.
func (s *viewState) askedFor(query string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.askedQuery == query
}

// recordDecision keeps the decision rows for a query, so a later search for it shows them without asking again.
func (s *viewState) recordDecision(query string, rows []api.Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.askedQuery = query
	s.rows = rows
}

// decisionRows are the rows the decision recorded for the query asked last.
func (s *viewState) decisionRows(query string) []api.Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.askedQuery != query {
		return nil
	}
	return s.rows
}
