//go:build amd64 && !purego

#include "textflag.h"

// func box2AVX2(dst, r0, r1 *byte, n int)
// VPMADDUBSW with ones sums byte pairs; VPAVGB is rounding-biased so the
// rounding is explicit: (a+b+c+d+2)>>2.
TEXT ·box2AVX2(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ r0+8(FP), SI
	MOVQ r1+16(FP), DX
	MOVQ n+24(FP), CX
	VPCMPEQB Y15, Y15, Y15
	VPABSB   Y15, Y15       // bytes of 1
	VPCMPEQW Y14, Y14, Y14
	VPSRLW   $15, Y14, Y14  // words of 1
	VPADDW   Y14, Y14, Y14  // words of 2

loop:
	VMOVDQU    (SI), Y0
	VMOVDQU    32(SI), Y1
	VMOVDQU    (DX), Y2
	VMOVDQU    32(DX), Y3
	VPMADDUBSW Y15, Y0, Y0
	VPMADDUBSW Y15, Y1, Y1
	VPMADDUBSW Y15, Y2, Y2
	VPMADDUBSW Y15, Y3, Y3
	VPADDW     Y2, Y0, Y0
	VPADDW     Y3, Y1, Y1
	VPADDW     Y14, Y0, Y0
	VPADDW     Y14, Y1, Y1
	VPSRLW     $2, Y0, Y0
	VPSRLW     $2, Y1, Y1
	VPACKUSWB  Y1, Y0, Y0
	VPERMQ     $0xD8, Y0, Y0
	VMOVDQU    Y0, (DI)
	ADDQ       $64, SI
	ADDQ       $64, DX
	ADDQ       $32, DI
	SUBQ       $32, CX
	JNZ        loop
	VZEROUPPER
	RET
