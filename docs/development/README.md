# Development

    make test          # go test -race
    make purego        # Go reference only
    make tiers         # RASTERGO_CPU=generic|avx2|avx512
    make vet           # amd64, arm64 and host
    make nofma         # arm64 cross-compile scan for fused multiply-adds
    make bench         # then: benchstat purego.txt asm.txt
    make arm64-check   # native arm64 equality, goldens, Impl, purego, short bench

Compare tiers: `go test -run '^$' -bench . -count 6 . > asm.txt` and the same
with `-tags purego > purego.txt`, then `benchstat purego.txt asm.txt`.

Regenerate goldens only when the reference changes on purpose, on amd64 with the
reference path:

    RASTERGO_UPDATE_GOLDEN=1 GOAMD64=v1 go test -tags purego -run TestGolden .

Changing which tiers a kernel keeps updates `testdata/impl_expected.txt` in the
same diff.

Fuzz: `go test -run '^$' -fuzz FuzzLuma -fuzztime 30s .` (Scale2to1, Luma,
Rotate90, Resample). Seeds live in `testdata/fuzz/<Target>`.
