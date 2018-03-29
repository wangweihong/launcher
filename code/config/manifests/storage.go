package manifests

const (
	storageSh = `#!/bin/bash

PLATFORM="ubuntu"

parsePlatform(){
    ubuntuos=$(cat /etc/*release*  | grep -i ubuntu)
    centosos=$(cat /etc/*release*  | grep -i centos)

    if [ ${#ubuntuos} -gt 0 ];then
        PLATFORM="ubuntu"
    fi

    if [ ${#centosos} -gt 0 ];then
        PLATFORM="centos"
    fi
}

startVespaceStorageUbuntu(){
    storageName=storage
    docker rm -f ${storageName} 2>/dev/null || true
    docker run -d -m 1G --restart=always --net=host \
      --privileged=true \
      -v /var/local/ufleet/vespace/storage/lib:/var/lib/vespace \
      -v /var/local/ufleet/vespace/storage/log:/var/log/vespace \
      -v /dev/:/dev/ \
      -e etcdname={{ .EtcdName }} \
      -e etcd1={{ .Etcd1IP }} \
      -e etcd2={{ .Etcd2IP }} \
      -e etcd3={{ .Etcd3IP }} \
      -e manager_addr={{ .ManagerAddr }} \
      -e hostrootpasswd={{ .RootPasswd }} \
      --name ${storageName} \
      {{ .ImageStorageUbuntu }}
}

startVespaceStorageCentos(){
    storageName=storage
    docker rm -f ${storageName} 2>/dev/null || true
    docker run -d -m 1G --restart=always --net=host \
      --privileged=true \
      -v /var/local/ufleet/vespace/storage/lib:/var/lib/vespace \
      -v /var/local/ufleet/vespace/storage/log:/var/log/vespace \
      -v /dev/:/dev/ \
      -e etcdname={{ .EtcdName }} \
      -e etcd1={{ .Etcd1IP }} \
      -e etcd2={{ .Etcd2IP }} \
      -e etcd3={{ .Etcd3IP }} \
      -e manager_addr={{ .ManagerAddr }} \
      -e hostrootpasswd={{ .RootPasswd }} \
      --name ${storageName} \
      {{ .ImageStorageCentos }}
}

main(){
    # parse platform
    parsePlatform

    if [ "x${PLATFORM}" == "xubuntu" ];then
        startVespaceStorageUbuntu
    fi

    if [ "x${PLATFORM}" == "xcentos" ];then
        startVespaceStorageCentos
    fi
}
main
`
)
