package manifests

const (
	kubeletConf = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{ .CaCert }}
    server: https://{{ .Hostip }}:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubelet-csr
  name: kubelet-csr
- context:
    cluster: kubernetes
    user: tls-bootstrap-token-user
  name: tls-bootstrap-token-user@kubernetes
current-context: kubelet-csr
kind: Config
preferences: {}
users:
- name: kubelet-csr
  user:
    client-certificate-data: {{ .ApiserverKubeletClientCert }}
    client-key-data: {{ .ApiserverKubeletClientKey }}
- name: tls-bootstrap-token-user
  user:
    token: {{ .JoinToken }}`

    kubeletSh = `#!/bin/bash

cgroupdriver=$(docker info 2>/dev/null| grep "Cgroup Driver" | awk '{print $3}' 2>/dev/null)
[ ${#cgroupdriver} -eq 0 ] &&  echo "get failed, set to default systemd." && cgroupdriver="systemd"

kubeletName=k8s-kubelet
mkdir -p /etc/kubernetes/manifests
docker rm -f ${kubeletName} 2>/dev/null || true
docker run -m 1G -c 5120 -d \
    --name=${kubeletName} \
    --restart=always \
    --net=host \
    --pid=host \
    --privileged \
    --oom-score-adj=-900 \
    -v /sys:/sys:ro \
    -v /var/run:/var/run:rw \
    -v /var/lib/docker/:/var/lib/docker:rw \
    -v /var/lib/kubelet/:/var/lib/kubelet:rw \
    -v /var/lib/etcd/:/var/lib/etcd:rw \
    -v /var/lib/cni/:/var/lib/cni:rw \
    -v /etc/cni/net.d/:/etc/cni/net.d/:rw \
    -v /opt/cni/bin/:/opt/cni/bin/:rw \
    -v /etc/kubernetes/:/etc/kubernetes/:rw \
    {{ .ImageKubelet }} \
    nsenter --target=1 --mount --wd=. -- ./usr/bin/kubelet \
        --kubeconfig=/etc/kubernetes/kubelet.conf \
        --cloud-provider="" \
        --pod-manifest-path=/etc/kubernetes/manifests \
        --pod-infra-container-image=ufleet.io/google_containers/pause-amd64:3.0 \
        --tls-cert-file=/etc/kubernetes/pki/apiserver-kubelet-client.crt \
        --tls-private-key-file=/etc/kubernetes/pki/apiserver-kubelet-client.key \
        --allow-privileged=true \
        --network-plugin=cni \
        --cni-conf-dir=/etc/cni/net.d \
        --cni-bin-dir=/opt/cni/bin \
        --cluster-dns=10.96.0.10 \
        --cluster-domain=cluster.local \
        --cgroup-driver=${cgroupdriver} \
        --cgroup-root=/ \
        --fail-swap-on=false \
        --register-node=true \
        --node-ip={{ .Hostip }} \
        --hostname-override={{ .Hostname }}`
)
