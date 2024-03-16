# go run -ldflags "-X mikrotik-exporter/cmd.version=6.6.7-BETA -X mikrotik-exporter/cmd.shortSha=`git rev-parse HEAD`" main.go version

VERSION=`cat VERSION`
SHORTSHA=`git rev-parse --short HEAD`

LDFLAGS=-X main.appVersion=$(VERSION)
LDFLAGS+=-X main.shortSha=$(SHORTSHA)

build:
	go build -ldflags "$(LDFLAGS)" .

utils:
	go get github.com/mitchellh/gox
	go get github.com/tcnksm/ghr

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
