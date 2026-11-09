//go:build arm64 && !purego

#include "textflag.h"

// func lumaNEON(dst, src *byte, n int)
// LD3 splits 16 RGB pixels into planes; UMULL/UMLAL accumulate
// 38R+75G+15B in 16 bits (at most 32640), RSHRN #7 adds 64 and shifts.
TEXT ·lumaNEON(SB), NOSPLIT, $0-24
	MOVD dst+0(FP), R0
	MOVD src+8(FP), R1
	MOVD n+16(FP), R3
	MOVD $38, R4
	MOVD $75, R5
	MOVD $15, R6
	VDUP R4, V16.B16
	VDUP R5, V17.B16
	VDUP R6, V18.B16

loop:
	VLD3.P 48(R1), [V0.B16, V1.B16, V2.B16]
	VUMULL  V0.B8, V16.B8, V4.H8
	VUMLAL  V1.B8, V17.B8, V4.H8
	VUMLAL  V2.B8, V18.B8, V4.H8
	VUMULL2 V0.B16, V16.B16, V5.H8
	VUMLAL2 V1.B16, V17.B16, V5.H8
	VUMLAL2 V2.B16, V18.B16, V5.H8
	WORD $0x0f098c86 // rshrn  v6.8b,  v4.8h, #7
	WORD $0x4f098ca6 // rshrn2 v6.16b, v5.8h, #7
	VST1.P [V6.B16], 16(R0)
	SUBS   $16, R3, R3
	BNE    loop
	RET
