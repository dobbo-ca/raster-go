# Design

## Shape

`raster.go` validates and delegates; each kernel is an `internal/` package.
`internal/cpu` detects AVX2 (CPUID, OSXSAVE, XGETBV; stdlib and a few lines of
asm, no `x/sys/cpu`) and honours `RASTERGO_CPU`. NEON needs no detection.

## Tiers

Each asm kernel is a pair: a Go reference (also the tail the asm leaves) and an
asm body that handles whole blocks only. Files: `<k>_amd64.{go,s}`,
`<k>_arm64.{go,s}`, `<k>_noasm.go` (`purego` or other arches). `Impl` reports the
tier; `testdata/impl_expected.txt` lists the kept tiers per kernel and arch.

## Bounds

Asm is unexported. The Go stub validates planes and slices every row to the
exact bytes the asm may touch (`dst[n-1]`, `src[3n+3]` checks before the call),
so a wrong length panics in Go instead of faulting in asm. Tests place buffers
against a PROT_NONE guard page; widths 1, 15-17, 31-35, 63-67 and odd strides.

## Arithmetic

- Scale2to1: `(a+b+c+d+2)>>2`. AVX2: `VPMADDUBSW` with ones plus explicit
  rounding (`VPAVGB` is rounding-biased). NEON: `UADDLP`/`UADALP` then `RSHRN #2`.
- Luma: `(38R+75G+15B+64)>>7`. The weights sum to 128 and fit a signed byte as
  `VPMADDUBSW` needs. AVX2: `VPSHUFB` gather, `VPMADDUBSW`; NEON: `LD3`,
  `UMULL`/`UMLAL`, `RSHRN #7`.
- Resample: Catmull-Rom (a = -0.5), widened by the scale when downscaling,
  taps normalised to Q14 summing to exactly 16384, 8-bit intermediate after the
  horizontal pass, int32 accumulation, round-half-up, clamp. Tap generation is
  the only float code; every product is wrapped in `float64()` so arm64 cannot
  fuse it, and `scripts/nofma.sh` checks the compiler output.
- Rotate90: 16x16 tiles.

NEON instructions the Go assembler lacks (`UADDLP`, `UADALP`, `RSHRN`,
`RSHRN2`) are `WORD`s with the mnemonic in a comment, encodings checked against
clang's disassembler.
