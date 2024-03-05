# syntax=docker/dockerfile:1

# Build mkfs as an static
FROM gcc:10.5.0 as mke2fs
RUN wget https://mirrors.edge.kernel.org/pub/linux/kernel/people/tytso/e2fsprogs/v1.45.6/e2fsprogs-1.45.6.tar.gz; tar -xvzf ./e2fsprogs-1.45.6.tar.gz
WORKDIR /e2fsprogs-1.45.6/
RUN ./configure --enable-static=yes CFLAGS='-g -O2 -static'
RUN make
RUN make -C misc mke2fs.static

# build swap as static
FROM gcc:10.5.0 as swap
RUN git clone git://git.kernel.org/pub/scm/utils/util-linux/util-linux.git
WORKDIR /util-linux/
RUN apt-get update; apt-get install -y gettext bison autopoint flex
RUN ./autogen.sh
RUN ./configure LDFLAGS="-static"
RUN make LDFLAGS="-all-static" swapon
RUN make LDFLAGS="-all-static" mkswap

# Build rootio
FROM golang:1.21-alpine as rootio
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src
WORKDIR /src/rootio
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o rootio

# build fattools as static
FROM gcc:10.5.0 as fattools
RUN git clone https://github.com/dosfstools/dosfstools
WORKDIR /dosfstools/
RUN ./autogen.sh
RUN ./configure
RUN make LDFLAGS="--static"

# build lvm2 as static
FROM alpine:3.18 as lvm
RUN apk update && apk add lvm2-static=2.03.21-r3

# Build final image
FROM scratch
COPY --from=mke2fs /e2fsprogs-1.45.6/misc/mke2fs.static /sbin/mke2fs
COPY --from=swap util-linux/swapon /sbin/swapon
COPY --from=swap util-linux/mkswap /sbin/mkswap
COPY --from=fattools dosfstools/src/mkfs.fat /sbin/mkfs.fat
COPY --from=lvm /usr/sbin/lvm.static /sbin/lvm
COPY --from=rootio /src/rootio/rootio /usr/bin/rootio
ENTRYPOINT ["/usr/bin/rootio"]
