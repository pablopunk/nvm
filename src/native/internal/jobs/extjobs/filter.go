package extjobs

import (
	"path/filepath"
	"strings"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/files"
)

// fileFilter is the part of a files.changed trigger that decides which changed paths reach the command.
type fileFilter struct {
	includeHidden bool
	extensions    map[string]bool
	kind          api.FileKindFilter
	ignore        []string
}

func newFileFilter(trigger api.Trigger) fileFilter {
	filter := fileFilter{includeHidden: trigger.IncludeHidden, kind: trigger.FileKind, ignore: trigger.Ignore}
	if len(trigger.Extensions) > 0 {
		filter.extensions = map[string]bool{}
		for _, extension := range trigger.Extensions {
			filter.extensions[strings.ToLower(strings.TrimPrefix(extension, "."))] = true
		}
	}
	return filter
}

// matches reports whether a changed path passes the filter. Ignore patterns match the file name alone.
func (f fileFilter) matches(path string) bool {
	name := filepath.Base(path)
	if !f.includeHidden && strings.HasPrefix(name, ".") {
		return false
	}
	for _, pattern := range f.ignore {
		if matched, _ := filepath.Match(pattern, name); matched {
			return false
		}
	}
	if f.extensions != nil && !f.extensions[files.ExtensionOf(name)] {
		return false
	}
	return kindMatches(f.kind, files.KindOf(name))
}

// kindMatches narrows by the file kind. An empty filter or "file" narrows nothing, as the TypeScript API does.
func kindMatches(filter api.FileKindFilter, kind files.Kind) bool {
	switch filter {
	case api.FileKindImage:
		return kind == files.KindImage
	case api.FileKindVideo:
		return kind == files.KindVideo
	case api.FileKindMedia:
		return kind == files.KindImage || kind == files.KindVideo
	default:
		return true
	}
}
