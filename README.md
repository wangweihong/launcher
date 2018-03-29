# auto deploy tool of Kubernetes

base on K8s v1.8+, and support create k8s cluster from restful request

## dependence on

* CentOS7+，Ubuntu14.04+ (and other Unix-like system)
* Docker v1.12.6+

## main features
### create cluster
```bash
curl -X POST -H 'Content-Type: application/json' -d <body> http://192.168.0.171:8886/clusters/new
```

body is like this:
```json
{
	"name": "iSingle",
	"masters": [{
		"hostip": "192.168.4.109",
		"username": "root",
		"userpwd": "Cs123456",
		"clustername": "iSingle",
		"podnetwork": "10.244.0.0/16"
	}, {
		"hostip": "192.168.4.106",
		"username": "root",
		"userpwd": "Cs123456",
		"clustername": "iSingle",
		"podnetwork": "10.244.0.0/16"
	}, {
		"hostip": "192.168.4.107",
		"username": "root",
		"userpwd": "Cs123456",
		"clustername": "iSingle",
		"podnetwork": "10.244.0.0/16"
	}],
	"images": [{
            "id": "calico_cni",
            "name": "calico-cni",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682569395,
            "description": "common",
            "package": "kubernetes1881",
            "size": 20732326,
            "path": "calico/cni:v1.10.0"
          }, {
            "id": "calico_kube_policy_controller",
            "name": "calico-kube-policy-controller",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682569482,
            "description": "common",
            "package": "kubernetes1881",
            "size": 19306100,
            "path": "calico/kube-policy-controller:v0.7.0"
          }, {
            "id": "calico_node",
            "name": "calico-node",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682616232,
            "description": "common",
            "package": "kubernetes1881",
            "size": 70111581,
            "path": "calico/node:v2.5.1"
          }, {
            "id": "coredns",
            "name": "coredns",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682569494,
            "description": "common",
            "package": "kubernetes1881",
            "size": 14929532,
            "path": "coredns/coredns:0.9.10"
          }, {
            "id": "etcd_amd64",
            "name": "etcd-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682596350,
            "description": "common",
            "package": "kubernetes1881",
            "size": 45138461,
            "path": "google_containers/etcd-amd64:3.0.17"
          }, {
            "id": "external_dns",
            "name": "external-dns",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682636616,
            "description": "common",
            "package": "kubernetes1881",
            "size": 58080699,
            "path": "launcher/external-dns:v1.0.0"
          }, {
            "id": "jnlp_slave",
            "name": "jnlp-slave",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682636621,
            "description": "Jenkins work node",
            "package": "kubernetes1881",
            "size": 91229600,
            "path": "ufleet/jnlp-slave:v1.3.2"
          }, {
            "id": "k8s-deps-bin",
            "name": "k8s-deps-bin",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682688182,
            "description": "binary files which k8s dependence on",
            "package": "kubernetes1881",
            "size": 69966523,
            "path": "launcher/k8s-deps-bin:v1.8.8"
          }, {
            "id": "k8s-deps-tool",
            "name": "k8s-deps-tool",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682666327,
            "description": "tool files which k8s dependence on",
            "package": "kubernetes1881",
            "size": 35763216,
            "path": "launcher/k8s-deps-tool:v1.0.0"
          }, {
            "id": "k8s_dns_dnsmasq_nanny_amd64",
            "name": "k8s-dns-dnsmasq-nanny-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682647078,
            "description": "common",
            "package": "kubernetes1881",
            "size": 11302699,
            "path": "google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.5"
          }, {
            "id": "k8s_dns_sidecar_amd64",
            "name": "k8s-dns-sidecar-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682666365,
            "description": "common",
            "package": "kubernetes1881",
            "size": 11322733,
            "path": "google_containers/k8s-dns-sidecar-amd64:1.14.5"
          }, {
            "id": "keepalived",
            "name": "keepalived",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682776264,
            "description": "common",
            "package": "kubernetes1881",
            "size": 131680270,
            "path": "launcher/keepalived:v1.0.3"
          }, {
            "id": "kube_apiserver_amd64",
            "name": "kube-apiserver-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682706538,
            "description": "common",
            "package": "kubernetes1881",
            "size": 29820722,
            "path": "google_containers/kube-apiserver-amd64:v1.8.8"
          }, {
            "id": "kube_controller_manager_amd64",
            "name": "kube-controller-manager-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682716357,
            "description": "common",
            "package": "kubernetes1881",
            "size": 25927335,
            "path": "google_containers/kube-controller-manager-amd64:v1.8.8"
          }, {
            "id": "kube_dns_amd64",
            "name": "k8s-dns-kube-dns-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682716361,
            "description": "common",
            "package": "kubernetes1881",
            "size": 13093307,
            "path": "google_containers/k8s-dns-kube-dns-amd64:1.14.5"
          }, {
            "id": "kube_proxy_amd64",
            "name": "kube-proxy-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682727221,
            "description": "common",
            "package": "kubernetes1881",
            "size": 28831184,
            "path": "google_containers/kube-proxy-amd64:v1.8.8"
          }, {
            "id": "kube_scheduler_amd64",
            "name": "kube-scheduler-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682726654,
            "description": "common",
            "package": "kubernetes1881",
            "size": 13049530,
            "path": "google_containers/kube-scheduler-amd64:v1.8.8"
          }, {
            "id": "kubelet",
            "name": "kubelet",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682796318,
            "description": "common",
            "package": "kubernetes1881",
            "size": 100820419,
            "path": "launcher/kubelet:v1.8.8"
          }, {
            "id": "nfs_client_provisioner",
            "name": "nfs-client-provisioner",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682750099,
            "description": "Storage class extension",
            "package": "kubernetes1881",
            "size": 12594067,
            "path": "external_storage/nfs-client-provisioner:v1.0.0"
          }, {
            "id": "ntp",
            "name": "ntp",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682750256,
            "description": "时间同步服务",
            "package": "kubernetes1881",
            "size": 3129726,
            "path": "ufleet/ntp:v1.5.0.3"
          }, {
            "id": "pause_amd64",
            "name": "pause-amd64",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682760076,
            "description": "common",
            "package": "kubernetes1881",
            "size": 312851,
            "path": "google_containers/pause-amd64:3.0"
          }, {
            "id": "prometheus_node_exporter",
            "name": "prometheus-node-exporter",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682776272,
            "description": "monitor",
            "package": "kubernetes1881",
            "size": 6781755,
            "path": "prometheus/node-exporter:v1.0.0"
          }, {
            "id": "scavenger",
            "name": "scavenger",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682776279,
            "description": "common",
            "package": "kubernetes1881",
            "size": 3092193,
            "path": "launcher/scavenger:v2.0.3"
          }, {
            "id": "storage_centos",
            "name": "vespace storage centos",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682883292,
            "description": "vespace strategy",
            "package": "kubernetes1881",
            "size": 200943566,
            "path": "vespace/storage-centos:v5.4.0"
          }, {
            "id": "storage_ubuntu",
            "name": "vespace storage ubuntu",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682960153,
            "description": "vespace strategy",
            "package": "kubernetes1881",
            "size": 204714422,
            "path": "vespace/storage-ubuntu:v5.2.0"
          }, {
            "id": "traefik",
            "name": "traefik",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682850338,
            "description": "common",
            "package": "kubernetes1881",
            "size": 13028701,
            "path": "launcher/traefik:v1.4.2"
          }, {
            "id": "vespace_ha_strategy",
            "name": "vespace strategy",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682993279,
            "description": "vespace strategy",
            "package": "kubernetes1881",
            "size": 229273505,
            "path": "vespace/manager-ha:v5.2.0"
          }, {
            "id": "vespace_strategy",
            "name": "vespace strategy",
            "user_id": "admin",
            "status": "Ready",
            "update_time": 1521682900493,
            "description": "vespace strategy",
            "package": "kubernetes1881",
            "size": 200943566,
            "path": "vespace/storage-centos:v5.4.0"
          }],
    "info": {
        "vip": "192.168.4.251",
        "version": "v1.8.6"
    }
}
```

### add master
```bash
curl -X POST -H 'Content-Type: application/json' -d <body> http://192.168.0.171:8886/clusters/iSingle/masters/new
```

body is like this:
```json
{
	"hostip": "192.168.4.106",
	"username": "root",
	"userpwd": "Cs123456",
	"clustername": "iSingle"
}
```

### add node
```bash
curl -X POST -H 'Content-Type: application/json' -d <body> http://192.168.0.171:8886/clusters/iSingle/nodes/new
```

body is like this:
```json
{
	"hostip": "192.168.4.106",
	"username": "root",
	"userpwd": "Cs123456",
	"clustername": "iSingle"
}
```

## Change Log:
### 2018.3.26
- support specified all images' version to install

### 2018.3
- support specified k8s version to install

### 2017.10
- use kubeadm to create and manage k8s deploy

### 2017.5 main modify
- 镜像文件交有由镜像仓库管理
