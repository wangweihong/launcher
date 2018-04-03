#!/bin/bash

## ============= Log =============
LogMSG=""

## ============= Utils ===========

Log::Register(){
    funcName=$1
    [[ ${#funcName} -eq 0 ]] && return 1
    LogMSG="${LogMSG}[ $funcName ]"
}

Log::UnRegister(){
    funcName=$1
    [[ ${#funcName} -eq 0 ]] && return 1

    LogMSG=$(echo "$LogMSG" | sed "s/\(.*\)\[ ${funcName} \]/\1/")
}

Log::RegisterFile(){
    fileName=$1
    [[ ${#fileName} -eq 0 ]] && return 1

    LogMSG="${LogMSG}[ $fileName ]"
}

Log::UnRegisterFile(){
    fileName=$1
    [[ ${#funcName} -eq 0 ]] && return 1

    LogMSG=$(echo "$LogMSG" | sed "s#\(.*\)\[ ${fileName} \]#\1#")
}

Log::Identifier(){
    itf=$1
    [[ ${#itf} -eq 0 ]] && return 1

    LogMSG="[ $itf ]${LogMSG}"
}

Log(){
    msg=$1
    echo -e "[ `date` ]${LogMSG} $msg"
}

Log::ParseIdentifier(){
    identifier=""

    IFS_BACKUP="${IFS}"
    IFS=' '
    read -r -a params <<< "$@"
    len=${#params[@]}
    for((i=0;i<$len;i++))
    do
        param=$(echo "${params[$i]}" | tr '[:lower:]' '[:upper:]')
        case ${param} in
            "-LOGID")
                i=`expr $i + 1`
                identifier=${params[$i]};;
        esac
    done
    IFS="${IFS_BACKUP}"

    [[ ${#identifier} -gt 0 ]] && Log::Identifier "${identifier}"
}

## ============= Utils =============
Utils::exitCode(){
    code=$1
    if [ $code -gt 0 ];then
        Log "Failed, exit soon.[ExitCode: $code]"
        exit $code
    fi
    Log "OK."
}

## ============= Docker =============
#
# For compatibility, k8s release version alreay has tar package,
#   so, generate tar image first and then zip it.
#
Docker::saveTgz(){
    Log::Register "$FUNCNAME"

    image=$1
    tag=$2
    destDir=$3

    #check param
    [[ ${#@} -lt 2 ]] && Log "Wrong param numberi.\n\n  Usage: %0 <image> <tag> [dest dir]" && return 1
    [[ ${#destDir} -eq 0 ]] && destDir=$(pwd)
    [[ ! -e $destDir ]] && Log "$destDir isn't there, create $destDir" && mkdir -p $destDir

    if ! command -v tar > /dev/null 2>&1; then
        Log "Failed, tar isn't install."
        return 1
    fi

    image2=$(echo $image | sed 's/\//-/g')
    image2=$(echo ${image2} | sed 's/:/-/g') # for compatibility, some system's tar not support ':', it will cause failed.
    tempDir=`date +%s`
    rm -rf /tmp/${tempDir}
    mkdir -p /tmp/${tempDir}

    Log "docker save ${image}:${tag} -o /tmp/${tempDir}/${image2}-${tag}.tar"
    docker save ${image}:${tag} -o /tmp/${tempDir}/${image2}-${tag}.tar
    [[ $? -gt 0 ]] && Log "failed." && return 4

    WORKDIR=$(pwd)
    cd /tmp/${tempDir}/

    Log "tar -czf ${image2}-${tag}.tar.gz ${image2}-${tag}.tar in /tmp/${tempDir}"
    tar -czf ${image2}-${tag}.tar.gz ${image2}-${tag}.tar
    if [ $? -eq 0 ];then
        Log "move /tmp/${tempDir}/${image2}-${tag}.tar.gz to $destDir"
        mv ${image2}-${tag}.tar.gz $destDir/
    else
        Log "Failed, please check."
    fi

    cd $WORKDIR
    rm -rf /tmp/${tempDir}

    Log::UnRegister "$FUNCNAME"
}

#
# For compatibility, k8s release version alreay has tar package,
#   so, generate tar image first and then zip it.
#
Docker::loadTgz(){
    Log::Register "$FUNCNAME"

    fileName=$1

    # check command deps
    if ! command -v tar > /dev/null 2>&1; then
        Log "Failed, tar isn't install."
        return 1
    fi

    # check file name
    [[ ${#fileName} -eq 0 ]] && Log "param number wrong, \n\n  Usage: $0 <fileName>" && return 2
    tgz=$(echo $fileName | grep ".tgz$")
    tarGz=$(echo $fileName | grep ".tar.gz$")
    [[ ${#tgz} -eq 0 ]] && [[ ${#tarGz} -eq 0 ]] && Log "file: [$fileName] should end with .tar.gz or .tgz" && return 3

    tempDir=`date +%s`
    rm -rf /tmp/${tempDir}
    mkdir -p /tmp/${tempDir}
    tar -xzf $fileName -C /tmp/${tempDir}/
    [[ $? -gt 0 ]] && Log "tar -xzf file $fileName failed" && return 4

    find /tmp/${tempDir} -type f -exec docker load -i "{}" \;
    [[ $? -gt 0 ]] && Log "docker load file failed." && return 5
    rm -rf /tmp/${tempDir}

    Log::UnRegister "$FUNCNAME"
}

Utils::synctime() {
    Log::Register "$FUNCNAME"

    Log "use ntp to keep time sync in the future"
    docker run -d --privileged=true --restart=always --add-host=ntpd.youruncloud.com:${NTPD_HOST} --name=ufleet-ntp -v /etc/localtime:/etc/localtime ufleet.io/ufleet/ntp:v1.5.0.3 || true

    Log "waitting until ntpd ready. max=3 minutes"
    for ((i=0;i<180;i++))
    do
        Log "$i times, waiting for clock sync ..."
        ready1=$(docker logs ufleet-ntp 2>&1 | grep 'offset 0.')
        ready2=$(docker logs ufleet-ntp 2>&1 | grep 'offset -0.')
        [[ ${#ready1} -gt 0 || ${#ready2} -gt 0 ]] && Log "clock sync." && break
        sleep 1
    done

    Log "restart ntp container for time sync with system."
    docker restart ufleet-ntp || true
    docker restart ufleet-ntp || true

    Log::UnRegister "$FUNCNAME"
}

Utils::uni_synctime() {
    Log::Register "$FUNCNAME"

    Log "remove synctime container. "
    docker rm -f  ufleet-ntp || true

    Log::UnRegister "$FUNCNAME"
}


common::mountshare() {
    Log::Register "${FUNCNAME}"

    Log "make /var/lib/kubelet mount as shared ..."
    mkdir -p /var/lib/kubelet

    # remove older config
    sed -i '/mount --bind \/var\/lib\/kubelet \/var\/lib\/kubelet/d' /etc/rc.local
    sed -i '/mount --make-shared \/var\/lib\/kubelet/d' /etc/rc.local

    # update config
    sed -i "2 i mount --bind /var/lib/kubelet /var/lib/kubelet" /etc/rc.local
    sed -i "3 i mount --make-shared /var/lib/kubelet" /etc/rc.local

    # mount
    mount --bind /var/lib/kubelet /var/lib/kubelet
    mount --make-shared /var/lib/kubelet

    Log::UnRegister "${FUNCNAME}"
}

common::setFirewall(){
    Log::Register "${FUNCNAME}"

    Log "setenforce 0"
    setenforce 0 2>/dev/null || true
    # ubuntu
    command -v ufw >/dev/null 2>&1
    [[ $? -eq 0 ]] && service ufw stop || true
    # centos
    command -v systemctl > /dev/null 2>&1
    if [ $? -eq 0 ];then
        systemctl disable firewalld || true
        systemctl stop firewalld || true
    fi

    Log::UnRegister "${FUNCNAME}"
}

common::updateIptablesConfig(){
    Log::Register "${FUNCNAME}"

    # remove older config
    sed -i '/net.bridge.bridge-nf-call-iptables/d' /etc/sysctl.conf
    sed -i '/net.bridge.bridge-nf-call-ip6tables/d' /etc/sysctl.conf

    # update config
    blankLine=$(tail -n 1 /etc/sysctl.conf)
    [ ${#blankLine} -gt 0 ] && echo "" >> /etc/sysctl.conf
    echo "net.bridge.bridge-nf-call-iptables=1" >> /etc/sysctl.conf
    echo "net.bridge.bridge-nf-call-ip6tables=1" >> /etc/sysctl.conf

    /sbin/sysctl -p
    /sbin/sysctl --system

    # for docker version > 1.13
    [ ! -e /etc/rc.local ] && echo '#!/bin/bash' >> /etc/rc.local && echo "" >> /etc/rc.local && chmod a+rx /etc/rc.local
    sed -i '/iptables -P FORWARD ACCEPT/d'  /etc/rc.local
    sed -i "2 i iptables -P FORWARD ACCEPT" /etc/rc.local

    iptables -P FORWARD ACCEPT

    Log::UnRegister "${FUNCNAME}"
}

common::updateDockerConfig(){
    Log::Register "${FUNCNAME}"

    slaveFlags=$(cat /lib/systemd/system/docker.service| grep -i "MountFlags=slave")
    [ ${#slaveFlags} -eq 0 ] && return

    sed -i "s/MountFlags=slave/MountFlags=/g" /lib/systemd/system/docker.service
    command -v systemctl &> /dev/null
    if [ $? -eq 0 ];then
        systemctl daemon-reload || true
        systemctl restart -f docker || true
    else
        service docker restart || true
    fi

    # wait for docker ready
    for((i=0;i<180;i++))
    do
        content=$(docker ps -a 2> /dev/null)
        [ ${#content} -gt 0 ] && break
    done

    Log::UnRegister "${FUNCNAME}"
}

# ========== Platform Parse=============
PLATFORM=""
common::parsePlatform(){
   Log::Register "${FUNCNAME}"

   ubuntuos=$(cat /etc/*release*  | grep -i ubuntu)
   debianos=$(cat /etc/*release*  | grep -i debian)
   centosos=$(cat /etc/*release*  | grep -i centos)
   redhatos=$(cat /etc/*release*  | grep -i redhat)

   if [[ ${#ubuntuos} -gt 0 || ${#debianos} -gt 0 ]];then
       PLATFORM="ubuntu"
   fi

   if [[ ${#centosos} -gt 0 || ${#redhatos} -gt 0 ]];then
       PLATFORM="centos"
   fi

   if [ ${#PLATFORM} -eq 0 ];then
       Log "Unkown system. just support ubuntu, debian, centos, redhat" && Utils::exitCode 1
   fi

   Log::UnRegister "${FUNCNAME}"
}