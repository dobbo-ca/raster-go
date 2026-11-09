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
alone and that condition is open. The linux/arm64 hosted runner and the
amd64 runs are in the CI log of each PR (`Kernel bench` steps).

**Kept (amd64, AVX2): provisional.** The same two kernels have AVX2 paths.
They were written without an AVX2 host to hand (Rosetta on the Mac does not
report AVX2); the hosted ubuntu-24.04 runner is the first machine to execute
them, and amd64 numbers for the keep rule come from its `Kernel bench` step.
The 9800X3D judgement runs through kleio's `lib-bench.yml` after merge.

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

**Golden hashes** are generated on amd64 with `-tags purego GOAMD64=v1` and
checked on every tier and arch.
