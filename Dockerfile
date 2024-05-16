ARG GO_BUILD_IMAGE

# Use the Go build image as the builder
FROM ${GO_BUILD_IMAGE} as builder

# Update and install necessary packages
RUN apt-get update && apt-get install -y hwloc jq libhwloc-dev mesa-opencl-icd ocl-icd-opencl-dev

# Set the working directory and copy the project files
WORKDIR /go/src/github.com/gh-efforts/extend
COPY . .

RUN make ffi-deps
RUN go mod download

ARG NETWORK_TARGET
RUN make ${NETWORK_TARGET} && cp ./extend /usr/bin/

# Use the buildpack-deps image for the final image
FROM buildpack-deps:bookworm-curl

# Copy necessary files from the builder
COPY --from=builder /go/src/github.com/gh-efforts/extend/extend /usr/bin/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libOpenCL.so* /lib/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libhwloc.so* /lib/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libnuma.so* /lib/
COPY --from=builder /usr/lib/x86_64-linux-gnu/libltdl.so* /lib/

# Install jq
RUN apt-get update && apt-get install -y --no-install-recommends jq

ENTRYPOINT ["/usr/bin/extend"]
CMD ["--help"]