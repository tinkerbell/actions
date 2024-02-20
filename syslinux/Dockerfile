# syntax=docker/dockerfile:experimental

# Build syslinux as an syslinux3.86
FROM gcc:10.2.0 as syslinux3.86
RUN apt-get update -y; apt-get install -y nasm build-essential uuid-dev
RUN wget https://cdn.kernel.org/pub/linux/utils/boot/syslinux/3.xx/syslinux-3.86.tar.gz; tar -xvzf ./syslinux-3.86.tar.gz
WORKDIR /syslinux-3.86/libinstaller
RUN make
WORKDIR /syslinux-3.86/linux
RUN gcc -Wp,-MT,syslinux.o,-MMD,./.syslinux.o.d -W -Wall -Wstrict-prototypes -D_FILE_OFFSET_BITS=64 -g -Os -I. -I.. -I../libinstaller -static -c -o syslinux.o syslinux.c
RUN gcc -Wp,-MT,syslxmod.o,-MMD,./.syslxmod.o.d -W -Wall -Wstrict-prototypes -D_FILE_OFFSET_BITS=64 -g -Os -I. -I.. -I../libinstaller -static -c -o syslxmod.o ../libinstaller/syslxmod.c
RUN gcc -Wp,-MT,bootsect_bin.o,-MMD,./.bootsect_bin.o.d -W -Wall -Wstrict-prototypes -D_FILE_OFFSET_BITS=64 -g -Os -I. -I.. -I../libinstaller -static -c -o bootsect_bin.o ../libinstaller/bootsect_bin.c
RUN gcc -Wp,-MT,ldlinux_bin.o,-MMD,./.ldlinux_bin.o.d -W -Wall -Wstrict-prototypes -D_FILE_OFFSET_BITS=64 -g -Os -I. -I.. -I../libinstaller -static -c -o ldlinux_bin.o ../libinstaller/ldlinux_bin.c
RUN gcc -s -static -o syslinux syslinux.o syslxmod.o bootsect_bin.o ldlinux_bin.o

# Build syslinux
FROM golang:1.15-alpine as syslinux
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src/syslinux/
WORKDIR /src/syslinux
ENV GO111MODULE=on
RUN --mount=type=cache,sharing=locked,id=gomod,target=/go/pkg/mod/cache \
    --mount=type=cache,sharing=locked,id=goroot,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o syslinux

# Build final image
FROM alpine
COPY --from=syslinux3.86 /syslinux-3.86/mbr/mbr.bin /mbr.bin.386
COPY --from=syslinux3.86 /syslinux-3.86/linux/syslinux /syslinux.386
COPY --from=syslinux /src/syslinux/syslinux .
ENTRYPOINT ["/syslinux"]
