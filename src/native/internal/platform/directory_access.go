package platform

// DirectoryAccess is the intent "check whether I can change entries in this directory".
type DirectoryAccess interface {
	CanWrite(path string) bool
}
