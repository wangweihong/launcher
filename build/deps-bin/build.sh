#!/bin/bash

K8sDepsBinPath=$(cd `dirname $0`;pwd)
imageTag="192.168.18.250:5002/launcher/k8s-deps-bin"

main(){
    cd ${K8sDepsBinPath}

    # build docker image
    docker build . -t "${imageTag}:${VERSION}"
}
main
