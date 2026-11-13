package thumbs

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

func TestRenderRejectsOversizedPixelHeader(t *testing.T) {
	header := make([]byte, 33)
	copy(header, "\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], "IHDR")
	binary.BigEndian.PutUint32(header[16:20], 100000)
	binary.BigEndian.PutUint32(header[20:24], 100000)
	header[24], header[25] = 8, 6
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	fsys := fake.NewFS()
	fsys.Seed("/huge.png", header)
	if _, err := Render(fsys, "/huge.png", Edge); err != ErrTooLarge {
		t.Fatalf("Render error = %v, want ErrTooLarge", err)
	}
}
