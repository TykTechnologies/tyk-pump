ARG GO_VERSION=1.16

FROM golang:${GO_VERSION} as builder

# This Dockerfile facilitates bleeding edge development docker image builds
# directly from source. To build a development image, run `make docker`.

RUN apt-get update && apt-get install -yqq make xxd

WORKDIR /opt/tyk-pump
ADD . /opt/tyk-pump

RUN make build

RUN /opt/tyk-pump/tyk-pump --version

FROM golang:${GO_VERSION}

COPY --from=builder /opt/tyk-pump/tyk-pump /opt/tyk-pump/tyk-pump

WORKDIR /opt/tyk-pump

RUN /opt/tyk-pump/tyk-pump --version

COPY pump.example.conf pump.conf

ENTRYPOINT ["/opt/tyk-pump/tyk-pump"]