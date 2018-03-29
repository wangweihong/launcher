package job

import (
	"ufleet/launcher/code/model/base"
	"ufleet/launcher/code/model/common"
	"strings"
	"encoding/json"
	"fmt"
)

//Job 安装，添加
type Job struct {
	JobId  string  // 操作+资源类型+名称+时间，如 create-cluster-<clustername>-16562653
	Type   common.JobType   `json:"type"`
	Key    string    		`json:"key"`    // 对于cluster则为clustername，对于master、node、etcd，则为： clustername/masterip, clustername/nodeip, clustername/etcdip
	WorkerId string  		`json:"workerid"` // 如果为空，表示还没有分配，否则，应该为workerid
	Status string    		`json:"status"`
}

// GetAllJobs 获取所有jobs
func GetAllJobs() (allJobs []Job, err error) {
	key := strings.Join([]string{common.MemberKey, common.NewJobs}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return nil, fmt.Errorf("get jobs from etcd failed: %s", err.Error())
	}
	for tempJob := range values {
		job := Job{}
		if newErr := json.Unmarshal([]byte(values[tempJob]), &job); newErr != nil {
			err = fmt.Errorf(strings.Join([]string{err.Error()}, newErr.Error()))
			continue
		}
		allJobs = append(allJobs, job)
	}
	return
}

// SetWorkerJob 添加worker job，或者设置worker job
func SetWorkerJob(job Job) error {
	key := strings.Join([]string{common.MemberKey, common.NewJobs, job.JobId}, common.Sep)
	newValue, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job failed: " + err.Error())
	}
	if err = base.Put(key, string(newValue)); err != nil {
		return fmt.Errorf("set job failed: %s", err.Error())
	}
	return nil
}

// rmCompleteJob 删除已经完成的job
func RmCompleteJob(job Job) error {
	// 删除已经完成的任务
	key := strings.Join([]string{common.MemberKey, common.NewJobs, job.JobId}, common.Sep)
	err := base.Delete(key)
	if err == nil {
		return nil
	}
	return fmt.Errorf("job(%s) is finished, and but remove this job failed: %s", job.JobId, err.Error())
}
