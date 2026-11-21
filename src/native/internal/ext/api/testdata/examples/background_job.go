// Example: a trigger-driven background job. The host runs it when new screenshots land on the
// Desktop (debounced, no overlap, bounded file list in LaunchContext) and it keeps a bounded
// index in storage. It has no view, so it never shows the palette.
package main

import (
	"fmt"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
)

const maxIndexedScreenshots = 500

func Commands(ctx api.Context) ([]api.Command, error) {
	return []api.Command{{
		ID:    "index-new-screenshots",
		Title: "Index New Screenshots",
		Mode:  api.ModeBackground,
		Triggers: []api.Trigger{{
			Type:       api.TriggerFilesChanged,
			Roots:      []string{"~/Desktop"},
			FileKind:   api.FileKindImage,
			DebounceMs: 1500,
			Ignore:     []string{"*.tmp"},
		}},
	}}, nil
}

func RunAction(ctx api.Context, in api.Invocation) (api.Result, error) {
	if in.Ref.ID != "index-new-screenshots" {
		return api.Result{}, fmt.Errorf("unknown action %q", in.Ref.ID)
	}
	var index []string
	if _, err := ctx.Storage().Get("paths", &index); err != nil {
		return api.Result{}, err
	}
	for _, file := range in.Launch.Files {
		if strings.HasPrefix(file.Name, "Screenshot") {
			index = append(index, file.Path)
		}
	}
	if len(index) > maxIndexedScreenshots {
		index = index[len(index)-maxIndexedScreenshots:]
	}
	ctx.Logs().Log(api.LogInfo, "indexed screenshots", map[string]int{"total": len(index)})
	return api.Result{}, ctx.Storage().Set("paths", index)
}
