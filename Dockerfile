FROM alpine:3.5
WORKDIR /tyk-pump
COPY ./tyk-pump/pump.example.conf /tyk-pump/pump.conf
COPY ./dist/tyk-pump /tyk-pump/tyk-pump
CMD ["./tyk-pump","--c=/tyk-pump/pump.conf"]
