
CURRENT_DIR = $(shell pwd)
VERSION = $(shell git describe --always)
REVISION = $(shell git rev-parse HEAD)
DATE = $(shell date +%Y%m%d%H%M%S)
USER = $(shell whoami)
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)

LDFLAGS=\
	-X github.com/prometheus/common/version.Version=$(VERSION) \
	-X github.com/prometheus/common/version.Revision='$(REVISION) \
	-X github.com/prometheus/common/version.BuildDate=$(DATE) \
	-X github.com/prometheus/common/version.BuildUser=$(USER) \
	-X github.com/prometheus/common/version.Branch=$(BRANCH) \
	-w -s


.PHONY: build
build:
	go build -o mikrotik-exporter \
		-ldflags "$(LDFLAGS)" \
		-gcflags=-trimpath=$(CURRENT_DIR) \
		-asmflags=-trimpath=$(CURRENT_DIR) \
		./cli/

.PHONY: whydeadcode
whydeadcode:
	go build -o mikrotik-exporter \
		-ldflags="-dumpdep $(LDFLAGS)" \
		-gcflags=-trimpath=$(CURRENT_DIR) \
		-asmflags=-trimpath=$(CURRENT_DIR) \
		./cli/ 2>&1 | whydeadcode

.PHONY: build_arm64
build_arm64:
	GOARCH=arm64 \
	GOOS=linux \
	go build -v -o mikrotik-exporter-linux-arm64  \
		--ldflags "$(LDFLAGS)" \
		-gcflags=-trimpath=$(CURRENT_DIR) \
		-asmflags=-trimpath=$(CURRENT_DIR) \
		./cli/

.PHONY: run
run:
	go run ./cli -config-file config.yml -log-level debug

.PHONY: lint
lint:
	golangci-lint run --fix || true
	# go install go.uber.org/nilaway/cmd/nilaway@latest
	nilaway ./... || true
	typos
	# go install golang.org/x/vuln/cmd/govulncheck@lates
	govulncheck ./...



.PHONY: format
format:
	golangci-lint fmt


.PHONY: test
test:
	go test ./...
