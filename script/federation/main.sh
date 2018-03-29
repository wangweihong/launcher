#!/bin/bash

HOMESCRIPT=$(cd `dirname $0`; pwd)

. ${HOMESCRIPT}/../common/utils.sh
. ${HOMESCRIPT}/kubernetes.conf
. ${HOMESCRIPT}/coredns_installer.sh
. ${HOMESCRIPT}/fedinstaller.sh

main(){
    Log "go in script home"
    cd ${HOMESCRIPT}
    Utils::exitCode $?

    Log "install files"
    coredns::install_files
    Utils::exitCode $?

    Log "add cluster admin privilege to default"
    coredns::add_admin_to_default
    Utils::exitCode $?

    Log "create serviceaccount: ${serviceaccount_helm}"
    coredns::create_serviceaccount
    Utils::exitCode $?

    Log "create rolebinding"
    coredns::create_rolebinding
    Utils::exitCode $?

    Log "use aliyun url instread helm default"
    coredns::use_aliyun_url
    Utils::exitCode $?

    Log "init helm server in k8s"
    coredns::init_helm_server
    Utils::exitCode $?

    Log "waiting for tiller(helm server) ready"
    coredns::waiting_for_tiller_ready
    Utils::exitCode $?

    Log "install core dns"
    coredns::install_coredns
    Utils::exitCode $?

    Log "ready to install federation"
    fedinstaller::Install
    Utils::exitCode $?

    Log "Done"
}
Log::RegisterFile $0
Log::ParseIdentifier $@
main $*
Log::UnRegisterFile $0