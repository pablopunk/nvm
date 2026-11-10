package real

import (
	"io"
	"os"
)

// streamFileOps permits deterministic failure injection at the filesystem boundaries.
type streamFileOps struct {
	writeFile   func(*os.File) io.Writer
	syncFile    func(*os.File) error
	publishFile func(*os.Root, string, string, bool) error
	removeFile  func(*os.Root, string) error
}

func (o streamFileOps) writer(file *os.File) io.Writer {
	if o.writeFile != nil {
		return o.writeFile(file)
	}
	return file
}
func (o streamFileOps) sync(file *os.File) error {
	if o.syncFile != nil {
		return o.syncFile(file)
	}
	return file.Sync()
}
func (o streamFileOps) publish(root *os.Root, from, to string, overwrite bool) error {
	if o.publishFile != nil {
		return o.publishFile(root, from, to, overwrite)
	}
	if overwrite {
		return root.Rename(from, to)
	}
	return root.Link(from, to)
}
func (o streamFileOps) remove(root *os.Root, name string) error {
	if o.removeFile != nil {
		return o.removeFile(root, name)
	}
	return root.Remove(name)
}
