package app

import (
	"github.com/egoist/mygo"

	"github.com/pablopunk/nvm/native/internal/features/notes"
	"github.com/pablopunk/nvm/native/internal/features/snippets"
	"github.com/pablopunk/nvm/native/internal/logx"
	"github.com/pablopunk/nvm/native/internal/store"
)

// openUserData wires the notes and snippets to the app's one store (opened with the directory's lease in
// attachServices), so only one instance writes; a second instance reads both features and writes nothing, and is
// told so. The store is flushed and closed when the app quits, after its windows have closed.
func (p *App) openUserData() {
	st := p.userStore
	if st == nil {
		logx.Default().Warn("notes and snippets are unavailable", logx.F{"reason": "the store did not open"})
		p.installFailureReporting(nil)
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
	p.installFailureReporting(st.InstallCrashFlush())
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
				p.Toast(quarantineToast(notice))
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
