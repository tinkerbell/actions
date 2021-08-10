# syntax=docker/dockerfile:experimental

# Build kexec
FROM golang:1.14-alpine as kexec
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /go/src/github.com/thebsdbox/kexec/v1/
ENV GO111MODULE=on
WORKDIR /go/src/github.com/thebsdbox/kexec/v1
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o kexec

FROM gcc:9.4.0 as uefi
RUN apt-get update; apt-get install -y build-essential uuid-dev iasl git nasm python3-distutils
RUN git clone --branch uefipayload --recursive https://github.com/linuxboot/edk2 uefipayload
WORKDIR /uefipayload
RUN make -C BaseTools
# Use "bash" as replacement for	"sh"
RUN rm /bin/sh && ln -s /bin/bash /bin/sh
RUN source ./edksetup.sh && build -a X64 -p UefiPayloadPkg/UefiPayloadPkg.dsc -b DEBUG -t GCC5 -D BOOTLOADER=LINUXBOOT

# Build final image
FROM scratch
COPY --from=kexec /go/src/github.com/thebsdbox/kexec/v1/kexec .
COPY --from=uefi /uefipayload/Build/UefiPayloadPkgX64/DEBUG_GCC5/FV/UEFIPAYLOAD.fd .
ENTRYPOINT ["/kexec"]