#!/bin/bash

SCAVENGERPATH=$(cd `dirname $0`;pwd)
imageTag="192.168.18.250:5002/launcher/scavenger:v2.0.3"

build_binary(){
    cd ${SCAVENGERPATH}
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -installsuffix cgo -o scavenger
}

build_image(){
    cd ${SCAVENGERPATH}
    docker build -t ${imageTag} .
}

main(){
    # 编译二进制文件
    build_binary

    # 编译 docker 镜像
    build_image
}
main
