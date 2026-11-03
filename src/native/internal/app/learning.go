package app

import (
	"fmt"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/palette"
	"github.com/pablopunk/nvm/native/internal/platform/real"
	"github.com/pablopunk/nvm/native/internal/search/usage"
	"github.com/pablopunk/nvm/native/internal/store"
)

// queriesDocument is queries.json: the root's query history, oldest first.
type queriesDocument struct {
	Queries []string `json:"queries"`
}

// openLearning opens the app's store and the two things the root learns: the usage memory (usage.json) and the
// query history (queries.json). It returns the store's notices, which the caller shows once the palette exists.
// The store is closed when the app quits, which writes what is still pending. If the store cannot be opened,
// the learning works for this run and is not saved.
func (p *App) openLearning() <-chan store.Notice {
	ports := real.NewPorts()
	s, err := store.Open(ports.FS, searchClock, configDir())
	if err != nil {
		logx.Default().Warn("learning is not saved", logx.F{"error": err.Error()})
		p.learn = usage.NewMemory(searchClock)
		p.history = palette.NewQueryHistory(nil, nil)
		return nil
	}
	mygo.App.OnQuit(func() { _ = s.Close() })
	p.appStore = s

	learn, err := usage.Open(s, searchClock, logx.Default())
	if err != nil {
		logx.Default().Warn("usage memory opened empty", logx.F{"error": err.Error()})
	}
	p.learn = learn

	queries := store.Doc(s, "queries.json", 1, func() queriesDocument { return queriesDocument{} })
	saved, err := queries.Load()
	if err != nil {
		logx.Default().Warn("query history opened empty", logx.F{"error": err.Error()})
	}
	p.history = palette.NewQueryHistory(saved.Queries, func(entries []string) {
		_ = queries.Update(func(d *queriesDocument) { d.Queries = entries })
	})
	return s.Notices()
}

// showLearningNotices tells the user, without blocking, about learned files that were set aside. It reads only
// the notices already queued, so it never waits.
func (p *App) showLearningNotices(notices <-chan store.Notice) {
	for {
		select {
		case notice, ok := <-notices:
			if !ok {
				return
			}
			if notice.Kind == store.Quarantined {
				p.Toast(fmt.Sprintf("Saved rankings could not be read and were moved to %s", notice.Path))
			}
		default:
			return
		}
	}
}
