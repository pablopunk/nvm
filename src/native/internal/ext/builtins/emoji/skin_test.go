package emoji

import (
	"errors"
	"testing"

	"github.com/pablopunk/nvm/native/internal/ext/api"
	"github.com/pablopunk/nvm/native/internal/features/emoji"
)

// fakeToneStorage holds the value under the tone key, as the built-in's storage does, or fails every read.
type fakeToneStorage struct {
	api.Storage
	value  int
	stored bool
	failed bool
}

func (s *fakeToneStorage) Get(key string, into any) (bool, error) {
	if s.failed {
		return false, errors.New("storage unreadable")
	}
	if key != toneKey || !s.stored {
		return false, nil
	}
	*into.(*int) = s.value
	return true, nil
}

func (s *fakeToneStorage) Set(key string, value any) error {
	s.value, s.stored = value.(int), true
	return nil
}

// toneContext serves only the storage the tone reads.
type toneContext struct {
	api.Context
	storage *fakeToneStorage
}

func (c toneContext) Storage() api.Storage { return c.storage }

// TestSkinToneFallsBackToTheLegacySettingUntilATone checks the settings file's tone answers only while the built-in
// stores none: a stored tone, even Default, wins, and a missing or unreadable store reads as the default.
func TestSkinToneFallsBackToTheLegacySettingUntilATone(t *testing.T) {
	const legacyMedium = int(emoji.SkinToneMedium)
	cases := []struct {
		name    string
		legacy  int
		storage fakeToneStorage
		want    emoji.SkinTone
	}{
		{"nothing stored reads the legacy tone", legacyMedium, fakeToneStorage{}, emoji.SkinToneMedium},
		{"a stored tone wins over the legacy one", legacyMedium, fakeToneStorage{value: int(emoji.SkinToneLight), stored: true}, emoji.SkinToneLight},
		{"a stored Default wins over the legacy tone", legacyMedium, fakeToneStorage{stored: true}, emoji.SkinToneDefault},
		{"an out-of-range stored tone reads as Default", legacyMedium, fakeToneStorage{value: 9, stored: true}, emoji.SkinToneDefault},
		{"an out-of-range legacy tone reads as Default", 9, fakeToneStorage{}, emoji.SkinToneDefault},
		{"an unreadable store reads as Default", legacyMedium, fakeToneStorage{failed: true}, emoji.SkinToneDefault},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := tc.storage
			e := New(nil, tc.legacy)
			if got := e.tone(toneContext{storage: &storage}); got != tc.want {
				t.Fatalf("tone = %v, want %v", got, tc.want)
			}
		})
	}
}
