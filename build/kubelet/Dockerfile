# description kubelet 1.5.1
FROM ubuntu:16.04

RUN apt-get update -y && apt-get install -y \
    socat \
    ebtables \
    ethtool \
    && rm -rf /var/lib/apt/lists/*

COPY kubelet /usr/bin/kubelet
RUN chmod a+rx /usr/bin/kubelet
