//go:build arm64 && !purego

package scale

import "github.com/dobbo-ca/raster-go/internal/cpu"

// box2NEON writes n outputs (n a multiple of 16) from 2n bytes of each row.
//
//go:noescape
func box2NEON(dst, r0, r1 *byte, n int)

func boxGray(dst, r0, r1 []byte, dw int) int {
	n := dw &^ 15
	if n == 0 || !cpu.NEON() {
		return 0
	}
	_, _, _ = dst[n-1], r0[2*n-1], r1[2*n-1]
	box2NEON(&dst[0], &r0[0], &r1[0], n)
	return n
}

// Impl names the tier of the grey path.
func Impl() string {
	if cpu.NEON() {
		return "neon"
	}
	return "generic"
}
