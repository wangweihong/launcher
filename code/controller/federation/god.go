package federation

import (
	"strings"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/base"
	"encoding/json"
	"fmt"
	"log"
)

func GetFederation(federationName string) (Federation, bool) {
	existFed := Federation{}
	key := strings.Join([]string{common.FederationKey, federationName}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return existFed, false
	}

	if values != nil && values[key] != "" {
		_ = json.Unmarshal([]byte(values[key]), &existFed)
	} else {
		return existFed, false
	}

	return existFed, true
}

func GetFederations()(map[string]Federation, bool){
	v := map[string]Federation{}

	key := strings.Join([]string{common.FederationKey}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return v, false
	}

	for key := range values {
		path := strings.Split(key, "/")
		fedName := path[len(path) - 1]
		v[fedName], _ = GetFederation(fedName)
	}
	return v, true
}

func (fed *Federation) SaveFederationStatus() error {
	if fed.FedInfo.Name == "" {
		return fmt.Errorf("federation name or federation could not be empty")
	}

	key := strings.Join([]string{common.FederationKey}, common.Sep)
	isExist, err := base.IsExist(key)
	if !isExist {
		ok := false
		for i:=0;i<3;i++ {
			err = base.Put(key, "")
			if err != nil {
				continue
			}
			ok = true
		}
		if !ok {
			return fmt.Errorf("could not create key: %s", key)
		}
	}

	key = strings.Join([]string{common.FederationKey, fed.FedInfo.Name}, common.Sep)
	fedValue, err := json.Marshal(&fed)
	if err != nil {
		return err
	}
	err = base.Put(key, string(fedValue))

	return err
}

// DeleteCluster delete a specfic cluster by clustername
func DeleteFederation(federationName string) {
	key := strings.Join([]string{common.FederationKey, federationName}, common.Sep)
	exist, _ := base.IsExist(key)
	if !exist {
		// already delete.
		return
	}

	err := base.Delete(key)
	if err != nil {
		log.Printf("delete federation(%s) from etcd failed. ErrorMsg: %s", federationName, err.Error())
	}
}
