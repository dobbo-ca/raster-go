.PHONY: test purego tiers vet lint nofma bench arm64-check

test:
	go test -race ./...

purego:
	go test -tags purego ./...

tiers:
	for t in generic avx2 avx512; do RASTERGO_CPU=$$t go test -count=1 ./... || exit 1; done

vet:
	go vet ./... && GOARCH=arm64 go vet ./... && GOARCH=amd64 go vet ./...

lint:
	golangci-lint run

nofma:
	./scripts/nofma.sh

bench:
	go test -run '^$$' -bench . -benchtime 300ms -count 6 .

# Native arm64 check (Mac or arm64 runner): equality against the references,
# goldens, Impl table, purego, forced tiers and a short bench.
arm64-check:
	go test -count=1 ./...
	go test -count=1 -tags purego ./...
	RASTERGO_CPU=generic go test -count=1 ./...
	go test -run '^$$' -bench . -benchtime 100ms .
