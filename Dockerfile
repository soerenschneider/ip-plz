FROM golang:1.24.2 as builder

ENV MODULE=github.com/soerenschneider/ip-plz
ENV CGO_ENABLED=0

WORKDIR /build/
ADD go.mod go.sum /build/
RUN go mod download
ADD . /build/

RUN go build -tags app -ldflags="-X $MODULE/internal.BuildVersion=$(git describe --tags --abbrev=0 || echo dev) -X $MODULE/internal.CommitHash=$(git rev-parse HEAD)" -o "ip-plz"

FROM gcr.io/distroless/base
COPY --from=builder "/build/ip-plz" /ip-plz
USER 65532:65532
ENTRYPOINT ["/ip-plz"]
