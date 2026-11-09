// Package cpu reports which SIMD tiers the kernels may use. RASTERGO_CPU
// (generic, avx2 or avx512) caps the tier; no kernel has an avx512 path yet,
// so avx512 behaves as avx2. Detection uses the standard library only.
package cpu

import "os"

// Forced is the RASTERGO_CPU value read at init.
var Forced = os.Getenv("RASTERGO_CPU")

func allowed() bool { return Forced != "generic" }
