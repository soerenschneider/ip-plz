FROM golang:1.20.3 as builder
ENV MODULE=github.com/soerenschneider/ip-plz
WORKDIR /build/
ADD . /build/
RUN go build -ldflags="-X $MODULE/internal.BuildVersion=$(git describe --tags --abbrev=0 || echo dev) -X $MODULE/internal.CommitHash=$(git rev-parse HEAD)" -o "ip-plz" main.go

FROM gcr.io/distroless/base
COPY --from=builder "/build/ip-plz" /ip-plz
USER 65532:65532
ENTRYPOINT ["/ip-plz"]
