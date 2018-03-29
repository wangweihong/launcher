#!/bin/bash

SCRIPTHOME=$(cd `dirname $0`;pwd)
BUILDDIR=$(cd ${SCRIPTHOME}/../../common;pwd)
ADDONDIR=$(cd ${SCRIPTHOME}/../addon;pwd)

. ${SCRIPTHOME}/../common/kubernetes.conf
. ${SCRIPTHOME}/../common/utils.sh
. ${SCRIPTHOME}/../common/dependence.sh

export PATH=$PATH:/usr/local/bin

showUsage(){
    cat << END
	Usage: $0
	 Like: $0

END
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

# TODO: check why need restart kubelet.
restartK8sKubelet(){
    for ((i=0;i<60;i++))
    do
        apiserver=$(docker ps -a | grep kube-apiserver)
        if [ ${#apiserver} -gt 0 ];then
            sleep 5
            Log "Restart k8s-kubelet"
            docker restart k8s-kubelet
            break
        fi
        sleep 2
    done
}

init(){
    Log::Register "${FUNCNAME}"

    restartK8sKubelet &
    kubeadm init --config ${SCRIPTHOME}/kubeadm.yaml --skip-preflight-checks
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

preUsing(){
    Log::Register "${FUNCNAME}"

    rm -rf /root/.kube
    mkdir -p /root/.kube
    cp -i /etc/kubernetes/admin.conf /root/.kube/config
    chown 0:0 /root/.kube/config

    Log::UnRegister "${FUNCNAME}"
}

allowDeployPodOnMaster(){
    Log::Register "${FUNCNAME}"

    [[ ${#NODE_NAME} -eq 0 ]] && Log "couldn't get NODE_NAME from kubernetes.conf." && Utils::exitCode 1

    Log "allow pod deploy in master node."
    Log "kubectl taint nodes $NODE_NAME node-role.kubernetes.io/master:NoSchedule-"
    for ((i=0;i<30;i++))
    do
        Log "try to taint node $i time. maybe kube-apiserver not ready."
        kubectl taint nodes $NODE_NAME node-role.kubernetes.io/master:NoSchedule- 2>/dev/null
        [[ $? -eq 0 ]] && break
        sleep 2
    done
    [[ $? -gt 0 ]] && Log "[ Error ] failed to taint master node ${NODE_NAME}." && Utils::exitCode 1

    Log::UnRegister "${FUNCNAME}"
}


ca_import() {
    Log::Register "${FUNCNAME}"

    for ((i=0;i<60;i++))
    do
        [ ! -f /etc/kubernetes/pki/ca.crt ] && sleep 2 && continue
        break
    done
    [ ! -f /etc/kubernetes/pki/ca.crt ] && Log::UnRegister "${FUNCNAME}" && return

    if [[ "x${PLATFORM}" == "xubuntu" ]];then
        Log "remove older k8s-ca.crt"
        rm -rf /usr/local/share/ca-certificates/k8s-ca.crt
        rm -rf /etc/ssl/certs/k8s-ca.pem
        update-ca-certificates

        Log "update k8s-ca"
        cp -f /etc/kubernetes/pki/ca.crt /usr/local/share/ca-certificates/k8s-ca.crt
        update-ca-certificates
    fi

    if [[ "x${PLATFORM}" == "xcentos" ]];then
        Log "remove older k8s-ca.crt"
        rm -rf /etc/pki/ca-trust/source/anchors/k8s-ca.crt
        update-ca-trust

        Log "update k8s-ca"
        cp -f /etc/kubernetes/pki/ca.crt /etc/pki/ca-trust/source/anchors/k8s-ca.crt
        update-ca-trust
    fi

    Log::UnRegister "${FUNCNAME}"
}

recopyVespaceIfNeeded() {
    Log::Register "${FUNCNAME}"

    for ((i=0;i<5;i++))
    do
        vesExist=$(docker ps -a | grep vespace | grep -v grep)
        [[ ${#vesExist} -gt 0 ]] && Log "vespace already start." && break
        rm -rf /tmp/vespaceTemp
        mkdir -p /tmp/vespaceTemp
        mv /etc/kubernetes/manifests/*vespace*.yaml /tmp/vespaceTemp/
        Log "waiting for vespace container stop."
        sleep 1
        Log "waiting for vespace container start."
        mv /tmp/vespaceTemp/* /etc/kubernetes/manifests/
        sleep 5
    done

    Log::UnRegister "${FUNCNAME}"
}

configApiserver(){
    Log::Register "${FUNCNAME}"

    sed -i "39 i \    - --runtime-config=batch/v2alpha1=true" /etc/kubernetes/manifests/kube-apiserver.yaml
    sed -i "39 i \    - --runtime-config=batch/v1=true"       /etc/kubernetes/manifests/kube-apiserver.yaml

    Log::UnRegister "${FUNCNAME}"
}

main(){
    Log::Register "${FUNCNAME}"

    common::parsePlatform
    Utils::exitCode $?

    dependence::checkDeps
    Utils::exitCode $?

    preInit
    installSoft

    Log "start import ca function backend."
    ca_import &

    /bin/bash ${SCRIPTHOME}/../common/kubelet.sh
    Utils::exitCode $?

    init
    preUsing
    allowDeployPodOnMaster
    
    configApiserver

    Log::UnRegister "${FUNCNAME}"
}

Log::RegisterFile $0
Log::ParseIdentifier $@
main $*
Log::UnRegisterFile $0
