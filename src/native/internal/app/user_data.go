package app

import (
	"fmt"

	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/features/snippets"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/store"
)

// openUserData wires the notes and snippets to the app's one store (opened with the directory's lease in
// attachServices), so only one instance writes; a second instance reads both features and writes nothing, and is
// told so. The store is flushed and closed when the app quits, after its windows have closed.
func (p *App) openUserData() {
	st := p.userStore
	if st == nil {
		logx.Default().Warn("notes and snippets are unavailable", logx.F{"reason": "the store did not open"})
		return
	}
	if p.userReadOnly {
		logx.Default().Warn("notes and snippets open read-only", logx.F{})
	}
	p.notes = notes.New(st, searchClock, p.userReadOnly)
	p.snips = snippets.New(st, searchClock, p.userReadOnly)
	p.userNotices = st.Notices()
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { p.quitting = true })
	mygo.App.OnQuit(func() {
		p.closeEdits()
		_ = st.Close()
	})
	installCrashFlush(st)
}

// installCrashFlush makes a panic in a background call write the unsaved drafts first. The safego reporter is the
// composition root's hook for failures, so it is installed here, once, with the clock the app uses.
func installCrashFlush(st *store.Store) {
	flushOnPanic := st.InstallCrashFlush()
	safego.SetDefaults(safego.Defaults{
		Clock: searchClock,
		Reporter: safego.ReporterFunc(func(f safego.Failure) string {
			if f.Panic {
				flushOnPanic()
			}
			logx.Default().Warn("background call failed", logx.F{"name": f.Name, "attempt": f.Attempt, "panic": f.Panic})
			return ""
		}),
	})
}

// showUserDataNotices tells the user, without blocking, about notes or snippets that were set aside, and that
// this instance is read-only. It reads only the notices already queued.
func (p *App) showUserDataNotices() {
	if p.userReadOnly {
		p.Toast("Quick Notes and Snippets are open in another Nevermind window, so they are read-only here")
	}
	for {
		select {
		case notice, ok := <-p.userNotices:
			if !ok {
				return
			}
			if notice.Kind == store.Quarantined {
				p.Toast(fmt.Sprintf("Saved notes or snippets could not be read and were moved to %s", notice.Path))
			}
		default:
			return
		}
	}
}

// closeEdits ends every open editing session, saving what is pending, before the store closes.
func (p *App) closeEdits() {
	for id, edit := range p.edits {
		_ = edit.sess.Close()
		delete(p.edits, id)
	}
}
