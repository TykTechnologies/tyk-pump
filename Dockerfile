FROM golang:1.25@sha256:83978e9c0c95d28fe29a9be9095b45d42c8d2ee75c3243f32b0dd1f0daec9043 as builder

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