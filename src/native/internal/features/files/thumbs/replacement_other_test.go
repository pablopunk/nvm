//go:build !windows

package thumbs

func replacementBlockedByReader(error) bool { return false }
