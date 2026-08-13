FROM golang:1.26@sha256:2005724102f45917a63e9d092fc0e4ea56ea575048ce147caad5f5f61502c365 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOEXPERIMENT=boringcrypto go build -tags=boringcrypto -o tyk-pump .

FROM debian:bookworm-slim@sha256:8af0e5095f9964007f5ebd11191dfe52dcb51bf3afa2c07f055fc5451b78ba0e

WORKDIR /opt/tyk-pump

COPY --from=builder /app/tyk-pump .

COPY pump.example.conf /opt/tyk-pump/pump.conf

EXPOSE 8080

ENTRYPOINT ["/opt/tyk-pump/tyk-pump"]
CMD ["--conf=/opt/tyk-pump/pump.conf"]