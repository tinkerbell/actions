# syntax=docker/dockerfile:1

# Build mkfs as an static
FROM gcc:10.5.0 AS mke2fs
RUN wget https://mirrors.edge.kernel.org/pub/linux/kernel/people/tytso/e2fsprogs/v1.45.6/e2fsprogs-1.45.6.tar.gz; tar -xvzf ./e2fsprogs-1.45.6.tar.gz
WORKDIR /e2fsprogs-1.45.6/
RUN ./configure --enable-static=yes CFLAGS='-g -O2 -static'
RUN make
RUN make -C misc mke2fs.static

# Static mkswap + swapon via busybox-static. Busybox dispatches applets by
# argv[0], so copying the single binary to /sbin/mkswap and /sbin/swapon in
# the final stage is sufficient. Replaces a full util-linux source build
# that was adding ~10-15 min to every CI run.
FROM alpine:3.18 AS swap
RUN apk add --no-cache busybox-static

# Build rootio
FROM golang:1.21-alpine AS rootio
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src
WORKDIR /src/rootio
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o rootio

# build fattools as static
FROM gcc:10.5.0 AS fattools
RUN git clone https://github.com/dosfstools/dosfstools
WORKDIR /dosfstools/
RUN ./autogen.sh
RUN ./configure
RUN make LDFLAGS="--static"

# build lvm2 as static
FROM alpine:3.18 AS lvm
RUN apk update && apk add lvm2-static=2.03.21-r3

# build mdadm as static.
# Pinned to 4.1: 4.2+ introduced a hard libudev dependency that doesn't
# statically link cleanly against musl. limits.h + sysmacros.h includes
# are required because musl doesn't pull them in transitively.
FROM alpine:3.18 AS mdadm
ARG MDADM_VERSION=4.1
RUN apk add --no-cache gcc make musl-dev linux-headers wget tar xz
WORKDIR /build
RUN wget -qO- https://cdn.kernel.org/pub/linux/utils/raid/mdadm/mdadm-${MDADM_VERSION}.tar.xz | tar -xJ
WORKDIR /build/mdadm-${MDADM_VERSION}
RUN make CXFLAGS="-O2 -static -include limits.h -include sys/sysmacros.h -Wno-error" LDFLAGS="-static" mdadm

# Build final image
FROM scratch
COPY --from=mke2fs /e2fsprogs-1.45.6/misc/mke2fs.static /sbin/mke2fs
COPY --from=swap /bin/busybox.static /sbin/swapon
COPY --from=swap /bin/busybox.static /sbin/mkswap
COPY --from=fattools dosfstools/src/mkfs.fat /sbin/mkfs.fat
COPY --from=lvm /usr/sbin/lvm.static /sbin/lvm
COPY --from=mdadm /build/mdadm-4.1/mdadm /sbin/mdadm
COPY --from=rootio /src/rootio/rootio /usr/bin/rootio
ENTRYPOINT ["/usr/bin/rootio"]
