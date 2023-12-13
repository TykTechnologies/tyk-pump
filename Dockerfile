FROM debian:buster-slim

ENV TYKVERSION 1.2.0

LABEL Description="Tyk Pump docker image" Vendor="Tyk" Version=$TYKVERSION

RUN apt-get update \
 && apt-get upgrade -y \
 && apt-get install -y --no-install-recommends \
            curl ca-certificates apt-transport-https gnupg \
 && curl -L https://packagecloud.io/tyk/tyk-pump/gpgkey | apt-key add - \
 && apt-get purge -y gnupg \
 && apt-get autoremove -y \
 && rm -rf /root/.cache

RUN echo "deb https://packagecloud.io/tyk/tyk-pump/debian/ jessie main" | tee /etc/apt/sources.list.d/tyk_tyk-pump.list \
 && apt-get update \
 && apt-get install -y tyk-pump=$TYKVERSION \
 && rm -rf /var/lib/apt/lists/*

COPY ./pump.mongo.conf /opt/tyk-pump/pump.conf
VOLUME ["/opt/tyk-pump/"]

WORKDIR /opt/tyk-pump

CMD ["/opt/tyk-pump/tyk-pump", "-c", "/opt/tyk-pump/pump.conf"]
