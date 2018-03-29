#!/bin/bash

SCRIPTHOME=$(cd `dirname $0`;pwd)
CONFDIR=${SCRIPTHOME}/../conf

. $SCRIPTHOME/../../common/utils.sh

#params
MASTERIP=""

parseParams(){
    Log::Register "${FUNCNAME}"

    IFS_BACKUP="${IFS}"
    IFS=' '
    read -r -a params <<< "$@"
    len=${#params[@]}
    for((i=0;i<$len;i++))
    do
        case ${params[$i]} in
            "-m")
                i=`expr $i + 1`
                MASTERIP=${params[$i]};;
        esac
    done
    IFS="${IFS_BACKUP}"

    Log::UnRegister "${FUNCNAME}"
}

showUsage(){
    cat << END

	Usage: $0 -m <MasterIp>
	Like: $0 -m 192.168.0.102

END
}

installAddon(){
    Log::Register "${FUNCNAME}"

    # check
    Log "install addon"
    [[ ${#MASTERIP} -eq 0 ]] && showUsage && Utils::exitCode 1

    # in ha cluster mode, slave machine don't have $CONFDIR/addon
    [[ ! -d $CONFDIR ]] && Log "$CONFDIR/addon not exist, return soon." && return

    # setting
    content=$(ls $CONFDIR/*.yaml 2>/dev/null| wc -l)
    [ $content -eq 0 ] && Log "There isn't any file in $CONFDIR, will go out soon." && return
    
    cd $CONFDIR
    Log "$(find . -type f -exec sed -i "s/MASTERIP/${MASTERIP}/g" "{}" \;)"
    find . -type f -exec sed -i "s/MASTERIP/${MASTERIP}/g" "{}" \;

    # install
    errMsg=""
    for file in *.yaml
    do
        for ((i=0;i<30;i++))
        do
            kubectl apply -f $file
            [ $? -eq 0 ] && errMsg="" && break

            Log "kubectl apply -f $file failed! try $i time.."
            errMsg="kubectl apply -f $file failed!"
            sleep 2
        done
    done
    if [ ${#errMsg} -gt 0 ];then
        Log "$errMsg"
        Utils::exitCode 1
    fi

    Log::UnRegister "${FUNCNAME}"
}

main(){
    Log::Register "${FUNCNAME}"

    parseParams $@
    installAddon

    Log::UnRegister "${FUNCNAME}"
}

Log::RegisterFile $0
Log::ParseIdentifier $@
main $@
Log::UnRegisterFile $0
