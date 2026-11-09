# raster-go

Go raster kernels (scale, luma, rotate) with SIMD assembly, pure-Go references
and strict limits. Standard library only, no cgo, MIT licensed.

| Function | What | Tiers |
|---|---|---|
| `Scale2to1` | exact 2:1 box, `(a+b+c+d+2)>>2` | grey: AVX2, NEON; RGB: Go |
| `Resample` | separable Catmull-Rom, Q14 taps | Go |
| `Luma` | RGB to grey, `(38R+75G+15B+64)>>7` | AVX2, NEON |
| `Rotate90` | quarter turn | Go |

Every call takes a `Limits` (pixels, scratch bytes, output bytes) and a
`context.Context`. See `docs/guides/memory-limits.md`, `docs/design.md` and
`docs/decisions.md`. `-tags purego` or `RASTERGO_CPU=generic` forces the Go path.

Status: v0.x, API may change.
