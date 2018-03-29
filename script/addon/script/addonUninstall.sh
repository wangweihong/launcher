#!/bin/bash

SCRIPTHOME=$(cd `dirname $0`;pwd)
CONFDIR=$(cd ${SCRIPTHOME}/../conf;pwd)

. $SCRIPTHOME/../../common/utils.sh

stopProxy(){
    Log::Register "${FUNCNAME}"

    proxyProcess=$(ps -ef | grep -v grep | grep "\/proxy\/bin\/proxy")
    [[ ${#proxyProcess} -gt 0 ]] && echo ${proxyProcess} | awk '{print $2}' | xargs kill -9 || true

    Log::UnRegister "${FUNCNAME}"
}
removeAddon(){
    Log::Register "${FUNCNAME}"

    Log "remove addon"

    # check
    [[ ! -e $CONFDIR ]] && Log "$CONFDIR/addon not exist, return soon." && return

    cd $CONFDIR
    find . -type f -exec kubectl delete -f "{}" \;

    Log::UnRegister "${FUNCNAME}"
}

cleanEnv(){
    Log::Register "${FUNCNAME}"

    rm -rf /var/log/vespace
    rm -rf /var/lib/vespace

    rm -rf /var/local/ufleet

    Log::UnRegister "${FUNCNAME}"
}

main(){
    Log::Register "${FUNCNAME}"

    stopProxy
    removeAddon
    cleanEnv

    Log::UnRegister "${FUNCNAME}"
}

Log::RegisterFile $0
Log::ParseIdentifier $@
main
Log::UnRegisterFile $0
