
VERSION=`git describe --always | cut -d- -f1`
DATE=`date +%Y%m%d%H%M%S`
USER=`whoami`
BRANCH=`git branch | grep '^\*' | cut -d ' ' -f 2`
REVISION=`git describe --always`


LDFLAGS=\
	-X github.com/prometheus/common/version.Version=$(VERSION) \
	-X github.com/prometheus/common/version.Revision='$(REVISION) \
	-X github.com/prometheus/common/version.BuildDate=$(DATE) \
	-X github.com/prometheus/common/version.BuildUser=$(USER) \
	-X github.com/prometheus/common/version.Branch=$(BRANCH)


.PHONY: build
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


.PHONY: test
test:
	go test ./...
