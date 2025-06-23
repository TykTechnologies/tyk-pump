FROM golang:1.23.10 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOEXPERIMENT=boringcrypto go build -tags=boringcrypto -o tyk-pump .

FROM debian:bookworm-slim

WORKDIR /opt/tyk-pump

COPY --from=builder /app/tyk-pump .

COPY pump.example.conf /opt/tyk-pump/pump.conf

EXPOSE 8080

ENTRYPOINT ["/opt/tyk-pump/tyk-pump"]
CMD ["--conf=/opt/tyk-pump/pump.conf"]