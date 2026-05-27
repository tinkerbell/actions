FROM golang:1.26-alpine AS base

FROM base AS build-amd64
RUN apk add --no-cache grub grub-bios git ca-certificates gcc musl-dev

FROM base AS build-arm64
RUN apk add --no-cache grub git ca-certificates gcc musl-dev

FROM build-${TARGETARCH} AS grub2disk
WORKDIR /src
COPY go.mod go.sum /src/
RUN go mod download
COPY . /src/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags " -s -w" -o /src/grub2disk/grub2disk /src/grub2disk/

FROM alpine:3.20.2
COPY --from=grub2disk /src/grub2disk/grub2disk /usr/sbin/grub2disk
ENTRYPOINT ["/usr/sbin/grub2disk"]
