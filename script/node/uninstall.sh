#!/bin/bash

SCRIPTHOME=$(cd `dirname $0`;pwd)

. ${SCRIPTHOME}/../common/utils.sh

export PATH=$PATH:/usr/local/bin

removeKubelet() {
    Log::Register "${FUNCNAME}"

    Log "stop and rm k8s-kubelet"
    docker rm -f k8s-kubelet 2>/dev/null || true

    Log::UnRegister "${FUNCNAME}"
}

kubeadmReset(){
    Log::Register "${FUNCNAME}"

	if command -v kubeadm &> /dev/null;then
		kubeadm reset 2>/dev/null || true
	else
		Log "There isn't kubeadm, do you have already installed ?"
	fi

	Log::UnRegister "${FUNCNAME}"
}

removeInstallDir(){
    Log::Register "${FUNCNAME}"

    Log "remove cni"
    rm -rf /opt/cni

    Log "remove kubeadm"
    rm -rf /etc/systemd/system/kubelet.service.d
    rm -f  /usr/local/bin/kubeadm

    Log "remove kubectl"
    rm -f /etc/systemd/system/kubelet.service
    rm -f /usr/local/bin/kubectl

    Log "remove kubelet"
    rm -rf /etc/kubernetes
    rm -f  /usr/local/bin/kubelet

    Log::UnRegister "${FUNCNAME}"
}

removeOtherCon(){
    Log::Register "${FUNCNAME}"

    docker rm -f ufleet-uflow-slave 2> /dev/null || true

    Log::UnRegister "${FUNCNAME}"
}

cleanEnv(){
    Log::Register "${FUNCNAME}"

    Log "Clean Network"
    ip link set cni0 down && ip link delete cni0 type bridge || true
    ip link set flannel.1 down && ip link delete flannel.1 type bridge || true
    docker network ls | awk '{print $2}' | grep -v '^ID$' | grep -v '^bridge$' | grep -v '^host$' | grep -v '^none$' | xargs docker network rm 2>/dev/null || true

    Log "Remove container in system."
    docker rm -f $(docker ps -a | awk '{print $1}') 2>/dev/null || true
    docker rm -f $(docker ps -a | awk '{print $1}') 2>/dev/null || true

    Log "Unmount /var/lib/kubelet"
    cat /proc/mounts | awk '{print $2}' | grep '/var/lib/kubelet/' | xargs umount || true
    umount /var/lib/kubelet 2>/dev/null || true

    Log "Remove dirs"
    rm -rf /etc/kubernetes
    rm -rf /etc/cni/net.d
    rm -rf /var/lib/cni
    rm -rf /var/lib/kubelet
    rm -rf /var/lib/etcd
    rm -rf /var/local/ufleet
    rm -rf /exports/*

    Log::UnRegister "${FUNCNAME}"
}

cleanNet(){
    Log::Register "${FUNCNAME}"

    Log "remove calico created devices"
    modprobe -r ipip

    Log::UnRegister "${FUNCNAME}"
}

main(){
    Log::Register "${FUNCNAME}"

    removeKubelet
    removeOtherCon
    kubeadmReset
    removeInstallDir
    Utils::uni_synctime
    cleanEnv
    cleanNet

    Log::Register "${FUNCNAME}"
}

Log::RegisterFile $0
Log::ParseIdentifier $@
main
Log::UnRegisterFile $0
