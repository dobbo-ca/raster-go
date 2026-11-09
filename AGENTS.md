# AGENTS.md

Go raster kernels with AVX2/NEON assembly and a pure-Go reference. Public repo:
hosted CI only, no cgo, no dependencies outside the standard library, no real
documents or PII in tests (use `internal/fixture`).

- Layout: thin facade in `raster.go`; one package per kernel under `internal/`.
- Each asm kernel has a `_generic`/reference path, `_amd64`/`_arm64` files and a
  `purego` build tag. The Go reference is the oracle: output must be
  bit-identical on every tier and arch (`testdata/golden/hashes.txt`).
- Integer kernels only. Any float (tap generation) wraps every product in
  `float64()`; `scripts/nofma.sh` must stay clean.
- Every entry point takes `Limits` and `ctx`; validate before touching asm.
- Keep rule: asm stays only if it beats the Go reference by 1.5x. Record the
  bench in `docs/decisions.md` and update `testdata/impl_expected.txt`.
- `make test`, `make purego`, `make tiers`, `make vet`, `make nofma`, `make arm64-check`.
