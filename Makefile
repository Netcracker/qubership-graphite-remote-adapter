GO    := go
pkgs   = ./...

all: mod-tidy format test assets go-build test-version

assets:
	@echo ">> writing assets"
	@$(GO) get github.com/go-bindata/go-bindata/...
	@$(GO) install github.com/go-bindata/go-bindata/...
	# Using "-mode 0644" and "-modtime 1" to make assets make target deterministic.
	# It sets all file permissions and time stamps to 0644 and 1
	@go-bindata $(bindata_flags) -mode 0644 -modtime 1 -pkg ui -o ui/bindata.go -prefix 'ui/' ui/templates/... ui/static/...
	@$(GO) fmt ./ui

go-build:
go-build:
	@echo ">> building qubership-graphite-remote-adapter"
	@GOOS=linux \
	GOARCH=amd64 \
	$(GO) build -o build/qubership-graphite-remote-adapter \
	-ldflags="-X 'github.com/prometheus/common/version.Version=$(shell cat VERSION)' \
	-X 'github.com/prometheus/common/version.Revision=$(shell git rev-parse HEAD)' \
	-X 'github.com/prometheus/common/version.Branch=$(shell git branch --show-current)' \
	-X 'github.com/prometheus/common/version.BuildDate=$(shell date +"%Y%m%d-%H:%M:%S")'"
	-a .

clean:
	[ -f ui/bindata.go ] && rm ui/bindata.go

format:
	@echo ">> formatting code"
	@$(GO) fmt $(pkgs)

mod-tidy:
	@echo ">> tidy go mods"
	@$(GO) mod tidy

test:
	@echo ">> running tests"
	@$(GO) test $(pkgs)

test-version:
	@echo ">> building qubership-graphite-remote-adapter"
	@echo ">> testing version info"
	@build/qubership-graphite-remote-adapter --version

vet:
	@echo ">> vetting code"
	@$(GO) vet $(pkgs)
