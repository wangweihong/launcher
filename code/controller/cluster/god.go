package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
	"ufleet/launcher/code/config"
	"ufleet/launcher/code/model/3party/etcd"
	"ufleet/launcher/code/model/base"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/utils"
)

// CreateCluster 创建cluster入口
// TODO: 需添加超时限制
func CreateCluster(clu *Cluster) {
	var numAllSteps int = 17
	var countStep int = 0

	logPrefix := fmt.Sprintf("[ %s ][ CreateCluster ]", clu.Name)
	// Step : 初始化Cluster集群和所有Master节点状态
	// 由于在apiserver入口处已经做了检查，到这里可以保证集群不存在，故不再做集群是否存在的检查

	// Step : check version and others
	if len(clu.K8sVersion) == 0 {
		clu.Status = common.GlobalClusterStatusFailed
		clu.ErrorMsg = "k8s version should be specified."
		log.Printf("%s %s", logPrefix, clu.ErrorMsg)
		clu.saveStatusIfExist()
		return
	}

	// Step : 存储集群节点个数，作为单master节点和多master节点的判断，方便后续判断
	clu.BaseMasters = len(clu.Masters)
	if clu.BaseMasters <= 0 {
		clu.Status = common.GlobalClusterStatusFailed
		clu.ErrorMsg = "cluster master number could not be empty."
		log.Printf("%s %s", logPrefix, clu.ErrorMsg)
		clu.saveStatusIfExist()
		return
	}

	// Step : 创建客户端连接
	for i := range clu.Masters {
		if _, err := clu.Masters[i].GetSSHClient(); err != nil {
			clu.Status = common.GlobalClusterStatusFailed
			clu.ErrorMsg = fmt.Sprintf("%s get ssh client failed. ErrorMsg: %s", clu.Masters[i].HostIP, err.Error())
			log.Printf("%s %s", logPrefix, clu.ErrorMsg)
			clu.saveStatusIfExist()
			break
		}
		go func(master *Master) {
			logPrefix := fmt.Sprintf("[ %s ][ %s ][ CreateCluster ]", master.ClusterName, master.HostIP)
			// Timeout.
			time.Sleep(time.Minute * 30)
			if master.Status == common.GlobalNodeStatusPending {
				log.Printf("%s Maybe something wrong, this task already took half hours, and I am going warn user!", logPrefix)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "the machine is really slow for me to create master, anyway, I will continue.")
			}
		}(&clu.Masters[i])
	}
	if clu.Status == common.GlobalClusterStatusFailed {
		clu.setMastersStatusIfExist(common.GlobalNodeStatusFailed, clu.ErrorMsg)
		return
	}
	defer func() {
		// Step : 释放客户端连接
		for i := range clu.Masters {
			if err := clu.Masters[i].releaseSSHClient(); err != nil {
				clu.Status = common.GlobalClusterStatusFailed
				clu.ErrorMsg = fmt.Sprintf("%s release ssh client failed. ErrorMsg: %s", clu.Masters[i].HostIP, err.Error())
				log.Printf("%s %s", logPrefix, clu.ErrorMsg)
				clu.saveStatusIfExist()
			}
		}
	}()

	// master节点大于1时，需要执行此项严格检查
	log.Printf("%s check master param", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	if clu.BaseMasters > 1 {
		err := clu.strictCheck()
		if err != nil {
			errMsg := "strict check failed. ErrorMsg: " + err.Error()
			log.Printf("%s %s", logPrefix, errMsg)
			clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
			return
		}
	} else {
		clu.Vip = clu.Masters[0].HostIP
	}

	// Step : 卸载，清理远程机器
	log.Printf("%s clean machines", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	var wg sync.WaitGroup
	wg.Add(clu.BaseMasters)
	for i := range clu.Masters {
		go func(m Master) {
			defer wg.Done()
			// 卸载
			m.RemoveMaster()
		}(clu.Masters[i])
	}
	// 等待全部卸载任务完成
	wg.Wait()

	// Step 00: 检查并设置状态， 设置各节点主机名等初始化操作
	log.Printf("%s pre init", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err := clu.preInit()
	if err != nil {
		errMsg := "pre init failed. ErrorMsg: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// Step : 获取 JoinToken
	err = clu.genJoinToken()
	log.Printf("%s get join token", logPrefix)
	if err != nil {
		errMsg := "generate join token failed. ErrorMsg: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// Step : 生成证书、管理配置文件
	// 单master节点和多master节点最大的区别就在于配置文件上。
	log.Printf("%s generate cert and config files", logPrefix)
	configTempDir := fmt.Sprintf("%s/%s", config.GDefault.LocalTempDir, strconv.FormatInt(clu.CreateTime, 10)) // 证书临时目录，在保存到 Etcd后删除
	err = clu.genCert(configTempDir)
	if err != nil {
		errMsg := fmt.Sprintf("generate cert failed. ErrorMsg: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalNodeStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 将证书信息写入到etcd中
	log.Printf("%s write cluster information into etcd", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	for i := range clu.Masters {
		clu.Masters[i].saveStatusIfExist()
	}
	clu.saveStatusIfExist()

	// Step : 生成 etcd 和 k8s kubeadm 配置文件
	// 单master节点和多master节点最大的区别就在于配置文件上。
	if clu.BaseMasters == 1 {
		err = clu.	genAloneConfig(configTempDir)
	} else {
		err = clu.genConfig(configTempDir)
	}
	if err != nil {
		errMsg := fmt.Sprintf("generate config failed. ErrorMsg: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// Step : 发送文件到各主机
	log.Printf("%s send files to remote machine", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = clu.sendFilesToRemote(configTempDir)
	if err != nil {
		errMsg := fmt.Sprintf("send file to remote master machine failed. ErrorMsg: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// Step : 导入ufleet 镜像仓库ca证书
	log.Printf("%s import register ca cert", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	wg.Add(clu.BaseMasters)
	for i := range clu.Masters {
		go func(master *Master) {
			defer wg.Done()

			sshClient, err := master.GetSSHClient()
			if err != nil {
				errMsg := fmt.Sprintf("send ssh client failed. ErrorMsg: %s", err.Error())
				log.Printf("%s[ %s ] %s", logPrefix, master.HostIP, errMsg)
				clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
				return
			}
			err = clu.Masters[i].importRegistryCa(sshClient)
			if err != nil {
				errMsg := fmt.Sprintf("import register ca failed. ErrorMsg: %s", err.Error())
				log.Printf("%s[ %s ] %s", logPrefix, master.HostIP, errMsg)
				clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
				return
			}

			// save in db
			clu.Masters[i].saveStatusIfExist()
		}(&clu.Masters[i])
	}

	// Step : 部署 ETCD集群
	log.Printf("%s begin create etcd cluster", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = clu.createEtcdCluster()
	if err != nil {
		errMsg := fmt.Sprintf("create etcd cluster failed. ErrorMsg: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// Step : 等待 ETCD集群完成初始化和选举leader
	IPs := make([]string, 0, 3)
	for i := range clu.Masters {
		IPs = append(IPs, clu.Masters[i].HostIP)
	}
	lEtcd := etcd.LEtcd{}
	err = lEtcd.Init(IPs)
	if err != nil {
		log.Printf("%s get etcd client failed: %s", logPrefix, err.Error())
	} else {
		err = lEtcd.WaitUntilReady()
		if err != nil {
			log.Printf("%s wait for etcd ready failed: %s", logPrefix, err.Error())
		} else {
			log.Printf("%s etcd cluster status: ready", logPrefix)
		}
	}

	// Step : 创建 k8s 集群
	log.Printf("%s begin create k8s cluster", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	wg.Add(clu.BaseMasters)
	for i := range clu.Masters {
		newCountStep := countStep
		go func(master *Master, isLeader bool) {
			defer wg.Done()
			logPrefix := fmt.Sprintf("[ %s ][ %s ][ CreateMaster ]", clu.Name, master.HostIP)
			log.Printf("%s get ssh client", logPrefix)
			sshClient, err := master.GetSSHClient()
			if err != nil {
				log.Printf("%s get ssh client failed! ErrorMsg: %s", logPrefix, err.Error())
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "get ssh client failed! ErrorMsg: "+err.Error())
				return
			}

			log.Printf("[ %s ][ %s ][ CreateMaster ] execute master/install.sh", clu.Name, master.HostIP)
			clu.exitIfRecreate("", false)
			master.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { newCountStep = newCountStep + 1; return newCountStep }(), numAllSteps), true)
			logDir := config.GDefault.RemoteLogDir + "/master"
			getErrMsgCmd := fmt.Sprintf("errMsg=$(tail -n 2 %s/install.log | head -n 1) && echo $errMsg | tr '[' '<' | tr ']' '>' | sed 's/<[^>]*>*//g'", logDir)
			cmd := fmt.Sprintf("mkdir -p %s && cd %s/script/master/ && /bin/bash install.sh &> %s/install.log", logDir, config.GDefault.RemoteTempDir, logDir)
			if _, err = utils.Execute(cmd, sshClient); err != nil {
				// 安装 Master 节点失败
				errMsg, _ := utils.Execute(getErrMsgCmd, sshClient)
				log.Printf("%s install master node failed! ErrorMsg: %s", logPrefix, errMsg)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "install master node failed! ErrorMsg: "+errMsg)
				return
			}

			// 安装coredns
			if isLeader {
				log.Printf("[ %s ][ %s ][ CreateMaster ] deploy coredns", clu.Name, master.HostIP)
				clu.exitIfRecreate("", false)
				master.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { newCountStep = newCountStep + 1; return newCountStep }(), numAllSteps), true)
				for i := 0; i < 180; i++ {
					cmd = fmt.Sprintf("cd %s/script/coredns && chmod a+rx ./deploy.sh && "+
						"./deploy.sh 10.96.0.0/16 10.244.0.0/16 | kubectl apply -f - &> %s/install.log", config.GDefault.RemoteTempDir, logDir)
					_, err = utils.Execute(cmd, sshClient)
					if err == nil {
						break
					}
					time.Sleep(time.Second)
				}
				if err != nil {
					// 部署coredns失败
					errMsg := fmt.Sprintf("deploy coredns failed")
					log.Printf("%s %s", logPrefix, errMsg)
					master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
					return
				}
			}

			log.Printf("%s install essential addons", logPrefix)
			clu.exitIfRecreate("", false)
			master.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { newCountStep = newCountStep + 1; return newCountStep }(), numAllSteps), true)
			logDir = config.GDefault.RemoteLogDir + "/addon"
			getErrMsgCmd = fmt.Sprintf("errMsg=$(tail -n 2 %s/addonctl.log | head -n 1) && echo $errMsg | "+
				"tr '[' '<' | tr ']' '>' | sed 's/<[^>]*>*//g'", logDir)
			cmd = fmt.Sprintf("mkdir -p %s && cd %s/script/addon/ && "+
				"./addonctl -a install -m %s -logID %s > %s/addonctl.log 2>&1", logDir, config.GDefault.RemoteTempDir, master.HostIP, master.HostIP, logDir)
			if _, err = utils.Execute(cmd, sshClient); err != nil {
				// 安装 addon 失败
				errMsg, _ := utils.Execute(getErrMsgCmd, sshClient)
				log.Printf("%s  install essential addons failed! ErrorMsg: %s", logPrefix, errMsg)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "install essential addons failed! ErrorMsg: "+errMsg)
				return
			}

			log.Printf("%s Get admin config, cert and key from system.", logPrefix)
			adminConfig, err := master.getAdminConfig(sshClient)
			if err != nil {
				// 获取 admin config失败
				errMsg := fmt.Sprintf("get admin config from system failed! ErrorMsg: %s", err.Error())
				log.Printf("%s %s", logPrefix, errMsg)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
				return
			}
			clu.AdminContext = adminConfig
			if err = master.getCertAndKeys(sshClient); err != nil {
				errMsg := fmt.Sprintf("get apiserver cert and key failed! ErrorMsg: %s", err.Error())
				log.Printf("%s %s", logPrefix, errMsg)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
				return
			}

			log.Printf("%s install vespace strategy", logPrefix)
			clu.exitIfRecreate("", false)
			master.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { newCountStep = newCountStep + 1; return newCountStep }(), numAllSteps), true)
			logDir = config.GDefault.RemoteLogDir + common.GlobalConfigVespacePath
			getErrMsgCmd = fmt.Sprintf("errMsg=$(tail -n 2 %s/strategy.log | head -n 1) && echo $errMsg | "+
				"tr '[' '<' | tr ']' '>' | sed 's/<[^>]*>*//g'", logDir)
			cmd = fmt.Sprintf("mkdir -p %s && cd %s%s && /bin/bash vespace.sh -m %s -logID %s > %s/strategy.log 2>&1",
				logDir, config.GDefault.RemoteTempDir, common.GlobalConfigVespacePath, master.HostIP, master.HostIP, logDir)
			if _, err = utils.Execute(cmd, sshClient); err != nil {
				// 安装 vespace strategy 失败
				errMsg, _ := utils.Execute(getErrMsgCmd, sshClient)
				log.Printf("%s  install vespace strategy failed! ErrorMsg: %s", logPrefix, errMsg)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "install vespace strategy failed! ErrorMsg: "+errMsg)
				return
			}

			// 清理已退出的容器
			log.Printf("%s clean already exited containers.", logPrefix)
			clu.exitIfRecreate("", false)
			master.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { newCountStep = newCountStep + 1; return newCountStep }(), numAllSteps), true)
			cmd = "docker ps -a  | grep -v grep | grep \"Exited\" | awk '{print $1}' | xargs docker rm"
			utils.Execute(cmd, sshClient)

			log.Printf("%s Done.", logPrefix)
			// 成功暂不修改为完成状态，待检查vip后再统一修改

			countStep = newCountStep
			return
		}(&clu.Masters[i], i == 0)
	}
	// 等待 Masters创建完成
	wg.Wait()

	// Step : Modify configmap for ha clusters, and use vip instead masters' hostip
	if clu.BaseMasters > 1 {
		sshClient, err := clu.Masters[0].GetSSHClient()
		if err != nil {
			log.Printf("%s get ssh client failed! ErrorMsg: %s", logPrefix, err.Error())
			clu.Masters[0].saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "get ssh client failed! ErrorMsg: "+err.Error())
		} else {
			cmd := fmt.Sprintf("chmod a+rx %s/script/master/updateConfigmap.sh && %s/script/master/updateConfigmap.sh https://%s:%d %s", config.GDefault.RemoteTempDir, config.GDefault.RemoteTempDir, clu.Vip, 6443, clu.Vip)
			if resp, err := utils.Execute(cmd, sshClient); err != nil {
				// update configmap failed
				log.Printf("%s  update configmap failed! ErrorMsg: %s, %s", logPrefix, resp, err)
				clu.Masters[0].saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "pdate configmap failed! ErrorMsg: "+resp+", "+err.Error())
			}
		}
	}

	// Step : 检查各个Master节点的状态
	for i := range clu.Masters {
		if clu.Masters[i].Status == common.GlobalNodeStatusFailed {
			// 有一个节点创建失败，即集群创建失败
			errMsg := clu.Masters[i].HostIP + " status is failed."
			log.Printf("%s %s", logPrefix, errMsg)
			clu.setNotFailedMastersStatusIfExist(common.GlobalNodeStatusFailed, errMsg)
			clu.ErrorMsg = errMsg
			clu.Status = common.GlobalClusterStatusFailed
			clu.saveStatusIfExist()
			return
		}
	}

	// Step : 保存到 Etcd并清理环境
	clu.cleanEnv(configTempDir)

	logPrefix = fmt.Sprintf("[ %s ][ CreateCluster ]", clu.Name)

	// Step : 使用集群IP测试APIServer
	log.Printf("%s start check apiserver", logPrefix)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	if err = APICheck(clu.CluInfo.CaCert, clu.CluInfo.APIClientCert, clu.CluInfo.APIClientKey, "https://"+clu.CluInfo.Vip+":6443", config.GK8sDefault.TimesOfCheckApiserver); err == nil {
		log.Printf("%s check apiserver ok.", logPrefix)
	} else {
		errMsg := fmt.Sprintf("Check apiserver failed! Check apiserver time out or vip is invalid(HA mode: "+
			"vip(%s) must on the same network segment as the masters and has not been used by others)", clu.CluInfo.Vip)
		log.Printf("%s %s", logPrefix, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 打上标签
	log.Printf("%s set master's labels", logPrefix)
	clu.exitIfRecreate("", false)
	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	for _, master := range clu.Masters {
		if err = master.setMasterLabel(); err != nil {
			// 设置标签失败
			log.Printf("%s set master's labels failed! ErrorMsg: %s", logPrefix, err.Error())
			master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "install set master's labels failed! ErrorMsg: "+err.Error())
			return
		}
	}

	clu.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	clu.readyToExit(common.GlobalClusterStatusRunning, "", common.GlobalNodeStatusRunning, "")
	log.Printf("%s Done.", logPrefix)
	return
}

// RemoveCluster 卸载cluster
func RemoveCluster(clustername string) {
	masters, exist := GetMasters(clustername)
	if exist {
		for _, master := range masters {
			go func(master Master) {
				master.Status = common.GlobalClusterStatusPending
				master.SaveStatus()
				master.RemoveMaster()
			}(master)
		}
	}
	DeleteCluster(clustername)
}

// CheckCluster check if cluster exist in etcd
func CheckCluster(clusterName string) bool {
	key := strings.Join([]string{common.ClusterKey, clusterName}, common.Sep)
	exist, _ := base.IsExist(key)
	return exist
}

// CheckMaster check if master node exist in cluster
func CheckMaster(clusterName, hostip string) bool {
	key := strings.Join([]string{common.ClusterKey, clusterName, common.MasterKey, hostip}, common.Sep)
	exist, _ := base.IsExist(key)
	return exist
}

// CheckNode check if node exist in cluster
func CheckNode(clusterName, hostip string) bool {
	key := strings.Join([]string{common.ClusterKey, clusterName, common.NodeKey, hostip}, common.Sep)
	exist, _ := base.IsExist(key)
	return exist
}

// GetClusterObject 根据cluster.Name获取cluster对象
func GetCluster(clustername string) (*Cluster, error) {
	cluValue, found := GetClusterDetail(clustername)
	if !found {
		// 集群不存在
		return nil, fmt.Errorf("cluster:%s not exist", clustername)
	}
	clu := new(Cluster)
	clu.Name = clustername
	clu.BaseMasters, _ = strconv.Atoi(cluValue["basemasters"])
	clu.Status = cluValue["status"]
	clu.ErrorMsg = cluValue["errormsg"]
	// 获取master信息
	masterValue, found := GetMasters(clu.Name)
	if !found || len(masterValue) == 0 {
		return nil, fmt.Errorf("can't find any master node")
	}
	for _, master := range masterValue {
		clu.Masters = append(clu.Masters, master)
	}
	nodeValue, found := GetNodes(clu.Name)
	if found {
		for key := range nodeValue {
			clu.Nodes = append(clu.Nodes, nodeValue[key])
		}
	}
	sCluster, err := GetStorageFromDB(clu.Name)
	if err == nil {
		clu.SClusters = append(clu.SClusters, sCluster...)
	}
	pImageInfo := GetImagesInfo(clu.Name)
	if pImageInfo != nil {
		clu.Images = pImageInfo
	}
	// 获取cluInfo信息
	pCluInfo := GetClusterInfo(clu.Name)
	if pCluInfo == nil {
		return nil, fmt.Errorf("can't get cluster info")
	}
	clu.CluInfo = *pCluInfo
	return clu, nil
}

// GetCluster get a specfic cluster by clustername from etcd
func GetClusterDetail(clusterName string) (map[string]string, bool) {
	v := map[string]string{}
	if !CheckCluster(clusterName) {
		log.Printf("CheckCluster failed: %s", clusterName)
		return v, false
	}
	key := strings.Join([]string{common.ClusterKey, clusterName}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		log.Printf("db Get %s failed.", key)
		return v, false
	}

	for key := range values {
		basekeyLen := len(common.ClusterKey + "/" + clusterName + "/")
		if len(key) <= basekeyLen {
			continue
		}
		validKey := key[basekeyLen:]
		etcdPaths := strings.Split(validKey, "/")
		if len(etcdPaths) <= 0 || etcdPaths[0] == "" {
			continue
		}
		etcdKey := etcdPaths[0]
		_, ok := v[etcdKey]
		if ok {
			// 已经解析过，不需要再次解析
			continue
		}
		v[etcdKey], _ = values[key]
	}
	return v, true
}

// GetClusters get all clusters from etcd
func GetClusters() (map[string]Cluster, bool) {
	v := map[string]Cluster{}
	values, err := base.Get(common.ClusterKey)
	if err != nil {
		return v, false
	}
	if len(values) <= 0 {
		return v, true
	}

	for key := range values {
		baseLen := len(common.ClusterKey + "/")
		if len(key) <= baseLen {
			continue
		}
		validKey := key[baseLen:]
		etcdPaths := strings.Split(validKey, "/")
		if len(etcdPaths) <= 0 || etcdPaths[0] == "" {
			continue
		}
		etcdKey := etcdPaths[0]
		_, ok := v[etcdKey]
		if ok {
			// 已经解析过，不需要再次解析
			continue
		}
		clu, err := GetCluster(etcdKey)
		if err == nil {
			v[etcdKey] = *clu
		}
	}
	return v, true
}

// GetMaster get a specfic master by masterip and clustername from etcd
func GetMaster(clusterName, masterip string) (Master, bool) {
	v := Master{}
	if !CheckMaster(clusterName, masterip) {
		return v, false
	}
	key := strings.Join([]string{common.ClusterKey, clusterName, common.MasterKey, masterip}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return v, false
	}

	if values != nil && values[key] != "" {
		_ = json.Unmarshal([]byte(values[key]), &v)
	} else {
		return v, false
	}
	return v, true
}

// GetMasters get all masters of a specfic cluster from etcd
func GetMasters(clusterName string) (map[string]Master, bool) {
	v := map[string]Master{}
	if !CheckCluster(clusterName) {
		return v, false
	}
	key := strings.Join([]string{common.ClusterKey, clusterName, common.MasterKey}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return v, false
	}

	for key := range values {
		path := strings.Split(key, "/")
		hostip := path[len(path)-1]
		v[hostip], _ = GetMaster(clusterName, hostip)
	}
	return v, true
}

// GetNode get a specfic node data by nodename and clustername from etcd
func GetNode(clusterName, hostip string) (Node, bool) {
	v := Node{}
	if !CheckNode(clusterName, hostip) {
		return v, false
	}
	key := strings.Join([]string{common.ClusterKey, clusterName, common.NodeKey, hostip}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return v, false
	}

	if values != nil && values[key] != "" {
		// fmt.Println(res.Node.Value)
		// 获取到value
		_ = json.Unmarshal([]byte(values[key]), &v)
	} else {
		return v, false
	}

	return v, true
}

// GetNodes get all nodes of a specfic cluster from etcd
func GetNodes(clusterName string) (map[string]Node, bool) {
	v := map[string]Node{}
	if !CheckCluster(clusterName) {
		return v, false
	}
	key := strings.Join([]string{common.ClusterKey, clusterName, common.NodeKey}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return v, false
	}

	for key := range values {
		// key
		path := strings.Split(key, "/")
		hostip := path[len(path)-1]
		v[hostip], _ = GetNode(clusterName, hostip)
	}
	return v, true
}

// DeleteCluster delete a specfic cluster by clustername
func DeleteCluster(clusterName string) {
	key := strings.Join([]string{common.ClusterKey, clusterName}, common.Sep)
	exist, _ := base.IsExist(key)
	if !exist {
		// already delete.
		return
	}

	key = strings.Join([]string{common.ClusterKey, clusterName}, common.Sep)
	err := base.Delete(key)
	if err != nil {
		log.Printf("delete cluster(%s) from etcd failed. ErrorMsg: %s", clusterName, err.Error())
	}
}

// DeleteMaster delete a specfic master from cluster by masterip
func DeleteMaster(clusterName, masterip string) {
	key := strings.Join([]string{common.ClusterKey, clusterName, common.MasterKey, masterip}, common.Sep)
	exist, _ := base.IsExist(key)
	if !exist {
		// already delete.
		return
	}

	key = strings.Join([]string{common.ClusterKey, clusterName, common.MasterKey, masterip}, common.Sep)
	err := base.Delete(key)
	if err != nil {
		log.Printf("delete master(%s) in cluster(%s) from etcd failed. ErrorMsg: %s", masterip, clusterName, err.Error())
	}
}

// DeleteNode delete a specfic node from cluster by nodename
func DeleteNode(clusterName, hostip string) {
	key := strings.Join([]string{common.ClusterKey, clusterName, common.NodeKey, hostip}, common.Sep)
	exist, _ := base.IsExist(key)
	if !exist {
		// already delete.
		return
	}

	key = strings.Join([]string{common.ClusterKey, clusterName, common.NodeKey, hostip}, common.Sep)
	err := base.Delete(key)
	if err != nil {
		log.Printf("delete node from etcd failed. ErrorMsg: %s", err.Error())
	}
}

// GetAllNodes get all nodes from etcd
func GetAllNodes() []string {
	nodeList := []string{}
	// Get clusters
	clusters, exist := GetClusters()
	if !exist {
		return nodeList
	}
	// Get masters and nodes
	for cluster := range clusters {
		masters, exist := GetMasters(cluster)
		if exist {
			for master := range masters {
				nodeList = append(nodeList, master)
			}
		}
		nodes, exist := GetNodes(cluster)
		if exist {
			for node := range nodes {
				nodeList = append(nodeList, node)
			}
		}
	}
	return nodeList
}

// GetClusterDNSIP get cluster dns ip
func GetClusterDNSIP(clusterName string) string {
	key := strings.Join([]string{common.ClusterKey, clusterName, common.MasterKey}, common.Sep)
	values, err := base.Get(key)
	if err != nil {
		return ""
	}
	if len(values) < 1 {
		return ""
	}
	p := values[key]
	master, err := base.Get(p)
	if err != nil || len(master) < 1 {
		return ""
	}
	v := Master{}
	_ = json.Unmarshal([]byte(master[p]), &v)
	return v.DNSServiceIP
}

// GetClusterInfo get cluster info
func GetClusterInfo(clusterName string) *CluInfo {
	v := new(CluInfo)
	key := strings.Join([]string{common.ClusterKey, clusterName, common.Info}, common.Sep)
	values, err := base.Get(key)
	for i := 0; i < 3 && err != nil; i++ {
		values, err = base.Get(key)
	}
	if err != nil || values == nil || values[key] == "" {
		return nil
	}
	err = json.Unmarshal([]byte(values[key]), v)
	if err != nil {
		return nil
	}
	return v
}

func GetImagesInfo(clusterName string) []Image {
	v := make([]Image, 0)

	key := strings.Join([]string{common.ClusterKey, clusterName, common.ImagesKey}, common.Sep)
	values, err := base.Get(key)
	for i := 0; i < 3 && err != nil; i++ {
		values, err = base.Get(key)
	}
	if err != nil || values == nil {
		return nil
	}
	err = json.Unmarshal([]byte(values[key]), &v)
	if err != nil {
		return v
	}
	return v
}

func CheckClusterExist(clusterName string) bool {
	key := strings.Join([]string{common.ClusterKey, clusterName}, common.Sep)
	found, _ := base.IsExist(key)
	for i := 0; i < 3 && !found; i++ {
		found, _ = base.IsExist(key)
	}
	return found
}
