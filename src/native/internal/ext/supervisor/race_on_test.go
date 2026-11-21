//go:build race

package supervisor

// addressSpaceCap is the child's address-space limit in tests. The race runtime reserves a large range of address
// space at start, so a 2 GiB limit stops the child before it runs; 16 GiB still stops the memory bomb, which asks for
// 64 GiB.
const addressSpaceCap = 16 << 30
