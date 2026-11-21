//go:build race

package codemode

// testMemoryLimit is the program child's address-space cap in tests. The race runtime reserves a large range of
// address space when it starts, so the 2 GiB the app uses would stop the child before it ran a line.
const testMemoryLimit = 16 << 30
