unexport GOFLAGS

calibnet: GOFLAGS+=-tags=calibnet
calibnet: extend

extend: ffi-deps
	go build $(GOFLAGS) -o extend ./
.PHONY: extend

ffi-deps:
	git submodule update --init --recursive
	make -C extern/filecoin-ffi
