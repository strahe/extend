unexport GOFLAGS

calibnet: GOFLAGS+=-tags=calibnet
calibnet: extend

extend:
	go build $(GOFLAGS) -o extend ./
.PHONY: extend