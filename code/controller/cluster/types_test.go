package cluster

import (
	"testing"
	"encoding/json"
)

func TestImageType(t *testing.T) {
	value := `{
	"Status" : "pending",
	"info" : {
		"version" : "v1.8.8"
	},
	"Masters" : [{
			"RegistryIP" : "",
			"UserName" :	"root",
			"KubeServiceIP" : "10.96.0.1",
			"PodNetwork" : "10.244.0.0/16",
			"Name" : "dev",
			"ServiceNetwork" : "10.96.0.0/12",
			"ClusterName" : "dev",
			"HostName" : "192-168-3-25",
			"Status" :"pending",
			"HostIP" : "192.168.3.25",
			"UserPwd" : "Cs123456",
			"HostSSHPort" : 22,
			"DNSServiceIP" :"10.96.0.10"
		}
	],
	"Name" : "dev",
	"images" : [{
			"category" : "calico",
			"status" : "Ready",
			"update_time" :1521513972440,
			"description" : "common",
			"package" : "kubernetes1881",
			"size" : 21024905,
			"path" : "calico/cni:v1.10.0",
			"id" : "calico_cni",
			"name" : "calico-cni"
		}, {
			"category" : "calico",
			"status" : "Ready",
			"update_time" : 1521513972445,
			"description" : "common",
			"package" : "kubernetes1881",
			"size" : 19387307,
			"path" : "calico/kube-policy-controller:v0.7.0",
			"id" : "calico_kube_policy_controller",
			"name" : "calico-kube-policy-controller"
		}
]}`

	clu := Cluster{}
	err := json.Unmarshal([]byte(value), &clu)
	if err != nil {
		t.Error(err.Error())
	}
	if len(clu.Images) <= 0 {
		t.Error("get images empty")
	}
}
