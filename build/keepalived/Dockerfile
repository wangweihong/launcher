FROM ubuntu
MAINTAINER Youruncloud <www.youruncloud.com>

ENV KEEPALIVED_VERSION 1:1.2.19-1
RUN apt-get update && \
    apt-get install --no-install-recommends --no-install-suggests -y keepalived=${KEEPALIVED_VERSION} && \
    apt-get install docker-compose -y && \
    rm -rf /var/lib/apt/lists/*

COPY keepalived /etc/keepalived/
COPY docker-entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
