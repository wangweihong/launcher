package federation

import (
	"log"
	"strconv"
	"fmt"
	"ufleet/launcher/code/utils"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/config"
	"ufleet/launcher/code/controller/cluster"
	"os"
	"io/ioutil"
	"strings"
	"time"
	"runtime"
	"ufleet/launcher/code/config/manifests"
)

// Create 创建联邦集群
func (fed *Federation) Create() {
	logPrefix := fmt.Sprintf("[ %s ][ Create ]", fed.FedInfo.Name)

	// 检查参数
	log.Printf("%s check params", logPrefix)
	if fed.FedInfo.Name == "" {
		errMsg := fmt.Sprintf("federation name could not be empty or is invalid(a-z)")
		log.Printf("%s %s", logPrefix, errMsg)
		fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
		return
	}
	if len(fed.FedCluLeaders) == 0 {
		errMsg := fmt.Sprintf("eaders is empty")
		log.Printf("%s %s", logPrefix, errMsg)
		fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
		return
	}
	if len(fed.FedCluLeaders) != 1 {
		errMsg := fmt.Sprintf("not support federation ha mode")
		log.Printf("%s %s", logPrefix, errMsg)
		fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
		return
	}

	// 获取master节点
	log.Printf("%s get masters", logPrefix)
	masters, found := cluster.GetMasters(fed.FedCluLeaders[0])
	if !found {
		errMsg := fmt.Sprintf("cluster:%s not found", fed.FedCluLeaders[0])
		log.Printf("%s %s", logPrefix, errMsg)
		fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
		return
	}
	if len(masters) != 1 {
		errMsg := fmt.Sprintf("not support cluster ha mode or master cloud not be empty")
		log.Printf("%s %s", logPrefix, errMsg)
		fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
		return
	}

	// 清理环境:也许上次安装但没有卸载
	log.Printf("%s clean env, will call Remove soon", logPrefix)
	fed.Remove()

	// 获取master节点的相关信息
	log.Printf("%s get master relative information", logPrefix)
	for _, master := range masters {
		// 临时目录
		tempDir := "/tmp/" + strconv.Itoa(time.Now().UTC().Nanosecond())

		// 生成配置文件
		log.Printf("%s generate config file", logPrefix)
		if err := fed.genConfig(master, tempDir); err != nil {
			errMsg := fmt.Sprintf("generate config file failed: %s", err.Error())
			log.Printf("%s %s", logPrefix, errMsg)
			fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
			return
		}

		// 拷贝到远程机器
		log.Printf("%s copy to remote machine", logPrefix)
		if err := fed.copyToRemote(master, tempDir); err != nil {
			errMsg := fmt.Sprintf("copy to remote machine failed: %s", err.Error())
			log.Printf("%s %s", logPrefix, errMsg)
			fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
			return
		}

		// 执行初始化和安装任务
		log.Printf("%s execute init env and init federation", logPrefix)
		if err := fed.execute(master); err != nil {
			errMsg := fmt.Sprintf("execute install file failed: %s", err.Error())
			log.Printf("%s %s", logPrefix, errMsg)
			fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
			return
		}

		// 添加节点
		log.Printf("%s add follower into federation", logPrefix)
		err := fed.AddFollower(master.ClusterName)
		if err != nil {
			errMsg := fmt.Sprintf("add follower into federation failed: %s", err.Error())
			log.Printf("%s %s", logPrefix, errMsg)
			fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
			return
		}

		// 删除临时目录
		log.Printf("%s clean remote temp directory", logPrefix)
		os.RemoveAll(tempDir)
	}
	log.Printf("%s check federation's status", logPrefix)
	for _, master := range masters {
		// 检查安装后状态
		if err := fed.checkStatus(master); err != nil {
			errMsg := fmt.Sprintf("check installed federation status failed: %s", err.Error())
			log.Printf("%s %s", logPrefix, errMsg)
			fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
			return
		}
	}

	err := fed.SaveFedStatusIfExist(common.GlobalFederationStatusRunning, "")
	if err != nil {
		errMsg := fmt.Sprintf("it's finish, but save federation status in etcd failed")
		log.Printf("%s %s", logPrefix, errMsg)
		fed.SaveFedStatusIfExist(common.GlobalFederationStatusFailed, errMsg)
		return
	}

	log.Printf("%s Done", logPrefix)
	return
}

// 卸掉联邦组件
func (fed *Federation) Remove() error {
	logPrefix := fmt.Sprintf("[ %s ][ Remove ]", fed.FedInfo.Name)

	// 执行删除脚本
	cmd := `# remove federation
	kubectl delete cluster --all --context=` + fed.FedInfo.Name + ` || true
	kubectl delete pods --all -n federation-system  || true
	kubectl delete deployment --all -n federation-system  || true
	kubectl delete clusterrolebinding ` + config.GK8sDefault.SAHelm + `-rolebinding || true
	kubectl delete serviceaccount --all -n federation-system  || true
	kubectl delete secret --all -n federation-system  || true
	kubectl delete ns federation-system || true
	# reset helm
	helm del --purge $(helm list -q -a) || true
	helm del --purge $(helm list -q -a) || true
	helm del --purge coredns || true
	helm reset --force || true
	kubectl delete clusterrolebinding add-on-cluster-admin || true
	kubectl delete sa ` + config.GK8sDefault.SAHelm + ` -n kube-system || true
`

	// 联邦机器
	log.Printf("%s begin to remove federation: %s", logPrefix, fed.FedInfo.Name)
	ok := false
	for i:=0;i<len(fed.FedCluLeaders);i++ {
		log.Printf("%s get leader masters", logPrefix)
		fedMasters, found := cluster.GetMasters(fed.FedCluLeaders[i])
		if !found {
			continue
		}

		for _, fedMaster := range fedMasters {
			sshClient, err := fedMaster.GetSSHClient()
			if err != nil {
				continue
			}
			log.Printf("%s execute remove command in remote machine", logPrefix)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				log.Printf("%s remove federation failed: %s, will try again if other leader node exist", logPrefix, err.Error())
				continue
			}
		}

		ok = true
		break
	}

	if !ok {
		return fmt.Errorf("remove federation failed")
	}
	log.Printf("%s Done", logPrefix)
	return nil
}

// AddFollower 添加联邦节点
func (fed *Federation) AddFollower(clusterName string) error {
	logPrefix := fmt.Sprintf("[ %s ][ AddFollower ]", fed.FedInfo.Name)

	suitName := strings.ToLower(utils.GetValidCh(clusterName))

	// 获取cluster信息
	log.Printf("%s get cluster info by cluster name", logPrefix)
	cluInfo := cluster.GetClusterInfo(clusterName)
	if cluInfo == nil {
		return fmt.Errorf("can't get cluster info by cluster name")
	}

	// 获取follower所有master主机
	fellowerMasters, found := cluster.GetMasters(clusterName)
	if !found {
		return fmt.Errorf("can't get any master in cluster: %s", clusterName)
	}

	// 联邦机器
	runtimeError := fmt.Errorf("")
	ok := false
	for i:=0;i<len(fed.FedCluLeaders);i++ {
		log.Printf("%s get leader masters", logPrefix)
		fedMasters, found := cluster.GetMasters(fed.FedCluLeaders[i])
		if !found {
			continue
		}

		finish := false
		for _, fedMaster := range fedMasters {
			sshClient, err := fedMaster.GetSSHClient()
			if err != nil {
				runtimeError = fmt.Errorf(runtimeError.Error() + err.Error())
				continue
			}

			// 生成临时的配置文件
			log.Printf("%s generate temp config file", logPrefix)
			tempDir := "/tmp/" + strconv.Itoa(time.Now().Nanosecond())
			cmd := fmt.Sprintf("mkdir -p %s && echo \"%s\" > %s/kubeconfig",
				tempDir, cluInfo.AdminContext, tempDir)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				runtimeError = fmt.Errorf(runtimeError.Error() + err.Error())
				continue
			}

			// 导入secret
			log.Printf("%s import admin secret", logPrefix)
			cmd = fmt.Sprintf("kubectl create secret generic %s --from-file=%s/kubeconfig" + " -n federation-system",
				suitName, tempDir)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				runtimeError = fmt.Errorf(runtimeError.Error() + err.Error())
				continue
			}

			for _, fellowerMaster := range fellowerMasters {
				// 生成临时配置文件
				log.Printf("%s generate temp add cluster config", logPrefix)
				filepath := config.GDefault.CurrentDir + "/config/templates/template-federation-add-cluster.yaml"
				buf, err := ioutil.ReadFile(filepath)
				if err != nil {
					runtimeError = fmt.Errorf(runtimeError.Error() + err.Error())
					continue
				}
				strBuf := string(buf)
				strBuf  = strings.Replace(strBuf, "{{ .ServiceAddress }}", "https://" + fellowerMaster.HostIP + ":6443", 1)
				strBuf  = strings.Replace(strBuf, "{{ .ClusterName }}", suitName, 1)
				strBuf  = strings.Replace(strBuf, "{{ .ClusterSecretName }}", suitName, 1)
				cmd = fmt.Sprintf("mkdir -p %s && echo \"%s\" > %s/cluster.yaml",
					tempDir, strBuf, tempDir)
				_, err = utils.Execute(cmd, sshClient)
				if err != nil {
					runtimeError = fmt.Errorf(runtimeError.Error() + err.Error())
					continue
				}

				// 添加节点
				log.Printf("%s add cluster info federation", logPrefix)
				cmd = fmt.Sprintf("kubectl create -f %s/cluster.yaml --context=%s", tempDir, fed.FedInfo.Name)
				_, err = utils.Execute(cmd, sshClient)
				if err != nil {
					runtimeError = fmt.Errorf(runtimeError.Error() + err.Error())
					continue
				}

				finish = true
				break
			}

			// 删除临时的配置文件
			cmd = fmt.Sprintf("rm -rf %s", tempDir)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				// 删除临时目录失败不认为为失败
				runtimeError = fmt.Errorf("%s %s", runtimeError.Error(), err.Error())
				log.Printf("%s %s failed!", logPrefix, cmd)
			}

			if finish {
				break
			}
		}
		if finish {
			ok = true
			break
		}
	}
	if !ok {
		log.Printf("%s add cluster to federation failed: %s", logPrefix, runtimeError.Error())
		return fmt.Errorf("add cluster to federation failed: %s", runtimeError.Error())
	}

	log.Printf("[ Federation ][ AddFollower ] Done")
	return nil
}

// DelFollower 删除联邦节点
func (fed *Federation) DelFollower(clusterName string) error {
	logPrefix := fmt.Sprintf("[ %s ][ DelFollower ]", fed.FedInfo.Name)
	suitName := strings.ToLower(utils.GetValidCh(clusterName))

	// 获取cluster信息
	log.Printf("%s get cluster info by cluster name", logPrefix)
	cluInfo := cluster.GetClusterInfo(clusterName)
	if cluInfo == nil {
		log.Printf("%s get cluster info by cluster name failed", logPrefix)
		return fmt.Errorf("can't get cluster info by cluster name")
	}

	// 联邦机器
	ok := false
	for i:=0;i<len(fed.FedCluLeaders);i++ {
		log.Printf("%s get leader masters", logPrefix)
		fedMasters, found := cluster.GetMasters(fed.FedCluLeaders[i])
		if !found {
			log.Printf("%s get leader clusters by cluster name", logPrefix)
			continue
		}

		for _, fedMaster := range fedMasters {
			log.Printf("%s begin to delete cluster from federation", logPrefix)
			sshClient, err := fedMaster.GetSSHClient()
			if err != nil {
				log.Printf("%s get ssh client failed: %s", logPrefix, err.Error())
				continue
			}

			// 删除节点
			cmd := fmt.Sprintf("ubectl delete cluster %s --context=%s", suitName, fed.FedInfo.Name)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				log.Printf("%s delete cluster from federation failed: %s", logPrefix, err.Error())
				continue
			}

			// 删除secret
			log.Printf("%s import admin secret", logPrefix)
			cmd = fmt.Sprintf("kubectl delete secret %s -n federation-system", suitName)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				log.Printf("%s delete secret failed: %s", logPrefix, err.Error())
				return err
			}
		}

		ok = true
		break
	}
	if !ok {
		log.Printf("%s delete cluster from federation failed", logPrefix)
		return fmt.Errorf("delete cluster from federation failed")
	}

	log.Printf("%s Done", logPrefix)
	return nil
}

// 生成配置文件
func (fed *Federation) genConfig(master cluster.Master, tempDir string) error {
	var err error

	baseScriptDir := config.GDefault.CurrentDir + "/script/federation"
	destScriptDir := tempDir + "/script/federation"

	// 拷贝一份脚本到临时目录
	if err := utils.CopyDir(baseScriptDir, destScriptDir); err != nil {
		return err
	}

	// values.yaml
	valuesObject := struct {
		Zones string
		EtcdEndpoint string
	}{
		config.GK8sDefault.FederationZones,
		fmt.Sprintf("http://%s:%d", master.HostIP, config.GK8sDefault.EtcdListenPort),
	}
	if err = utils.TmplReplaceByObject(destScriptDir + "/coredns/values.yaml", manifests.GetFederationValuesYaml(), valuesObject, 0666); err != nil {
		return err
	}

	// rolebinding.yaml
	rolebindingObject := struct {
		ServiceAccountHelm  string
	}{
		config.GK8sDefault.SAHelm,
	}
	if err = utils.TmplReplaceByObject(destScriptDir + "/role/rolebinding.yaml", manifests.GetFederationHelmRolebindingYaml(), rolebindingObject, 0666); err != nil {
		return err
	}

	// coredns-provider.conf
	corednsProviderObject := struct {
		EtcdEndpoints string
		Zones         string
	}{
		"http://" + master.HostIP + ":" + strconv.Itoa(config.GK8sDefault.EtcdListenPort),
		config.GK8sDefault.FederationZones,
	}
	if err = utils.TmplReplaceByObject(destScriptDir + "/coredns-provider.conf", manifests.GetCorednsProviderConf(), corednsProviderObject, 0666); err != nil {
		return err
	}

	// fedinstaller.sh
	fedinstallerObject := struct {
		FederationName string
		Zone string
		Hostip string
	}{
		fed.FedInfo.Name,
		config.GK8sDefault.FederationZones,
		master.HostIP,
	}
	if err = utils.TmplReplaceByObject(destScriptDir + "/fedinstaller.sh", manifests.GetFederationInstallerSh(), fedinstallerObject, 0777); err != nil {
		return err
	}

	return nil
}

// 拷贝到远程机器
func (fed *Federation) copyToRemote(master cluster.Master, tempDir string) error {
	// 获取 ssh client 和 设定远程目录
	sshClient, err := master.GetSSHClient()
	if err != nil {
		return err
	}
	destDir := config.GDefault.RemoteTempDir

	// 清理远程目录
	cmd := fmt.Sprintf("rm -rf %s", destDir)
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		return err
	}

	// 列出需要拷贝的文件
	dir := map[string]string{
		config.GDefault.CurrentDir + "/common/federation/":     destDir + "/common/federation/",
		config.GDefault.CurrentDir + "/config/kubernetes.conf": destDir + "/script/federation/",
		config.GDefault.CurrentDir + "/script/common/":         destDir + "/script/common/",
		config.GDefault.CurrentDir + "/script/master/":         destDir + "/script/master/",
		tempDir + "/script/federation/":              destDir + "/script/federation/",
	}

	// 拷贝文件
	// clean dir for later copy
	err = utils.SendToRemote(sshClient, dir)
	return err
}

// 执行安装和初始化任务
func (fed *Federation) execute(master cluster.Master) error {
	sshClient, err := master.GetSSHClient()
	if err != nil {
		return fmt.Errorf("get ssh client failed: %s", err.Error())
	}
	destDir := fmt.Sprintf("%s/script/federation", config.GDefault.RemoteTempDir)

	// 按顺序执行脚本
	logfile := config.GDefault.RemoteLogDir + "/federation.log"
	// 安装coredns 并初始化联邦
	cmd := fmt.Sprintf("cd %s && /bin/bash main.sh -logID %s >> %s 2>&1", destDir, fed.FedInfo.Name, logfile)
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		return fmt.Errorf("execute main.sh(install coredns and federation init) failed: %s", err.Error())
	}

	return nil
}

// 检查状态
func (fed *Federation) checkStatus(master cluster.Master) error {
	// 获取 ssh client

	// 检查添加联邦节点状态

	return nil
}

func (fed *Federation) SaveFedStatusIfExist(status, errMsg string) error {
	logPrefix := fmt.Sprintf("[ %s ][ SaveFedStatusIfExist ]", fed.FedInfo.Name)

	fed.FedInfo.Status = status
	fed.FedInfo.ErrorMsg = errMsg
	_, found := GetFederation(fed.FedInfo.Name)
	if !found {
		errMsg := fmt.Sprintf("federation(%s) not exist, maybe already remove", fed.FedInfo.Name)
		log.Printf("%s %s", logPrefix, errMsg)
		runtime.Goexit()
	}

	err := fed.SaveFederationStatus()
	return err
}