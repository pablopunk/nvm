package child

import "syscall"

// applyMemoryLimit caps the child's address space. Soft and hard limits are both set, so the extension
// cannot raise it again. The Go runtime needs about 1 GiB of address space to start, so the host should
// pass at least that (the runtime spike measured the floor at between 1 and 2 GiB).
func applyMemoryLimit(bytes uint64) error {
	limit := syscall.Rlimit{Cur: bytes, Max: bytes}
	return syscall.Setrlimit(syscall.RLIMIT_AS, &limit)
}
