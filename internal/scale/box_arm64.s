//go:build arm64 && !purego

#include "textflag.h"

// func box2NEON(dst, r0, r1 *byte, n int)
// UADDLP/UADALP sum the 2x2 block into 16-bit lanes, RSHRN #2 is the
// explicit (sum+2)>>2. The Go assembler has no mnemonics for these, so they
// are WORDs (encodings checked against clang's disassembly).
TEXT ·box2NEON(SB), NOSPLIT, $0-32
	MOVD dst+0(FP), R0
	MOVD r0+8(FP), R1
	MOVD r1+16(FP), R2
	MOVD n+24(FP), R3

loop:
	VLD1.P 32(R1), [V0.B16, V1.B16]
	VLD1.P 32(R2), [V2.B16, V3.B16]
	WORD $0x6e202804 // uaddlp v4.8h, v0.16b
	WORD $0x6e202825 // uaddlp v5.8h, v1.16b
	WORD $0x6e206844 // uadalp v4.8h, v2.16b
	WORD $0x6e206865 // uadalp v5.8h, v3.16b
	WORD $0x0f0e8c86 // rshrn  v6.8b,  v4.8h, #2
	WORD $0x4f0e8ca6 // rshrn2 v6.16b, v5.8h, #2
	VST1.P [V6.B16], 16(R0)
	SUBS   $16, R3, R3
	BNE    loop
	RET
