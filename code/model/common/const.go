package common

import "fmt"

type JobType int

const (
	ClusterCreate JobType = iota
	MasterAdd
	NodeAdd
	EtcdAdd
	FederationCreate
)

const (
	BaseKey = "/ufleet/launcher"
	Sep = "/"
	ClusterKey = BaseKey + "/clusters"
	MasterKey  = "masters"
	NodeKey    = "nodes"
	ImagesKey  = "images"
	Status     = "status"
	Name       = "name"
	Info       = "info"

	MemberKey = BaseKey + "/member"
	NewJobs   = ".newjobs"
	NewWorker = ".regworker"
	Heartbeat = ".heartbeat"
	HeartbeatBase = ".heartbeatbase"

	UfleetMasterKey = "ufleet/master"

	FederationKey = BaseKey + "/federation"
	FedInfo       = "info"
	FedLeaders    = "leaders"
	FedFollowers  = "followers"

	StorageKey = "storage"
)

const (
	EnvUfleetNodeId = "UFLEET_NODE_ID"
)

const (
	HeartGroupup = 3
	MaxLostTimes = 3
)

var (
	NotFound = fmt.Errorf("not found")
	EtcdNodeNumberNotThree = fmt.Errorf("can't get etcd1 etcd2 or etcd3 from etcd cluster")
)


// etcd key path
const (
	GlobalNodeStatusFailed  = "error"
	GlobalNodeStatusRunning = "running"
	GlobalNodeStatusPending = "pending"

	GlobalClusterStatusFailed  = "error"
	GlobalClusterStatusRunning = "running"
	GlobalClusterStatusPending = "pending"

	GlobalFederationStatusFailed = "error"
	GlobalFederationStatusRunning = "running"
	GlobalFederationStatusPending = "pending"

	GlobalConfigCertPath       = "/pki"
	GlobalConfigEtcdPath       = "/etcd"
	GlobalConfigKubeadmPath    = "/kubeadm"
	GlobalConfigKeepalivedPath = "/keepalived"
	GlobalConfigVespacePath    = "/vespace"
	GlobalConfigStoragePath    = "/storage"
	GlobalConfigCalicoPath     = "/calico"
	GlobalConfigExportPath     = "/exporter"

	GlobalDefaultNetPodSubnet     = "10.244.0.0/16"
	GlobalDefaultNetServiceSubnet = "10.96.0.1/16"

	GlobalNodetypeMaster = iota
	GlobalNodetypeNode
	GlobalNodetypeEtcd
	GlobalNodetypeMasterNode
	GlobalNodetypeMasterEtcd
	GlobalNodetypeMasterNodeEtcd
)