package apiserver

import (
	"net/http"

	"log"

	"github.com/ant0ine/go-json-rest/rest"
	"ufleet/launcher/code/controller/cluster"
	"time"
	"ufleet/launcher/code/model/common"
	"fmt"
	"ufleet/launcher/code/model/job"
)

// GetClusters return all clusters data
func GetClusters(w rest.ResponseWriter, r *rest.Request) {
	clusters, found := cluster.GetClusters()
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Not Found!"))
		return
	}
	w.WriteJson(clusters)
}

// GetCluster search a specific cluster use provided clusername and return
func GetCluster(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	cluster, found := cluster.GetClusterDetail(clusterName)
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Not Found!"))
		return
	}
	w.WriteJson(cluster)
}

// PostCluster use post data create a k8s cluster
func PostCluster(w rest.ResponseWriter, r *rest.Request) {
	tcluster := cluster.Cluster{}
	err := r.DecodeJsonPayload(&tcluster)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "POST DATA ERROR: " + err.Error()))
		return
	}

	if cluster.CheckCluster(tcluster.Name) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(402, "CLUSTER EXIST!"))
		return
	}
	for _, master := range tcluster.Masters {
		if cluster.CheckMaster(tcluster.Name, master.HostIP) ||
			cluster.CheckNode(tcluster.Name, master.HostIP) {
			w.WriteHeader(http.StatusBadRequest)
			w.WriteJson(ErrorResponse(403, "MASTER NODE EXIST!"))
			return
		}
	}

	// 添加集群创建任务
	if err := RegCreateClusterJob(tcluster); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.WriteJson(map[string]string{"Message": "ADD CREATE JOB FAILED: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "create cluster"})
}

// RemoveCluster remove cluster info from etcd
func RemoveCluster(w rest.ResponseWriter, r *rest.Request) {
	logPrefix := "[ RemoveCluster ]"
	clusterName := r.PathParam("clustername")

	log.Printf("%s Remove cluster: %s", logPrefix, clusterName)
	masters, exist := cluster.GetMasters(clusterName)
	if exist {
		for i := range masters {
			go func(master cluster.Master) {
				log.Printf("%s start backend job: remove master(%s)", logPrefix, master.HostIP)
				master.RemoveMaster()
			}(masters[i])
		}
	}

	cluster.DeleteCluster(clusterName)
	w.WriteHeader(http.StatusOK)
}

// PostCluster use post data create a k8s cluster
func LoadCluster(w rest.ResponseWriter, r *rest.Request) {
	clul := cluster.ClusterL{}
	err := r.DecodeJsonPayload(&clul)
	if err != nil {
		log.Printf("Load cluster failed: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "POST DATA ERROR!"))
		return
	}
	log.Printf("Load cluster: %s", clul.Clustername)

	log.Printf("Check cluster(%s) start.", clul.Clustername)
	if cluster.CheckCluster(clul.Clustername) {
		log.Printf("Check cluster(%s) failed.", clul.Clustername)
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(402, "CLUSTER EXIST!"))
		return
	}
	log.Printf("Check cluster(%s) ok.", clul.Clustername)

	log.Printf("Load cluster(%s) start.", clul.Clustername)
	err = clul.LoadCluster()
	if err != nil {
		log.Printf("Load cluster(%s) failed. ErrorMsg: %s", clul.Clustername, err.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "POST DATA ERROR: "+err.Error()))
		return
	}
	log.Printf("Load cluster(%s) ok.", clul.Clustername)

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "load cluster success!"})
}

// RegCreateClusterJob 注册添加集群任务
func RegCreateClusterJob(clu cluster.Cluster) error {
	logPrefix := fmt.Sprintf("[ %s ][ CreateCluster ]", clu.Name)
	// Step : 存储集群节点个数，作为单master节点和多master节点的判断，方便后续判断
	clu.BaseMasters = len(clu.Masters)
	if clu.BaseMasters <= 0 {
		log.Printf("%s %s", logPrefix, clu.ErrorMsg)
		return fmt.Errorf("master could not be empty")
	}

	clu.CluInfo.CreateTime = time.Now().Unix()
	clu.Status = common.GlobalClusterStatusPending
	clu.SaveStatus()
	for i := range clu.Masters {
		clu.Masters[i].CreateTime = clu.CluInfo.CreateTime
		clu.Masters[i].Status = common.GlobalNodeStatusPending
		clu.Masters[i].ErrorMsg = ""
		clu.Masters[i].SaveStatus()
	}

	timer := time.Now().Nanosecond()
	newJob := job.Job{}
	newJob.JobId = fmt.Sprintf("create-cluster-%s-%d", clu.Name, timer)
	newJob.Key = clu.Name
	newJob.Type = common.ClusterCreate
	err := RegJob(newJob)

	return err
}