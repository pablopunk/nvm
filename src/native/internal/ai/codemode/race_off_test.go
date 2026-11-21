//go:build !race

package codemode

// testMemoryLimit is the program child's address-space cap in tests: the same 2 GiB the app uses.
const testMemoryLimit = 2 << 30
