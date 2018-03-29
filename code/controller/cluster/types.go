package cluster

import (
	"golang.org/x/crypto/ssh"
)

// Host basis host information
type Host struct {
	HostName       string `json:"hostname"`
	HostIP         string `json:"hostip"`
	UserName       string `json:"username"`
	UserPwd        string `json:"userpwd,omitempty"`
	Prikey         string `json:"prikey,omitempty"`
	PrikeyPwd      string `json:"prikeypwd,omitempty"`
	HostSSHPort    string `json:"hostsshport,omitempty"`
	HostSSHNetwork string `json:"hostsshnetwork,omitempty"`
	SCName         string `json:"storageclustername,omitempty"`
	SVip           string `json:"storagevip,omitempty"`
	sshClient      *ssh.Client
}

type StorageCluster struct {
	ClusterName  string `json:"clustername"`
	SClusterName string `json:"sclustername"`
	Vip          string `json:"vip"`
	Nodes      []string `json:"nodes"`
}

type Cluster struct {
	Name        string           `json:"name"`
	Status      string           `json:"status"`
	Masters     []Master         `json:"masters"`
	Nodes       []Node           `json:"nodes"`
	SClusters   []StorageCluster `json:"storageclusters"`
	Images      []Image          `json:"images"`
	ErrorMsg    string           `json:"errormsg"`
	CluInfo                      `json:"info"`
}

type Image struct {
	Id          string `json:"id"`
	Name        string `json:"name,omitempty"`
	UserId      string `json:"user_id,omitempty"`
	Category    string `json:"calico,omitempty"`
	Status      string `json:"status,omitempty"`
	UpdateTime  interface{}  `json:"update_time,omitempty"`
	Description string `json:"description,omitempty"`
	Package     string `json:"package,omitempty"`
	Size        int    `json:"size,omitempty"`
	Path        string `json:"path"`
}

// CluInfo cluster info
type CluInfo struct {
	Vip            string `json:"vip"`
	K8sVersion     string `json:"version"`
	CreateTime     int64  `json:"createtime"`
	BaseMasters    int    `json:"basemasters"`
	JoinToken      string `json:"jointoken"`
	CaCert         string `json:"cacert"`
	CaKey          string `json:"cakey"`
	APIServerCert  string `json:"apiservercert"`
	APIServerKey   string `json:"apiserverkey"`
	APIClientCert  string `json:"apiclientcert"`
	APIClientKey   string `json:"apiclientkey"`
	FrontProxyCert string `json:"frontproxycacert"`
	FrontProxyKey  string `json:"frontproxycakey"`
	FrPxyCliCert   string `json:"frontproxyclientcert"`
	FrPxyCliKey    string `json:"frontproxyclientkey"`
	SaPub          string `json:"sapub"`
	SaKey          string `json:"sakey"`
	AdminContext   string `json:"admincontext"` // 经过base64编码
}

// Master master of kubernetes
type Master struct {
	Host
	CreateTime     int64  `json:"createtime"`
	ClusterName    string `json:"clustername"`
	Registry       string `json:"registry,omitempty"`
	ServiceNetwork string `json:"servicenetwork"`
	PodNetwork     string `json:"podnetwork"`
	KubeServiceIP  string `json:"kubeserviceip"`
	DNSServiceIP   string `json:"dnsserviceip"`
	CaCert         string `json:"cacert"`
	APIServerCert  string `json:"apiservercert"`
	APIServerKey   string `json:"apiserverkey"`
	APIClientCert  string `json:"apiclientcert"`
	APIClientKey   string `json:"apiclientkey"`
	ErrorMsg       string `json:"errormsg"`
	Status         string `json:"status"`
	Progress       string `json:"progress"` // 安装进度
}

// Node node of kubernetes
type Node struct {
	Host
	CreateTime     int64  `json:"createtime"`
	CaCert         string `json:"cacert"`
	ClusterName    string `json:"clustername"`
	Registry       string `json:"registry,omitempty"`
	MasterIP       string `json:"masterip"`
	APIClientCert  string `json:"apiclientcert"`
	APIClientKey   string `json:"apiclientkey"`
	Status         string `json:"status"`
	ErrorMsg       string `json:"errormsg"`
	Progress       string `json:"progress"` // 安装进度
}
