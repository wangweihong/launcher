docker run -it --rm \
    --net=host \
    --pid=host \
    --privileged \
    -v /sys:/sys:ro \
    -v /var/run:/var/run:rw \
    -v /var/lib/docker/:/var/lib/docker:rw \
    -v /var/lib/kubelet/:/var/lib/kubelet:shared \
    -v /var/lib/etcd/:/var/lib/etcd:rw \
    -v /var/lib/cni/:/var/lib/cni:rw \
    -v /etc/cni/net.d/:/etc/cni/net.d/:rw \
    -v /opt/cni/bin/:/opt/cni/bin/:rw \
    -v /etc/kubernetes/:/etc/kubernetes/:rw \
    192.168.18.250:5002/launcher/kubelet:v1.9.3 \
    nsenter --target=1 --mount --wd=. -- ./usr/bin/kubelet \
      --kubeconfig=/etc/kubernetes/kubelet.conf \
      --require-kubeconfig=true\
      --pod-manifest-path=/etc/kubernetes/manifests \
      --pod-infra-container-image=k8s/pause:v3.0 \
      --allow-privileged=true \
      --network-plugin=cni \
      --cni-conf-dir=/etc/cni/net.d \
      --cni-bin-dir=/opt/cni/bin \
      --cluster-dns=10.96.0.10 \
      --cluster-domain=cluster.local 
