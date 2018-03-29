#!/usr/bin/env bash

SCRIPTHOME=$(cd `dirname $0`;pwd)
PROJECTHOME=$(cd ${SCRIPTHOME}/../..;pwd)

# modify by function
k8s_last_version=""
k8s_new_version=""

update::exitCode(){
    code=$1
    if [ $code -gt 0 ];then
        echo "Failed, exit soon.[ExitCode: $code]"
        exit $code
    fi
    echo "OK."
}

update::setProxy(){
    export https_proxy=http://127.0.0.1:9080
    export http_proxy=http://127.0.0.1:9080
}

update::getLastVersion(){
    k8s_last_version=$(cat ${PROJECTHOME}/config/kubernetes.conf  | grep latest_version | sed 's/latest_version=//g' | sed 's/\n//g')
}

# TODO. 需要添加判断版本号是否合法
update::getNewVersion(){
    url="https://github.com/kubernetes/kubernetes/releases"
    versionPrefix="v1.9"
    k8s_new_version=$(curl -X GET ${url} | grep 'releases/tag' | grep -v beta | grep -v alpha | tr '\>' ' ' | tr '\<' ' ' | awk '{print $3}' | grep ${versionPrefix} | head -n 1 | sed 's/\n//g')
    update::exitCode $?
}

update::downloadNewVersion(){
    destDir=$1
    url="https://github.com/kubernetes/kubernetes/releases/download/${k8s_new_version}/kubernetes.tar.gz"

    if [ ! -e ${destDir} ];then
        mkdir -p ${destDir}
    fi
    cd ${destDir}

    wget ${url}
}

update::downloadNewImages(){
    destDir=$1

    # 进入最新版本所在目录
    cd ${destDir}

    # 解压缩
    tar -xzf kubernetes.tar.gz

    # 下载最新镜像
    export KUBERNETES_SERVER_ARCH=amd64
    export KUBERNETES_SKIP_CONFIRM=1
    unset KUBERNETES_DOWNLOAD_TESTS
    cd kubernetes
    /bin/bash cluster/get-kube-binaries.sh
    update::exitCode $?
}

update::updateImages(){
    destDir=$1
    prefix="192.168.18.250:5002/launcher"

    # 进入镜像所在目录
    ## 进入到server.tar.gz 所在目录
    cd ${destDir}/kubernetes/server
    ## 解压压缩文件
    tar -xzf kubernetes-server-linux-amd64.tar.gz
    update::exitCode $?
    ## 进入解压后的目录
    cd ${destDir}/kubernetes/server/kubernetes/server/bin

    # 加载所有镜像
    for file in *.tar
    do
        docker load -i ${file}
        update::exitCode $?
    done

    # 重新打tag并上传镜像
    for image in `docker images | grep ${k8s_new_version} | grep -v "${prefix}" | grep -v index.youruncloud.com | grep -v "192.168.18.250:5002" | awk '{print $1}'`
    do
        # 重新打tag
        docker tag ${image}:${k8s_new_version} ${prefix}/${image}-amd64:${k8s_new_version}
        update::exitCode $?

        # 上传镜像
        docker push ${prefix}/${image}-amd64:${k8s_new_version}
        update::exitCode $?
    done
}

update::updateKubelet(){
    destDir=$1

    # 拷贝二进制文件
    rm -rf ${PROJECTHOME}/build/kubelet/bin
    mkdir -p ${PROJECTHOME}/build/kubelet/bin
    cp ${destDir}/kubernetes/server/kubernetes/server/bin/kubelet ${PROJECTHOME}/build/kubelet/bin

    # 修改镜像版本
    sed -i "s/^IMAGE_TAG=.*/IMAGE_TAG=\"192.168.18.250:5002\/launcher\/kubelet:${k8s_new_version}\"/g" ${PROJECTHOME}/build/kubelet/build.sh
    cd ${PROJECTHOME}
    git add ${PROJECTHOME}/build/kubelet/build.sh

    # 编译
    cd ${PROJECTHOME}/build/kubelet
    /bin/bash build.sh
    update::exitCode $?

    # 上传镜像
    docker push 192.168.18.250:5002/launcher/kubelet:${k8s_new_version}
    update::exitCode $?
}

update::updateHyperkube(){
    destDir=$1

    # 拷贝二进制文件
    rm -rf ${PROJECTHOME}/build/hyperkube/bin
    mkdir -p ${PROJECTHOME}/build/hyperkube/bin
    cp ${destDir}/kubernetes/server/kubernetes/server/bin/hyperkube ${PROJECTHOME}/build/hyperkube/bin

    # 设置版本号
    export VERSION=${k8s_new_version}

    # 编译并上传镜像
    cd ${PROJECTHOME}/build/hyperkube
    /bin/bash build.sh
    update::exitCode $?
}

update::updateBin(){
    destDir=$1

    # 拷贝二进制文件
    mkdir -p ${PROJECTHOME}/build/deps-bin
    mkdir -p ${PROJECTHOME}/build/deps-bin/kubeadm/usr/local/bin
    cp -f ${destDir}/kubernetes/server/kubernetes/server/bin/kubeadm ${PROJECTHOME}/build/deps-bin/kubeadm/usr/local/bin/kubeadm
    update::exitCode $?
    mkdir -p ${PROJECTHOME}/build/deps-bin/kubectl/usr/local/bin
    cp -f ${destDir}/kubernetes/server/kubernetes/server/bin/kubectl ${PROJECTHOME}/build/deps-bin/kubectl/usr/local/bin/kubectl
    update::exitCode $?

    # 设置版本号
    export VERSION=${k8s_new_version}

    # 编译并上传镜像
    cd ${PROJECTHOME}/build/deps-bin
    /bin/bash build.sh
    update::exitCode $?

    # 修改版本号
    # TODO. 注意，这里可能会修改到其他非kubernetes镜像的版本，待改进
    sed -i "s/${k8s_last_version}/${k8s_new_version}/g" ${PROJECTHOME}/config/kubernetes.conf
    sed -i "s/${k8s_last_version}/${k8s_new_version}/g" ${PROJECTHOME}/Dockerfile

    # 添加到git，或者提交修改到git仓库
    git add ${PROJECTHOME}/config/kubernetes.conf
    git add ${PROJECTHOME}/Dockerfile

    # 上传镜像
    docker push 192.168.18.250:5002/launcher/k8s-deps-bin:${k8s_new_version}
    update::exitCode $?
}

update::updateCode(){
    # 上传修改
    git config --global user.name "miancai.li"
    git config --global user.email "miancai.li@yourongcloud.com"
    git commit -m "update-robot: update kubernetes to ${k8s_new_version}"
    git push origin HEAD:refs/heads/v1.9.0
}

main(){
    # 新kubernetes版本存放的临时目录
    newVersionDir=/tmp/newKubernetes/temp
    rm -rf ${newVersionDir}

    # 配置代理
    update::setProxy

    # 获取当前版本号
    update::getLastVersion

    # 获取最新版本号
    update::getNewVersion

    # 判断是否需要更新版本
    [[ ${#k8s_last_version} -eq 0 ]] && echo "get last version failed" && exit 1
    [[ ${#k8s_new_version} -eq 0 ]] && echo "get new version failed" && exit 1
    [[ ${k8s_last_version} == ${k8s_new_version} ]] && echo "no need to update, now already update to new version." && exit 0

    # 下载最新版本源码
    update::downloadNewVersion ${newVersionDir}

    # 下载镜像
    update::downloadNewImages ${newVersionDir}

    # 上传最新镜像
    update::updateImages ${newVersionDir}

    # 拷贝kubelet二进制文件并编译打包，上传镜像
    update::updateKubelet ${newVersionDir}

    # 拷贝hyperkube二进制文件并编译打包，上传镜像
    update::updateHyperkube ${newVersionDir}

    # 拷贝二进制文件，修改源码，配置文件，添加并上传修改
    update::updateBin ${newVersionDir}

    # 上传代码
    update::updateCode

    # 删除临时目录
    rm -rf ${newVersionDir}
}
main
