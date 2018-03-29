package apiserver

import (
	"net/http"

	"github.com/ant0ine/go-json-rest/rest"
	"ufleet/launcher/code/controller/cluster"
)

// GetStorage get storage cluster
func GetStorage(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	sclusterName := r.PathParam("sclustername")
	cluster, err := cluster.GetStorageCluster(clusterName, sclusterName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "Failed: " + err.Error()))
		return
	}
	w.WriteJson(cluster)
}

// PostNode create a k8s node host from post data
func PostStorage(w rest.ResponseWriter, r *rest.Request) {
	sCluster := cluster.StorageCluster{}
	err := r.DecodeJsonPayload(&sCluster)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "POST DATA ERROR!"))
		return
	}

	if cluster.CheckStorageCluster(sCluster.ClusterName, sCluster.SClusterName) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(402, "STORAGE CLUSTER ALREADY EXIST"))
		return
	}
	err = cluster.CreateStorageCluster(sCluster.ClusterName, sCluster.SClusterName, sCluster.Vip, sCluster.Nodes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.WriteJson(map[string]string{"Message": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "CREATE STORAGE CLUSTER SUCCESS"})
}

// DeleteStorage delete storage cluster
func DeleteStorage(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	sclusterName := r.PathParam("sclustername")

	err := cluster.RemoveStorageCluster(clusterName, sclusterName)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(map[string]string{"Message": err.Error()})
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "DELETE STORAGE CLUSTER: " + sclusterName})
}

// AddStorageNode add storage node
func AddStorageNode(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	sclusterName := r.PathParam("sclustername")
	nodeip := r.PathParam("nodename") // nodename -> nodeip

	if !cluster.CheckStorageCluster(clusterName, sclusterName) {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(404, "STORAGE CLUSTER NOT EXIST!"))
		return
	}

	err := cluster.AddStorageNode(clusterName, sclusterName, nodeip)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteJson(ErrorResponse(405, "ADD NODE TO STORAGE CLUSTER ERROR: " + err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "ADD NODE TO STORAGE CLUSTER"})
}

// RemoveStorageNode remove storage node
func RemoveStorageNode(w rest.ResponseWriter, r *rest.Request) {
	clusterName := r.PathParam("clustername")
	sclusterName := r.PathParam("sclustername")
	nodeip := r.PathParam("nodename") // nodename -> nodeip

	// TODO. maybe remove failed, should retry
	cluster.RemoveStorageNode(clusterName, sclusterName, nodeip)

	w.WriteHeader(http.StatusOK)
	w.WriteJson(map[string]string{"Message": "REMOVE NODE TO STORAGE CLUSTER"})
}