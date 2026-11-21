package files

import (
	"slices"
	"testing"
)

// TestExtensionOfFollowsNodeExtname checks the extension rule the index and the MIME table share: lowercased, no dot,
// none for a name whose only dot leads it.
func TestExtensionOfFollowsNodeExtname(t *testing.T) {
	cases := map[string]string{
		"Photo.JPG":        "jpg",
		"archive.tar.gz":   "gz",
		".bashrc":          "",
		"notes":            "",
		"trailing.":        "",
		"dir/with.dot/a.b": "b",
		"/abs/path/x.PDF":  "pdf",
	}
	for name, want := range cases {
		if got := ExtensionOf(name); got != want {
			t.Errorf("ExtensionOf(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestKindOfExtensionClassifiesImagesAndVideos checks the image and video sets of file-utils.ts.
func TestKindOfExtensionClassifiesImagesAndVideos(t *testing.T) {
	cases := map[string]Kind{
		"png": KindImage, "heic": KindImage, "tif": KindImage,
		"mp4": KindVideo, "m4v": KindVideo, "mkv": KindVideo,
		"pdf": KindFile, "svg": KindFile, "": KindFile,
	}
	for ext, want := range cases {
		if got := kindOfExtension(ext); got != want {
			t.Errorf("kindOfExtension(%q) = %v, want %v", ext, got, want)
		}
	}
}

// TestKindOfNameNeedsNoFileRead checks the name-only classification a palette uses before a preview loads.
func TestKindOfNameNeedsNoFileRead(t *testing.T) {
	if KindOf("/Users/me/Pictures/IMG_0001.HEIC") != KindImage || KindOf("clip.mov") != KindVideo || KindOf("plan.pdf") != KindFile {
		t.Fatal("KindOf must classify by name alone")
	}
}

// TestKindMediaSelectsImagesAndVideosAndFileAddsNothing checks the kind filter rule of extensionsForFindOptions.
func TestKindMediaSelectsImagesAndVideosAndFileAddsNothing(t *testing.T) {
	media := extensionsForKinds([]Kind{KindMedia})
	if !slices.Contains(media, "png") || !slices.Contains(media, "mp4") || slices.Contains(media, "pdf") {
		t.Fatalf("media extensions = %v", media)
	}
	if got := extensionsForKinds([]Kind{KindFile}); len(got) != 0 {
		t.Fatalf("the file kind must not narrow the results, got %v", got)
	}
	images := extensionsForKinds([]Kind{KindImage, KindFile})
	if !slices.Contains(images, "jpg") || slices.Contains(images, "mov") {
		t.Fatalf("image plus file extensions = %v", images)
	}
}

// TestKindStringUsesTheExtensionAPIVocabulary checks the names the palette displays and the API filters by.
func TestKindStringUsesTheExtensionAPIVocabulary(t *testing.T) {
	want := map[Kind]string{KindFile: "file", KindImage: "image", KindVideo: "video", KindMedia: "media"}
	for kind, name := range want {
		if kind.String() != name {
			t.Errorf("Kind(%d).String() = %q, want %q", kind, kind.String(), name)
		}
	}
}

// TestMIMETypeTableMatchesMimeTypeTS ports the MIME table of mime-type.ts: each listed extension, and the fallback.
func TestMIMETypeTableMatchesMimeTypeTS(t *testing.T) {
	cases := map[string]string{
		"a.png":   "image/png",
		"a.JPEG":  "image/jpeg",
		"a.tiff":  "image/tiff",
		"a.heic":  "image/heic",
		"a.mov":   "video/quicktime",
		"a.mkv":   "video/x-matroska",
		"a.txt":   "text/plain",
		"a.md":    "text/markdown",
		"a.json":  "application/json",
		"a.pdf":   "application/pdf",
		"a.htm":   "text/html",
		"a.csv":   "text/csv",
		"a.xyz":   "application/octet-stream",
		"noext":   "application/octet-stream",
		"a.wmv":   "application/octet-stream",
		"a.blend": "application/octet-stream",
	}
	for name, want := range cases {
		if got := MIMEType(name); got != want {
			t.Errorf("MIMEType(%q) = %q, want %q", name, got, want)
		}
	}
}
