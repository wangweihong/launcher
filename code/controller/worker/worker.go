package worker

import (
	"strings"
	"time"
	"strconv"
	"fmt"
	"log"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/job"
	"ufleet/launcher/code/controller/cluster"
	"ufleet/launcher/code/controller/federation"
)

// NewWorker 启动新的worker
func NewWorker(){
	worker := Worker{}
	worker.WorkerId = "worker-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	worker.SyncVal = 0
	worker.LostTimes = 0

	// 注册新的worker
	RegWorker(worker)

	for {
		// 启动心跳
		UpdateWorkerHealth(worker)

		// 循环检查job
		DoJob(worker)

		time.Sleep(time.Second * common.HeartGroupup)
	}
}


// RegWorker 注册worker节点
func RegWorker(worker Worker) error {
	logPrefix := fmt.Sprintf("[ %s ]", worker.WorkerId)
	// 获取当前所有worker，并检查worker是否存在，若存在，在日志中显示相关信息并退出
	allWorkers, err := GetAllWorkers()
	if err != nil {
		log.Printf("%s %s", logPrefix, err.Error())
	}

	// 检查worker是否存在，若存在，在日志中显示相关信息并退出
	for _, oldWorker := range allWorkers {
		if oldWorker.WorkerId == worker.WorkerId {
			log.Printf("%s worker(%s) already exist", logPrefix, worker.WorkerId)
			return nil
		}
	}

	// 添加新的worker
	err = SetWorker(worker)

	return err
}

// UpdateWorkerHealth worker心跳
func UpdateWorkerHealth(worker Worker) {
	logPrefix := fmt.Sprintf("[ %s ]", worker.WorkerId)

	// 从ETCD中获取基础心跳值
	base, err := GetHeartbeatBase()
	if err != nil {
		log.Printf("%s get base heartbeat failed: %s", logPrefix, err.Error())
		base = 0
	}

	// 增长并写入心跳值
	newHeartbeat := base + common.HeartGroupup
	worker.SyncVal = newHeartbeat
	if err := SetWorker(worker); err != nil {
		// 设置心跳失败，等待下次心跳再次获取
		log.Printf("%s set worker heartbeat failed: %s", logPrefix, err.Error())
	}
}

// DoJob 每个worker的监控函数，从etcd中获取自己的任务，并执行
func DoJob(worker Worker){
	logPrefix := fmt.Sprintf("[ %s ]", worker.WorkerId)
	defer func(){
		if err := recover(); err != nil {
			errMsg := fmt.Sprint(err)
			log.Printf("%s I'm panic: %s", logPrefix, errMsg)
		}
	}()

	// 从Etcd中获取所有任务
	allJobs, err := job.GetAllJobs()
	if err != nil {
		log.Printf(err.Error())
	}
	if len(allJobs) == 0 {
		// no job, waiting for next time check.
		return
	}

	// 过滤出属于自己的任务以及新的任务
	newJobs := make([]job.Job, 0, 0)
	runningJob := 0
	for _, tempJob := range allJobs {
		if tempJob.WorkerId != worker.WorkerId {
			continue
		}
		if tempJob.Status == "" {
			newJobs = append(newJobs, tempJob)
		}else{
			runningJob += 1
		}
	}

	// if no my job, waiting for next time check.
	if runningJob == 0 && len(newJobs) == 0 {
		return
	}

	// 执行任务
	log.Printf("%s new jobs: %d, running job: %d", logPrefix, len(newJobs), runningJob)
	for i := range newJobs {
		// 检查是否是未执行的任务，不是跳过
		if newJobs[i].Status != "" {
			continue
		}

		switch newJobs[i].Type {
		case common.ClusterCreate:
			clu, err := cluster.GetCluster(newJobs[i].Key)
			if err != nil {
				log.Printf("%s could not found cluster: %s, seems already removed, will delete this job", logPrefix, newJobs[i].Key)
				if err := job.RmCompleteJob(newJobs[i]); err != nil {
					log.Printf("%s remove job(%s, %s) failed", logPrefix, newJobs[i].JobId, newJobs[i].Key)
				}
				continue
			}

			// 创建Cluster任务
			log.Printf("%s begin create cluster job: %s", logPrefix, newJobs[i].Key)
			newJobs[i].Status = "append"
			// 写入Etcd中
			if err := job.SetWorkerJob(newJobs[i]); err != nil {
				log.Printf("%s %s", logPrefix, err.Error())
				continue
			}
			go func(tempJob job.Job){
				defer func(){
					// 删除已经完成的任务
					for {
						err := job.RmCompleteJob(tempJob)
						if err == nil {
							break
						}
						time.Sleep(time.Second * common.HeartGroupup)
					}
				}()

				cluster.CreateCluster(clu)
			}(newJobs[i])

		case common.NodeAdd:
			values := strings.Split(newJobs[i].Key, "/")
			if len(values) < 2 {
				log.Printf("%s node key is no right: %s, will delete this job.", logPrefix, newJobs[i].Key)
				if err := job.RmCompleteJob(newJobs[i]); err != nil {
					log.Printf("%s remove job(%s, %s) failed.", logPrefix, newJobs[i].JobId, newJobs[i].Key)
				}
				continue
			}
			clustername := values[0]
			nodename := values[1]
			node, found := cluster.GetNode(clustername, nodename)
			if !found {
				log.Printf("%s could not found node: %s, will delete this job", logPrefix, newJobs[i].Key)
				if err := job.RmCompleteJob(newJobs[i]); err != nil {
					log.Printf("%s remove job(%s, %s) failed", logPrefix, newJobs[i].JobId, newJobs[i].Key)
				}
				continue
			}

			// 创建node任务
			log.Printf("%s begin create node job: %s", logPrefix, newJobs[i].Key)
			newJobs[i].Status = "append"
			// 写入Etcd中
			if err := job.SetWorkerJob(newJobs[i]); err != nil {
				log.Printf("%s %s", logPrefix, err.Error())
				continue
			}
			go func(tempJob job.Job, node cluster.Node){
				defer func(){
					// 删除已经完成的任务
					for {
						err := job.RmCompleteJob(tempJob)
						if err == nil {
							break
						}
						log.Printf(err.Error())
						time.Sleep(time.Second * common.HeartGroupup)
					}
				}()

				clu, err := cluster.GetCluster(node.ClusterName)
				if err != nil {
					log.Println("could not get node's cluster, because: ", err)
				}
				clu.AddNode(&node)
			}(newJobs[i], node)
		case common.MasterAdd:
			values := strings.Split(newJobs[i].Key, "/")
			if len(values) < 2 {
				log.Printf("%s master key is no right: %s, will delete this job", logPrefix, newJobs[i].Key)
				if err := job.RmCompleteJob(newJobs[i]); err != nil {
					log.Printf("%s remove job(%s, %s) failed", logPrefix, newJobs[i].JobId, newJobs[i].Key)
				}
				continue
			}
			clustername := values[0]
			mastername := values[1]
			master, found := cluster.GetMaster(clustername, mastername)
			if !found {
				log.Printf("%s could not found master: %s, will delete this job", logPrefix, newJobs[i].Key)
				if err := job.RmCompleteJob(newJobs[i]); err != nil {
					log.Printf("%s remove job(%s, %s) failed", logPrefix, newJobs[i].JobId, newJobs[i].Key)
				}
				continue
			}

			// 创建node任务
			log.Printf("%s begin add master job: %s", logPrefix, newJobs[i].Key)
			newJobs[i].Status = "append"
			// 写入Etcd中
			if err := job.SetWorkerJob(newJobs[i]); err != nil {
				log.Printf("%s %s", logPrefix, err.Error())
				continue
			}

			go MstTakeJob(master, newJobs[i])
		case common.FederationCreate:
			// 创建node任务
			log.Printf("%s begin create federation job: %s", logPrefix, newJobs[i].Key)
			newJobs[i].Status = "append"
			// 写入Etcd中
			if err := job.SetWorkerJob(newJobs[i]); err != nil {
				log.Printf("%s %s", logPrefix, err.Error())
				continue
			}

			go FCTakeJob(newJobs[i])
		default:
			log.Printf("%s unkown job type, the key is: %s", logPrefix, newJobs[i].Key)
		}
	}
}

func MstTakeJob(master cluster.Master, tjob job.Job) error {
	defer func(){
		// 删除已经完成的任务
		for {
			err := job.RmCompleteJob(tjob)
			if err == nil {
				break
			}
			log.Printf(err.Error())
			time.Sleep(time.Second * common.HeartGroupup)
		}
	}()

	clu, err := cluster.GetCluster(master.ClusterName)
	if err != nil {
		return err
	}
	clu.AddMaster(&master)

	return nil
}

func FCTakeJob(tjob job.Job) error {
	defer func(){
		// 删除已经完成的任务
		for {
			err := job.RmCompleteJob(tjob)
			if err == nil {
				break
			}
			log.Printf(fmt.Sprintf("[ %s ][ %s ]", tjob.JobId, err.Error()))
			time.Sleep(time.Second * common.HeartGroupup)
		}
	}()

	fed, found := federation.GetFederation(tjob.Key)
	if !found {
		return common.NotFound
	}

	fed.Create()
	return nil
}