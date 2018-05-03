package cluster

import (
	"ufleet/launcher/code/config"
	"ufleet/launcher/code/config/manifests"
	"ufleet/launcher/code/model/3party/etcd"
	"ufleet/launcher/code/model/base"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/utils"
	"ufleet/launcher/code/utils/certs"

	"fmt"
	"strconv"
	"time"

	"sync"

	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strings"
)

var (
	cLock = new(sync.Mutex) //cLock cluster lock
)

func (clu *Cluster) AddMaster(m *Master) {
	numAllSteps := 12
	countStep := 0

	// param check
	if m == nil || m.HostIP == "" {
		errMsg := "master should not be empty, or master'ip should not be empty"
		log.Printf(errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
	}

	logPrefix := fmt.Sprintf("[ %s ][ %s ][ AddMaster ]", clu.Name, m.HostIP)

	// 检查信息
	log.Printf("%s check information", logPrefix)
	if clu.BaseMasters < 2 {
		errMsg := "add master just allow in ha mode"
		log.Printf(errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 检查是否为同一个子网
	if config.GK8sDefault.CheckSubnetwork == "true" {
		if isSame, err := m.sameSubnetwork(clu.Vip); err != nil || !isSame {
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = fmt.Sprintf("%s master ip should be same subnetwork with vip(vip: %s, ip: %s)", logPrefix, clu.Vip, m.HostIP)
			}
			log.Printf(errMsg)
			m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
			return
		}
	}

	// 将master信息补充完整
	m.CaCert = clu.CaCert
	m.APIClientCert = clu.APIClientCert
	m.APIClientKey = clu.APIClientKey
	for _, master := range clu.Masters {
		if len(m.PodNetwork) == 0 {
			m.PodNetwork = master.PodNetwork
		}
		if len(m.DNSServiceIP) == 0 {
			m.DNSServiceIP = master.DNSServiceIP
		}
		if len(m.ServiceNetwork) == 0 {
			m.ServiceNetwork = master.ServiceNetwork
		}
		if len(m.KubeServiceIP) == 0 {
			m.KubeServiceIP = master.KubeServiceIP
		}
		if (len(m.PodNetwork) != 0) && (len(m.DNSServiceIP) != 0) &&
			(len(m.ServiceNetwork) != 0) && (len(m.KubeServiceIP) != 0) {
			break
		}
	}

	// 获取ssh client
	log.Printf("%s get ssh client", logPrefix)
	sshClient, err := m.GetSSHClient()
	if err != nil {
		errMsg := "get ssh client failed: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}
	defer m.releaseSSHClient()

	// Step 00: 检查并设置状态， 设置各节点主机名等初始化操作
	log.Printf("%s set host name", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = m.setHostname(sshClient)
	if err != nil {
		errMsg := "set hostname failed: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}
	m.saveStatusIfExist()

	// 先卸载，防止上次安装，但没有卸载就再次安装
	log.Printf("%s clean env, avoid never uninstall and add master into this cluster", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	m.RemoveMaster()

	// 临时目录
	configTempDir := fmt.Sprintf("%s/%s", config.GDefault.LocalTempDir, strconv.FormatInt(clu.CreateTime, 10)) // 证书临时目录，在保存到 Etcd后删除

	// 生成证书
	log.Printf("%s generate certs and keys", logPrefix)
	if err := m.genApiserverCert(clu.Vip, configTempDir); err != nil {
		errMsg := "generate apiserver cert failed: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 生成配置文件
	log.Printf("%s generate config file", logPrefix)
	if err := m.genConfig(clu, configTempDir); err != nil {
		errMsg := "generate config failed: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 拷贝到远程主机
	log.Printf("%s copy files to remove machine", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	if err := m.sendInstallFiles(sshClient, configTempDir, false, false); err != nil {
		errMsg := "send install files failed: " + err.Error()
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// Step : 导入ufleet 镜像仓库ca证书
	log.Printf("%s import register ca cert", logPrefix)
	err = m.importRegistryCa(sshClient)
	if err != nil {
		errMsg := fmt.Sprintf("import register ca failed. ErrorMsg: %s", err.Error())
		log.Printf("%s[ %s ] %s", logPrefix, m.HostIP, errMsg)
		clu.readyToExit(common.GlobalClusterStatusFailed, errMsg, common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 添加etcd节点
	//        sendFilesToRemote 函数会把各自节点的文件发送到 REMOTE_TEMP_DIR 指定的目录中，默认为： /root/k8s
	log.Printf("%s add etcd node in etcd cluster", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	IPs := make([]string, 0, 3)
	for i := range clu.Masters {
		IPs = append(IPs, clu.Masters[i].HostIP)
	}
	lEtcd := etcd.LEtcd{}
	err = lEtcd.Init(IPs)
	if err != nil {
		log.Printf("%s add etcd node to etcd cluster failed: %s", logPrefix, err)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "add etcd node in etcd cluster failed: "+err.Error())
		return
	}
	if err = lEtcd.MemberAdd(m.HostIP); err != nil {
		log.Printf("%s add etcd node to etcd cluster failed: %s", logPrefix, err)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "add etcd node in etcd cluster failed: "+err.Error())
		return
	}

	log.Printf("%s start etcd node", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	logDir := config.GDefault.RemoteLogDir + "/etcd"
	getErrMsgComm := fmt.Sprintf("errMsg=$(tail -n 2 %s/etcdStart.log | head -n 1) && echo $errMsg | tr '[' '<' | tr ']' '>' | sed 's/<[^>]*>*//g'", logDir)
	cmd := fmt.Sprintf("mkdir -p %s && cd %s%s && /bin/bash etcdStart.sh &> %s/etcdStart.log",
		logDir, config.GDefault.RemoteTempDir, common.GlobalConfigEtcdPath, logDir)
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		errMsg, _ := utils.Execute(getErrMsgComm, sshClient)
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "start etcd node Failed! "+errMsg)
		return
	}

	// 检查etcd集群是否已经同步完成
	log.Printf("%s waiting for etcd cluster ready", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	lEtcd2 := etcd.LEtcd{}
	err = lEtcd2.Init([]string{m.HostIP})
	if err != nil {
		errMsg := fmt.Sprintf("init launcher etcd failed: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}
	if err = lEtcd2.WaitUntilReady(); err != nil {
		errMsg := fmt.Sprintf("waiting for etcd cluster ready failed: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 执行添加任务
	log.Printf("%s execute install command", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	cmd = fmt.Sprintf("/bin/bash %s/script/master/install.sh", config.GDefault.RemoteTempDir)
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		errMsg := fmt.Sprintf("execute install script failed: %s(%s)", err.Error(), cmd)
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 获取apiserver证书和密钥
	log.Printf("%s get apiserver cert and key", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	if err = m.getCertAndKeys(sshClient); err != nil {
		errMsg := fmt.Sprintf("get apiserver cert and key failed: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	// 打上标签
	log.Printf("%s set labels", logPrefix)
	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	if err = m.setMasterLabel(); err != nil {
		errMsg := fmt.Sprintf("set labels failed: %s", err.Error())
		log.Printf("%s %s", logPrefix, errMsg)
		m.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, errMsg)
		return
	}

	m.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	m.saveStatusWithMsgIfExist(common.GlobalNodeStatusRunning, "")
	log.Printf("%s Done", logPrefix)
	return
}

// CreateNode add a node to exist k8s master
func (clu *Cluster) AddNode(n *Node) {
	var numAllSteps int = 9
	var countStep int = 0

	logPrefix := fmt.Sprintf("[ %s ][ %s ][ CreateNode ]", n.ClusterName, n.HostIP)

	if n.HostSSHPort == "" {
		n.HostSSHPort = "22"
	}
	if n.HostSSHNetwork == "" {
		n.HostSSHNetwork = "tcp"
	}
	n.Registry = "index.youruncloud.com"
	n.SaveStatus()

	log.Printf("%s check ssh connect", logPrefix)
	sshClient, err := SSHClient(n.Host)
	if err != nil {
		log.Printf("%s Connect Failed!", logPrefix)
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "connect failed! ErrorMsg: "+err.Error())
		return
	}
	go func() {
		// Timeout.
		time.Sleep(time.Minute * 30)
		if n.Status == common.GlobalNodeStatusPending {
			log.Printf("%s Maybe something wrong, this task already took half hours, and I am going warn user!", logPrefix)
			n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "the machine is really slow for me to create node, anyway, I will continue.")
		}
	}()
	defer sshClient.Close()

	/* 清理环境 */
	log.Printf("%s clean the env, maybe last time already install but not uninstall", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	n.RemoveNode()

	// Set Hostname
	err = n.setHostname(sshClient)
	if err != nil {
		// 设置 主机名失败
		log.Printf("%s set hostname(%s) failed! ErrorMsg: %s", logPrefix, n.HostName, err.Error())
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "set hostname("+n.HostName+") failed! ErrorMsg: "+err.Error())
		return
	} else {
		log.Printf("%s set hostname(%s) Success!", logPrefix, n.HostName)
	}
	// Set DNS
	err = n.setDNS(sshClient)
	if err != nil {
		// 设置DNS失败
		log.Printf("%s set dns failed! ErrorMsg: %s", logPrefix, err.Error())
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "set dns failed! ErrorMsg: "+err.Error())
		return
	} else {
		log.Printf("%s set dns Success!", logPrefix)
	}

	// Get cluster info, like apiserver cert and key, join token
	log.Printf("%s get apiserver-kubelet-client.crt/key and join token from cluster info", logPrefix)
	cluInfo := &clu.CluInfo
	if cluInfo == nil || len(cluInfo.JoinToken) == 0 || len(cluInfo.CaCert) == 0 || len(cluInfo.APIClientCert) == 0 || len(cluInfo.APIClientKey) == 0 || len(cluInfo.Vip) == 0 {
		log.Printf("%s get cluster info failed", logPrefix)
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "get cluster info failed")
		return
	}
	n.CaCert = cluInfo.CaCert
	n.APIClientCert = cluInfo.APIClientCert
	n.APIClientKey = cluInfo.APIClientKey
	n.MasterIP = cluInfo.Vip
	n.saveStatusIfExist()

	log.Printf("%s checking master apiserver", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	available := n.checkMaster()
	if !available {
		log.Printf("%s check apiserver failed!", logPrefix)
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "check apiserver failed!")
		return
	}

	// 生成配置文件: apiserver-kubelet-client.crt/key 文件
	configTempDir := fmt.Sprintf("%s/%s", config.GDefault.LocalTempDir, strconv.FormatInt(time.Now().Unix(), 10)) // 证书临时目录，在保存到 Etcd后删除

	log.Printf("%s generate config and certs", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = n.genConfigAndCerts(configTempDir, cluInfo.JoinToken)
	if err != nil {
		if err != common.EtcdNodeNumberNotThree {
			log.Printf("%s gen config and certs Failed! ErrorMsg: %s", logPrefix, err.Error())
			n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "generate apiserver-kubelet-client.crt/key files failed. ErrorMsg: "+err.Error())
			return
		}
		// TODO. 由于当前vespace不支持增删节点，为了保证整体问题，如果增删master节点，不再安装storage
		log.Printf("%s %s, not start storage node", logPrefix, err.Error())
	}

	log.Printf("%s send install files", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = n.sendInstallFiles(sshClient, configTempDir)
	if err != nil {
		log.Printf("%s Send install files Failed! ErrorMsg: %s", logPrefix, err.Error())
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "send install files to remote failed. ErrorMsg: "+err.Error())
		return
	}

	log.Printf("%s import register ca", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = n.importRegistryCa(sshClient)
	if err != nil {
		log.Printf("%s import register ca Failed! ErrorMsg: %s", logPrefix, err.Error())
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "import register ca failed. ErrorMsg: "+err.Error())
		return
	}

	log.Printf("%s execute install.sh", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	logDir := config.GDefault.RemoteLogDir + "/node"
	getErrMsgComm := fmt.Sprintf("errMsg=$(tail -n 2 %s/install.log | head -n 1) && echo $errMsg | tr '[' '<' | tr ']' '>' | sed 's/<[^>]*>*//g'", logDir)
	cmd := fmt.Sprintf("mkdir -p %s &&cd %s/script/node/ && /bin/bash install.sh -logID %s > %s/install.log 2>&1",
		logDir, config.GDefault.RemoteTempDir, n.HostIP, logDir)
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		errMsg, _ := utils.Execute(getErrMsgComm, sshClient)
		log.Printf("%s execute install.sh Failed! ErrorMsg: %s", logPrefix, errMsg)
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "execute install.sh failed. ErrorMsg: "+errMsg)
		return
	}

	// 重新给node打上标签
	log.Printf("%s set node's labels", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = n.setNodeLabel()
	if err != nil {
		log.Printf("%s set node's labels Failed! ErrorMsg: %s", logPrefix, err.Error())
		n.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "set node's labels failed. ErrorMsg: "+err.Error())
		return
	}

	// TODO. 判断node节点是否安装成功

	// 清理环境
	log.Printf("%s clean remote tmp dir after install", logPrefix)
	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	err = n.cleanEnv(sshClient, configTempDir)
	if err != nil {
		log.Printf("%s clean remote env Failed! ErrorMsg: %s", logPrefix, err.Error())
	}

	n.exitIfRecreate(fmt.Sprintf("%d/%d", func() int { countStep = countStep + 1; return countStep }(), numAllSteps), true)
	n.saveStatusWithMsgIfExist(common.GlobalNodeStatusRunning, "")
	log.Printf("%s Done", logPrefix)
}

// RemoveMaster 删除master节点
func (clu *Cluster) RemoveMaster(masterip string) {
	logPrefix := fmt.Sprintf("[ %s ][ %s ]", clu.Name, masterip)

	var wg sync.WaitGroup

	rMaster, found := GetMaster(clu.Name, masterip)
	if !found {
		log.Printf("%s master(%s) already remove, can't found", logPrefix, masterip)
		return
	}

	// 删除etcd节点
	// 删除calico节点
	// 从k8s集群中删除节点
	wg.Add(3)
	go func() {
		defer wg.Done()

		err := clu.removeEtcd(rMaster.Host, 3)
		if err != nil {
			log.Printf("%s[ remove etcd ] %s", logPrefix, err.Error())
		} else {
			log.Printf("%s[ remove etcd ] remove etcd success", logPrefix)
		}
	}()
	go func() {
		defer wg.Done()

		if err := clu.removeCalicoNode(rMaster.Host, 3); err != nil {
			log.Printf("%s[ remove calico node ] %s", logPrefix, err.Error())
		} else {
			log.Printf("%s[ remove calico node ] remove calico node from calico cluster success", logPrefix)
		}
	}()
	go func() {
		defer wg.Done()

		err := clu.RemoveNodeInK8s(rMaster.HostName, 3)
		if err != nil {
			log.Printf("%s[ remove node from k8s ] %s, will try again", logPrefix, err.Error())
		} else {
			log.Printf("%s[ remove node from k8s ] remove node from k8s success.", logPrefix)
		}
	}()
	// 等待任务完成
	wg.Wait()

	// 删除master节点上的文件
	go rMaster.RemoveMaster()

	return
}

// RemoveNode 删除calico node，从集群中删除node， 清理node上的文件
func (clu *Cluster) RemoveNode(nodeip string) {
	var wg sync.WaitGroup

	rNode, found := GetNode(clu.Name, nodeip)
	if !found {
		log.Printf("node(%s) already remove, can't found", nodeip)
		return
	}

	// 删除calico node
	// 从k8s集群中删除节点
	wg.Add(2)
	go func() {
		defer wg.Done()

		err := clu.removeCalicoNode(rNode.Host, 3)
		if err != nil {
			log.Printf("[ %s ][ %s ][ remove calico node ] %s, already retry 3 times", clu.Name, rNode.HostIP, err.Error())
		} else {
			log.Printf("[ %s ][ %s ][ remove calico node ] remove calico node from calico cluster success", clu.Name, rNode.HostIP)
		}
	}()
	go func() {
		defer wg.Done()

		err := clu.RemoveNodeInK8s(rNode.HostName, 3)
		if err != nil {
			log.Printf("[ %s ][ %s ][ remove node in k8s ] %s failed, already retry 3 times", clu.Name, rNode.HostIP, err.Error())
		} else {
			log.Printf("[ %s ][ %s ][ remove node in k8s ] success", clu.Name, rNode.HostIP)
		}
	}()
	// 等待全部卸载任务完成
	wg.Wait()

	go rNode.RemoveNode()
}

// sendFilesToRemote 发送文件到远程主机
func (clu *Cluster) sendFilesToRemote(configTempDir string) error {
	var wg sync.WaitGroup
	wg.Add(clu.BaseMasters)
	for i := range clu.Masters {
		go func(master *Master, num int) {
			defer wg.Done()
			logPrefix := fmt.Sprintf("[ %s ][ %s ][ sendFilesToRemote ]", clu.Name, master.HostIP)
			sshClient, err := master.GetSSHClient()
			if err != nil {
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "[sendFilesToRemote] get ssh client Failed! "+err.Error())
				return
			}

			// Step : 发送文件, 其中： BaseMasters 为1为单master集群，序列号为 0的，默认为 leader
			log.Printf("%s send files to remote machine", logPrefix)
			err = master.sendInstallFiles(sshClient, configTempDir, clu.BaseMasters == 1, num == 0)
			if err != nil {
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "[sendFilesToRemote] send files to remote machine Failed! "+err.Error())
				return
			}
		}(&clu.Masters[i], i)
	}
	// 等待全部任务完成
	wg.Wait()

	// 检查主机状态，如果有异常的，返回异常
	for _, master := range clu.Masters {
		if master.Status == common.GlobalNodeStatusFailed {
			return fmt.Errorf(master.ErrorMsg)
		}
	}

	return nil
}

/*
 * 单主机配置文件生成
 * 存放位置：
 * 	      {{tempDir}}/#type/#hostip/#files
 */
func (clu *Cluster) genAloneConfig(tempDir string) error {
	var err error

	// 获取master
	master := clu.Masters[0]

	/* 拷贝script 到 tempDir */
	if err := utils.CopyDir(config.GDefault.CurrentDir+"/script", tempDir+"/"+master.HostIP+"/script"); err != nil {
		return fmt.Errorf("copy %s/script to %s/script failed: %s", config.GDefault.CurrentDir, tempDir, err.Error())
	}

	/* 获取变量值 */
	etcdPeerPort := strconv.Itoa(config.GK8sDefault.EtcdPeerPort)
	etcdListenPort := strconv.Itoa(config.GK8sDefault.EtcdListenPort)
	etcdToken := fmt.Sprintf("%s-%s", clu.Name, clu.getRandomString(6))
	etcdCluster := fmt.Sprintf("infra-%s=http://%s:%s", master.HostIP, master.HostIP, etcdPeerPort)
	destDir := tempDir + "/" + master.HostIP + "/script"
	etcdEndpoints := "- http://" + master.HostIP + ":" + etcdListenPort
	calicoEtcdCluster := "http://" + master.HostIP + ":" + etcdListenPort
	externalEtcdEndpoints := calicoEtcdCluster

	// image array to map
	imageMaps := imageArray2Map(clu.Images)
	imageEtcdstart, found := imageMaps["etcd_amd64"]
	if !found {
		return fmt.Errorf("can't find image: %s", "etcd_amd64")
	}
	imageEtcdstart = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageEtcdstart)

	imageNtp, found := imageMaps["ntp"]
	if !found {
		return fmt.Errorf("can't find image: %s", "ntp")
	}
	imageNtp = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageNtp)

	imageVespaceStrategy, found := imageMaps["vespace_strategy"]
	if !found {
		return fmt.Errorf("can't find image: %s", "vespace_strategy")
	}
	imageVespaceStrategy = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageVespaceStrategy)

	imageCalicoNode, found := imageMaps["calico_node"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calico_node")
	}
	imageCalicoNode = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoNode)

	imageCalicoCni, found := imageMaps["calico_cni"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calico_cni")
	}
	imageCalicoCni = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoCni)

	imageCalicoKubePolicyController, found := imageMaps["calico_kube_policy_controller"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calico_kube_policy_controller")
	}
	imageCalicoKubePolicyController = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoKubePolicyController)

	imageCalicoctl, found := imageMaps["calicoctl"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calicoctl")
	}
	imageCalicoctl = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoctl)

	imageExternalDns, found := imageMaps["external_dns"]
	if !found {
		return fmt.Errorf("can't find image: %s", "external_dns")
	}
	imageExternalDns = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageExternalDns)

	imageCoredns, found := imageMaps["coredns"]
	if !found {
		return fmt.Errorf("can't find image: %s", "coredns")
	}
	imageCoredns = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCoredns)

	imageKubelet, found := imageMaps["kubelet"]
	if !found {
		return fmt.Errorf("can't find image: %s", "kubelet")
	}
	imageKubelet = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKubelet)

	imagePrometheusNodeExporter, found := imageMaps["prometheus_node_exporter"]
	if !found {
		return fmt.Errorf("can't find image: %s", "prometheus_node_exporter")
	}
	imagePrometheusNodeExporter = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imagePrometheusNodeExporter)

	imagePrometheusServer, enablePrometheusServer := imageMaps["prometheus_server"]
	if enablePrometheusServer {
		imagePrometheusServer = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imagePrometheusServer)
	}
	imageTraefik, found := imageMaps["traefik"]
	if !found {
		return fmt.Errorf("can't find image: %s", "traefik")
	}
	imageTraefik = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageTraefik)

	imageScavenger, found := imageMaps["scavenger"]
	if !found {
		return fmt.Errorf("can't find image: %s", "scavenger")
	}
	imageScavenger = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageScavenger)

	imageProvisioner, found := imageMaps["provisioner"]
	if !found {
		return fmt.Errorf("can't find image: %s", "provisioner")
	}
	imageProvisioner= fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageProvisioner)

	/* 生成所需的全部配置文件 */
	// etcdstart.sh
	etcdStartObject := struct {
		Hostip         string
		EtcdCluster    string
		PeerPort       string
		ListenPort     string
		Token          string
		NtpdHost       string
		ImageNtp       string
		ImageEtcdAmd64 string
	}{
		master.HostIP,
		etcdCluster,
		etcdPeerPort,
		etcdListenPort,
		etcdToken,
		config.GDefault.NtpdHost,
		imageNtp,
		imageEtcdstart,
	}
	if err = utils.TmplReplaceByObject(tempDir+common.GlobalConfigEtcdPath+"/"+master.HostIP+"/etcdStart.sh", manifests.GetEtcdstartSh(), etcdStartObject, 0777); err != nil {
		return err
	}

	// kubeadm.yaml
	kubeadmObject := struct {
		Hostip        string
		Hostname      string
		EtcdEndpoints string
		PodSubnet     string
		K8sVersion    string
		JoinToken     string
		ServiceSubnet string
	}{
		clu.Vip,
		master.HostName,
		etcdEndpoints,
		master.PodNetwork,
		clu.K8sVersion,
		clu.JoinToken,
		common.GlobalDefaultNetServiceSubnet,
	}
	if len(master.ServiceNetwork) != 0 {
		kubeadmObject.ServiceSubnet = master.ServiceNetwork
	}
	if err = utils.TmplReplaceByObject(destDir+"/master/kubeadm.yaml", manifests.GetKubeadmYaml(), kubeadmObject, 0666); err != nil {
		return err
	}

	// vespace.sh
	vespaceObject := struct {
		EtcdIP               string
		ManagerAddr          string
		RootPasswd           string
		ImageVespaceStrategy string
	}{
		master.HostIP,
		master.HostIP,
		"nopasswd",
		imageVespaceStrategy,
	}
	if len(master.UserPwd) > 0 {
		vespaceObject.RootPasswd = master.UserPwd
	}
	if err = utils.TmplReplaceByObject(tempDir+common.GlobalConfigVespacePath+"/"+master.HostIP+"/vespace.sh", manifests.GetVespaceSh(), vespaceObject, 0777); err != nil {
		return err
	}

	// calico.yaml
	calicoObject := struct {
		CalicoEtcdCluster               string
		DefaultNetPodSubnet             string
		ImageCalicoNode                 string
		ImageCalicoCni                  string
		ImageCalicoKubePolicyController string
		ImageCalicoctl                  string
	}{
		calicoEtcdCluster,
		common.GlobalDefaultNetPodSubnet,
		imageCalicoNode,
		imageCalicoCni,
		imageCalicoKubePolicyController,
		imageCalicoctl,
	}
	if err = utils.TmplReplaceByObject(destDir+"/addon/conf/calico.yaml", manifests.GetCalicoYaml(), calicoObject, 0666); err != nil {
		return err
	}

	// exporter.yaml
	prometheusNodeExporterObject := struct {
		ImagePrometheusNodeExporter string
	}{
		imagePrometheusNodeExporter,
	}
	if err = utils.TmplReplaceByObject(destDir+"/addon/conf/exporter.yaml", manifests.GetPromethusNodeExporterYaml(), prometheusNodeExporterObject, 0666); err != nil {
		return err
	}

	//promethus server.yaml
	if enablePrometheusServer {
		prometheusServerObject := struct {
			ImagePrometheusServer string
			PrometheusServicePort int
		}{
			imagePrometheusServer,
			config.GDefault.PrometheusPort,
		}
		if err = utils.TmplReplaceByObject(destDir+"/addon/conf/promethus_server.yaml", manifests.GetPrometheusServerYaml(), prometheusServerObject, 0666); err != nil {
			return err
		}
	}

	// template-traefik.yaml
	traefikObject := struct {
		ImageTraefik string
	}{
		imageTraefik,
	}
	if err = utils.TmplReplaceByObject(destDir+"/addon/conf/traefik.yaml", manifests.GetTraefikYaml(), traefikObject, 0666); err != nil {
		return err
	}

	// kubernetes.conf
	kubernetesObject := struct {
		K8sVersion string
		NtpdHost   string
		Nodename   string
	}{
		clu.K8sVersion,
		config.GDefault.NtpdHost,
		master.HostName,
	}
	if err = utils.TmplReplaceByObject(destDir+"/common/kubernetes.conf", manifests.GetKubernetesConf(), kubernetesObject, 0666); err != nil {
		return err
	}

	// kubelet.sh
	kubeletObject := struct {
		K8sVersion   string
		Hostip       string
		Hostname     string
		ImageKubelet string
	}{
		clu.K8sVersion,
		clu.Masters[0].HostIP,
		clu.Masters[0].HostName,
		imageKubelet,
	}
	if err = utils.TmplReplaceByObject(destDir+"/common/kubelet.sh", manifests.GetKubeletSh(), kubeletObject, 0777); err != nil {
		return err
	}

	// external-dns.yaml
	externalDnsObject := struct {
		EtcdEndpoints    string
		ImageExternalDns string
	}{
		externalEtcdEndpoints,
		imageExternalDns,
	}
	if err = utils.TmplReplaceByObject(destDir+"/addon/conf/external-dns.yaml", manifests.GetExternalDnsYaml(), externalDnsObject, 0666); err != nil {
		return err
	}

	// template-rbac-storageclass.yaml
	if err = utils.TmplReplaceByObject(destDir+"/addon/conf/rbac-storageclass.yaml", manifests.GetRbacStorageclassYaml(), nil, 0666); err != nil {
		return err
	}

	// coredns.yaml.sed
	corednsObject := struct {
		EtcdEndpoint string
		ImageCoredns string
	}{
		fmt.Sprintf("http://%s:%d", clu.Vip, config.GK8sDefault.EtcdListenPort),
		imageCoredns,
	}
	if err = utils.TmplReplaceByObject(destDir+"/coredns/coredns.yaml.sed", manifests.GetCorednsSedYaml(), corednsObject, 0666); err != nil {
		return err
	}

	// template-scavenger.yaml
	scavengerObject := struct {
		ImageScavenger string
	}{
		imageScavenger,
	}
	if err = utils.TmplReplaceByObject(destDir+"/addon/conf/scavenger.yaml", manifests.GetScavengerYaml(), scavengerObject, 0666); err != nil {
		return err
	}

	log.Printf("proivsioner enabled: %v", clu.Provisioner.Enabled)
	if clu.Provisioner.Enabled {
		provisionerObject := struct {
			ImageProvisioner string
			VespaceUser      string
			VespacePassword  string
			VespaceHost      string
		}{
			imageProvisioner,
			clu.Provisioner.User,
			clu.Provisioner.Password,
			clu.Provisioner.Host,
		}
		if err = utils.TmplReplaceByObject(destDir+"/addon/conf/provisioner.yaml", manifests.GetProvisionerYaml(), provisionerObject, 0666); err != nil {
			return err
		}
	}

	return nil
}

/*
 * 多主机配置文件生成
 * 存放位置：
 * 	      {{tempDir}}/#type/#hostip/#file
 *   Like:{{tempDir}}/etcd/192.168.5.15/etcdStart.sh
 */
func (clu *Cluster) genConfig(tempDir string) error {
	var err error

	/* 拷贝script 到 tempDir */
	destDirMaps := make(map[string]string)
	for _, master := range clu.Masters {
		if err := utils.CopyDir(config.GDefault.CurrentDir+"/script", tempDir+"/"+master.HostIP+"/script"); err != nil {
			return fmt.Errorf("copy %s/script to %s/%s/script failed: %s", config.GDefault.CurrentDir, tempDir, master.HostIP, err.Error())
		}
		destDirMaps[master.HostIP] = tempDir + "/" + master.HostIP + "/script"
	}

	/* 获取需要的变量值 */
	managerAddr := ""
	etcdCluster := ""
	etcdEndpoints := ""
	etcdNodes := ""
	calicoEtcdCluster := ""
	etcd1IP := ""
	etcd2IP := ""
	etcd3IP := ""
	etcdPeerPort := strconv.Itoa(config.GK8sDefault.EtcdPeerPort)
	etcdListenPort := strconv.Itoa(config.GK8sDefault.EtcdListenPort)

	etcdDir := tempDir + common.GlobalConfigEtcdPath
	vespaceDir := tempDir + common.GlobalConfigVespacePath

	/* 从系统环境变量中获取 NTPD_HOST的值 */
	ntpdHost := os.Getenv("NTPD_HOST")
	if len(ntpdHost) == 0 {
		return fmt.Errorf("can't get NTPD_HOST env variable from system")
	}
	for i, master := range clu.Masters {
		if i == 0 {
			etcdCluster = "infra-" + master.HostIP + "=http://" + master.HostIP + ":" + etcdPeerPort
			etcdEndpoints = "- http://" + master.HostIP + ":" + etcdListenPort
			calicoEtcdCluster = "http://" + master.HostIP + ":" + etcdListenPort

			etcd1IP = master.HostIP
			managerAddr = master.HostIP
		} else {
			etcdCluster = etcdCluster + ",infra-" + master.HostIP + "=http://" + master.HostIP + ":" + etcdPeerPort
			etcdEndpoints = etcdEndpoints + "\n  - http://" + master.HostIP + ":" + etcdListenPort
			calicoEtcdCluster = calicoEtcdCluster + ",http://" + master.HostIP + ":" + etcdListenPort

			if i == 1 {
				etcd2IP = master.HostIP
			} else {
				etcd3IP = master.HostIP
			}

			managerAddr = managerAddr + "," + master.HostIP
		}
		etcdNodes = etcdNodes + " " + master.HostIP
	}
	externalEtcdCluster := calicoEtcdCluster

	// image array to map
	imageMaps := imageArray2Map(clu.Images)
	imageEtcdstart, found := imageMaps["etcd_amd64"]
	if !found {
		return fmt.Errorf("can't find image: %s", "etcd_amd64")
	}
	imageEtcdstart = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageEtcdstart)

	imageNtp, found := imageMaps["ntp"]
	if !found {
		return fmt.Errorf("can't find image: %s", "ntp")
	}
	imageNtp = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageNtp)

	imageVespaceHaStrategy, found := imageMaps["vespace_ha_strategy"]
	if !found {
		return fmt.Errorf("can't find image: %s", "vespace_ha_strategy")
	}
	imageVespaceHaStrategy = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageVespaceHaStrategy)

	imageCalicoNode, found := imageMaps["calico_node"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calico_node")
	}
	imageCalicoNode = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoNode)

	imageCalicoCni, found := imageMaps["calico_cni"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calico_cni")
	}
	imageCalicoCni = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoCni)

	imageCalicoKubePolicyController, found := imageMaps["calico_kube_policy_controller"]
	if !found {
		return fmt.Errorf("can't find image: %s", "calico_kube_policy_controller")
	}
	imageCalicoKubePolicyController = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCalicoKubePolicyController)

	imageExternalDns, found := imageMaps["external_dns"]
	if !found {
		return fmt.Errorf("can't find image: %s", "external_dns")
	}
	imageExternalDns = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageExternalDns)

	imageCoredns, found := imageMaps["coredns"]
	if !found {
		return fmt.Errorf("can't find image: %s", "coredns")
	}
	imageCoredns = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageCoredns)

	imageKeepalived, found := imageMaps["keepalived"]
	if !found {
		return fmt.Errorf("can't find image: %s", "keepalived")
	}
	imageKeepalived = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKeepalived)

	imageKubelet, found := imageMaps["kubelet"]
	if !found {
		return fmt.Errorf("can't find image: %s", "kubelet")
	}
	imageKubelet = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKubelet)

	imageScavenger, found := imageMaps["scavenger"]
	if !found {
		return fmt.Errorf("can't find image: %s", "scavenger")
	}
	imageScavenger = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageScavenger)

	imagePrometheusNodeExporter, found := imageMaps["prometheus_node_exporter"]
	if !found {
		return fmt.Errorf("can't find image: %s", "prometheus_node_exporter")
	}
	imagePrometheusNodeExporter = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imagePrometheusNodeExporter)

	imageTraefik, found := imageMaps["traefik"]
	if !found {
		return fmt.Errorf("can't find image: %s", "traefik")
	}
	imageTraefik = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageTraefik)

	/* 生成所需的全部配置文件 */
	// etcdstart.sh
	etcdToken := fmt.Sprintf("%s-%s", clu.Name, clu.getRandomString(6))

	etcdStartObject := struct {
		Hostip         string
		EtcdCluster    string
		PeerPort       string
		ListenPort     string
		Token          string
		NtpdHost       string
		ImageNtp       string
		ImageEtcdAmd64 string
	}{
		"",
		etcdCluster,
		etcdPeerPort,
		etcdListenPort,
		etcdToken,
		config.GDefault.NtpdHost,
		imageNtp,
		imageEtcdstart,
	}
	for i := range clu.Masters {
		etcdStartObject.Hostip = clu.Masters[i].HostIP
		if err = utils.TmplReplaceByObject(etcdDir+"/"+clu.Masters[i].HostIP+"/etcdStart.sh", manifests.GetEtcdstartSh(), etcdStartObject, 0777); err != nil {
			return err
		}
	}

	// kubeadm.yaml
	kubeadmObject := struct {
		Hostip        string
		Hostname      string
		EtcdEndpoints string
		PodSubnet     string
		K8sVersion    string
		JoinToken     string
		ServiceSubnet string
	}{
		"",
		"",
		etcdEndpoints,
		"",
		clu.K8sVersion,
		clu.JoinToken,
		common.GlobalDefaultNetServiceSubnet,
	}
	if len(clu.Masters[0].ServiceNetwork) > 0 {
		kubeadmObject.ServiceSubnet = clu.Masters[0].ServiceNetwork
	}
	for i := range clu.Masters {
		kubeadmObject.Hostip = clu.Masters[i].HostIP
		kubeadmObject.Hostname = clu.Masters[i].HostName
		if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[i].HostIP]+"/master/kubeadm.yaml", manifests.GetKubeadmYaml(), kubeadmObject, 0666); err != nil {
			return err
		}
	}

	// keepalived
	keepalivedObject := struct {
		VirtualIP         string
		VirtualRouterID   string
		Interface         string
		KeepalivedName    string
		ContainersToCheck string
		ImageKeepalived   string
	}{
		clu.Vip,
		utils.GetIPField(clu.Vip, 4),
		"",
		"",
		"kube-apiserver",
		imageKeepalived,
	}
	for i := range clu.Masters {
		netcard, err := clu.Masters[i].getNetworkCardName()
		if err != nil {
			return err
		}
		keepalivedObject.Interface = netcard
		keepalivedObject.KeepalivedName = strings.Replace(clu.Masters[i].HostIP, ".", "-", -1)
		if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[i].HostIP]+"/daemonset/keepalived.yaml", manifests.GetKeepalivedYaml(), keepalivedObject, 0666); err != nil {
			return err
		}
	}

	// vespaces.sh
	vespaceObject := struct {
		ManagerAddr            string
		Etcd1IP                string
		Etcd2IP                string
		Etcd3IP                string
		EtcdName               string
		VespaceName            string
		RootPasswd             string
		ImageVespaceHaStrategy string
	}{
		managerAddr,
		etcd1IP,
		etcd2IP,
		etcd3IP,
		"",
		"",
		"nopasswd",
		imageVespaceHaStrategy,
	}
	for i := range clu.Masters {
		switch clu.Masters[i].HostIP {
		case etcd1IP:
			vespaceObject.EtcdName = "etcd1"
			break
		case etcd2IP:
			vespaceObject.EtcdName = "etcd2"
			break
		default:
			vespaceObject.EtcdName = "etcd3"
		}
		vespaceObject.VespaceName = strings.Replace(clu.Masters[i].HostIP, ".", "-", -1)
		if len(clu.Masters[i].UserPwd) > 0 {
			vespaceObject.RootPasswd = clu.Masters[i].UserPwd
		} else {
			vespaceObject.RootPasswd = "nopasswd"
		}
		if err = utils.TmplReplaceByObject(vespaceDir+"/"+clu.Masters[i].HostIP+"/vespace.sh", manifests.GetHaVespaceSh(), vespaceObject, 0777); err != nil {
			return err
		}
	}

	// calico.yaml
	calicoObject := struct {
		CalicoEtcdCluster               string
		DefaultNetPodSubnet             string
		ImageCalicoNode                 string
		ImageCalicoCni                  string
		ImageCalicoKubePolicyController string
	}{
		calicoEtcdCluster,
		common.GlobalDefaultNetPodSubnet,
		imageCalicoNode,
		imageCalicoCni,
		imageCalicoKubePolicyController,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/calico.yaml", manifests.GetCalicoYaml(), calicoObject, 0666); err != nil {
		return err
	}

	// exporter.yaml
	prometheusNodeExporterObject := struct {
		ImagePrometheusNodeExporter string
	}{
		imagePrometheusNodeExporter,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/exporter.yaml", manifests.GetPromethusNodeExporterYaml(), prometheusNodeExporterObject, 0666); err != nil {
		return err
	}


	// template-traefik.yaml
	traefikObject := struct {
		ImageTraefik string
	}{
		imageTraefik,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/traefik.yaml", manifests.GetTraefikYaml(), traefikObject, 0666); err != nil {
		return err
	}

	// external-dns.yaml
	externalDnsObject := struct {
		EtcdEndpoints    string
		ImageExternalDns string
	}{
		externalEtcdCluster,
		imageExternalDns,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/external-dns.yaml", manifests.GetExternalDnsYaml(), externalDnsObject, 0666); err != nil {
		return err
	}

	// template-rbac-storageclass.yaml
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/rbac-storageclass.yaml", manifests.GetRbacStorageclassYaml(), nil, 0666); err != nil {
		return err
	}

	// kubernetes.conf
	k8sObject := struct {
		K8sVersion string
		NtpdHost   string
		Nodename   string
	}{
		clu.K8sVersion,
		ntpdHost,
		"",
	}
	for i := range clu.Masters {
		k8sObject.Nodename = clu.Masters[i].HostName
		if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[i].HostIP]+"/common/kubernetes.conf", manifests.GetKubernetesConf(), k8sObject, 0666); err != nil {
			return err
		}
	}

	// kubelet.sh
	kubeletObject := struct {
		K8sVersion   string
		Hostip       string
		Hostname     string
		ImageKubelet string
	}{
		clu.K8sVersion,
		"",
		"",
		imageKubelet,
	}
	for i := range clu.Masters {
		kubeletObject.Hostip = clu.Masters[i].HostIP
		kubeletObject.Hostname = clu.Masters[i].HostName
		if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[i].HostIP]+"/common/kubelet.sh", manifests.GetKubeletSh(), kubeletObject, 0777); err != nil {
			return err
		}
	}

	// coredns.yaml.sed
	corednsObject := struct {
		EtcdEndpoint string
		ImageCoredns string
	}{
		fmt.Sprintf("http://%s:%d", clu.Vip, config.GK8sDefault.EtcdListenPort),
		imageCoredns,
	}
	for i := range clu.Masters {
		if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[i].HostIP]+"/coredns/coredns.yaml.sed", manifests.GetCorednsSedYaml(), corednsObject, 0666); err != nil {
			return err
		}
	}

	// template-scavenger.yaml
	scavengerObject := struct {
		ImageScavenger string
	}{
		imageScavenger,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/scavenger.yaml", manifests.GetScavengerYaml(), scavengerObject, 0666); err != nil {
		return err
	}

	return nil
}

func (clu *Cluster) cleanEnv(configTempDir string) {
	// Step : 删除临时文件及目录
	retryTimes := 3

	// clean remote machine's temp dir
	for _, master := range clu.Masters {
		logPrefix := fmt.Sprintf("[ %s ][ %s ][ cleanEnv ]", clu.Name, master.HostIP)
		for i := 0; i < retryTimes; i++ {
			sshClient, err := master.GetSSHClient()
			if err != nil {
				log.Printf("%s get ssh client failed, ErrorMsg: %s", logPrefix, err.Error())
				continue
			}
			err = master.cleanEnv(sshClient)
			if err == nil {
				break
			}
			log.Printf("%s[ try %d time] clean remote temp dir failed, ErrorMsg: %s", logPrefix, i, err.Error())
		}
	}

	logPrefix := fmt.Sprintf("[ %s ][ cleanEnv ]", clu.Name)
	// clean local temp dir
	for i := 0; i < retryTimes; i++ {
		err := os.RemoveAll(configTempDir)
		if err == nil {
			break
		}
		log.Printf("%s try to delete dir(%s) failed, ErrorMsg: %s", logPrefix, configTempDir, err.Error())
	}

	// 删除已经超过一天的临时文件夹
	list, err := ioutil.ReadDir(config.GDefault.LocalTempDir) //要读取的目录地址DIR，得到列表
	if err != nil {
		fmt.Printf("read dir(%s) error", config.GDefault.LocalTempDir)
	}
	for _, info := range list { //遍历目录下的内容，获取文件详情，同os.Stat(filename)获取的信息
		if info.IsDir() && time.Now().After(info.ModTime().AddDate(0, 0, 1)) {
			// 目录，最近依次修改在一天前，删除，防止临时目录累积
			waitRemoveDir := fmt.Sprintf("%s/%s", config.GDefault.LocalTempDir, info.Name())
			log.Printf("remove create time more than 1 day's temp directory - %s", waitRemoveDir)
			if err = os.RemoveAll(waitRemoveDir); err != nil {
				log.Printf("remove dir - %s failed: %s", info.Name(), err.Error())
			}
		}
	}
}

func (clu *Cluster) createEtcdCluster() error {
	var wg sync.WaitGroup
	wg.Add(len(clu.Masters))
	for i := range clu.Masters {
		go func(master *Master) {
			defer wg.Done()
			sshClient, err := master.GetSSHClient()
			if err != nil {
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "[ createEtcdCluster ] create etcd cluster Failed! ErrorMsg: "+err.Error())
			}

			logPrefix := fmt.Sprintf("[ %s ][ %s ][ createEtcdCluster ]", clu.Name, master.HostIP)
			// Step : 启动 Etcd，组成 集群
			//        sendFilesToRemote 函数会把各自节点的文件发送到 REMOTE_TEMP_DIR 指定的目录中，默认为： /root/k8s
			log.Printf("%s start etcd node", logPrefix)
			logDir := config.GDefault.RemoteLogDir + "/etcd"
			getErrMsgCmd := fmt.Sprintf("errMsg=$(tail -n 2 %s/etcdStart.log | head -n 1) && echo $errMsg | tr '[' '<' | tr ']' '>' | sed 's/<[^>]*>*//g'",
				logDir)
			cmd := fmt.Sprintf("mkdir -p %s && cd %s%s && /bin/bash etcdStart.sh &> %s/etcdStart.log",
				logDir, config.GDefault.RemoteTempDir, common.GlobalConfigEtcdPath, logDir)
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				errMsg, _ := utils.Execute(getErrMsgCmd, sshClient)
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "start etcd node Failed! "+errMsg)
				return
			}
		}(&clu.Masters[i])
	}
	// 等待全部任务完成
	wg.Wait()

	// 检查主机状态，如果有异常的，返回异常
	for _, master := range clu.Masters {
		if master.Status == common.GlobalNodeStatusFailed {
			return fmt.Errorf(master.ErrorMsg)
		}
	}

	return nil
}

func (clu *Cluster) preInit() error {
	var wg sync.WaitGroup
	wg.Add(len(clu.Masters))
	for i := range clu.Masters {
		go func(master *Master) {
			defer wg.Done()

			sshClient, err := master.GetSSHClient()
			if err != nil {
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "get ssh client failed! "+err.Error())
				return
			}

			// Step : 获取mac地址并设置主机名
			log.Printf("[ " + clu.Name + " ][ " + master.HostIP + " ][ preInit ] set host name.")
			err = master.setHostname(sshClient)
			if err != nil {
				master.saveStatusWithMsgIfExist(common.GlobalNodeStatusFailed, "set hostname failed! "+err.Error())
				return
			}
			master.saveStatusIfExist() // 将对主机名的修改写入到etcd中

			// Step: 添加dns解析
			err = master.setDNS(sshClient)
			if err != nil {
				return
			}

		}(&clu.Masters[i])
	}
	// 等待所有任务完成
	wg.Wait()

	// 检查主机状态，如果有异常的，返回异常
	for _, master := range clu.Masters {
		if master.Status == common.GlobalNodeStatusFailed {
			return fmt.Errorf(master.ErrorMsg)
		}
	}

	return nil
}

/*
 * 证书文件生成
 * 存放位置：
 * 	      {{tempDir}}/#type/#hostip/#file
 *   Like:{{tempDir}}/vespace/192.168.5.15/vespace.sh
 */
func (clu *Cluster) genCert(tempDir string) error {
	cfg := &certs.MasterConfiguration{}

	cfg.API.AdvertiseAddress = clu.Vip
	cfg.API.BindPort = 6443

	cfg.CertificatesDir = tempDir + common.GlobalConfigCertPath
	cfg.Networking.PodSubnet = common.GlobalDefaultNetPodSubnet
	cfg.Networking.ServiceSubnet = common.GlobalDefaultNetServiceSubnet

	// 生成除apiserver外其他证书
	err := certs.CreatePKIAssetsWithoutApiserver(cfg)
	if err != nil {
		fmt.Printf("error: %s", err.Error())
		return err
	}

	var bytes []byte

	// 将集群证书以字符串的方式保存到集群变量中，由于apiserver后续没有使用到，而且ha模式下各个节点的apiserver证书不相同，为了统一，不再保存apiserver证书及密钥
	// ca
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/ca.crt"); err != nil {
		return err
	}
	clu.CluInfo.CaCert = string(bytes)
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/ca.key"); err != nil {
		return err
	}
	clu.CluInfo.CaKey = string(bytes)

	// apiserver-kubelet-client
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/apiserver-kubelet-client.crt"); err != nil {
		return err
	}
	clu.CluInfo.APIClientCert = string(bytes)
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/apiserver-kubelet-client.key"); err != nil {
		return err
	}
	clu.CluInfo.APIClientKey = string(bytes)

	// front-proxy-ca
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/front-proxy-ca.crt"); err != nil {
		return err
	}
	clu.CluInfo.FrontProxyCert = string(bytes)
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/front-proxy-ca.key"); err != nil {
		return err
	}
	clu.CluInfo.FrontProxyKey = string(bytes)

	// front-proxy-client
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/front-proxy-client.crt"); err != nil {
		return err
	}
	clu.CluInfo.FrPxyCliCert = string(bytes)
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/front-proxy-client.key"); err != nil {
		return err
	}
	clu.CluInfo.FrPxyCliKey = string(bytes)

	// sa
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/sa.pub"); err != nil {
		return err
	}
	clu.CluInfo.SaPub = string(bytes)
	if bytes, err = ioutil.ReadFile(cfg.CertificatesDir + "/sa.key"); err != nil {
		return err
	}
	clu.CluInfo.SaKey = string(bytes)

	// 将证书写入到master节点信息中
	for i := range clu.Masters {
		clu.Masters[i].CaCert = clu.CluInfo.CaCert
		clu.Masters[i].APIClientCert = clu.CluInfo.APIClientCert
		clu.Masters[i].APIClientKey = clu.CluInfo.APIClientKey
	}

	// ===================================

	// 拷贝出各个master的证书目录并生成发对应的apiserver证书
	baseCertDir := cfg.CertificatesDir
	for i := range clu.Masters {
		// 拷贝出自己的目录 tempDir + /master.ip + GlobalConfigCertPath
		cfg.CertificatesDir = tempDir + "/" + clu.Masters[i].HostIP + common.GlobalConfigCertPath
		cfg.APIServerCertSAN = clu.Masters[i].HostIP

		// 拷贝
		if err = utils.CopyDir(baseCertDir, cfg.CertificatesDir); err != nil {
			return err
		}

		// 生成apiserver证书
		if err = certs.CreatePKIAssetsJustApiserver(cfg, clu.Masters[i].HostName); err != nil {
			return err
		}
	}

	return nil
}

func (clu *Cluster) genJoinToken() error {
	tokenID := clu.getRandomString(6)
	tokenSecret := clu.getRandomString(16)
	clu.JoinToken = fmt.Sprintf("%s.%s", tokenID, tokenSecret)
	return nil
}

func (clu *Cluster) getRandomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	result := make([]byte, 0, 16)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

// strictCheck 严格检查，将对集群的要求放在这里
func (clu *Cluster) strictCheck() error {
	// Vespaces 要求：
	// Ha 机器台数为 3
	if len(clu.Masters) != 3 {
		return fmt.Errorf("expect machines' number is: 3, real is: %d", len(clu.Masters))
	}

	// Keepalived 要求：需加载 ip_vs 内核模块
	for i := range clu.Masters {
		// 加载内核模块
		err := clu.Masters[i].loadModprobe()
		if err != nil {
			return fmt.Errorf("(%s) load mod proble failed: %s", clu.Masters[i].HostIP, err.Error())
		}
	}

	// 检查是否为同一个子网
	if config.GK8sDefault.CheckSubnetwork == "true" {
		if err := clu.checkSubnetwork(); err != nil {
			return err
		}
	}

	return nil
}

// checkSubnetwork 检查masters和vip是否在统一子网内
func (clu *Cluster) checkSubnetwork() error {
	var vip net.IP
	var mVip net.IP      // 经过掩码处理后的vip
	var mMask net.IPMask // 掩码

	// 检查VIP是否有设置，且是否有效
	vip = net.ParseIP(clu.Vip)
	if vip == nil {
		return fmt.Errorf("VIP(%s) is invalid", clu.Vip)
	}

	// 检查VIP，以及masters是否在同一个网段，不在同一个网段，VIP切换会造成无法访问服务的错误
	ips := make([]net.IP, 0, len(clu.Masters)+1)
	// 处理ip
	for i := 0; i < len(clu.Masters); i++ {
		cmd := fmt.Sprintf("ip addr | sed 's/\\ /\\n/g' | grep %s", clu.Masters[i].HostIP)
		sshClient, err := clu.Masters[i].GetSSHClient()
		if err != nil {
			return err
		}
		resp, err := utils.Execute(cmd, sshClient)
		if err != nil {
			return err
		}
		resp = strings.Replace(resp, "\n", "", -1)
		ip, ipNet, err := net.ParseCIDR(resp) // resp 必须满足： 192.168.4.12/24 的格式
		if err != nil {
			return err
		}
		realIP := ip.Mask(ipNet.Mask)
		if realIP == nil {
			return fmt.Errorf("parse mask ip failed: ip -- %s, mask -- %s", ip.String(), ipNet.Mask.String())
		}
		ips = append(ips, realIP)

		if i == 0 {
			// 处理vip
			mMask = ipNet.Mask
			mVip = vip.Mask(ipNet.Mask)
			if mVip == nil {
				return fmt.Errorf("parse mask ip failed: ip -- %s, mask -- %s", mVip.String(), ipNet.Mask.String())
			}
			ips = append(ips, mVip)
		}
	}
	// 比较经过掩码处理后的ip是否相等
	for i := 0; i < len(ips); i++ {
		if mVip.Equal(ips[i]) {
			continue
		}
		return fmt.Errorf("vip and masters' ip should in the same subnetwork, please check. (vip: %s, master ip: %s, mask: %s)",
			clu.Vip, clu.Masters[i].HostIP, mMask)
	}

	return nil
}

// ClusterL 纳管节点的cluster
// TODO: 应该提供统一的cluster集群类型，将与普通的cluster合并
type ClusterL struct {
	Authway       string `json:"authway"`
	Clustername   string `json:"clustername"`
	Cacert        string `json:"cacert"`
	Hostip        string `json:"hostip"`
	APIServerCert string `json:"apiservercert"`
	APIServerKey  string `json:"apiserverkey"`
}

// LoadCluster load the default cluster data
func (clul *ClusterL) LoadCluster() error {
	logPrefix := fmt.Sprintf("[ %s ][ LoadCluster ]", clul.Clustername)
	log.Printf("%s create new cluster", logPrefix)
	newMaster := Master{}
	newMaster.ClusterName = clul.Clustername
	newMaster.HostIP = clul.Hostip
	newMaster.CaCert = clul.Cacert
	newMaster.APIServerCert = clul.APIServerCert
	newMaster.APIServerKey = clul.APIServerKey

	log.Printf("%s ready to create cluster", logPrefix)
	/* 创建集群 */
	newCluster := Cluster{}
	newCluster.Name = newMaster.ClusterName
	newCluster.Vip = newMaster.HostIP
	newCluster.CaCert = newMaster.CaCert
	newCluster.APIClientCert = newMaster.APIServerCert
	newCluster.APIClientKey = newMaster.APIServerKey

	log.Printf("%s ready install in etcd", logPrefix)
	/* 存储到ETCD中 */
	newCluster.ErrorMsg = ""
	newCluster.Status = common.GlobalClusterStatusRunning
	newCluster.SaveStatus()

	newMaster.ErrorMsg = ""
	newMaster.Status = common.GlobalNodeStatusRunning
	newMaster.SaveStatus()

	return nil
}

// RemoveNodeInK8s 从k8s集群中删除node节点
func (clu *Cluster) RemoveNodeInK8s(nodename string, retryTimes int) error {
	cmd := fmt.Sprintf("kubectl delete node %s ", nodename)
	getNodesCmd := fmt.Sprintf("kubectl get nodes")
	ok := false
	for times := 0; times < retryTimes; times++ {
		for i := range clu.Masters {
			if clu.Masters[i].HostName == nodename {
				continue
			}
			sshClient, err := clu.Masters[i].GetSSHClient()
			if err != nil {
				continue
			}

			// 判断是否已经删除
			resp, err := utils.Execute(getNodesCmd, sshClient)
			if err == nil {
				if !strings.Contains(resp, nodename) {
					// node not exist, already remove from cluster, will return soon
					return nil
				}
			}

			// 尝试删除
			_, err = utils.Execute(cmd, sshClient)
			if err != nil {
				continue
			}
			ok = true
			break
		}
		if ok {
			break
		}
	}
	if !ok {
		return fmt.Errorf("remove node: %s from k8s failed", nodename)
	}
	return nil
}

// removeCalicoNode 在calico中删除节点
func (clu *Cluster) removeCalicoNode(node Host, retryTimes int) error {
	errMsg := ""

	ok := false
	for i := 0; i < retryTimes; i++ {
		for _, master := range clu.Masters {
			if master.HostIP == node.HostIP {
				continue
			}
			etcdSSHClient, err := master.GetSSHClient()
			if err != nil {
				errMsg = fmt.Sprintf("%s, %s", errMsg, err.Error())
				continue
			}
			cmd := fmt.Sprintf("export ETCD_ENDPOINTS=http://%s:%d && calicoctl delete node %s", master.HostIP, config.GK8sDefault.EtcdListenPort, node.HostName)
			if res, err := utils.Execute(cmd, etcdSSHClient); err != nil {
				errMsg = fmt.Sprintf("%s, %s, %s", errMsg, res, err.Error())
				if etcdSSHClient != nil {
					etcdSSHClient.Close()
					etcdSSHClient = nil
				}
				continue
			}
			if etcdSSHClient != nil {
				etcdSSHClient.Close()
				etcdSSHClient = nil
			}
			ok = true
			break
		}
		if ok {
			break
		}
	}
	if !ok {
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// removeEtcd remove etcd node from cluster
// @param eM etcd master host
func (clu *Cluster) removeEtcd(eM Host, retryTimes int) error {
	errMsg := ""

	IPs := make([]string, 0, 3)
	for i := 0; i < len(clu.Masters); i++ {
		IPs = append(IPs, clu.Masters[i].HostIP)
	}
	lEtcd := etcd.LEtcd{}
	err := lEtcd.Init(IPs)
	if err != nil {
		return err
	}

	ok := false
	for i := 0; i < retryTimes; i++ {
		if err := lEtcd.MemberRemove(eM.HostIP); err == nil {
			ok = true
			break
		} else {
			errMsg = fmt.Sprintf("%s, %s", errMsg, err.Error())
		}
	}
	if !ok {
		return fmt.Errorf("%s", errMsg)
	}
	return nil
}

// saveStatus save cluster status to etcd
func (clu *Cluster) SaveStatus() {
	cLock.Lock()
	defer cLock.Unlock()

	key := strings.Join([]string{common.ClusterKey, clu.Name}, common.Sep)
	exist, _ := base.IsExist(key)
	if !exist {
		// if not exist, create it.
		key := strings.Join([]string{common.ClusterKey, clu.Name}, common.Sep)
		err := base.Put(key, "")
		if err != nil {
			log.Printf(err.Error())
		}
	}

	key = strings.Join([]string{common.ClusterKey, clu.Name, common.Status}, common.Sep)
	err := base.Put(key, clu.Status)
	if err != nil {
		log.Printf(err.Error())
	}
	key = strings.Join([]string{common.ClusterKey, clu.Name, common.ImagesKey}, common.Sep)
	imagesInfo, err := json.Marshal(&clu.Images)
	if err != nil {
		log.Printf(err.Error())
	}
	err = base.Put(key, string(imagesInfo))
	if err != nil {
		log.Printf(err.Error())
	}
	key = strings.Join([]string{common.ClusterKey, clu.Name, common.Name}, common.Sep)
	err = base.Put(key, clu.Name)
	if err != nil {
		log.Printf(err.Error())
	}

	cluInfo, err := json.Marshal(&clu.CluInfo)
	if err != nil {
		log.Printf(err.Error())
	}
	key = strings.Join([]string{common.ClusterKey, clu.Name, common.Info}, common.Sep)
	err = base.Put(key, string(cluInfo))
	if err != nil {
		log.Printf(err.Error())
	}
}

// saveStatusIfExist Save status if cluster in etcd. But just return false when:
//     1. cluster not in etcd
//     2. cluster is already installed.
func (clu *Cluster) saveStatusIfExist() bool {
	logPrefix := fmt.Sprintf("[ %s ][ Etcd Cluster saveStatusIfExist ]", clu.Name)

	cLock.Lock()
	defer cLock.Unlock()

	key := strings.Join([]string{common.ClusterKey, clu.Name}, common.Sep)

	if config.GDefault.ServiceType != "module" {
		exist, _ := base.IsExist(key)
		if !exist {
			return false
		}

		/* if cluster already recreate, return false */
		v := CluInfo{}
		key = strings.Join([]string{common.ClusterKey, clu.Name, common.Info}, common.Sep)
		values, err := base.Get(key)
		for i := 0; i < 3 && err != nil; i++ {
			log.Printf("%s retry (get %s) %d time, because: %s", logPrefix, key, i+1, err.Error())
			values, err = base.Get(key)
		}
		if err == nil && values != nil && values[key] != "" {
			err = json.Unmarshal([]byte(values[key]), &v)
			if err == nil && clu.CluInfo.CreateTime != v.CreateTime {
				log.Printf("%s cluster already recreate, last create time: %s, now is: %s",
					logPrefix, strconv.FormatInt(clu.CluInfo.CreateTime, 10),
					strconv.FormatInt(v.CreateTime, 10))
				return false
			}
			if err != nil {
				log.Printf("%s parse cluster info from json info failed", logPrefix)
			}
		} else {
			log.Printf("%s get cluster info from etcd failed", logPrefix)
		}
	}

	key = strings.Join([]string{common.ClusterKey, clu.Name, common.Status}, common.Sep)
	err := base.Put(key, clu.Status)
	if err != nil {
		log.Printf(err.Error())
	}
	key = strings.Join([]string{common.ClusterKey, clu.Name, common.Name}, common.Sep)
	err = base.Put(key, clu.Name)
	if err != nil {
		log.Printf(err.Error())
	}
	key = strings.Join([]string{common.ClusterKey, clu.Name, common.ImagesKey}, common.Sep)
	imageinfo, err := json.Marshal(clu.Images)
	if err != nil {
		log.Printf(err.Error())
	}
	err = base.Put(key, string(imageinfo))
	if err != nil {
		log.Printf(err.Error())
	}

	cluInfo, err := json.Marshal(&clu.CluInfo)
	if err != nil {
		log.Printf(err.Error())
	}
	key = strings.Join([]string{common.ClusterKey, clu.Name, common.Info}, common.Sep)
	err = base.Put(key, string(cluInfo))
	if err != nil {
		log.Printf(err.Error())
	}
	return true
}

/*readyToExit 保存状态后程序将退出
 * 保存状态后程序将退出
 * @param cStatus: cluster status
 * @param cErrMsg: cluster error message
 * @param mStatus: master status
 * @param mErrMsg: master error message
 */
func (clu *Cluster) readyToExit(cStatus string, cErrMsg string, mStatus string, mErrMsg string) {
	logPrefix := fmt.Sprintf("[ %s ][ readyToExit ]", clu.Name)
	clu.setMastersStatusIfExist(mStatus, mErrMsg)

	clu.ErrorMsg = cErrMsg
	clu.Status = cStatus
	exist := clu.saveStatusIfExist()
	if !exist {
		// 不存在，原集群已经被删除，本 goroutine应该退出
		log.Printf("%s cluster already installed or already remove. go exit()", logPrefix)
		runtime.Goexit()
	}
}

// setMastersStatusIfExist 修改集群内所有主机的状态，一般在集群创建之初调用
func (clu *Cluster) setMastersStatusIfExist(status string, errmsg string) {
	for i := range clu.Masters {
		clu.Masters[i].Status = status
		clu.Masters[i].ErrorMsg = errmsg
		clu.Masters[i].saveStatusIfExist()
	}
}

// setNotFailedMastersStatusIfExist 修改集群中状态不为完成状态的主机状态，一般在失败后调用
func (clu *Cluster) setNotFailedMastersStatusIfExist(status string, errmsg string) {
	for i := range clu.Masters {
		if clu.Masters[i].Status != common.GlobalNodeStatusFailed {
			clu.Masters[i].Status = status
			clu.Masters[i].ErrorMsg = errmsg
			clu.Masters[i].saveStatusIfExist()
		}
	}
}

// exitIfRecreate if already recreate, this goroutine go exit directly.
//                if not, and syncProgress is true, will record the progress of install steps(如果未删除，则记录当前进度)
func (clu *Cluster) exitIfRecreate(progress string, syncProgress bool) {
	if config.GDefault.ServiceType == "module" {
		// in module running type, just return
		return
	}

	logPrefix := fmt.Sprintf("[ %s ][ exitIfRecreate ]", clu.Name)

	// 检查是否存在，若不存在，则认为集群已经删除，退出
	if found := CheckClusterExist(clu.Name); !found {
		log.Printf("%s seems cluster already removed. go exit()", logPrefix)
		runtime.Goexit()
	}

	// 检查集群是否已经被重建
	v := GetClusterInfo(clu.Name)
	if v != nil && v.CreateTime != clu.CluInfo.CreateTime {
		log.Printf("%s last create time: %s,but now is: %s", logPrefix,
			strconv.FormatInt(clu.CluInfo.CreateTime, 10), strconv.FormatInt(v.CreateTime, 10))
		log.Printf("%s cluster is already recreated. go exit() ", logPrefix)
		runtime.Goexit()
	}

	if !syncProgress {
		return
	}
	for i := range clu.Masters {
		clu.Masters[i].Progress = progress
		clu.Masters[i].saveStatusIfExist()
	}
}
