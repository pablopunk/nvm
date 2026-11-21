package real

import (
	"crypto/rand"
	"errors"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/pablopunk/nvm/native/internal/platform"
)

const clipboardOwnerFormat transfer.Format = "application/x-nevermind-clipboard-owner"

type clipboardOwnership string

// Snapshot materializes all advertised representations and their item boundaries before any temporary write.
func (c *clipboard) Snapshot() (platform.ClipboardSnapshot, error) {
	return mygo.Clipboard.Read()
}

func (c *clipboard) WriteTemporaryText(text string) (platform.ClipboardOwnership, error) {
	owner := clipboardOwnership(rand.Text())
	err := mygo.Clipboard.Write(transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte(text)),
		transfer.Bytes(concealedFormat, []byte("1")),
		transfer.Bytes(clipboardOwnerFormat, []byte(owner)),
	)))
	if err != nil {
		return nil, err
	}
	return owner, nil
}

func (c *clipboard) StillOwns(ownership platform.ClipboardOwnership) (bool, error) {
	owner, ok := ownership.(clipboardOwnership)
	if !ok || owner == "" {
		return false, nil
	}
	current, err := mygo.Clipboard.ReadFormat(clipboardOwnerFormat)
	if errors.Is(err, transfer.ErrFormat) {
		return false, nil
	}
	return err == nil && string(current) == string(owner), err
}

// RestoreIfOwned checks the private write token and restores in one main-thread operation. A user copy has no token.
func (c *clipboard) RestoreIfOwned(snapshot platform.ClipboardSnapshot, ownership platform.ClipboardOwnership) error {
	data, ok := snapshot.(transfer.Data)
	if !ok {
		return platform.ErrUnsupported
	}
	var restoreErr error
	mygo.RunOnMain(func() {
		owned, err := c.StillOwns(ownership)
		if err != nil {
			restoreErr = err
			return
		}
		if owned {
			restoreErr = mygo.Clipboard.Write(data)
		}
	})
	return restoreErr
}

var _ platform.ClipboardSnapshotter = (*clipboard)(nil)
