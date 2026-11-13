// clipboardcheck exercises the native macOS clipboard snapshot capability in CI.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/pablopunk/nvm/native/internal/platform"
	"github.com/pablopunk/nvm/native/internal/platform/real"
)

const customFormat transfer.Format = "application/x-nevermind-ci-smoke"

func main() {
	done := make(chan error, 1)
	mygo.App.OnReady(func() { go func() { done <- runSmoke() }() })
	go func() {
		select {
		case err := <-done:
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			mygo.App.Quit()
		case <-time.After(90 * time.Second):
			fmt.Fprintln(os.Stderr, "clipboard smoke timed out")
			os.Exit(1)
		}
	}()
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "clipboard smoke could not start")
		os.Exit(1)
	}
}

func runSmoke() error {
	ports := real.NewPorts()
	clip := ports.Clipboard.(platform.ClipboardSnapshotter)
	original, err := clip.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot original clipboard: %w", err)
	}
	defer func() {
		owner, err := clip.WriteTemporaryText("cleanup")
		if err == nil {
			_ = clip.RestoreIfOwned(original, owner)
		}
	}()

	if err := mygo.Clipboard.Write(fixture()); err != nil {
		return fmt.Errorf("seed clipboard fixture: %w", err)
	}
	before, err := clip.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot seeded clipboard: %w", err)
	}
	text := "temporary smoke value"
	owner, err := clip.WriteTemporaryText(text)
	if err != nil {
		return fmt.Errorf("write temporary clipboard value: %w", err)
	}
	if err := clip.RestoreIfOwned(before, owner); err != nil {
		return fmt.Errorf("restore seeded clipboard: %w", err)
	}
	after, err := clip.Snapshot()
	if err != nil {
		return fmt.Errorf("read restored clipboard: %w", err)
	}
	if err := equalData(before, after); err != nil {
		return fmt.Errorf("restored clipboard differs: %w", err)
	}

	// A same-value user copy is a new ownership state: restoration must leave it untouched.
	owner, err = clip.WriteTemporaryText(text)
	if err != nil {
		return fmt.Errorf("write second temporary value: %w", err)
	}
	newCopy := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(text)), transfer.Bytes("org.nspasteboard.ConcealedType", []byte("1"))))
	if err := mygo.Clipboard.Write(newCopy); err != nil {
		return fmt.Errorf("seed newer user copy: %w", err)
	}
	newer, err := clip.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot newer user copy: %w", err)
	}
	if err := clip.RestoreIfOwned(before, owner); err != nil {
		return fmt.Errorf("check newer clipboard ownership: %w", err)
	}
	current, err := clip.Snapshot()
	if err != nil {
		return fmt.Errorf("read newer clipboard copy: %w", err)
	}
	if err := equalData(newer, current); err != nil {
		return fmt.Errorf("newer clipboard copy was changed: %w", err)
	}
	return nil
}

func equalData(want, got platform.ClipboardSnapshot) error {
	w, ok := want.(transfer.Data)
	if !ok {
		return fmt.Errorf("snapshot has unexpected representation")
	}
	g, ok := got.(transfer.Data)
	if !ok {
		return fmt.Errorf("read has unexpected representation")
	}
	wi, gi := w.Items(), g.Items()
	if len(wi) != len(gi) {
		return fmt.Errorf("item boundaries changed")
	}
	for n := range wi {
		wf, gf := wi[n].Formats(), gi[n].Formats()
		if len(wf) != len(gf) {
			return fmt.Errorf("item representations changed")
		}
		formats := make(map[transfer.Format]bool, len(gf))
		for _, format := range gf {
			formats[format] = true
		}
		for _, format := range wf {
			if !formats[format] {
				return fmt.Errorf("item representations changed")
			}
			wb, we := wi[n].Read(format)
			gb, ge := gi[n].Read(format)
			if we != nil || ge != nil || !bytes.Equal(wb, gb) {
				return fmt.Errorf("clipboard representation changed")
			}
		}
	}
	return nil
}

func fixture() transfer.Data {
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		panic(err)
	}
	files, _ := transfer.FileData("/tmp/nevermind-clipboardcheck-file")
	items := files.Items()
	return transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("smoke text")), transfer.Bytes(transfer.HTML, []byte("<b>smoke</b>")), transfer.Bytes("text/rtf", []byte("{\\rtf1 smoke}")), transfer.Bytes(customFormat, []byte("opaque"))),
		transfer.NewItem(transfer.Bytes(transfer.PNG, pngData.Bytes())),
		items[0],
	)
}
