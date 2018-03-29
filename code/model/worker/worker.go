package worker

import (
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/base"
	"strconv"
	"strings"
	"fmt"
	"encoding/json"
)

// Node launcher node
type Worker struct {
	WorkerId  string `json:"workerid"` // worker-时间，如 worker-1653565445
	SyncVal int    `json:"syncval"`
	LostTimes int  `json:"losttimes"` // 防止抖动，添加lost次数的记录，当超过允许的最大值，才会被认为已经lost
}

// GetAllWorkers 获取所有workers
func GetAllWorkers() (allWorkers []Worker, err error) {
	key := strings.Join([]string{common.MemberKey, common.NewWorker}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return nil, err
	}
	for i := range values {
		oldWorker := Worker{}
		newErr := json.Unmarshal([]byte(values[i]), &oldWorker)
		if newErr != nil {
			err = fmt.Errorf(strings.Join([]string{err.Error()}, "in RegWorker, unmarshal failed: "+ err.Error()))
			continue
		}
		allWorkers = append(allWorkers, oldWorker)
	}
	return allWorkers, err
}

// SetWorker 增加worker，或者更新worker
func SetWorker(worker Worker) error {
	key := strings.Join([]string{common.MemberKey, common.NewWorker, worker.WorkerId}, common.Sep)
	value, err := json.Marshal(worker)
	if err != nil {
		return err
	}
	err = base.Put(key, string(value))

	return err
}

func RmWorker(worker Worker) error {
	key := strings.Join([]string{common.MemberKey, common.NewWorker, worker.WorkerId}, common.Sep)
	err := base.Delete(key)
	return err
}

// GetHeartbeatBase 获取基础心跳值
func GetHeartbeatBase() (int, error) {
	key := strings.Join([]string{common.MemberKey, common.Heartbeat, common.HeartbeatBase}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		// 获取心跳数据失败
		return -1, fmt.Errorf("get heartbeat failed: %s", err.Error())
	}
	hbb, err := strconv.Atoi(values[key])
	if err != nil {
		// 解析心跳数据失败
		return -1, fmt.Errorf("parse heartbeat failed: %s, %s", values[key], err.Error())
	}
	return hbb, nil
}

// SetHeartbeatBase 设置基础心跳值
func SetHeartbeatBase(value int) error {
	key := strings.Join([]string{common.MemberKey, common.Heartbeat, common.HeartbeatBase}, common.Sep)
	err := base.Put(key, strconv.Itoa(value))
	return err
}