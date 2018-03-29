package apiserver

import (
	"ufleet/launcher/code/controller/cluster"
)

// ClusterResource recourse of cluster
type ClusterResource struct {
	Clusters map[string]*cluster.Cluster
}

// Error error type for return
type Error struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}
