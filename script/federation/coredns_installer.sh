#!/bin/bash
HOMESCRIPT=$(cd `dirname $0`;pwd)
SOFTWARE_DIR=$(cd ${HOMESCRIPT}/../../common; pwd)

coredns::install_files(){
    Log::Register "${FUNCNAME}"

    [ ! -e ${SOFTWARE_DIR}/federation ] && echo "${SOFTWARE_DIR}/federation not exist" && exit 1

    cp -f ${SOFTWARE_DIR}/federation/helm/usr/local/bin/helm       /usr/local/bin/
    Utils::exitCode $?
    cp -f ${SOFTWARE_DIR}/federation/kubefed/usr/local/bin/kubefed /usr/local/bin/
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

coredns::add_admin_to_default(){
    Log::Register "${FUNCNAME}"

    kubectl create clusterrolebinding add-on-cluster-admin --clusterrole=cluster-admin --serviceaccount=kube-system:default

    Log::UnRegister "${FUNCNAME}"
}

coredns::create_serviceaccount(){
    Log::Register "${FUNCNAME}"

    kubectl create serviceaccount ${serviceaccount_helm} -n kube-system
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

coredns::create_rolebinding(){
    Log::Register "${FUNCNAME}"

    kubectl create -f role/rolebinding.yaml
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

coredns::use_aliyun_url(){
    Log::Register "${FUNCNAME}"

    helm init --upgrade -i registry.cn-hangzhou.aliyuncs.com/google_containers/tiller:v2.6.1 --stable-repo-url https://kubernetes.oss-cn-hangzhou.aliyuncs.com/charts
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

coredns::init_helm_server(){
    Log::Register "${FUNCNAME}"

    helm init --service-account ${serviceaccount_helm} --tiller-image=${image_helm_tiller} --kube-context ${context_admin}
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

coredns::install_coredns(){
    Log::Register "${FUNCNAME}"

    helm install --namespace kube-system --name coredns -f coredns/values.yaml stable/coredns
    Utils::exitCode $?

    Log::UnRegister "${FUNCNAME}"
}

coredns::waiting_for_tiller_ready(){
    Log::Register "${FUNCNAME}"

    for ((i=0;i<600;i++)) # waiting 10 minutes.
    do
        status=$(kubectl get pods -n kube-system | grep tiller | awk '{print $3}')
        if [ "x${status}" == "xRunning" ];then
            sleep 20
            return
        fi
        sleep 1
     done

     Log::UnRegister "${FUNCNAME}"
}
