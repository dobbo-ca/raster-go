#!/bin/sh
# No-FMA guard: cross-compile for arm64 and fail on any fused multiply-add the
# Go compiler emitted for this module (spec 3.3). Compiles, runs nothing.
set -eu
out=$(GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -a -gcflags='github.com/dobbo-ca/raster-go/...=-S' ./... 2>&1 |
	grep -E '\bF(N)?M(ADD|SUB)[DS]?\b' || true)
if [ -n "$out" ]; then
	printf '%s\n' "$out"
	echo "fused multiply-add found: wrap the product in float64()" >&2
	exit 1
fi
echo "no-FMA scan: clean"
