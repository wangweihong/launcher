package manifests

const (
	kubernetesConf = `[ default ]
version={{ .K8sVersion }}
change_hostname=true        ## if it's true, will auto modify machine's host name.
times_of_check_apiserver=20 ## after installed kubernetes cluster, the times try to connect apiserver. each time will cost 5 second.
times_of_check_etcd=600     ## before create new master node, the times of try to check etcd cluster status. each time will cost 1 second.
context_admin=kubernetes-admin@kubernetes
check_subnetwork=true       ## if set to true, will check vip and masters' subnetwork same or not in ha mode
image_repository=ufleet.io  ## image repository

[ remote ]
dir_home=/tmp/ufleet/launcher/files
dir_log=/tmp/ufleet/launcher/logs

[ etcd ]
listen_port=12379
peer_port=12380

[ helm ]
serviceaccount_helm=helmor

[ federation ]
zones=example.com.

# Dynamic variable for kubernetes, generated by kube-launcher
#     DO NOT EDIT THESE VARIABLE BY HAND -- YOUR CHANGES WILL CAUSE PROBLEM
[ dynamic ]
NTPD_HOST={{ .NtpdHost}}
NODE_NAME={{ .Nodename }}
`
)
