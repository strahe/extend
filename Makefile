unexport GOFLAGS

GO_BUILD_IMAGE?=golang:1.22.3
VERSION?=$(shell git describe --always --tag --dirty)
docker_sanitized_version=$(shell echo ${VERSION} | sed 's:/:-:g')
IMAGE_NAME?=gh-efforts/extend

extend: ffi-deps
	go build $(GOFLAGS) -o extend ./
.PHONY: extend

mainnet: extend

calibnet: GOFLAGS+=-tags=calibnet
calibnet: extend

ffi-deps:
	git submodule update --init --recursive
	make -C extern/filecoin-ffi

# mainnet
.PHONY: docker-mainnet
docker-mainnet: DOCKER_FILE ?= Dockerfile
docker-mainnet: NETWORK_TARGET ?= mainnet
docker-mainnet: IMAGE_TAG ?= $(docker_sanitized_version)
docker-mainnet: docker-build-image-template

# Calibnet
.PHONY: docker-calibnet
docker-calibnet: DOCKER_FILE ?= Dockerfile
docker-calibnet: NETWORK_TARGET ?= calibnet
docker-calibnet: IMAGE_TAG ?= $(docker_sanitized_version)
docker-calibnet: docker-build-image-template

.PHONY: docker-build-image-template
docker-build-image-template:
	docker build -f $(DOCKER_FILE) \
		--build-arg NETWORK_TARGET=$(NETWORK_TARGET) \
		--build-arg GO_BUILD_IMAGE=$(GO_BUILD_IMAGE) \
		-t $(IMAGE_NAME):$(NETWORK_TARGET)-latest \
		-t $(IMAGE_NAME):$(NETWORK_TARGET)-$(IMAGE_TAG) \
		.
