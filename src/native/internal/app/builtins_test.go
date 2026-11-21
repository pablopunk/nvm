package app

import (
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/search"
)

func TestBuiltInRowKeepsItsSectionToneRankAndLearningKey(t *testing.T) {
	folderRow := search.Item{Kind: "Acme", Tone: "purple", Score: scoreExtension}
	item := api.Item{Title: "Sound", Section: search.KindCommands, Score: 42, LearnKey: "pane:sound", Appearance: &api.Appearance{Foreground: api.ColorBlue}}
	row := builtInRow(folderRow, item)
	if row.Kind != search.KindCommands || row.Tone != "blue" || row.Score != 42 || row.LearnKey != "pane:sound" {
		t.Fatalf("row = kind %q tone %q score %d learn %q", row.Kind, row.Tone, row.Score, row.LearnKey)
	}
}

func TestBuiltInRowWithoutToneHasNone(t *testing.T) {
	row := builtInRow(search.Item{Kind: "Acme", Tone: "purple", Score: scoreExtension}, api.Item{Title: "Sound"})
	if row.Tone != "" || row.Kind != "Acme" || row.Score != scoreExtension {
		t.Fatalf("row = kind %q tone %q score %d", row.Kind, row.Tone, row.Score)
	}
}
