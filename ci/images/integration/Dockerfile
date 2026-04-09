FROM debian:buster-slim@sha256:32220ea48be72979c9f0810d69f3de9145c9584c2ee966c6ec26edbcf4640c0b

ARG conf_file=/conf/tyk-pump/tyk-pump.conf

ADD pump.tar.gz /opt/tyk-pump

VOLUME ["/conf"]
WORKDIR /opt/tyk-pump

ENTRYPOINT ["/opt/tyk-pump/tyk-pump", "--conf=${conf_file}"]
