// Package rotate holds the quarter-turn kernel (Go only: no asm in the plan).
package rotate

import "context"

const tile = 16

// Quarter rotates a sw*sh plane into dst (sh wide, sw high), one 16x16 tile at
// a time for cache locality. The caller has validated every length.
func Quarter(ctx context.Context, dst []byte, dstStride int, src []byte, srcStride, sw, sh, ch int, clockwise bool) error {
	for y0 := 0; y0 < sw; y0 += tile {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x0 := 0; x0 < sh; x0 += tile {
			for y := y0; y < min(y0+tile, sw); y++ {
				for x := x0; x < min(x0+tile, sh); x++ {
					sy, sx := x, sw-1-y
					if clockwise {
						sy, sx = sh-1-x, y
					}
					copy(dst[y*dstStride+x*ch:][:ch], src[sy*srcStride+sx*ch:][:ch])
				}
			}
		}
	}
	return nil
}
