FROM alpine:3.13.6 as builder
ENV QEMU_NAME="qemu"
ARG QEMU_REV="6.1.0"
ENV QEMU_SRC_BASENAME="${QEMU_NAME}-${QEMU_REV}"
ENV QEMU_SRC_URL="https://download.qemu.org/${QEMU_SRC_BASENAME}.tar.xz"

RUN apk add --update --upgrade \
    gcc \
    glib-dev glib-static \
    libc-dev \
    linux-headers \
    make \
    perl \
    pkgconf \
    python3 \
    samurai \
    zlib-dev zlib-static \
    bash git parted patch xz \
    curl-dev curl-static nettle-dev nettle-static brotli-dev brotli-static nghttp2-dev nghttp2-static openssl-libs-static

WORKDIR /work
RUN wget "${QEMU_SRC_URL}" -O- | tar xJ
WORKDIR /work/${QEMU_SRC_BASENAME}

RUN ./configure \
    --disable-debug-info \
    --disable-system \
    --disable-werror \
    --disable-linux-user \
    --enable-nettle \
    --enable-curl \
    --enable-tools \
    --static

# Fixup brotli linking
RUN sed -e 's|-lbrotlidec|-lbrotlicommon-static -lbrotlidec-static -lbrotlienc-static|' -i build/build.ninja

RUN make -j$(nproc) qemu-img
RUN cp build/qemu-img /bin/qemu-img

# Build partprobe
FROM golang:1.17-alpine as partprobe
COPY . /src
WORKDIR /src
RUN go mod tidy
RUN go build partprobe.go

FROM scratch as collect
COPY --from=builder /bin/qemu-img /bin/qemu-img
COPY --from=builder /bin/sh /bin/sh
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /lib/ld-musl*  /lib/
COPY --from=partprobe /src/partprobe /bin/partprobe
COPY entrypoint.sh /entrypoint.sh

FROM scratch
COPY --from=collect / /
ENTRYPOINT /entrypoint.sh
