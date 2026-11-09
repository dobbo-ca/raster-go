//go:build arm64 && !purego

package cpu

// AVX2 reports whether the AVX2 kernels may run.
func AVX2() bool { return false }

// NEON reports whether the NEON kernels may run. NEON is architecturally
// guaranteed on arm64, so there is nothing to detect.
func NEON() bool { return allowed() }
