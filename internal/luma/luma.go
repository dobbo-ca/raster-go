// Package luma converts RGB to grey with integer BT.601 weights.
package luma

import "context"

// Y = (38R + 75G + 15B + 64) >> 7. The weights sum to 128 and fit a signed
// byte, which VPMADDUBSW needs; Y of 255,255,255 is 255.
const wr, wg, wb = 38, 75, 15

// Plane converts a w*h RGB plane to grey. The caller has validated lengths.
func Plane(ctx context.Context, dst []byte, dstStride int, src []byte, srcStride, w, h int) error {
	for y := 0; y < h; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		row(dst[y*dstStride:][:w], src[y*srcStride:][:3*w], w)
	}
	return nil
}

func rowGeneric(dst, src []byte, w int) {
	for x := 0; x < w; x++ {
		dst[x] = byte((wr*uint(src[3*x]) + wg*uint(src[3*x+1]) + wb*uint(src[3*x+2]) + 64) >> 7)
	}
}

func row(dst, src []byte, w int) {
	done := lumaAsm(dst, src, w)
	rowGeneric(dst[done:], src[3*done:], w-done)
}
