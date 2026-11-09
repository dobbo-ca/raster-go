//go:build arm64 && !purego

package luma

import "github.com/dobbo-ca/raster-go/internal/cpu"

// lumaNEON converts n pixels (n a multiple of 16) from 3n bytes.
//
//go:noescape
func lumaNEON(dst, src *byte, n int)

func lumaAsm(dst, src []byte, w int) int {
	n := w &^ 15
	if n == 0 || !cpu.NEON() {
		return 0
	}
	_, _ = dst[n-1], src[3*n-1]
	lumaNEON(&dst[0], &src[0], n)
	return n
}

// Impl names the tier in use.
func Impl() string {
	if cpu.NEON() {
		return "neon"
	}
	return "generic"
}
