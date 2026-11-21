//go:build !darwin && !linux

package real

type directoryAccess struct{}

func (directoryAccess) CanWrite(string) bool { return false }
