package manifests

const (
	kubeadmYaml = `apiVersion: kubeadm.k8s.io/v1alpha1
kind: MasterConfiguration
api:
  advertiseAddress: {{ .Hostip }}
  bindPort: 6443
etcd:
  endpoints:
  {{ .EtcdEndpoints }}
kubernetesVersion: {{ .K8sVersion }}
networking:
  dnsDomain: cluster.local
  serviceSubnet: {{ .ServiceSubnet }}
  podSubnet: {{ .PodSubnet }}
token: {{ .JoinToken }}
nodeName: {{ .Hostname }}
certificatesDir: /etc/kubernetes/pki
imageRepository: ufleet.io/google_containers
`
)
