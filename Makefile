
VERSION=`git describe --always`

LDFLAGS=-X main.appVersion=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" .

.PHONY: build_arm64
build_arm64:
	CGO_ENABLED=0 \
	GOARCH=arm64 \
	GOOS=linux \
	go build -v -o mikrotik-exporter-linux-arm64  --ldflags "$(LDFLAGS)"

.PHONY: run
run:
	go run . -config-file config.yml -log-level debug

.PHONY: lint
lint:
	golangci-lint run --fix


.PHONY: format
format:
	wsl -fix . || true
	find . -name '*.go' -type f -exec gofumpt -w {} ';'
