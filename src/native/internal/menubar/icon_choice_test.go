package menubar

import (
	"bytes"
	"image/png"
	"slices"
	"testing"
)

const (
	testDarwin  = "darwin"
	testLinux   = "linux"
	testWindows = "windows"
)

func TestStyleListMirrorsTheOneXStyles(t *testing.T) {
	want := []string{"bolt-filled", "bolt-outline", "emoji", "app-icon", "gold-outline", "gold-filled", "gold-glossy"}
	var got []string
	for _, style := range iconStyles {
		got = append(got, style.value)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("styles = %q, want %q", got, want)
	}
	if DefaultIconStyle != "bolt-filled" {
		t.Fatalf("default = %q, want the 1.x default bolt-filled", DefaultIconStyle)
	}
}

func TestUnknownOrEmptyStyleDrawsTheDefault(t *testing.T) {
	for _, style := range []string{"", "no-such-style", "Bolt-Filled"} {
		if got := NormalizeIconStyle(style); got != DefaultIconStyle {
			t.Fatalf("NormalizeIconStyle(%q) = %q, want %q", style, got, DefaultIconStyle)
		}
		want := ChooseIcon(DefaultIconStyle, testDarwin, false)
		if got := ChooseIcon(style, testDarwin, false); got.Asset != want.Asset || got.Template != want.Template {
			t.Fatalf("ChooseIcon(%q) = %q, want the default %q", style, got.Asset, want.Asset)
		}
	}
}

func TestEveryStyleEmbedsItsPNGOnEveryPlatform(t *testing.T) {
	for _, style := range iconStyles {
		for _, goos := range []string{testDarwin, testLinux, testWindows} {
			for _, dark := range []bool{false, true} {
				choice := ChooseIcon(style.value, goos, dark)
				if _, err := png.Decode(bytes.NewReader(choice.PNG)); err != nil {
					t.Fatalf("%s on %s (dark=%v) asset %q is not an embedded PNG: %v", style.value, goos, dark, choice.Asset, err)
				}
			}
		}
	}
}

func TestMacOSTemplateStylesUseTheBlackGlyphAsTemplate(t *testing.T) {
	for _, style := range []string{"bolt-filled", "bolt-outline"} {
		for _, dark := range []bool{false, true} {
			choice := ChooseIcon(style, testDarwin, dark)
			if want := "nevermind-" + style + "@2x.png"; choice.Asset != want || !choice.Template {
				t.Fatalf("%s on macOS (dark=%v) = %q template=%v, want %q template=true", style, dark, choice.Asset, choice.Template, want)
			}
		}
	}
}

func TestColoredStylesAreNeverTemplates(t *testing.T) {
	for _, style := range []string{"app-icon", "gold-outline", "gold-filled", "gold-glossy"} {
		for _, goos := range []string{testDarwin, testLinux, testWindows} {
			for _, dark := range []bool{false, true} {
				choice := ChooseIcon(style, goos, dark)
				if choice.Template {
					t.Fatalf("%s on %s is a template, want its colors kept", style, goos)
				}
				if want := "nevermind-" + style + "@2x.png"; choice.Asset != want {
					t.Fatalf("%s on %s (dark=%v) asset = %q, want %q", style, goos, dark, choice.Asset, want)
				}
			}
		}
	}
}

func TestTemplateStylesPickTheWhiteGlyphOnDarkNonMacBars(t *testing.T) {
	for _, style := range []string{"bolt-filled", "bolt-outline"} {
		for _, goos := range []string{testLinux, testWindows} {
			dark := ChooseIcon(style, goos, true)
			if want := "nevermind-" + style + "-white@2x.png"; dark.Asset != want || dark.Template {
				t.Fatalf("%s on %s dark bar = %q template=%v, want %q template=false", style, goos, dark.Asset, dark.Template, want)
			}
			light := ChooseIcon(style, goos, false)
			if want := "nevermind-" + style + "@2x.png"; light.Asset != want || light.Template {
				t.Fatalf("%s on %s light bar = %q template=%v, want %q template=false", style, goos, light.Asset, light.Template, want)
			}
		}
	}
}

func TestEmojiStyleIsTheTitleOnMacOSAndAnImageElsewhere(t *testing.T) {
	if got := ChooseIcon("emoji", testDarwin, false); got.Asset != "" || !got.Template {
		t.Fatalf("emoji on macOS = %q template=%v, want a transparent template with no glyph", got.Asset, got.Template)
	}
	if got := TitleFor("emoji", testDarwin, ""); got != emojiTitle {
		t.Fatalf("emoji title = %q, want %q", got, emojiTitle)
	}
	if got := TitleFor("emoji", testDarwin, "04:59"); got != emojiTitle+" 04:59" {
		t.Fatalf("emoji title with text = %q", got)
	}
	if got := ChooseIcon("emoji", testLinux, false).Asset; got != "nevermind-emoji@2x.png" {
		t.Fatalf("emoji on Linux asset = %q, want nevermind-emoji@2x.png", got)
	}
	if got := TitleFor("emoji", testLinux, "04:59"); got != "04:59" {
		t.Fatalf("emoji title off macOS = %q, want the plain title", got)
	}
}

func TestImageStylesKeepTheirTitle(t *testing.T) {
	if got := TitleFor("bolt-filled", testDarwin, "04:59"); got != "04:59" {
		t.Fatalf("title = %q, want 04:59", got)
	}
}
