FROM golang:1.13-alpine

RUN apk add --no-cache curl git ca-certificates
RUN go get github.com/markbates/pkger/cmd/pkger
WORKDIR /go/src/mario
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY mario.go .
COPY pkg pkg
COPY cmd cmd
COPY config config
RUN \
  pkger -include /config && \
  go build -o mario cmd/mario/main.go

FROM alpine
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /go/src/mario/mario .
ENTRYPOINT ["./mario"]
CMD ["--help"]
