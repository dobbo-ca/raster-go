// Package raster provides 8-bit grey and RGB raster kernels: exact 2:1 scale,
// separable Catmull-Rom resample, RGB to luma and quarter-turn rotation.
//
// Every entry point takes a Limits (pixels, scratch bytes and output bytes per
// call; the zero value is the safe default) and a context checked between
// rows. Kernels write only into caller-allocated planes, except Resample,
// whose scratch is bounded by Limits.MaxScratchBytes before it is allocated;
// the caller reserves that from its own budget. There is no package budget.
//
// Where SIMD helps, the kernels have AVX2 (amd64) and NEON (arm64) assembly
// behind a pure-Go reference that is the oracle and the fallback; output is
// bit-identical across tiers and architectures. Build with -tags purego to
// force the Go path, or set RASTERGO_CPU=generic|avx2|avx512 to cap the tier
// at run time. Impl reports what a kernel dispatches to.
package raster
