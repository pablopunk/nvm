package menubar

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestSetIconStyleRedrawsTheGlyph(t *testing.T) {
	host := fake.NewTrayHost()
	item, err := New(Options{Host: host})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := item.SetIconStyle("gold-glossy"); err != nil {
		t.Fatalf("SetIconStyle: %v", err)
	}
	want := ChooseIcon("gold-glossy", runtime.GOOS, false)
	if icon, template := host.Trays()[0].Icon(); !bytes.Equal(icon, want.PNG) || template {
		t.Fatalf("icon after gold-glossy = %d bytes template=%v, want %q", len(icon), template, want.Asset)
	}
}

func TestSettingAnUnknownStyleDrawsTheDefault(t *testing.T) {
	host := fake.NewTrayHost()
	item, err := NewItem(host, platform.TrayOptions{}, "not-a-style")
	if err != nil {
		t.Fatalf("NewItem: %v", err)
	}
	want := ChooseIcon(DefaultIconStyle, runtime.GOOS, false)
	if icon, _ := host.Trays()[0].Icon(); !bytes.Equal(icon, want.PNG) {
		t.Fatalf("icon for unknown style is not the default %q", want.Asset)
	}
	item.Close()
}

func TestAppearanceChangeRedrawsForTheBar(t *testing.T) {
	host := fake.NewTrayHost()
	if _, err := New(Options{Host: host}); err != nil {
		t.Fatalf("New: %v", err)
	}
	tray := host.Trays()[0]
	tray.SetDark(true)
	tray.ChangeAppearance()
	want := ChooseIcon(DefaultIconStyle, runtime.GOOS, true)
	if icon, template := tray.Icon(); !bytes.Equal(icon, want.PNG) || template != want.Template {
		t.Fatalf("icon on a dark bar template=%v, want %q template=%v", template, want.Asset, want.Template)
	}
}

func TestEmojiStyleLeadsTheTitleOnMacOSOnly(t *testing.T) {
	host := fake.NewTrayHost()
	item, err := New(Options{Host: host, IconStyle: "emoji"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	item.SetTitle("04:59")
	want := TitleFor("emoji", runtime.GOOS, "04:59")
	if got := host.Trays()[0].Title(); got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
}
