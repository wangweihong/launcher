#!/bin/bash

K8sDepsToolPath=$(cd `dirname $0`;pwd)
imageTag="192.168.18.250:5002/launcher/k8s-deps-tool:v1.0.0"

main(){
    cd ${K8sDepsToolPath}

    # build docker image
    docker build . -t "${imageTag}"
}
main
