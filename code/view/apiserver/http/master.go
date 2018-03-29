package apiserver

import (
	"net/http"

	"fmt"
	"log"
	"time"

	"github.com/ant0ine/go-json-rest/rest"
	"ufleet/launcher/code/controller/cluster"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/job"
)

// GetMasters return all master node of a cluster
func GetMasters(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	masters, found := cluster.GetMasters(clusterName)
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Not Found!"))
		return
	}
	w.WriteJson(masters)
}

// GetMaster get master info
func GetMaster(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	masterName := r.PathParam("mastername")
	master, found := cluster.GetMaster(clusterName, masterName)
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Not Found!"))
		return
	}
	w.WriteJson(master)
}

func AddMaster(w rest.ResponseWriter, r *rest.Request) {
	master := cluster.Master{}
	err := r.DecodeJsonPayload(&master)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "POST DATA ERROR!"))
		return
	}

	if !cluster.CheckCluster(master.ClusterName) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(402, "CLUSTER DOESN'T EXIST!"))
		return
	}
	if cluster.CheckMaster(master.ClusterName, master.HostName) ||
		cluster.CheckNode(master.ClusterName, master.HostName) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(402, "MASTER EXIST!"))
		return
	}

	// 注册到添加master任务列表
	RegAddMasterJob(master)

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "add master to cluster"})
}

func RemoveMaster(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	masterName := r.PathParam("mastername")

	rMaster, found := cluster.GetMaster(clusterName, masterName)
	if !found || rMaster.HostIP == "" {
		cluster.DeleteMaster(clusterName, masterName)
		w.WriteHeader(http.StatusOK)
		return
	}

	clu, err := cluster.GetCluster(clusterName)
	if err != nil {
		log.Printf("[ %s ][ RemoveMaster ] %s", clusterName, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.WriteJson(map[string]string{"Message": err.Error()})
		return
	}

	clu.RemoveMaster(rMaster.HostIP)

	cluster.DeleteMaster(clusterName, masterName)
	w.WriteHeader(http.StatusOK)
}

func RegAddMasterJob(master cluster.Master) error {
	/* 检查参数 */
	// 检查网段是否和集群所在子网相同

	/* 将节点存储到 etcd中 */
	master.CreateTime = time.Now().Unix()
	master.ErrorMsg = ""
	master.Status = common.GlobalNodeStatusPending
	master.SaveStatus()

	// 添加job
	timer := time.Now().Nanosecond()
	newJob := job.Job{}
	newJob.JobId = fmt.Sprintf("add-master-%s-%d", master.HostIP, timer)
	newJob.Key = fmt.Sprintf("%s/%s", master.ClusterName, master.HostIP)
	newJob.Type = common.MasterAdd
	err := RegJob(newJob)

	return err
}
