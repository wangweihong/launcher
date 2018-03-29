#!/bin/bash

SCRIPTHOME=$(cd `dirname $0`;pwd)
BUILDDIR=$(cd ${SCRIPTHOME}/../../common;pwd)

. ${SCRIPTHOME}/../common/kubernetes.conf
. ${SCRIPTHOME}/../common/utils.sh
. ${SCRIPTHOME}/../common/dependence.sh

export PATH=$PATH:/usr/local/bin

setFirewall(){
    Log::Register "${FUNCNAME}"

	Log "setenforce 0"
    setenforce 0 2>/dev/null || true
    # ubuntu
    command -v ufw >/dev/null 2>&1
    [[ $? -gt 0 ]] && service ufw stop 2>/dev/null || true
    # centos
    command -v systemctl > /dev/null 2>&1
    if [ $? -gt 0 ];then
        systemctl disable firewalld || true
        systemctl stop firewalld || true
    fi

	Log::UnRegister "${FUNCNAME}"
}

installSoft(){
    Log::Register "${FUNCNAME}"

    Log "remove older containers"
    docker rm -f temp-k8s-deps-bin &> /dev/null || true
    docker rm -f temp-k8s-deps-tool &> /dev/null || true

    Log "create container -- temp-k8s-deps-bin and temp-k8s-deps-tool"
    docker run -dit --name="temp-k8s-deps-bin" ${image_repository}/launcher/k8s-deps-bin:${version}
    Utils::exitCode $?
    docker run -dit --name="temp-k8s-deps-tool" ${image_repository}/launcher/k8s-deps-tool:v1.0.0
    Utils::exitCode $?

    cd $BUILDDIR/

    Log "copy cni"
    mkdir -p /opt
    mkdir -p /etc/cni/net.d
    docker cp temp-k8s-deps-tool:/root/tool/cni/opt/cni /opt/
    docker cp temp-k8s-deps-tool:/root/tool/cni/usr/local/bin/* /usr/local/bin/

    Log "copy kubeadm"
    mkdir -p /etc/systemd/system/kubelet.service.d
    docker cp temp-k8s-deps-bin:/root/deps/kubeadm/etc/systemd/system/kubelet.service.d/10-kubeadm.conf /etc/systemd/system/kubelet.service.d/
    docker cp temp-k8s-deps-bin:/root/deps/kubeadm/usr/local/bin/kubeadm /usr/local/bin/

    Log "copy kubectl"
    docker cp temp-k8s-deps-bin:/root/deps/kubectl/usr/local/bin/kubectl /usr/local/bin/

    Log "remove no needed container -- temp-k8s-deps-bin and temp-k8s-deps-tool"
    docker rm -f temp-k8s-deps-bin
    docker rm -f temp-k8s-deps-tool

    export CNI_PATH=/opt/cni/bin

    Log::UnRegister "${FUNCNAME}"
}

preInit(){
    Log::Register "${FUNCNAME}"

    Log "mount shared dir"
    common::mountshare

    Log "close firewall"
    common::setFirewall

    Log "update iptables config"
    common::updateIptablesConfig

    Log "update docker config"
    common::updateDockerConfig

    Log::UnRegister "${FUNCNAME}"
}

main(){
    Log::Register "${FUNCNAME}"

    common::parsePlatform
    Utils::exitCode $?

    dependence::checkDeps
    Utils::exitCode $?

    Utils::synctime
    preInit
    installSoft

    /bin/bash ${SCRIPTHOME}/../common/kubelet.sh
    Utils::exitCode $?

    if [ -e ${SCRIPTHOME}/../daemonset/storage.sh ];then
        /bin/bash ${SCRIPTHOME}/../daemonset/storage.sh
    fi
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

Log::RegisterFile $0
Log::ParseIdentifier $@
main
Log::UnRegisterFile $0
