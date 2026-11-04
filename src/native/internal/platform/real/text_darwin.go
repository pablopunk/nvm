//go:build darwin

package real

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// visionScript is the Vision recognizer, run by the system Swift interpreter as Electron's OCR does. It prints one JSON
// object: the text, the mean confidence, the first language and one block per recognized line with its box in pixels.
//
//go:embed vision_text.swift
var visionScript string

// swiftInterpreter is the Swift that ships with the Command Line Tools; the recognizer needs no compile step.
const swiftInterpreter = "/usr/bin/swift"

// visionText reads text with the Vision framework through the Swift interpreter.
type visionText struct {
	helper platform.Helper
}

func newTextRecognizer(helper platform.Helper) platform.TextRecognizer {
	return visionText{helper: helper}
}

// Recognize writes the script to a private temporary file, runs it on path and parses what it prints.
func (v visionText) Recognize(ctx context.Context, path string, languages []string) (platform.TextResult, error) {
	checked, err := checkLanguages(languages)
	if err != nil {
		return platform.TextResult{}, err
	}
	script, err := writeTemporaryScript(visionScript)
	if err != nil {
		return platform.TextResult{}, err
	}
	defer os.Remove(script)
	result, err := v.helper.Run(ctx, swiftInterpreter, append([]string{script, path}, checked...), nil)
	if err != nil {
		return platform.TextResult{}, err
	}
	if result.ExitCode != 0 {
		return platform.TextResult{}, errors.New(recognitionFailure(result.Stderr))
	}
	return parseVisionOutput(result.Stdout)
}

// writeTemporaryScript writes source to a new file that only this user can read, and returns its path.
func writeTemporaryScript(source string) (string, error) {
	file, err := os.CreateTemp("", "nvm-vision-*.swift")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := file.WriteString(source); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

// screencapture is the system screenshot tool; -x keeps it silent.
const screencapture = "/usr/sbin/screencapture"

// screenCaptureMac saves the screen with the system tool. A region is passed with -R when it has a size.
type screenCaptureMac struct {
	helper platform.Helper
}

func newScreenCapturer(helper platform.Helper) platform.ScreenCapturer {
	return screenCaptureMac{helper: helper}
}

// Capture writes a PNG to dst. The OS asks for screen recording permission on first use; a refusal is a failed run.
func (s screenCaptureMac) Capture(ctx context.Context, dst string, region *platform.Rect) error {
	args := []string{"-x"}
	if region != nil && region.Width > 0 && region.Height > 0 {
		args = append(args, "-R", strings.Join([]string{strconv.Itoa(region.X), strconv.Itoa(region.Y), strconv.Itoa(region.Width), strconv.Itoa(region.Height)}, ","))
	}
	args = append(args, dst)
	result, err := s.helper.Run(ctx, screencapture, args, nil)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return errors.New(recognitionFailure(result.Stderr))
	}
	return nil
}
