FROM golang:1.26-alpine AS cexec
COPY . /src
WORKDIR /src/cexec
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o cexec

FROM scratch
COPY --from=cexec /src/cexec/cexec /usr/bin/cexec
ENTRYPOINT ["/usr/bin/cexec"]
