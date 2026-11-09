//go:build amd64 && !purego

package luma

import "github.com/dobbo-ca/raster-go/internal/cpu"

// lumaAVX2 converts n pixels (n a multiple of 16). It reads 3n+4 bytes of src.
//
//go:noescape
func lumaAVX2(dst, src *byte, n int)

func lumaAsm(dst, src []byte, w int) int {
	n := (w - 2) / 16 * 16 // the last load reads 4 bytes past 3n
	if n <= 0 || !cpu.AVX2() {
		return 0
	}
	_, _ = dst[n-1], src[3*n+3]
	lumaAVX2(&dst[0], &src[0], n)
	return n
}

// Impl names the tier in use.
func Impl() string {
	if cpu.AVX2() {
		return "avx2"
	}
	return "generic"
}
