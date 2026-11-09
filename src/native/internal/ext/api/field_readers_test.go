package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// unreadFields are the API struct fields no host code reads. Each is an answer the host only writes for extensions, or
// a field kept so that existing extension source still compiles. Entries can only be removed.
var unreadFields = map[string]string{
	"App.FirstSeenMs":                 "answer",
	"ChatSummary.CreatedAtMs":         "answer",
	"ChatSummary.ExtensionFiles":      "answer",
	"ChatSummary.UpdatedAtMs":         "answer",
	"ClipboardContent.RTF":            "answer",
	"ClipboardEntry.CreatedAtMs":      "answer",
	"DictationDevice.IsDefault":       "answer",
	"DraftConflict.CurrentContent":    "answer",
	"DraftConflict.StoredContent":     "answer",
	"DraftConflict.StoredVersion":     "answer",
	"FileRef.DateAddedMs":             "answer",
	"Image.Tint":                      "accepted so source compiles; 1.x never drew it",
	"IndexedFile.DateAddedMs":         "answer",
	"Job.ConsecutiveFailures":         "answer",
	"Job.LastDurationMs":              "answer",
	"Job.LastFinishedAt":              "answer",
	"Job.LastStartedAt":               "answer",
	"JobTrigger.EveryMs":              "answer",
	"LaunchContext.StartedAtMs":       "answer",
	"Pagination.PageSize":             "accepted so source compiles; 1.x never read it",
	"QuitResult.WasRunning":           "answer",
	"SavedDictation.CreatedAtMs":      "answer",
	"TemplateOptions.PromptMissing":   "accepted so source compiles; Missing is always reported",
	"TemplateResult.Missing":          "answer",
	"TemplateResult.Cursor":           "answer",
	"TweakRequest.ExtensionFile":      "read through the builder port",
	"UpdateState.AvailableVersion":    "answer",
	"UpdateState.Checking":            "answer",
	"UpdateState.DownloadedVersion":   "answer",
	"UpdateState.Downloading":         "answer",
	"UpdateState.ErrorMessage":        "answer",
	"UpdateState.Installing":          "answer",
	"WindowOptions.IgnoreMouseEvents": "not supported by the UI toolkit yet",
	"Metadata.Copyable":               "accepted so source compiles; 1.x never read it",
}

// hostSelectors collects every name read as x.Name in host code: the module's packages other than the API itself, the
// built-ins and builder (which only write fields) and generated tables.
func hostSelectors(t *testing.T) map[string]bool {
	t.Helper()
	selectors := map[string]bool{}
	root := "../.."
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "testdata" || path == "../api" || path == "../builtins" || path == "../../ai/builder" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "api_symbols.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				selectors[selector.Sel.Name] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return selectors
}

// apiStructFields lists Type.Field for the exported fields of the API's structs.
func apiStructFields(t *testing.T) map[string]string {
	t.Helper()
	fields := map[string]string{}
	for _, file := range parseAPISources(t) {
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if structure, ok := spec.Type.(*ast.StructType); ok {
				for _, field := range structure.Fields.List {
					for _, name := range field.Names {
						if name.IsExported() {
							fields[spec.Name.Name+"."+name.Name] = name.Name
						}
					}
				}
			}
			return false
		})
	}
	return fields
}

func readInsideAPI(t *testing.T) map[string]bool {
	t.Helper()
	selectors := map[string]bool{}
	sources, _ := filepath.Glob("*.go")
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				selectors[selector.Sel.Name] = true
			}
			return true
		})
	}
	return selectors
}

// TestEveryAPIFieldHasAReader is part of gate G4 of docs/extension-api-parity-plan.md: an input an extension sets must
// do something, so an accepted field with no reader fails here.
func TestEveryAPIFieldHasAReader(t *testing.T) {
	host, inside := hostSelectors(t), readInsideAPI(t)
	fields := apiStructFields(t)
	for qualified, name := range fields {
		_, allowed := unreadFields[qualified]
		read := host[name] || inside[name]
		if !read && !allowed {
			t.Errorf("%s has no reader: implement it, or remove it", qualified)
		}
	}
	for qualified := range unreadFields {
		if _, exists := fields[qualified]; !exists {
			t.Errorf("unreadFields lists %s, which no longer exists", qualified)
		}
	}
}
