package backend

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"

	"github.com/pablopunk/nvm/native/internal/platform"
)

// APIVersion is the desktop API major this client speaks. The backend answers it with the compatibility
// manifest and refuses versions it has retired.
const APIVersion = "1"

// clientName is the X-Nevermind-Client value the backend's compatibility checks expect from desktop clients.
const clientName = "desktop"

// defaultClientVersion is sent when the composition root does not provide a version. It sorts below every
// release, so a development build against a backend with a minimum version is reported as too old.
const defaultClientVersion = "0.0.0-dev"

// Compatibility header names. Every backend request carries all five, as the released desktop app does.
const (
	headerClient        = "X-Nevermind-Client"
	headerClientVersion = "X-Nevermind-Client-Version"
	headerAPIVersion    = "X-Nevermind-API-Version"
	headerPlatform      = "X-Nevermind-Platform"
	headerArch          = "X-Nevermind-Arch"
	headerRequestID     = "X-Request-ID"
)

// identity is the non-secret description of this client that every request carries.
type identity struct {
	version  string
	platform string
	arch     string
}

// defaultIdentity describes the running build, with the platform and architecture spelled the way the
// released desktop app spells them, so backend logs keep one vocabulary.
func defaultIdentity() identity {
	return identity{
		version:  defaultClientVersion,
		platform: nodePlatformName(runtime.GOOS),
		arch:     nodeArchName(runtime.GOARCH),
	}
}

// nodePlatformName maps a Go operating system name to the Node.js process.platform value.
func nodePlatformName(goos string) string {
	if goos == "windows" {
		return "win32"
	}
	return goos
}

// nodeArchName maps a Go architecture name to the Node.js process.arch value.
func nodeArchName(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return goarch
	}
}

// setIdentityHeaders writes the compatibility headers onto req.
func (id identity) setIdentityHeaders(req *platform.Request) {
	req.Header.Set(headerClient, clientName)
	req.Header.Set(headerClientVersion, id.version)
	req.Header.Set(headerAPIVersion, APIVersion)
	req.Header.Set(headerPlatform, id.platform)
	req.Header.Set(headerArch, id.arch)
}

// newRequestID returns a random identifier for one request, used for support and log correlation. It is
// empty when the system random source fails, and the header is then omitted.
func newRequestID() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(raw[:])
}
