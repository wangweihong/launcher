package cluster

import (
	"fmt"
	"ufleet/launcher/code/utils"
	"strings"
	"ufleet/launcher/code/model/common"
	"encoding/json"
	"ufleet/launcher/code/model/base"
	"log"
	"ufleet/launcher/code/config"
	"ufleet/launcher/code/config/manifests"
)

func GetStorageCluster(clustername, sclustername string) ([]Node, error) {
	nodes, found := GetNodes(clustername)
	if !found {
		return nil, fmt.Errorf("could not found storage cluster: %s", sclustername)
	}

	storage, err := GetStorageFromDB(clustername)
	if err != nil {
		return nil, err
	}
	exist := false
	for i := range storage {
		if storage[i].SClusterName == sclustername {
			exist = true
			break
		}
	}
	if !exist {
		// not found
		return nil, nil
	}

	snodes := make([]Node, 0)
	for i := range nodes {
		if nodes[i].SCName == sclustername {
			snodes = append(snodes, nodes[i])
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("can't find any node of storage cluster: %s", sclustername)
	}
	return snodes, nil
}

func CheckStorageCluster(clustername, sclustername string) bool {
	storage, err := GetStorageFromDB(clustername)
	if err != nil {
		return false
	}
	if storage == nil {
		return false
	}

	for i := range storage {
		if sclustername == storage[i].SClusterName {
			return true
		}
	}

	return false
}

func CreateStorageCluster(clustername string, sclustername string, vip string, nodeips []string) error {
	// get all nodes
	nodes, found := GetNodes(clustername)
	if !found || nodes == nil{
		return fmt.Errorf("node of cluster %s not found", clustername)
	}
	storage, err := GetStorageFromDB(clustername)
	if err != nil {
		return err
	}
	if storage == nil {
		storage = make([]StorageCluster, 0)
	}

	for i := range storage {
		if storage[i].SClusterName == sclustername {
			if storage[i].Vip == vip {
				// same is ok
				break
			} else {
				// not same vip, should not accept
				return fmt.Errorf("storage cluster [%s] already exist, and vip is different between with %s and %s", sclustername, storage[i].Vip, vip)
			}
		}
	}

	// for node in nodes
	for i := range nodeips {
		valid := false
		for j := range nodes {
			if nodes[j].HostIP == nodeips[i] {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("node ip -- %s not exist", nodeips[i])
		}
	}

	scluster := StorageCluster{}
	scluster.SClusterName = sclustername
	scluster.Vip          = vip
	forRecover := storage
	storage = append(storage, scluster)
	err = SaveStorageIntoDB(clustername, storage)
	if err != nil {
		return err
	}

	// setup keepalive container
	for i := range nodeips {
		err = AddStorageNode(clustername, sclustername, nodeips[i])
		if err != nil {
			log.Printf("add storage node %s failed: %v", nodeips[i], err)
			break
		}
	}
	if err != nil {
		// recover
		for i := range nodeips {
			err = RemoveStorageNode(clustername, sclustername, nodeips[i])
			if err != nil {
				log.Printf("recover: remove storage node %s failed: %v", nodeips[i], err)
			}
		}

		SaveStorageIntoDB(clustername, forRecover)
		return err
	}

	for i:=0;i<3;i++ {
		err := SaveStorageIntoDB(clustername, storage)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	return nil
}

func RemoveStorageCluster(clustername, sclusterName string) error {
	// get all nodes
	nodes, found := GetNodes(clustername)
	if !found || nodes == nil{
		return fmt.Errorf("nodes not found")
	}
	oldscluster, err := GetStorageFromDB(clustername)
	if err != nil {
		return err
	}
	for i := range nodes {
		if nodes[i].SCName == sclusterName {
			err = RemoveStorageNode(clustername,  sclusterName, nodes[i].HostIP)
			if err != nil {
				log.Printf("remove storage node %s failed: %v", nodes[i].HostIP, err)
			}
		}
	}

	sclusters := make([]StorageCluster, 0)
	for i := range oldscluster {
		if oldscluster[i].SClusterName != sclusterName {
			sclusters = append(sclusters, oldscluster[i])
		}
	}
	SaveStorageIntoDB(clustername, sclusters)

	return nil
}

func AddStorageNode(clustername, sclusterName, hostip string) error {
	// looking for storage cluster
	storage, err := GetStorageFromDB(clustername)
	if err != nil {
		return err
	}
	if storage == nil {
		return fmt.Errorf("cluster not exist")
	}

	vip := ""
	for i := range storage {
		if storage[i].SClusterName == sclusterName {
			vip = storage[i].Vip
		}
	}
	if len(vip) == 0 {
		return fmt.Errorf("vip is not exist")
	}

	clu, err := GetCluster(clustername)
	if err != nil {
		return err
	}
	if clu == nil {
		return fmt.Errorf("can't get cluster by cluster name: %s", clustername)
	}
	imageMaps := imageArray2Map(clu.Images)
	imageKeepalived, found := imageMaps["keepalived"]
	if !found { return fmt.Errorf("can't find image: %s", "keepalived") }
	imageKeepalived = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKeepalived)

	// looking for node ip
	node, found := GetNode(clustername, hostip)
	if !found {
		return fmt.Errorf("node(%s) not found", hostip)
	}

	if node.SCName != "" {
		return fmt.Errorf("this storage already add into cluster: %s", node.SCName)
	}

	// setup storage node
	sshClient,err := node.GetSSHClient()
	if err != nil {
		return err
	}

	netcard, err := node.getNetworkCardName()
	if err != nil {
		return err
	}

	res, err := utils.ParseTemplate(manifests.GetKeepalivedStorageCmd(), struct {VirtualIP, VirtualRouterID, Interface, ContainersToCheck, ImageKeepalived string}{
		vip, utils.GetIPField(vip, 4),netcard,"storage", imageKeepalived,
	})
	if err != nil {
		return err
	}

	_, err = utils.Execute(fmt.Sprintf(string(res)), sshClient)
	if err != nil {
		return fmt.Errorf("start keepalived container failed: %s: %s", string(res), err)
	}

	node.SCName = sclusterName
	node.SVip   = vip
	node.saveStatusIfExist()

	return nil
}

func RemoveStorageNode(clustername, sclustername, hostip string) error {
	// looking for storage cluster
	storage, err := GetStorageFromDB(clustername)
	if err != nil {
		return err
	}
	if storage == nil {
		// storage cluster not found
		return nil
	}

	node, found := GetNode(clustername, hostip)
	if !found {
		// not found
		return nil
	}

	if node.SCName == "" {
		// already remove
		return nil
	}

	sshClient,err := node.GetSSHClient()
	if err != nil {
		return err
	}

	cmd := manifests.GetRmKeepalivedStorageCmd()
	for i:=0;i<3;i++ {
		_, err = utils.Execute(cmd, sshClient)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	node.SCName = ""
	node.SVip = ""
	// TODO. maybe save failed
	node.saveStatusIfExist()

	return nil
}

// saveStatusIfExist Save status if master in etcd. But just return false when:
//     1. master not in etcd
//     2. master is already installed.
func SaveStorageIntoDB (clustername string, sc []StorageCluster) error {
	key := strings.Join([]string{common.ClusterKey, clustername, common.StorageKey}, common.Sep)

	storageData, err := json.Marshal(&sc)
	if err != nil {
		return err
	}

	err = base.Put(key, string(storageData))
	if err != nil {
		return err
	}

	return nil
}

func GetStorageFromDB (clustername string) ([]StorageCluster, error) {
	key := strings.Join([]string{common.ClusterKey, clustername, common.StorageKey}, common.Sep)

	sClusterMaps, err := base.Get(key)
	if err != nil {
		return nil, err
	}
	sClusterValue, found := sClusterMaps[key]
	if !found {
		// not found
		return nil, nil
	}

	var scluster []StorageCluster
	err = json.Unmarshal([]byte(sClusterValue), &scluster)
	if err != nil {
		return nil, err
	}

	filterNotEmpty := make([]StorageCluster, 0)
	for i := range scluster {
		// filter not empty storage cluster
		if scluster[i].SClusterName != "" && scluster[i].Vip != "" {
			filterNotEmpty = append(filterNotEmpty, scluster[i])
		}
	}

	return filterNotEmpty, nil
}