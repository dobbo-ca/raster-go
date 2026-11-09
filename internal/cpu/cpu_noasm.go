//go:build purego || !(amd64 || arm64)

package cpu

// AVX2 reports whether the AVX2 kernels may run.
func AVX2() bool { return false }

// NEON reports whether the NEON kernels may run.
func NEON() bool { return false }
