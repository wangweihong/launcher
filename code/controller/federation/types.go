package federation

// FederationInfo federation's info
type FederationInfo struct {
	CreateTime int64  `json:"createtime"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	ErrorMsg   string `json:"errormsg"`
}

// Federation federation of kubernetes
type Federation struct {
	FedInfo           FederationInfo `json:"info"`
	FedCluLeaders     []string       `json:"leaders"`
	FedCluFollowers   []string       `json:"followers"`
}
