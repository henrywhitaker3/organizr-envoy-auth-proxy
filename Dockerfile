FROM alpine:3.23.4 AS certs

RUN apk add ca-certificates

FROM golang:1.26 AS gob

ARG VERSION

WORKDIR /build

COPY . /build

RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags="-X main.version=${VERSION}" -a -o organizr-envoy-auth-proxy main.go

FROM scratch

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=gob /build/organizr-envoy-auth-proxy /organizr-envoy-auth-proxy

ENTRYPOINT [ "/organizr-envoy-auth-proxy" ]
