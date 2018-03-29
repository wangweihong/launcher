#!/usr/bin/env bash

tmpFile=/tmp/configmap
tmpFiles=/tmp/configmaps

server="https://127.0.0.1:6443"
advertiseAddress="127.0.0.1"

getAllConfigMaps(){
    # get all configmaps
    kubectl get configmap --all-namespaces | grep -v NAMESPACE | awk '{print $2" -o yaml -n "$1}' > $tmpFiles
}

getAndModifyConfigMaps(){
    while read line
    do
        # TODO. for security, should check the content of script
        eval "kubectl get configmap $line > $tmpFile"
        eval "sed -i 's/server:.*/server: $server/g' $tmpFile"
        eval "sed -i 's/advertiseAddress:.*/advertiseAddress: $advertiseAddress/g' $tmpFile"
        kubectl apply -f $tmpFile
    done < $tmpFiles
}

main(){
    server=$1
    advertiseAddress=$2
    [ ${#server} -eq 0 ] && exit 1
    [ ${#advertiseAddress} -eq 0 ] && exit 1
    echo "server: $server, advertiseAddress: $advertiseAddress"

    # get configmaps
    getAllConfigMaps

    # modify configmaps
    getAndModifyConfigMaps
}
main $@

exit 0