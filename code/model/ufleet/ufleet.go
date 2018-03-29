package ufleet

import (
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/base"

	"os"
	"strings"
	"k8s.io/apimachinery/pkg/util/json"
	"fmt"
)

type UfleetMasterNode struct {
	Id     string `json:"id"`
	Ticket int    `json:"ticket"`
}

// IsMaster 通过查看Etcd中的master，判断本节点身份是否为Master节点
// 特殊的，对于Ufleet单节点，不存在 interf.UfleetMasterKey 这个值
func IsMaster() (bool, error) {
	key := strings.Join([]string{common.UfleetMasterKey}, common.Sep)

	// 单机版的 Ufleet
	values, err := base.Get(key)
	if err == nil && len(values) == 0 {
		// 没有发现 interf.UfleetMasterKey 这个值则认为为单机版
		return true, nil
	}

	// Ha 模式下的 Ufleet
	// 获取环境变量值
	nodeId := os.Getenv(common.EnvUfleetNodeId)
	if len(nodeId) == 0 {
		// 无法获取环境变量值
		return false, fmt.Errorf("can't not get env %s from system", common.EnvUfleetNodeId)
	}

	// 获取当前的Master节点
	masterNode := UfleetMasterNode{}
	if err := json.Unmarshal([]byte(values[common.UfleetMasterKey]), &masterNode); err != nil {
		return false, fmt.Errorf("can't not unmarshal ufleet master node: %s", err.Error())
	}

	// 对比两者，一致则为master节点
	if nodeId == masterNode.Id {
		return true, nil
	}

	return false, nil
}
