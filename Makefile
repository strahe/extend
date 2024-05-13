unexport GOFLAGS

extend: ffi-deps
	go build $(GOFLAGS) -o extend ./
.PHONY: extend

calibnet: GOFLAGS+=-tags=calibnet
calibnet: extend

ffi-deps:
	git submodule update --init --recursive
	make -C extern/filecoin-ffi
