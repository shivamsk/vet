SHELL := /bin/bash
GITCOMMIT := $(shell git rev-parse HEAD)
VERSION := "$(shell git describe --tags --abbrev=0)-$(shell git rev-parse --short HEAD)"

all: clean setup vet

oapi-codegen-install:
	go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.10.1

protoc-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

oapi-codegen:
	oapi-codegen -package insightapi -generate types ./api/insights-v1.yml > ./gen/insightapi/insights.types.go
	oapi-codegen -package insightapi -generate client ./api/insights-v1.yml > ./gen/insightapi/insights.client.go
	oapi-codegen -package cpv1trials -generate types ./api/cp-v1-trials.yml > ./gen/cpv1trials/trials.types.go
	oapi-codegen -package cpv1trials -generate client ./api/cp-v1-trials.yml > ./gen/cpv1trials/trials.client.go
	oapi-codegen -package cpv1 -generate types ./api/cp-v1.yml > ./gen/cpv1/cp.types.go
	oapi-codegen -package cpv1 -generate client ./api/cp-v1.yml > ./gen/cpv1/cp.client.go

protoc-codegen:
	protoc -I ./api \
		--go_out=./gen/filterinput \
		--go_opt=paths=source_relative \
		./api/filter_input_spec.proto
	protoc -I ./api \
		--go_out=./gen/filtersuite \
		--go_opt=paths=source_relative \
		./api/filter_suite_spec.proto
	protoc -I ./api \
		--go_out=./gen/exceptionsapi \
		--go_opt=paths=source_relative \
		./api/exceptions_spec.proto

setup:
	mkdir -p out \
		gen/insightapi \
		gen/cpv1trials \
		gen/cpv1 \
		gen/filterinput \
		gen/filtersuite \
		gen/exceptionsapi

GO_CFLAGS=-X main.commit=$(GITCOMMIT) -X main.version=$(VERSION)
GO_LDFLAGS=-ldflags "-w $(GO_CFLAGS)"

vet: oapi-codegen protoc-codegen
	go build ${GO_LDFLAGS}

.PHONY: test
test:
	go test ./...

.PHONY: clean
clean:
	-rm -rf out
	-rm -rf gen

gosec:
	-docker run --rm -it -w /app/ -v `pwd`:/app/ securego/gosec \
	-exclude-dir=/app/gen -exclude-dir=/app/spec \
	/app/...
