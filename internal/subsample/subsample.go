// Package subsample holds the packed 1-bpp nearest-sample kernel (Go only: no
// asm until profiling says otherwise).
package subsample

import "context"

// ScratchBytes is the x-index table: one int32 per destination column.
func ScratchBytes(dw int) int64 { return 4 * int64(dw) }

// Run samples the source pixel at the centre of each destination pixel,
// ((2d+1)*s)/(2*dstN) per axis, MSB first; pad bits in a row's last byte are
// zero. The caller has validated every length and the limits.
func Run(ctx context.Context, dst []byte, dstStride, dw, dh int, src []byte, srcStride, sw, sh int) error {
	xs := make([]int32, dw)
	for x := range xs {
		xs[x] = int32((2*int64(x) + 1) * int64(sw) / (2 * int64(dw)))
	}
	for y := 0; y < dh; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		sy := int((2*int64(y) + 1) * int64(sh) / (2 * int64(dh)))
		row, out := src[sy*srcStride:], dst[y*dstStride:][:(dw+7)/8]
		clear(out)
		for x, sx := range xs {
			out[x>>3] |= (row[sx>>3] >> (7 - sx&7) & 1) << (7 - x&7)
		}
	}
	return nil
}
