
package scheduler

import (
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/ufleet"
	"ufleet/launcher/code/model/job"
	"ufleet/launcher/code/model/worker"
	"time"
	"strconv"
	"log"
)

// NewScheduler 启动新scheduler
func NewScheduler(){
	for {
		time.Sleep(time.Second * common.HeartGroupup)

		// 检查是否为主节点，否则继续检查
		isMaster, err := ufleet.IsMaster()
		if !isMaster {
			if err != nil {
				log.Println(err)
			}
			continue
		}

		// 执行scheduler任务
		DoScheduler()
		Update()
	}
}

// Scheduler 定时任务:
//     1.定时检查worker是否存活，如果已掉，将其相关的job置为未分配，并将其从列表中删除
//     2.为还没有分配的job分配对应的worker，如果没有存活的worker，在日志中显示相关信息
func DoScheduler() {
	logPrefix := "[ scheduler ]"

	aliveWorkers := make([]worker.Worker, 0, 20)
	lostWorkers  := make([]worker.Worker, 0, 10)
	newJobs      := make([]job.Job, 0, 20)

	// 获取所有worker节点，获取所有job信息
	// 从Etcd中获取所有任务
	allJobs, err := job.GetAllJobs()
	if err != nil {
		log.Printf("%s %s", logPrefix, err.Error())
	}
	if len(allJobs) == 0 {
		// no job, waiting for next time check.
		return
	}

	// 从Etcd中获取所有Worker
	allWorkers, err := worker.GetAllWorkers()
	if err != nil {
		log.Printf("%s %s", logPrefix, err.Error())
	}
	if len(allWorkers) == 0 {
		log.Printf("%s jobs' number: %d, but no worker.", logPrefix, len(allJobs))
		return
	}

	// 检查心跳信息，将已lost的worker，先将其相关的job置为未分配，再将其从列表中删除
	// 获取基础心跳
	base, err := worker.GetHeartbeatBase()
	if err != nil {
		// 获取基础心跳数据失败，重置基础心跳
		log.Printf("%s get base heartbeat failed: %s", logPrefix, err.Error())
		base = 0
	}

	// 获取已经lost的worker
	for i := range allWorkers {
		isLost := false
		if (allWorkers[i].SyncVal < base) && ((base - allWorkers[i].SyncVal) > common.HeartGroupup * 3) {
			allWorkers[i].LostTimes = allWorkers[i].LostTimes + 1
			if allWorkers[i].LostTimes > common.MaxLostTimes {
				isLost = true
				lostWorkers = append(lostWorkers, allWorkers[i])
			}
		} else {
			if allWorkers[i].LostTimes > 0 {
				log.Printf("%s worker(%s) is recover.", logPrefix, allWorkers[i].WorkerId)
			}
			allWorkers[i].LostTimes = 0
		}
		if !isLost {
			aliveWorkers = append(aliveWorkers, allWorkers[i])
		}
	}
	// 将已经lost的job重置为未分配状态
	for i := range allJobs {
		for _, lostWorker := range lostWorkers {
			if allJobs[i].WorkerId == lostWorker.WorkerId {
				log.Printf("%s worker(%s) lost, job(%s) will be redistributed to new worker.", logPrefix, allJobs[i].WorkerId, allJobs[i].JobId)
				allJobs[i].WorkerId = ""
			}
		}
	}

	// 过滤出所有未分配的任务
	for i := range allJobs {
		if allJobs[i].WorkerId == "" {
			allJobs[i].Status = ""
			newJobs = append(newJobs, allJobs[i])
		}
	}

	// 显示当前的任务信息
	log.Printf("[ scheduler ] workers' number: " + strconv.Itoa(len(allWorkers)) +
		", alive worker: " + strconv.Itoa(len(aliveWorkers)) +
		", jobs' number: " + strconv.Itoa(len(allJobs)) +
		", new jobs: " + strconv.Itoa(len(newJobs)))

	// 逐个给未分配worker的job分配worker
	for i := range newJobs {
		if newJobs[i].WorkerId == "" {
			suitableWorker, found := GetSuitableWorker(allJobs, aliveWorkers)
			if !found {
				log.Printf("%s can not get a worker for job(%s), waiting for next time.", logPrefix, newJobs[i].JobId)
				continue
			}
			log.Printf("%s distribute job(%s) to worker(%s).", logPrefix, newJobs[i].JobId, suitableWorker.WorkerId)
			newJobs[i].WorkerId = suitableWorker.WorkerId
		}
	}

	// 写入到Etcd中
	// 更新worker
	for i:=0;i<len(aliveWorkers);i++ {
		if err = worker.SetWorker(aliveWorkers[i]); err != nil {
			log.Printf(err.Error())
		}
	}
	// 删除已经lost的worker
	for i:=0;i<len(lostWorkers);i++ {
		err = worker.RmWorker(lostWorkers[i])
		if err != nil {
			log.Printf("%s delete worker(%s) failed: %s", logPrefix, lostWorkers[i].WorkerId, err.Error())
		}
	}
	// 更新job
	for i := range newJobs {
		if err = job.SetWorkerJob(newJobs[i]); err != nil {
			log.Printf("%s update job(%s) status failed: %s", logPrefix, newJobs[i].JobId, err.Error())
		}
	}
}

// Update 定时任务
//     1. 定时更新基础心跳，基础心跳将作为worker更新心跳值的基础，也是判断worker是否lost的判断依据
func Update(){
	logPrefix := "[ scheduler ]"

	// 获取基础心跳
	base, err := worker.GetHeartbeatBase()
	if err != nil {
		// 获取基础心跳数据失败，重置基础心跳
		log.Printf("%s get base heartbeat failed: %s", logPrefix, err.Error())
		base = 0
	}

	// 心跳自然增长，并写入到Etcd中
	base = base + common.HeartGroupup
	err = worker.SetHeartbeatBase(base)
	if err != nil {
		log.Printf("%s set base heartbeat failed: %s", logPrefix, err.Error())
	}
}

// GetSuitableWorker 为job获取合适的worker
func GetSuitableWorker(allJobs []job.Job, allWorkers []worker.Worker) (worker.Worker, bool) {
	if len(allJobs) == 0 || len(allWorkers) == 0 {
		// 找不到合适的worker
		return worker.Worker{}, false
	}

	// 统计那个worker身上的任务最少
	count := make([]int, len(allWorkers))
	for i:=0;i<len(allJobs);i++ {
		if allJobs[i].WorkerId == "" {
			continue
		}
		for j:=0;j<len(allWorkers);j++ {
			if allJobs[i].WorkerId == allWorkers[j].WorkerId {
				count[j] = count[j] + 1
				break
			}
		}
	}
	min := 0
	for i:=1; i<len(allWorkers);i++ {
		if count[i] < count[min] {
			min = i
		}
	}

	// 返回最少任务的worker
	return allWorkers[min], true
}
