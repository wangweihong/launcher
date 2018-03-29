package apiserver

import (
	"net/http"
	"fmt"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"ufleet/launcher/code/controller/cluster"
	"ufleet/launcher/code/model/job"
	"ufleet/launcher/code/model/common"
)

// GetNodes return all nodes information
func GetNodes(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	nodes, found := cluster.GetNodes(clusterName)
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Not Found!"))
		return
	}
	w.WriteJson(nodes)
}

// GetNode get a specific node by nodename and return node data
func GetNode(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	nodeName := r.PathParam("nodename")
	fmt.Println(clusterName, nodeName)
	node, found := cluster.GetNode(clusterName, nodeName)
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Not Found!"))
		return
	}
	w.WriteJson(node)
}

// PostNode create a k8s node host from post data
func PostNode(w rest.ResponseWriter, r *rest.Request) {
	node := cluster.Node{}
	err := r.DecodeJsonPayload(&node)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "POST DATA ERROR!"))
		return
	}

	if !cluster.CheckCluster(node.ClusterName) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(402, "CLUSTER DOESN'T EXIST!"))
		return
	}
	if cluster.CheckMaster(node.ClusterName, node.HostName) ||
		cluster.CheckNode(node.ClusterName, node.HostName) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "NODE EXIST!"))
		return
	}

	RegCreateNodeJob(node)

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "add node to cluster"})
}

// RemoveNode remove node date in etcd
func RemoveNode(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	nodeName := r.PathParam("nodename")
	node, exist := cluster.GetNode(clusterName, nodeName)

	if !exist {
		// 不存在，删除launcher中的信息后返回成功
		cluster.DeleteNode(clusterName, nodeName)
		w.WriteHeader(http.StatusOK)
		return
	}

	// get cluster
	clu, err := cluster.GetCluster(clusterName)
	if err != nil {
		// 无法获取到cluster
		// 继续删除node节点
		// TODO 如何保证网络不稳定或者etcd繁忙下的数据一致性
		go node.RemoveNode()
		cluster.DeleteNode(clusterName, nodeName)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 在集群中删除node
	clu.RemoveNode(node.HostIP)

	cluster.DeleteNode(clusterName, nodeName)
	w.WriteHeader(http.StatusOK)
}

// RegCreateNodeJob 注册node节点任务
func RegCreateNodeJob(n cluster.Node) error {
	/* 将节点存储到 etcd中 */
	n.CreateTime = time.Now().Unix()
	n.ErrorMsg = ""
	n.Status = common.GlobalNodeStatusPending
	n.SaveStatus()

	// 添加job
	timer := time.Now().Nanosecond()
	newJob := job.Job{}
	newJob.JobId = fmt.Sprintf("create-node-%s-%d", n.HostIP, timer)
	newJob.Key = fmt.Sprintf("%s/%s", n.ClusterName, n.HostIP)
	newJob.Type = common.NodeAdd
	err := RegJob(newJob)

	return err
}
