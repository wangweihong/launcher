#!/bin/bash

# 请注意，如果运行的主机没有配置正确的dns解析，可能会造成依赖软件下载不下来，导致创建镜像失败

SCRIPTDIR=$(cd `dirname $0`;pwd)
IMAGE_TAG="192.168.18.250:5002/launcher/kubelet:v1.9.3"

BINDIR=$(cd $SCRIPTDIR/bin;pwd)

. $SCRIPTDIR/../../script/common/utils.sh

build::kubelet::check(){
	command docker &>/dev/null
	[ $? -gt 0 ] && echo "docker already installed? please check." && exit 1
}

build::kubelet::build(){
	Log::Register "${FUNCNAME}"
	Log "go in $SCRIPTDIR"
	cd $SCRIPTDIR
	
	Log "check params..."
	[ ! -e $BINDIR/kubelet ] && echo "there is $BINDIR/kubelet? please check." && exit 1
	cp $BINDIR/kubelet ./	

	Log "docker build ..."
	docker build . -t $IMAGE_TAG

	Log "clean env..."
        docker rm -v $(docker ps -a | grep -v grep | grep Exited | awk '{print $1}' 2>/dev/null) || true
	docker rmi $(docker images | grep -v grep | grep \<none\> | awk '{print $3}' | grep -v IMAGE) 2>/dev/null || true
	rm -rf kubelet

	Log "leave $SCRIPTDIR"
	Log::UnRegister "${FUNCNAME}"
}

build::kubelet::main(){
	Log::Register "${FUNCNAME}"
	build::kubelet::check
	build::kubelet::build
	Log::UnRegister "${FUNCNAME}"
}
build::kubelet::main
