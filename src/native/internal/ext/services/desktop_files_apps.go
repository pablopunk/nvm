package services

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files"
	"github.com/pablopunk/nvm/native/internal/platform"
)

// The system tools that read what a file is and what an app declares it can open. Where a tool is missing, the list
// falls back to every app, sorted by name.
const (
	mdlsBinary      = "/usr/bin/mdls"
	plutilBinary    = "/usr/bin/plutil"
	openWithWorkers = 8
	openWithTimeout = 5 * time.Second
)

// Match scores of an app for a file: an extension beats a content type, which beats a generic document type.
const (
	extensionMatchScore   = 3
	contentTypeMatchScore = 2
	genericMatchScore     = 1
)

var quotedValuePattern = regexp.MustCompile(`"([^"]+)"`)

// documentType is one CFBundleDocumentTypes entry of an app's Info.plist, as plutil writes it in JSON.
type documentType struct {
	Extensions   []any `json:"CFBundleTypeExtensions"`
	ContentTypes []any `json:"LSItemContentTypes"`
}

type appInfoPlist struct {
	DocumentTypes []documentType `json:"CFBundleDocumentTypes"`
}

type scoredApp struct {
	app   api.App
	score int
}

// openWithApps lists the installed apps that can open a file, the best match first.
func (h *Handler) openWithApps(ctx context.Context, raw json.RawMessage) ([]api.App, error) {
	path, err := absolutePathArg(raw)
	if err != nil {
		return nil, err
	}
	if h.deps.Shell == nil {
		return nil, unavailable("apps are not available in this build")
	}
	apps, err := h.deps.Shell.InstalledApps()
	if err != nil {
		return nil, fmt.Errorf("list the installed apps: %w", err)
	}
	bundles := applicationBundles(apps)
	contentTypes := h.contentTypesOf(ctx, path)
	scored := h.scoreApps(ctx, bundles, files.ExtensionOf(path), contentTypes)
	return rankApps(scored, bundles), nil
}

// applicationBundles keeps the apps that are application bundles, which are the only ones with document types.
func applicationBundles(apps []platform.App) []api.App {
	out := make([]api.App, 0, len(apps))
	for _, app := range apps {
		if strings.HasSuffix(strings.ToLower(app.Path), ".app") {
			out = append(out, api.App{ID: app.ID, Name: app.Name, Path: app.Path})
		}
	}
	return out
}

// contentTypesOf returns the content type tree Spotlight gives a file, such as public.png and public.image.
func (h *Handler) contentTypesOf(ctx context.Context, path string) map[string]bool {
	types := map[string]bool{}
	out, ok := h.runTool(ctx, mdlsBinary, "-raw", "-name", "kMDItemContentTypeTree", path)
	if !ok {
		return types
	}
	for _, match := range quotedValuePattern.FindAllStringSubmatch(out, -1) {
		types[match[1]] = true
	}
	return types
}

// scoreApps scores every application bundle against the file, a few at a time.
func (h *Handler) scoreApps(ctx context.Context, bundles []api.App, extension string, contentTypes map[string]bool) []scoredApp {
	scored := make([]scoredApp, len(bundles))
	slots := make(chan struct{}, openWithWorkers)
	var wg sync.WaitGroup
	for i, bundle := range bundles {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int, bundle api.App) {
			defer wg.Done()
			defer func() { <-slots }()
			scored[i] = scoredApp{app: bundle, score: h.appScore(ctx, bundle.Path, extension, contentTypes)}
		}(i, bundle)
	}
	wg.Wait()
	return scored
}

// appScore is the best score of the document types an app declares. An app whose plist cannot be read scores zero.
func (h *Handler) appScore(ctx context.Context, appPath, extension string, contentTypes map[string]bool) int {
	best := 0
	for _, docType := range h.documentTypesOf(ctx, appPath) {
		best = max(best, documentTypeScore(docType, extension, contentTypes))
	}
	return best
}

func (h *Handler) documentTypesOf(ctx context.Context, appPath string) []documentType {
	out, ok := h.runTool(ctx, plutilBinary, "-convert", "json", "-o", "-", filepath.Join(appPath, "Contents", "Info.plist"))
	if !ok {
		return nil
	}
	var plist appInfoPlist
	if err := json.Unmarshal([]byte(out), &plist); err != nil {
		return nil
	}
	return plist.DocumentTypes
}

// runTool runs one system tool and returns its output, or false when it is missing, fails or runs past its deadline.
func (h *Handler) runTool(ctx context.Context, binary string, args ...string) (string, bool) {
	if h.deps.Commands == nil {
		return "", false
	}
	runCtx, cancel := context.WithTimeout(ctx, openWithTimeout)
	defer cancel()
	result, err := h.deps.Commands.Run(runCtx, platform.CommandSpec{Binary: binary, Args: args})
	if err != nil || result.ExitCode != 0 {
		return "", false
	}
	return string(result.Stdout), true
}

func documentTypeScore(docType documentType, extension string, contentTypes map[string]bool) int {
	extensions := lowerStrings(docType.Extensions)
	itemTypes := stringValues(docType.ContentTypes)
	switch {
	case extension != "" && slices.Contains(extensions, extension):
		return extensionMatchScore
	case anyTypeHeld(itemTypes, contentTypes):
		return contentTypeMatchScore
	case slices.Contains(extensions, "*"), slices.Contains(itemTypes, "public.data"), slices.Contains(itemTypes, "public.item"):
		return genericMatchScore
	}
	return 0
}

func anyTypeHeld(itemTypes []string, held map[string]bool) bool {
	for _, itemType := range itemTypes {
		if held[itemType] {
			return true
		}
	}
	return false
}

// rankApps keeps the scored apps, best score first then by name. When no app scores, it lists every bundle by name.
func rankApps(scored []scoredApp, bundles []api.App) []api.App {
	matching := slices.DeleteFunc(slices.Clone(scored), func(s scoredApp) bool { return s.score == 0 })
	if len(matching) == 0 {
		sorted := slices.Clone(bundles)
		slices.SortStableFunc(sorted, compareAppNames)
		return sorted
	}
	slices.SortStableFunc(matching, func(a, b scoredApp) int {
		return cmp.Or(cmp.Compare(b.score, a.score), compareAppNames(a.app, b.app))
	})
	out := make([]api.App, 0, len(matching))
	for _, s := range matching {
		out = append(out, s.app)
	}
	return out
}

func compareAppNames(a, b api.App) int {
	return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

// lowerStrings returns the string values of a decoded JSON list, lower-cased. Other values are ignored.
func lowerStrings(values []any) []string {
	out := stringValues(values)
	for i := range out {
		out[i] = strings.ToLower(out[i])
	}
	return out
}

func stringValues(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
