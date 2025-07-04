FROM golang:1.24-alpine AS cexec
RUN apk add --no-cache git ca-certificates gcc linux-headers musl-dev
COPY . /src
WORKDIR /src/cexec
RUN go mod download
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags "-linkmode external -extldflags '-static' -s -w" -o cexec

FROM scratch
COPY --from=cexec /src/cexec/cexec /usr/bin/cexec
ENTRYPOINT ["/usr/bin/cexec"]
