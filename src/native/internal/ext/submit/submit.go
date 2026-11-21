// Package submit opens a pull request that adds an installed extension to the Nevermind repository, through the GitHub
// CLI, as 1.x's extension PR submitter does: it checks the CLI and its sign-in, forks the repository when the user is not
// its owner, creates a branch from main, uploads the extension files and opens the pull request.
package submit

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Upstream is the repository extensions are submitted to, and OutputDir the folder in it that holds them.
type Upstream struct {
	Owner, Name string
	OutputDir   string
}

// Runner runs the GitHub CLI with args and returns its standard output.
type Runner func(args ...string) (string, error)

// Status is what the GitHub CLI can do: whether it is installed, and whether the user is signed in to it.
type Status struct {
	Installed bool
	Authed    bool
}

// ErrNotSignedIn is the reason an extension cannot be submitted while the CLI is missing or signed out. It is the 1.x
// wording, so the menu and the toast say the same thing.
//
//lint:ignore ST1005 user-facing copy, shown as written
var ErrNotSignedIn = errors.New("Requires GitHub CLI (gh auth login)")

// ErrAlreadyUpstream means the repository already has an extension with this id.
var ErrAlreadyUpstream = errors.New("an extension with this id already exists in the repository")

// File is one file of the extension, by its name inside the extension's folder.
type File struct {
	Name     string
	Contents []byte
}

// Extension is what is submitted: its id, its title, and its files.
type Extension struct {
	ID, Title string
	Files     []File
}

// Probe reports whether the CLI is installed and signed in. Signed in is checked only when it is installed.
func Probe(run Runner) Status {
	if _, err := run("--version"); err != nil {
		return Status{}
	}
	_, err := run("auth", "status")
	return Status{Installed: true, Authed: err == nil}
}

// Submit opens the pull request and returns its URL. Each step that fails names what failed, so the toast says where.
func Submit(run Runner, up Upstream, ext Extension) (string, error) {
	if !Probe(run).Authed {
		return "", ErrNotSignedIn
	}
	user, err := run("api", "user", "--jq", ".login")
	if err != nil {
		return "", fmt.Errorf("could not read the signed-in user: %w", err)
	}
	login := strings.TrimSpace(user)
	mainSHA, err := run("api", fmt.Sprintf("repos/%s/%s/branches/main", up.Owner, up.Name), "--jq", ".commit.sha")
	if err != nil {
		return "", fmt.Errorf("failed to read the repository at main: %w", err)
	}
	mainSHA = strings.TrimSpace(mainSHA)
	target := up.Owner
	if login != up.Owner {
		target = login
		if _, err := run("repo", "fork", up.Owner+"/"+up.Name, "--clone=false"); err != nil {
			return "", fmt.Errorf("failed to fork the repository: %w", err)
		}
	}
	outPath := up.OutputDir + "/" + ext.ID
	if _, err := run("api", fmt.Sprintf("repos/%s/%s/contents/%s/extension.json?ref=main", up.Owner, up.Name, outPath)); err == nil {
		return "", ErrAlreadyUpstream
	}
	branch := "submit-extension-" + ext.ID
	if _, err := run("api", "--method", "POST", fmt.Sprintf("repos/%s/%s/git/refs", target, up.Name),
		"-f", "ref=refs/heads/"+branch, "-f", "sha="+mainSHA); err != nil {
		return "", fmt.Errorf("failed to create branch %q; it may already exist: %w", branch, err)
	}
	for _, f := range ext.Files {
		content := base64.StdEncoding.EncodeToString(f.Contents)
		if _, err := run("api", "--method", "PUT", fmt.Sprintf("repos/%s/%s/contents/%s/%s", target, up.Name, outPath, f.Name),
			"-f", "message=Add extension "+ext.Title, "-f", "content="+content, "-f", "branch="+branch); err != nil {
			return "", fmt.Errorf("failed to upload %s: %w", f.Name, err)
		}
	}
	head := branch
	if target != up.Owner {
		head = target + ":" + branch
	}
	url, err := run("pr", "create", "--repo", up.Owner+"/"+up.Name, "--head", head,
		"--title", "Add extension: "+ext.Title,
		"--body", fmt.Sprintf("Submitted from Nevermind. Adds the `%s` extension under `%s`.", ext.Title, outPath))
	if err != nil {
		return "", fmt.Errorf("failed to create the pull request: %w", err)
	}
	return strings.TrimSpace(url), nil
}
