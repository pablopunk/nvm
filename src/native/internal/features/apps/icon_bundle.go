package apps

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path"
	"regexp"
	"strings"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// Bundle icons: the macOS app bundle names its icon in Contents/Info.plist and stores it in Contents/Resources as
// an .icns file, which holds PNG entries. The parser is pure so it is tested without a bundle; BundleIconPNG reads
// through the FS port only.

const (
	icnsMagic      = "icns"
	icnsHeaderSize = 8
	icnsChunkHead  = 8
	icnsExtension  = ".icns"
)

var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

var (
	directIconFilePattern = regexp.MustCompile(`<key>CFBundleIconFile</key>\s*<string>([^<]+)</string>`)
	iconFilesArrayPattern = regexp.MustCompile(`(?s)<key>CFBundleIconFiles</key>\s*<array>(.*?)</array>`)
	plistStringPattern    = regexp.MustCompile(`<string>([^<]+)</string>`)
)

// BundleIconFileNames returns the icon file names an Info.plist names, each also with the .icns extension when it
// lacks one, without repeats. The names are matched against the bundle's Resources listing, never used as paths.
func BundleIconFileNames(infoPlist string) []string {
	var names []string
	if m := directIconFilePattern.FindStringSubmatch(infoPlist); m != nil {
		names = append(names, strings.TrimSpace(m[1]))
	}
	for _, array := range iconFilesArrayPattern.FindAllStringSubmatch(infoPlist, -1) {
		for _, item := range plistStringPattern.FindAllStringSubmatch(array[1], -1) {
			names = append(names, strings.TrimSpace(item[1]))
		}
	}
	out := make([]string, 0, len(names)*2)
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" {
			continue
		}
		for _, candidate := range []string{name, withIcnsExtension(name)} {
			if !seen[candidate] {
				seen[candidate] = true
				out = append(out, candidate)
			}
		}
	}
	return out
}

func withIcnsExtension(name string) string {
	if strings.HasSuffix(strings.ToLower(name), icnsExtension) {
		return name
	}
	return name + icnsExtension
}

// PNGImagesFromICNS returns the PNG entries of an .icns file, in file order. Anything that is not an icns file, or
// whose chunks run past the end, yields what was read before the damage.
func PNGImagesFromICNS(icns []byte) [][]byte {
	if len(icns) < icnsHeaderSize || string(icns[:4]) != icnsMagic {
		return nil
	}
	var images [][]byte
	offset := icnsHeaderSize
	for offset+icnsChunkHead <= len(icns) {
		size := int(binary.BigEndian.Uint32(icns[offset+4 : offset+8]))
		if size < icnsChunkHead || offset+size > len(icns) {
			break
		}
		data := icns[offset+icnsChunkHead : offset+size]
		if bytes.HasPrefix(data, pngSignature) {
			images = append(images, append([]byte(nil), data...))
		}
		offset += size
	}
	return images
}

// pngArea returns the pixel count of a PNG from its IHDR, or 0 when the header is short.
func pngArea(png []byte) uint64 {
	if len(png) < 24 {
		return 0
	}
	width := uint64(binary.BigEndian.Uint32(png[16:20]))
	height := uint64(binary.BigEndian.Uint32(png[20:24]))
	return width * height
}

// BundleIconPNG returns the largest PNG icon of the app bundle at appPath, or ok=false when the bundle has no icns
// icon. The icon is the first resource the plist names (or the first .icns when it names none).
func BundleIconPNG(files platform.FS, appPath string) (data []byte, ok bool, err error) {
	infoPlist, _ := files.ReadFile(path.Join(appPath, "Contents", "Info.plist"))
	resourcesPath := path.Join(appPath, "Contents", "Resources")
	entries, err := files.ReadDir(resourcesPath)
	if err != nil || len(entries) == 0 {
		return nil, false, nil
	}
	byLower := make(map[string]string, len(entries))
	for _, entry := range entries {
		byLower[strings.ToLower(entry.Name())] = entry.Name()
	}
	iconName := ""
	for _, candidate := range BundleIconFileNames(string(infoPlist)) {
		if name, found := byLower[strings.ToLower(candidate)]; found {
			iconName = name
			break
		}
	}
	if iconName == "" {
		for _, entry := range entries {
			if strings.HasSuffix(strings.ToLower(entry.Name()), icnsExtension) {
				iconName = entry.Name()
				break
			}
		}
	}
	if iconName == "" || !strings.HasSuffix(strings.ToLower(iconName), icnsExtension) {
		return nil, false, nil
	}
	icns, err := files.ReadFile(path.Join(resourcesPath, iconName))
	if err != nil {
		return nil, false, err
	}
	var best []byte
	for _, png := range PNGImagesFromICNS(icns) {
		if best == nil || pngArea(png) > pngArea(best) {
			best = png
		}
	}
	if best == nil {
		return nil, false, errors.New("apps: bundle icon has no PNG entry")
	}
	return best, true, nil
}
