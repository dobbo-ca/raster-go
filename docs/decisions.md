# Decisions

Newest first. This file replaces a changelog.

## v0.1.0: first kernels, keep rule applied

**Kept (arm64, NEON).** Measured on darwin/arm64 (Apple M4 Pro, 2550x3300,
`bench/results/darwin-arm64.txt`, benchstat p=0.002, n=6):

| Kernel | Go reference | NEON | Speedup |
|---|---|---|---|
| `Scale2to1` grey | 1.91 ms | 0.129 ms | 14.8x |
| `Luma` | 5.87 ms | 0.419 ms | 14.0x |

Both clear the 1.5x bar of the keep rule (spec 14.2). The other half of the rule,
a kernel share of at least 5% of its stage's CPU in a profile of the Go build,
cannot be measured until kleio calls raster-go; the asm is kept on the speedup
alone and that condition is open. The hosted linux/arm64 run (the reproducible arm64
host, spec 14.2) is in the CI log of the PR (`Short kernel bench`, asm and
purego).

Hosted linux/arm64 (`bench/results/linux-arm64-hosted.txt`, n=3, too few for a
p-value): grey `Scale2to1` 3.55 ms to 0.358 ms (9.9x), `Luma` 11.0 ms to 1.67 ms
(6.6x). Same verdict.

**Kept (amd64, AVX2).** First executed on the hosted ubuntu-24.04 runner (AMD
EPYC 9V45, `bench/results/linux-amd64-hosted.txt`, n=5): the equality, golden
and fuzz tests passed there on the first run, and the benches were:

| Kernel | Go reference | AVX2 | Speedup |
|---|---|---|---|
| `Scale2to1` grey | 2.67 ms | 0.186 ms | 14.3x |
| `Luma` | 10.6 ms | 0.918 ms | 11.5x |

The Mac cannot run AVX2 (Rosetta does not report it), so the 9800X3D judgement
and any AVX-512 question go through kleio's `lib-bench.yml` after merge.

**Go only, by design in v0.1.0:**
- `Scale2to1` RGB: needs a de-interleave; the Go loop is as fast as the shipped
  kernels would be without one. Reported tier is the grey path's.
- `Resample` (Catmull-Rom): Go reference only, 27 ms for 2550x3300 to
  1275x1650 on the M4 Pro (spec expects 5-15 ms with asm). The vertical pass
  vectorises across x with `VPMADDWD`/`SMLAL`; deferred, not rejected, because
  the keep rule needs the Go profile first. `Impl("resample")` reports
  `generic`.
- `Rotate90`: tiled Go, no asm in the plan (spec 6.1 lists none).
- No AVX-512 path: nothing measured to justify one. `RASTERGO_CPU=avx512`
  behaves as `avx2`.

**Luma weights.** `Y = (38R + 75G + 15B + 64) >> 7`, not the 8-bit
`77/150/29`: 150 does not fit the signed byte `VPMADDUBSW` takes, and 7 bits
keep NEON, AVX2 and Go identical with one multiply-accumulate chain. Max error
against exact BT.601 is under one grey level.

**Scale2to1 odd sizes.** Output is `W/2 x H/2`; an odd last row or column is
dropped, not averaged.

**Limits.** `Limits{MaxPixels, MaxScratchBytes, MaxOutputBytes}`, zero is the
default (1<<26 px, 256 MiB, 256 MiB), negative is `ErrInvalid` (owner decision
D2). No package budget.

**File naming.** Kernels are `<k>.go` (reference), `<k>_amd64.go/.s`,
`<k>_arm64.go/.s` and `<k>_noasm.go`, not the spec's `<k>_generic.go` plus
`kernel_asm.go`/`kernel_purego.go`: the build tags select the arch files
directly, so no separate asm/purego switch file is needed. Deliberate.

**Resample scratch.** `tapStride` is capped at the source length (a row never
has more taps than samples) and `ScratchBytes` counts the float64 weight row
`build` uses per axis; `TestResampleScratchExact` asserts an allowed call
allocates no more than the reported need.

**Golden hashes** are generated on amd64 with `-tags purego GOAMD64=v1` and
checked on every tier and arch.
