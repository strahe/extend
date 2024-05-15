ARG GO_BUILD_IMAGE

FROM ${GO_BUILD_IMAGE} as builder

ARG NETWORK_TARGET
ENV NETWORK_TARGET=$NETWORK_TARGET

RUN apt-get update
RUN apt-get install -y \
  hwloc \
  jq \
  libhwloc-dev \
  mesa-opencl-icd \
  ocl-icd-opencl-dev

WORKDIR /go/src/github.com/gh-efforts/extend
COPY . /go/src/github.com/gh-efforts/extend

RUN make ffi-deps
RUN go mod download

RUN make $NETWORK_TARGET
RUN cp ./extend /usr/bin/

FROM buildpack-deps:bookworm-curl
COPY --from=builder /go/src/github.com/gh-efforts/extend/extend /usr/bin/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libOpenCL.so* /lib/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libhwloc.so* /lib/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libnuma.so* /lib/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libltdl.so* /lib/

RUN apt-get update
RUN apt-get install -y --no-install-recommends jq

ENTRYPOINT ["/usr/bin/extend"]
CMD ["--help"]