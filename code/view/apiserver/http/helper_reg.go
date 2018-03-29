package apiserver

import (
	"log"
	"ufleet/launcher/code/model/job"
)

// RegJob 注册新的job
func RegJob(tjob job.Job) error {
	logPrefix := "[ RegJob ]"

	// 获取所有job
	allJobs, err := job.GetAllJobs()
	if err != nil {
		log.Printf(err.Error())
	}

	// 检查job是否存在，若存在，在日志中显示相关信息并退出
	for _, oldJob := range allJobs {
		if oldJob.Type == tjob.Type && oldJob.Key == tjob.Key {
			log.Printf("%s job(%s) seems already exist, but not exit, be careful, this maybe will cause problem", logPrefix, tjob.Key)
		}
	}

	// 添加一个新的job
	err = job.SetWorkerJob(tjob)

	return err
}


