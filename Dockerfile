FROM golang:1.27@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf as builder

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