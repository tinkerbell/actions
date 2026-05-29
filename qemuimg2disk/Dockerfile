FROM alpine:3.21 AS builder
ENV QEMU_NAME="qemu"
ARG QEMU_REV="11.0.1"
ENV QEMU_SRC_BASENAME="${QEMU_NAME}-${QEMU_REV}"
ENV QEMU_SRC_URL="https://download.qemu.org/${QEMU_SRC_BASENAME}.tar.xz"

RUN apk add --update --upgrade \
    gcc \
    glib-dev glib-static \
    libc-dev \
    linux-headers \
    make \
    meson \
    ninja \
    perl \
    pkgconf \
    python3 py3-setuptools \
    samurai \
    zlib-dev zlib-static \
    bash git parted patch xz \
    curl-dev curl-static nettle-dev nettle-static brotli-dev brotli-static nghttp2-dev nghttp2-static openssl-libs-static \
    zstd-dev zstd-static libpsl-static libunistring-static libidn2-static bzip2-static

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

FROM golang:1.26-alpine AS qemuimg2disk
COPY . /src
WORKDIR /src/qemuimg2disk
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o qemuimg2disk .

# Collect all the files we need into a scratch image.
FROM scratch AS collect
COPY --from=builder /bin/qemu-img /bin/qemu-img
COPY --from=builder /bin/sh /bin/sh
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# qemu-img's static libcurl is built with --with-ca-bundle=/etc/ssl/cert.pem,
# so HTTPS sources fail without a CA bundle at that path. On Alpine this is a
# symlink, which `COPY --from` would preserve and break in the scratch image,
# so copy the resolved bundle directly.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/cert.pem
COPY --from=builder /lib/ld-musl*  /lib/
COPY --from=qemuimg2disk /src/qemuimg2disk/qemuimg2disk /usr/bin/qemuimg2disk
COPY --from=qemuimg2disk /src/qemuimg2disk/entrypoint.sh /usr/bin/entrypoint.sh

# Flatten the collection into a minimal layer set.
FROM scratch
COPY --from=collect / /
ENTRYPOINT ["/usr/bin/entrypoint.sh"]
