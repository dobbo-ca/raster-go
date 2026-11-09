# Memory limits

There is no package budget. Each call is bounded by its `Limits`; zero fields
select the defaults. A breach returns `*LimitError{Name, Got, Max}` wrapping
`ErrLimit` (permanent) before anything is allocated or written. Invalid planes
and negative limits return `ErrInvalid`.

| Field | Default | Counts |
|---|---|---|
| `MaxPixels` | 1<<26 | the larger of source and destination pixels |
| `MaxScratchBytes` | 256 MiB | bytes the call allocates internally |
| `MaxOutputBytes` | 256 MiB | bytes written to the destination |

A caller with a larger plane raises the cap within its own budget or calls by
band. The caller reserves `MaxScratchBytes` from its budget before calling.

## Per-operation memory

| Call | Scratch | Output |
|---|---|---|
| `Scale2to1` | 0 | `(W/2)*(H/2)*C` |
| `Luma` | 0 | `W*H` |
| `Rotate90` | 0 | `W*H*C` |
| `Resample` | `srcH*dstW*C` (intermediate) + `4*dstW*C` (accumulator row) + per axis `dstN*(2*stride+8)`, `stride = ceil(4*max(srcN/dstN,1))+2` | `dstW*dstH*C` |

`Resample` computes its scratch exactly (`ScratchBytes`, saturating) and checks
it before allocating; `TestResampleScratchLimit` asserts a refused call
allocates under 64 KiB.

## Time

Cost is linear in pixels. `ctx` is checked before every row (every tile row for
`Rotate90`), so cancellation latency is one row of work. Measured throughput on
the kernels is in `bench/results/` (Apple M4 Pro, arm64, and the hosted amd64
runner); a 2550x3300 grey `Scale2to1` is about 0.13 ms on NEON.

## Cost of the limiter

Validation and limit checks are a few integer comparisons per call, independent
of image size; not measurable against a kernel call.
