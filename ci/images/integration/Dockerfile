FROM debian:buster-slim

ARG conf_file=/conf/tyk-pump/tyk-pump.conf

ADD pump.tar.gz /opt/tyk-pump

VOLUME ["/conf"]
WORKDIR /opt/tyk-pump

ENTRYPOINT ["/opt/tyk-pump/tyk-pump", "--conf=${conf_file}"]
