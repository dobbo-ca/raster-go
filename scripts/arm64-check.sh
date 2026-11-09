#!/bin/sh
# Native arm64 check (spec 3.4): tests on every tier, a short bench saved to
# bench/results/<sha>-darwin-arm64-<model>.txt, then the arm64-check commit
# status on HEAD (needs gh and a pushed HEAD; a failed post only warns).
set -eu
go test -count=1 ./...
go test -count=1 -tags purego ./...
RASTERGO_CPU=generic go test -count=1 ./...
sha=$(git rev-parse HEAD)
model=$(sysctl -n machdep.cpu.brand_string 2>/dev/null || uname -m)
out="bench/results/$sha-$(go env GOOS)-$(go env GOARCH)-$(echo "$model" | tr -c 'A-Za-z0-9\n' - | sed 's/-*$//').txt"
go test -run '^$' -bench . -benchtime 100ms -count 3 . | tee "$out"
gh api "repos/dobbo-ca/raster-go/statuses/$sha" -f state=success -f context=arm64-check \
	-f description="make arm64-check passed on $model" >/dev/null ||
	echo "arm64-check status not posted (push $sha first)" >&2
