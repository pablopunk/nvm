//go:build !race

package supervisor

// addressSpaceCap is the child's address-space limit in tests: 2 GiB, which the memory bomb (64 GiB) exceeds.
const addressSpaceCap = 2 << 30
