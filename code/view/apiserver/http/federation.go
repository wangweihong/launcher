package apiserver

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/job"
	"ufleet/launcher/code/controller/federation"
	"time"
	"fmt"
	"ufleet/launcher/code/utils"
)

// GetNode get a specific node by nodename and return node data
func GetFederation(w rest.ResponseWriter, r *rest.Request) {
	federationName := r.PathParam("fedname")

	fed, found := federation.GetFederation(federationName)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.WriteJson(map[string]string{"Message": "federation not found"})
		return
	}

	w.WriteJson(fed)
}

// GetNode get a specific node by nodename and return node data
func GetFederations(w rest.ResponseWriter, r *rest.Request) {
	fed, found := federation.GetFederations()
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.WriteJson(map[string]string{"Message": "there is not any federation"})
		return
	}

	w.WriteJson(fed)
}

func PostFederation(w rest.ResponseWriter, r *rest.Request){
	fed := federation.Federation{}
	err := r.DecodeJsonPayload(&fed)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "POST DATA ERROR!"))
		return
	}

	if len(fed.FedInfo.Name) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "federation name could not be empty!"))
		return
	}
	if len(fed.FedCluLeaders) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "federation leader could not be empty!"))
		return
	}
	srcFederationName := fed.FedInfo.Name
	fed.FedInfo.Name=utils.GetLowerCh(srcFederationName)
	if fed.FedInfo.Name == "" || srcFederationName != fed.FedInfo.Name {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "federation(src: " + srcFederationName + ",valid: " + fed.FedInfo.Name + ") name could not be empty or is invalid(just support a-z)!"))
		return
	}
	// check federation is already exist or not
	_, found := federation.GetFederation(fed.FedInfo.Name)
	if found {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "federation(src: " + srcFederationName + ",valid: " + fed.FedInfo.Name + ") already exist!"))
		return
	}
	err = RegCreateFederationJob(fed)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "CREATE FEDERATION ERROR! ErrorMsg: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "create federation"})
}

func RemoveFederation(w rest.ResponseWriter, r *rest.Request){
	federationName := r.PathParam("fedname")
	fed, found := federation.GetFederation(federationName)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.WriteJson(map[string]string{"Message": "federation not found"})
		return
	}

	// 卸载联邦
	go fed.Remove()
	federation.DeleteFederation(federationName)
	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "remove federation"})
}

func AddFederationFollower(w rest.ResponseWriter, r *rest.Request){
	federationName := r.PathParam("fedname")
	clusterName := r.PathParam("clustername")

	fed, found := federation.GetFederation(federationName)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.WriteJson(map[string]string{"Message": "federation not found"})
	}

	err := fed.AddFollower(clusterName)
	if err != nil {
		w.WriteHeader(http.StatusNotExtended)
		w.WriteJson(map[string]string{"Message": "add follower into federation failed: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "add cluster into federation"})
}


func DeleteFederationFollower(w rest.ResponseWriter, r *rest.Request){
	federationName := r.PathParam("fedname")
	clusterName := r.PathParam("clustername")

	fed, found := federation.GetFederation(federationName)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		w.WriteJson(map[string]string{"Message": "federation not found"})
	}

	err := fed.DelFollower(clusterName)
	if err != nil {
		w.WriteHeader(http.StatusNotExtended)
		w.WriteJson(map[string]string{"Message": "delete follower in federation failed: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "delete cluster from federation"})
}

// RegCreateFederationJob 注册联邦任务
func RegCreateFederationJob(fed federation.Federation) error {
	/* 将节点存储到 etcd中 */
	fed.FedInfo.CreateTime = time.Now().Unix()
	fed.FedInfo.ErrorMsg = ""
	fed.FedInfo.Status = common.GlobalFederationStatusPending
	if err := fed.SaveFederationStatus(); err != nil {
		return err
	}

	// 添加job
	timer := time.Now().Nanosecond()
	newJob := job.Job{}
	newJob.JobId = fmt.Sprintf("create-federation-%s-%s", fed.FedInfo.Name, timer)
	newJob.Key = fed.FedInfo.Name
	newJob.Type = common.FederationCreate
	err := RegJob(newJob)

	return err
}