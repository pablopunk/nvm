package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/bundle"
	"github.com/pablopunk/nvm/native/internal/ext/submit"
	"github.com/pablopunk/nvm/native/internal/safego"
	"github.com/pablopunk/nvm/native/internal/search"
)

// nevermindRepo is the repository extensions are submitted to, and extensionSubmitDir the folder in it that holds them. 1.x
// submits to the same repository; 2.0 extensions are folders, so they go to a folder of their own.
var (
	nevermindRepo       = submit.Upstream{Owner: "pablopunk", Name: "nvm", OutputDir: "src/native/extensions"}
	githubStatusMaxAge  = time.Minute
	submitSuccessPrefix = "Pull request opened: "
	errNotInstalled     = errors.New("the extension is not installed")
)

// githubProbe is the cached state of the GitHub CLI. The menu reads it on every search, so the CLI is asked again in the
// background, not on the search.
type githubProbe struct {
	mu      sync.Mutex
	status  submit.Status
	at      time.Time
	probing bool
	run     submit.Runner
}

// githubStatus returns the cached CLI state and asks the CLI again when the cache is stale.
func (p *App) githubStatus() submit.Status {
	g := p.githubProbeState()
	g.mu.Lock()
	status := g.status
	stale := !g.probing && time.Since(g.at) > githubStatusMaxAge
	if stale {
		g.probing = true
	}
	g.mu.Unlock()
	if stale {
		safego.Go("github.probe", func(context.Context) {
			next := submit.Probe(g.run)
			g.mu.Lock()
			g.status, g.at, g.probing = next, time.Now(), false
			g.mu.Unlock()
		})
	}
	return status
}

func (p *App) githubProbeState() *githubProbe {
	if p.github == nil {
		p.github = &githubProbe{run: submit.Exec}
	}
	return p.github
}

// submitAction is the Submit as PR row of an extension the builder made. 1.x hides it when the CLI is not installed, shows it
// disabled with the sign-in reason when the CLI is not signed in, and runs it otherwise.
func (p *App) submitAction(extID string) (search.Action, bool) {
	status := p.githubStatus()
	if !status.Installed {
		return search.Action{}, false
	}
	if !status.Authed {
		return search.Action{Title: submitPRTitle, Disabled: submit.ErrNotSignedIn.Error()}, true
	}
	return search.Action{Title: submitPRTitle, Dismiss: search.DismissNever, Run: func(h search.Host) {
		p.submitExtensionPR(extID)
	}}, true
}

// submitExtensionPR opens the pull request in the background and reports its URL, or the step that failed, in a toast.
func (p *App) submitExtensionPR(extID string) {
	ext, err := p.extensionForSubmit(extID)
	if err != nil {
		p.toastFromWorker(err.Error())
		return
	}
	run := p.githubProbeState().run
	safego.Go("extension.submit", func(context.Context) {
		url, err := submit.Submit(run, nevermindRepo, ext)
		if err != nil {
			p.onUI(func() { p.toastFromWorker(err.Error()) })
			return
		}
		p.onUI(func() { p.toastFromWorker(submitSuccessPrefix + url) })
	})
}

// extensionForSubmit reads the files of an installed extension that a pull request carries.
func (p *App) extensionForSubmit(extID string) (submit.Extension, error) {
	dir, ok := p.extensionFolder(extID)
	if !ok {
		return submit.Extension{}, errNotInstalled
	}
	files := p.ports.FS
	b := bundle.Load(files, dir)
	if !b.Runnable() {
		return submit.Extension{}, fmt.Errorf("the extension cannot be submitted: %s", strings.Join(b.Problems, "; "))
	}
	manifest, err := files.ReadFile(dir + "/" + bundle.ManifestFile)
	if err != nil {
		return submit.Extension{}, err
	}
	source, err := files.ReadFile(dir + "/" + bundle.SourceFile)
	if err != nil {
		return submit.Extension{}, err
	}
	return submit.Extension{ID: b.Manifest.ID, Title: b.Manifest.Title, Files: []submit.File{
		{Name: bundle.ManifestFile, Contents: manifest},
		{Name: bundle.SourceFile, Contents: source},
	}}, nil
}
