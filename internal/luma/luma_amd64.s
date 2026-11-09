//go:build amd64 && !purego

#include "textflag.h"

// VPSHUFB gathers R,G of 4 pixels into the low 8 bytes and B,0 into the high
// 8 bytes of each 128-bit lane.
DATA shuf<>+0(SB)/8, $0x0a09070604030100
DATA shuf<>+8(SB)/8, $0x800b800880058002
GLOBL shuf<>(SB), RODATA|NOPTR, $16

// signed weights: (38,75) x4 then (15,0) x4
DATA wts<>+0(SB)/8, $0x4b264b264b264b26
DATA wts<>+8(SB)/8, $0x000f000f000f000f
GLOBL wts<>(SB), RODATA|NOPTR, $16

DATA rnd<>+0(SB)/8, $0x0040004000400040
DATA rnd<>+8(SB)/8, $0x0040004000400040
GLOBL rnd<>(SB), RODATA|NOPTR, $16

// func lumaAVX2(dst, src *byte, n int)
// 16 pixels per pass: Y = (38R+75G+15B+64)>>7 via VPMADDUBSW.
TEXT ·lumaAVX2(SB), NOSPLIT, $0-24
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	VBROADCASTI128 shuf<>(SB), Y13
	VBROADCASTI128 wts<>(SB), Y12
	VBROADCASTI128 rnd<>(SB), Y11

loop:
	VMOVDQU     (SI), X0
	VINSERTI128 $1, 24(SI), Y0, Y0
	VMOVDQU     12(SI), X1
	VINSERTI128 $1, 36(SI), Y1, Y1
	VPSHUFB     Y13, Y0, Y0
	VPSHUFB     Y13, Y1, Y1
	VPMADDUBSW  Y12, Y0, Y0
	VPMADDUBSW  Y12, Y1, Y1
	VPSRLDQ     $8, Y0, Y2
	VPSRLDQ     $8, Y1, Y3
	VPADDW      Y2, Y0, Y0
	VPADDW      Y3, Y1, Y1
	VPUNPCKLQDQ Y1, Y0, Y0
	VPADDW      Y11, Y0, Y0
	VPSRLW      $7, Y0, Y0
	VPACKUSWB   Y0, Y0, Y0
	VPERMQ      $0x08, Y0, Y0
	VMOVDQU     X0, (DI)
	ADDQ        $48, SI
	ADDQ        $16, DI
	SUBQ        $16, CX
	JNZ         loop
	VZEROUPPER
	RET
