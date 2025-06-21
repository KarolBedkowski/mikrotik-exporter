
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
	-X github.com/prometheus/common/version.Branch=$(BRANCH) \
	-w -s


.PHONY: build
build:
	go build -o mikrotik-exporter -ldflags "$(LDFLAGS)" ./cli/

.PHONY: build_arm64
build_arm64:
	GOARCH=arm64 \
	GOOS=linux \
	go build -v -o mikrotik-exporter-linux-arm64  --ldflags "$(LDFLAGS)" ./cli/

.PHONY: run
run:
	go run . -config-file config.yml -log-level debug

.PHONY: lint
lint:
	golangci-lint run --fix || true
	# go install go.uber.org/nilaway/cmd/nilaway@latest
	nilaway ./... || true
	typos


.PHONY: format
format:
	golangci-lint fmt


.PHONY: test
test:
	go test ./...
