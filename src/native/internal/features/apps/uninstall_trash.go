package apps

import "time"

// The Trash run. It is the only place a path is moved, and it moves nothing without re-checking it: the app is
// checked again, the process state is checked again, and each item is revalidated immediately before its move.
// The app bundle goes last, so a failure in the data never leaves a working app without its data.

// Trash runs a reviewed selection. Candidates are taken from the snapshot by ID; anything not in the snapshot is
// ignored. An empty selection moves nothing.
func (u *Uninstaller) Trash(snapshot UninstallSnapshot, values map[string]bool) TrashResult {
	selection := Selected(snapshot, values)
	if len(selection) == 0 {
		return TrashResult{Status: StatusFailed, Notes: []Note{{Code: codeZeroSelect, Message: "Select at least one item to move to Trash."}}}
	}
	var appItem *Candidate
	var associated []Candidate
	for i := range selection {
		if selection[i].Kind == KindApp {
			appItem = &selection[i]
		} else {
			associated = append(associated, selection[i])
		}
	}
	app, ref := u.checkApp(snapshot.AppPath, &snapshot)
	if ref != nil {
		return refusedResult(selection, ref)
	}
	var notes []Note
	if u.isRunning(app.Path) {
		if appItem == nil {
			return refusedResult(selection, refusal(codeRunning, "Quit this application before moving its data to Trash"))
		}
		if quitRef := u.quitGracefully(app.Path); quitRef != nil {
			return refusedResult(selection, quitRef)
		}
		notes = append(notes, Note{Code: "app-quit", Message: "The application was quit before anything was moved to Trash."})
	}
	moved, untouched, stop := u.trashAssociated(snapshot, associated)
	if appItem != nil {
		appRef := stop
		if appRef == nil {
			appRef = u.appTrashRefusal(snapshot, *appItem)
		}
		switch {
		case appRef != nil:
			untouched = append(untouched, Untouched{Path: appItem.Path, Refusal: *appRef})
		case u.moveToTrash(appItem.Path):
			moved = append(moved, appItem.Path)
		default:
			untouched = append(untouched, Untouched{Path: appItem.Path, Refusal: *refusalTrashFailed()})
		}
	}
	return TrashResult{Status: trashStatus(moved, untouched), Moved: moved, Untouched: untouched, Notes: notes}
}

func refusalTrashFailed() *Refusal {
	return refusal(codeTrashFailed, "Could not move to Trash. Check its permissions.")
}

func refusedResult(items []Candidate, ref *Refusal) TrashResult {
	untouched := make([]Untouched, 0, len(items))
	for _, item := range items {
		untouched = append(untouched, Untouched{Path: item.Path, Refusal: *ref})
	}
	return TrashResult{Status: StatusFailed, Untouched: untouched}
}

func trashStatus(moved []string, untouched []Untouched) string {
	switch {
	case len(untouched) == 0:
		return StatusComplete
	case len(moved) > 0:
		return StatusPartial
	default:
		return StatusFailed
	}
}

// moveToTrash moves one item through the Trash port. It reports success only when the port did.
func (u *Uninstaller) moveToTrash(p string) bool {
	return u.deps.Trash != nil && u.deps.Trash(p) == nil
}

// trashAssociated moves the data items one by one. A preflight pass refuses the items that already fail; each move
// then re-checks the app and the item. A stop (the app changed or started running) leaves the rest untouched.
func (u *Uninstaller) trashAssociated(snapshot UninstallSnapshot, items []Candidate) (moved []string, untouched []Untouched, stop *Refusal) {
	preflight := map[string]*Refusal{}
	for _, item := range items {
		if ref := u.revalidate(item); ref != nil {
			preflight[item.ID] = ref
		}
	}
	for i, item := range items {
		if ref := preflight[item.ID]; ref != nil {
			untouched = append(untouched, Untouched{Path: item.Path, Refusal: *ref})
			continue
		}
		if current := u.currentAppRefusal(snapshot); current != nil {
			for _, rest := range items[i:] {
				untouched = append(untouched, Untouched{Path: rest.Path, Refusal: *current})
			}
			return moved, untouched, current
		}
		if ref := u.revalidate(item); ref != nil {
			untouched = append(untouched, Untouched{Path: item.Path, Refusal: *ref})
			continue
		}
		if u.moveToTrash(item.Path) {
			moved = append(moved, item.Path)
		} else {
			untouched = append(untouched, Untouched{Path: item.Path, Refusal: *refusalTrashFailed()})
		}
	}
	return moved, untouched, nil
}

// currentAppRefusal says why the app may not be touched right now: it changed, or it is running.
func (u *Uninstaller) currentAppRefusal(snapshot UninstallSnapshot) *Refusal {
	app, ref := u.checkApp(snapshot.AppPath, &snapshot)
	if ref != nil {
		return ref
	}
	if u.isRunning(app.Path) {
		return refusal(codeRunning, "Quit this application before moving its data to Trash")
	}
	return nil
}

// appTrashRefusal is the last check before the app bundle itself moves.
func (u *Uninstaller) appTrashRefusal(snapshot UninstallSnapshot, appItem Candidate) *Refusal {
	if current := u.currentAppRefusal(snapshot); current != nil {
		return current
	}
	return u.revalidate(appItem)
}

// quitGracefully asks the app to quit and waits for it, polling on the Clock, for QuitTimeout. It never kills the
// app: a quit the app refuses or ignores leaves everything in place, and the review says so.
func (u *Uninstaller) quitGracefully(appPath string) *Refusal {
	requested := u.deps.RequestQuit != nil && u.deps.RequestQuit(appPath) == nil
	if !requested {
		if u.isRunning(appPath) {
			return refusal(codeQuitFailed, "Could not ask the application to quit. Nothing was moved to Trash.")
		}
		return nil
	}
	var waited time.Duration
	for {
		if !u.isRunning(appPath) {
			return nil
		}
		if waited >= u.deps.QuitTimeout {
			break
		}
		u.wait(u.deps.QuitPoll)
		waited += u.deps.QuitPoll
	}
	return refusal(codeQuitTimeout, "The app or a background helper is still running. Quit it and try again. Nothing was moved to Trash.")
}

// wait blocks for d on the Clock. Without a clock (tests that never quit an app) it returns at once.
func (u *Uninstaller) wait(d time.Duration) {
	if u.deps.Clock == nil {
		return
	}
	<-u.deps.Clock.After(d)
}
