package apps

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// Ported from app-bundle-icons.test.ts (3 cases).

// Ported: "appBundleIconFileNames reads CFBundleIconFile and adds icns extension".
func TestBundleIconFileNamesReadsDirectIconFile(t *testing.T) {
	names := BundleIconFileNames(`
    <plist><dict>
      <key>CFBundleIconFile</key>
      <string>AppIcon</string>
    </dict></plist>
  `)
	if want := []string{"AppIcon", "AppIcon.icns"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

// Ported: "appBundleIconFileNames reads CFBundleIconFiles arrays".
func TestBundleIconFileNamesReadsIconFileArrays(t *testing.T) {
	names := BundleIconFileNames(`
    <plist><dict>
      <key>CFBundleIcons</key>
      <dict>
        <key>CFBundlePrimaryIcon</key>
        <dict>
          <key>CFBundleIconFiles</key>
          <array>
            <string>Icon16</string>
            <string>Icon512.icns</string>
          </array>
        </dict>
      </dict>
    </dict></plist>
  `)
	if want := []string{"Icon16", "Icon16.icns", "Icon512.icns"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

// Ported: "pngImagesFromIcns extracts embedded png icon entries".
func TestPNGImagesFromICNSExtractsPNGEntries(t *testing.T) {
	png := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 24)...)
	entry := make([]byte, 8+len(png))
	copy(entry, "ic12")
	binary.BigEndian.PutUint32(entry[4:8], uint32(len(entry)))
	copy(entry[8:], png)
	header := make([]byte, 8)
	copy(header, "icns")
	binary.BigEndian.PutUint32(header[4:8], uint32(len(header)+len(entry)))
	got := PNGImagesFromICNS(append(header, entry...))
	if len(got) != 1 || !bytes.Equal(got[0], png) {
		t.Fatalf("PNGImagesFromICNS = %x, want one entry %x", got, png)
	}
}

// Hostile input beyond 1.x: a chunk that claims more bytes than the file holds ends the scan without a panic, and
// a non-icns file yields nothing.
func TestPNGImagesFromICNSStopsAtDamagedChunks(t *testing.T) {
	header := make([]byte, 8)
	copy(header, "icns")
	binary.BigEndian.PutUint32(header[4:8], 8)
	chunk := make([]byte, 8)
	copy(chunk, "ic12")
	binary.BigEndian.PutUint32(chunk[4:8], 999)
	if got := PNGImagesFromICNS(append(header, chunk...)); len(got) != 0 {
		t.Fatalf("damaged chunk yielded %d images", len(got))
	}
	if got := PNGImagesFromICNS([]byte("not an icns file at all")); got != nil {
		t.Fatalf("non-icns yielded %d images", len(got))
	}
}

// Hostile input: BundleIconPNG matches names only against the Resources listing, so an icon name that points
// elsewhere ("../../secret.icns") is never read, and the first .icns resource is the fallback.
func TestBundleIconPNGNeverReadsOutsideResources(t *testing.T) {
	files := fake.NewFS()
	const app = "/Applications/Example.app"
	files.Seed(app+"/Contents/Info.plist", []byte(`<dict><key>CFBundleIconFile</key><string>../../secret.icns</string></dict>`))
	files.Seed("/Applications/secret.icns", icnsWithPNG(t, 64, 64))
	files.Seed(app+"/Contents/Resources/Own.icns", icnsWithPNG(t, 16, 16))
	data, ok, err := BundleIconPNG(files, app)
	if err != nil || !ok {
		t.Fatalf("BundleIconPNG: ok=%v err=%v", ok, err)
	}
	if pngArea(data) != 16*16 {
		t.Fatalf("picked the wrong icon: area %d", pngArea(data))
	}
}

// icnsWithPNG builds an icns file holding one PNG of the given size.
func icnsWithPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	png := testPNG(t, width, height)
	entry := make([]byte, 8+len(png))
	copy(entry, "ic12")
	binary.BigEndian.PutUint32(entry[4:8], uint32(len(entry)))
	copy(entry[8:], png)
	header := make([]byte, 8)
	copy(header, "icns")
	binary.BigEndian.PutUint32(header[4:8], uint32(len(header)+len(entry)))
	return append(header, entry...)
}
