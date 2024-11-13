unexport GOFLAGS

GOFLAGS=-ldflags="-s -w"
GO_BUILD_IMAGE?=golang:1.23.2
VERSION?=$(shell git describe --always --tag --dirty)
docker_sanitized_version=$(shell echo ${VERSION} | sed 's:/:-:g')
IMAGE_NAME?=gh-efforts/extend

extend: ffi-deps
	go build $(GOFLAGS) -o extend ./
.PHONY: extend

test: ffi-deps
	go test $(GOFLAGS) ./...

ffi-deps:
	git submodule update --init --recursive
	make -C extern/filecoin-ffi
.PHONY: ffi-deps

.PHONY: docker
docker: DOCKER_FILE ?= Dockerfile
docker: IMAGE_TAG ?= $(docker_sanitized_version)
docker: docker-build-image-template

.PHONY: docker-build-image-template
docker-build-image-template:
	docker build -f $(DOCKER_FILE) \
		--build-arg GO_BUILD_IMAGE=$(GO_BUILD_IMAGE) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		.
