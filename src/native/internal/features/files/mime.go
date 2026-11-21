package files

// mimeByExtension is the MIME table of mime-type.ts. Extensions outside it report application/octet-stream.
var mimeByExtension = map[string]string{
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
	"bmp":  "image/bmp",
	"tif":  "image/tiff",
	"tiff": "image/tiff",
	"heic": "image/heic",
	"mp4":  "video/mp4",
	"mov":  "video/quicktime",
	"webm": "video/webm",
	"m4v":  "video/x-m4v",
	"mkv":  "video/x-matroska",
	"avi":  "video/x-msvideo",
	"txt":  "text/plain",
	"md":   "text/markdown",
	"json": "application/json",
	"pdf":  "application/pdf",
	"html": "text/html",
	"htm":  "text/html",
	"csv":  "text/csv",
}

// fallbackMIMEType is what a file with an unknown extension reports.
const fallbackMIMEType = "application/octet-stream"

// MIMEType returns the MIME type of a file by its extension. It never reads the file.
func MIMEType(name string) string {
	if mime, ok := mimeByExtension[ExtensionOf(name)]; ok {
		return mime
	}
	return fallbackMIMEType
}
