#!/bin/bash
SCRIPTHOME=$(cd `dirname $0`;pwd)

main(){
    cd ${SCRIPTHOME}
    local REGISTRY=192.168.18.250:5002/launcher/gcr.io/google_containers
    local ARCH=amd64
    local HYPERKUBE_BIN=_output/dockerized/bin/linux/${ARCH}/hyperkube

    local BASEIMAGE=gcr.io/google-containers/debian-hyperkube-base-${ARCH}:0.4.1
    local TEMP_DIR=$(mktemp -d -t hyperkubeXXXXXX)

    cp -r ./* ${TEMP_DIR}

    chmod a+rx ${TEMP_DIR}/bin/hyperkube

    cd ${TEMP_DIR} && sed -i.back "s|BASEIMAGE|${BASEIMAGE}|g" Dockerfile

    docker build -t ${REGISTRY}/hyperkube-${ARCH}:${VERSION} ${TEMP_DIR}
    docker push ${REGISTRY}/hyperkube-${ARCH}:${VERSION}
    rm -rf "${TEMP_DIR}"
}
main $*
