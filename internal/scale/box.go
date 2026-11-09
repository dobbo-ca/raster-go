// Package scale holds the exact 2:1 box kernel.
package scale

import "context"

// Box2to1 writes dw*dh output samples, each (a+b+c+d+2)>>2 of a 2x2 block.
// The caller has validated every length; slices below panic rather than read
// out of range.
func Box2to1(ctx context.Context, dst []byte, dstStride int, src []byte, srcStride, dw, dh, ch int) error {
	rb := dw * ch
	for y := 0; y < dh; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		boxRow(dst[y*dstStride:][:rb], src[2*y*srcStride:][:2*rb], src[(2*y+1)*srcStride:][:2*rb], dw, ch)
	}
	return nil
}

// boxRowGeneric is the reference: it also finishes the tail the asm leaves.
func boxRowGeneric(dst, r0, r1 []byte, dw, ch int) {
	for x := 0; x < dw; x++ {
		for c := 0; c < ch; c++ {
			i := 2*x*ch + c
			dst[x*ch+c] = byte((uint(r0[i]) + uint(r0[i+ch]) + uint(r1[i]) + uint(r1[i+ch]) + 2) >> 2)
		}
	}
}

// boxRow runs asm over the whole blocks of a grey row, generic over the rest.
func boxRow(dst, r0, r1 []byte, dw, ch int) {
	done := 0
	if ch == 1 {
		done = boxGray(dst, r0, r1, dw)
	}
	boxRowGeneric(dst[done*ch:], r0[2*done*ch:], r1[2*done*ch:], dw-done, ch)
}
