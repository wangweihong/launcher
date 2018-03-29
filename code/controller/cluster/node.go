package cluster

import (
	"sync"
	"log"
	"fmt"
	"golang.org/x/crypto/ssh"
	"ufleet/launcher/code/utils"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/config"
	"ufleet/launcher/code/model/3party/etcd"
	"os"
	"io/ioutil"
	"strings"
	"time"
	"runtime"
	"encoding/json"
	"ufleet/launcher/code/model/base"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"encoding/base64"
	"ufleet/launcher/code/config/manifests"
)

var (
	nLock = new(sync.Mutex) //mLock master lock
)

// RemoveNode remove a node from exist k8s master
func (n *Node) RemoveNode() {
	cmd := ""
	ret := ""
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ RemoveNode ]", n.ClusterName, n.HostIP)

	if n.HostSSHPort == "" {
		n.HostSSHPort = "22"
	}
	if n.HostSSHNetwork == "" {
		n.HostSSHNetwork = "tcp"
	}

	sshClient, err := SSHClient(n.Host)
	if err != nil {
		log.Printf("%s Connect Failed!", logPrefix)
		goto DELETENODE
	}
	defer sshClient.Close()

	log.Printf("%s Step 01: Send uninstall files", logPrefix)
	err = n.sendUninstallFiles(sshClient)
	if err != nil {
		log.Printf("%s Send uninstall files Failed! ErrorMsg: %s", logPrefix, err.Error())
		goto DELETENODE
	}

	log.Printf("%s Step 02: run node uninstall.sh", logPrefix)
	cmd = n.callUninstallScript()
	ret, err = utils.Execute(cmd, sshClient)
	if err != nil {
		log.Printf("%s run node uninstall.sh Failed! ErrorMsg: %s, %s", logPrefix, ret, err.Error())
		goto DELETENODE
	}

DELETENODE:
	log.Printf("%s Done", logPrefix)
}

// getHostName get hostname
func (n *Node) getHostName() string {
	cmd := `hostname`
	return cmd
}

// sendInstallFiles send cluster docker images, ca certs, cluster config files to remote host
func (n *Node) sendInstallFiles(sshClient *ssh.Client, configTempDir string) error {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ sendInstallFiles ]", n.ClusterName, n.HostIP)

	dir := map[string]string{
		config.GDefault.CurrentDir + "/ssl/ca.crt":				 config.GDefault.RemoteTempDir + "/script/ssl/",
		configTempDir + "/script/":                      config.GDefault.RemoteTempDir + "/script/",
		configTempDir + "/" + n.HostIP + "/kubernetes/": "/etc/kubernetes/",
	}

	found, _ := utils.Exists(configTempDir + "/" + n.HostIP + "/daemonset/storage.sh")
	if found {
		dir[configTempDir+"/"+n.HostIP+"/daemonset/storage.sh"] = config.GDefault.RemoteTempDir + "/script/daemonset/"
	}

	// clean dir for later copy
	cmd := "rm -rf " + config.GDefault.RemoteTempDir
	ret, err := utils.Execute(cmd, sshClient)
	if err != nil {
		log.Printf("%s clean send dest dir failed: %s, %s", logPrefix, ret, err.Error())
	}

	err = utils.SendToRemote(sshClient, dir)
	return err
}

// sendUninstallFiles send uninstall depended files.
func (n *Node) sendUninstallFiles(sshClient *ssh.Client) error {
	dir := map[string]string{
		config.GDefault.CurrentDir + "/script/": config.GDefault.RemoteTempDir + "/script/",
	}

	err := utils.SendToRemote(sshClient, dir)
	return err
}

// checkMaster check apiserver status
func (n *Node) checkMaster() bool {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ checkMaster ]", n.ClusterName, n.HostIP)
	// check apiserver status and install addons and check component status
	err := APICheck(n.CaCert, n.APIClientCert, n.APIClientKey, fmt.Sprintf("https://%s:6443", n.MasterIP), 10)
	if err != nil {
		log.Printf("%s %s", logPrefix, err.Error())
		return false
	}

	return true
}

// callUninstallScript call uninstall.sh script
func (n *Node) callUninstallScript() string {
	logDir := config.GDefault.RemoteLogDir + "/node"
	cmd := fmt.Sprintf("mkdir -p %s && cd %s/script/node/ && /bin/bash uninstall.sh -logID %s > %s/uninstall.log 2>&1",
		logDir, config.GDefault.RemoteTempDir, n.HostIP, logDir)
	return cmd
}

// setHostname 主机名规则： n + 集群名称（提取有效字符：字母、数字、-） + 节点IP
func (n *Node) setHostname(sshClient *ssh.Client) error {
	// Step : 设置主机名
	n.HostName = strings.ToLower("n-" + utils.GetValidCh(n.ClusterName) + "-" + strings.Replace(n.HostIP, ".", "-", -1)) // 转换成小写字母
	n.HostName = strings.Replace(n.HostName, "\n", "", -1)                                                               // 去掉换行符

	// Step : 修改主机名
	if config.GK8sDefault.ChangeHostname == "true" {
		cmd := fmt.Sprintf("hostnamectl set-hostname %s; hostname %s;"+
			"sed -i '/127.0.1.1/d' /etc/hosts;sed -i '2 i 127.0.1.1       %s' /etc/hosts", n.HostName, n.HostName, n.HostName)
		_, err := utils.Execute(cmd, sshClient)
		if err != nil {
			return err
		}
	}

	return nil
}

// setDNS 设置dns
func (n *Node) setDNS(sshClient *ssh.Client) error {
	cmd := `sed  '/^$/d' /etc/hosts && sed -i '/ufleet.io/d' /etc/hosts`
	cmd  = fmt.Sprintf("%s && echo -e \"\n%s  ufleet.io\" >> /etc/hosts", cmd, config.GDefault.RegistryIp)
	_, err := utils.Execute(cmd, sshClient)
	return err
}

// restartDocker 重启docker
func (n *Node) restartDocker(sshClient *ssh.Client) error {
	cmd := `service docker restart`
	_, err := utils.Execute(cmd, sshClient)
	return err
}

// importRegistryCa 导入内置的镜像仓库ca证书
func (n *Node) importRegistryCa(sshClient *ssh.Client) error {
	// 获取系统类型并拷贝证书到对应位置
	systemType, err := utils.GetSystemType(sshClient)
	if err != nil {
		return err
	}
	if systemType == "unkown" {
		return fmt.Errorf("parse system type failed, just support centos, redhat, ubuntu, debian")
	}
	if systemType == "ubuntu" || systemType == "debian" {
		cmd := `rm -rf /usr/local/share/ca-certificates/ufleet-register-ca.crt.crt && rm -rf /etc/ssl/certs/ufleet-register-ca.crt.pem && update-ca-certificates`
		cmd = fmt.Sprintf("%s && cp -f %s/script/ssl/ca.crt /usr/local/share/ca-certificates/ufleet-register-ca.crt && update-ca-certificates", cmd, config.GDefault.RemoteTempDir)

		_, err := utils.Execute(cmd, sshClient)
		if err != nil {
			return err
		}
	} else {
		cmd := `rm -rf /etc/pki/ca-trust/source/anchors/ufleet-register-ca.crt.crt && update-ca-trust`
		cmd = fmt.Sprintf("%s && cp -f %s/script/ssl/ca.crt /etc/pki/ca-trust/source/anchors/ufleet-register-ca.crt.crt && update-ca-trust", cmd, config.GDefault.RemoteTempDir)

		_, err := utils.Execute(cmd, sshClient)
		if err != nil {
			return err
		}
	}

	// 重启docker使docker使用新的证书
	err = n.restartDocker(sshClient)
	if err != nil {
		return err
	}

	// 空等 5秒，保证重启docker命令已经下发
	time.Sleep(5 * time.Second)

	// 等待docker准备好
	err = n.waitForDockerReady(sshClient, 180)

	return err
}

// waitForDockerReady 等待docker准备好
func (n *Node) waitForDockerReady(sshClient *ssh.Client, timeout int) (err error) {
	cmd := `docker images`

	for i:=0;i<timeout;i++ {
		_, err = utils.Execute(cmd, sshClient)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	return
}

// genConfigAndCerts generate config and certs
func (n *Node) genConfigAndCerts(tempDir string, joinToken string) error {
	var err error

	/* 拷贝script 到 tempDir */
	if err = utils.CopyDir(config.GDefault.CurrentDir+"/script", tempDir+"/script"); err != nil {
		return fmt.Errorf("copy " + config.GDefault.CurrentDir + "/script to " + tempDir + "/script failed. " + err.Error())
	}

	/* 获取变量值 */
	scriptDestDir := tempDir + "/script"

	// get cluster info
	cluInfo := GetClusterInfo(n.ClusterName)
	if cluInfo == nil {
		return fmt.Errorf("can't get cluster info by cluster name: %s", n.ClusterName)
	}
	clu, err := GetCluster(n.ClusterName)
	if err != nil {
		return err
	}

	// image array to map
	imageMaps := imageArray2Map(clu.Images)
	imageStorageUbuntu, found := imageMaps["storage_ubuntu"]
	if !found { return fmt.Errorf("can't find image: %s", "storage_ubuntu") }
	imageStorageUbuntu = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageStorageUbuntu)

	imageStorageCentos, found := imageMaps["storage_centos"]
	if !found { return fmt.Errorf("can't find image: %s", "storage_centos") }
	imageStorageCentos = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageStorageCentos)

	imageKubelet, found := imageMaps["kubelet"]
	if !found { return fmt.Errorf("can't find image: %s", "kubelet") }
	imageKubelet = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKubelet)

	/* 生成所有配置文件 */
	// kubernetes.conf
	kubernetesObject := struct {
		K8sVersion string
		NtpdHost   string
		Nodename   string
	}{
		cluInfo.K8sVersion,
		config.GDefault.NtpdHost,
		n.HostName,
	}
	if err = utils.TmplReplaceByObject(scriptDestDir+"/common/kubernetes.conf", manifests.GetKubernetesConf(), kubernetesObject, 0666); err != nil {
		return err
	}

	// kubelet.sh
	kubeletObject := struct {
		K8sVersion string
		Hostip string
		Hostname string
		ImageKubelet string
	}{
		cluInfo.K8sVersion,
		n.HostIP,
		n.HostName,
		imageKubelet,
	}
	if err = utils.TmplReplaceByObject(scriptDestDir+"/common/kubelet.sh", manifests.GetKubeletSh(), kubeletObject, 0777); err != nil {
		return err
	}

	// apiserver-kubelet-client.crt and apiserver-kubelet-client.key
	if err = utils.GenNewFile(tempDir+"/"+n.HostIP+"/kubernetes/pki/apiserver-kubelet-client.crt", n.APIClientCert); err != nil {
		return err
	}
	if err = utils.GenNewFile(tempDir+"/"+n.HostIP+"/kubernetes/pki/apiserver-kubelet-client.key", n.APIClientKey); err != nil {
		return err
	}

	// kubelet.conf
	kubeletConfObject := struct {
		Hostip string
		CaCert string
		ApiserverKubeletClientCert string
		ApiserverKubeletClientKey  string
		JoinToken string
	}{
		n.MasterIP,
		base64.StdEncoding.EncodeToString([]byte(n.CaCert)),
		base64.StdEncoding.EncodeToString([]byte(n.APIClientCert)),
		base64.StdEncoding.EncodeToString([]byte(n.APIClientKey)),
		joinToken,
	}
	if err = utils.TmplReplaceByObject(tempDir+"/"+n.HostIP+"/kubernetes/kubelet.conf", manifests.GetKubeletConf(), kubeletConfObject, 0666); err != nil {
		return err
	}

	// storage.sh
	etcd1Ip := ""
	etcd2Ip := ""
	etcd3Ip := ""
	retryTimes := 3
	mValues, found := GetMasters(n.ClusterName)
	if !found {
		return fmt.Errorf("can't get masters by cluster name: %s", n.ClusterName)
	}
	IPs := make([]string, 0, 3)
	for _, master := range mValues {
		IPs = append(IPs, master.HostIP)
	}
	lEtcdv2 := etcd.LEtcdv2{}
	err = lEtcdv2.Init(IPs, 2379)
	if err != nil {
		return fmt.Errorf("can init launcher etcd client: %s", err.Error())
	}
	for i:=0;i<retryTimes;i++ {
		if etcd1Ip, err = lEtcdv2.GetIpByName("etcd1");err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		return fmt.Errorf("can't get etcd1 failed: %s", err.Error())
	}
	if len(mValues) == 1 {
		etcd2Ip = etcd1Ip
		etcd3Ip = etcd1Ip
	} else {
		var err1, err2 error
		for i:=0;i<retryTimes;i++ {
			etcd2Ip, err1 = lEtcdv2.GetIpByName("etcd2")
			etcd3Ip, err2 = lEtcdv2.GetIpByName("etcd3")
			if err1 == nil && err2 == nil {
				break
			}
			err = fmt.Errorf("get etcd2 error: %s, get etcd3 error: %s", err1, err2)
			time.Sleep(time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("can't get etcd2/etcd3 failed: %s", err.Error())
	}
	if etcd1Ip == "" || etcd2Ip == "" || etcd3Ip == "" {
		// 获取etcd节点信息失败
		return common.EtcdNodeNumberNotThree
	}
	storageObject := struct {
		ManagerAddr string
		EtcdName    string
		Etcd1IP     string
		Etcd2IP		string
		Etcd3IP		string
		RootPasswd	string
		ImageStorageUbuntu string
		ImageStorageCentos string
	}{
		"",
		"etcd1",
		etcd1Ip,
		etcd2Ip,
		etcd3Ip,
		"nopasswd",
		imageStorageUbuntu,
		imageStorageCentos,
	}
	if etcd1Ip == etcd2Ip {
		// 单master节点
		storageObject.ManagerAddr = etcd1Ip
	} else {
		// ha模式
		storageObject.ManagerAddr = etcd1Ip+","+etcd2Ip+","+etcd3Ip
	}
	if len(n.UserPwd) > 0 {
		storageObject.RootPasswd = n.UserPwd
	}
	if err = utils.TmplReplaceByObject(tempDir+"/"+n.HostIP+"/daemonset/storage.sh", manifests.GetStorageSh(), storageObject, 0777); err != nil {
		return err
	}

	return nil
}

// saveStatusWithMsgIfExist save status
func (n *Node) saveStatusWithMsgIfExist(status string, msg string) {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ saveStatusWithMsgIfExist ]", n.ClusterName, n.HostIP)
	n.ErrorMsg = msg
	n.Status = status
	exist := n.saveStatusIfExist()
	if !exist {
		log.Printf("%s node seems already recreated or removed. go exit()", logPrefix)
		runtime.Goexit()
	}
}

// cleanEnv clean install depended files after install
func (n *Node) cleanEnv(sshClient *ssh.Client, configTempDir string) error {
	tryTimes := 3
	errMsg := ""
	var err error

	// clean remote temp dir
	cmd := "rm -rf " + config.GDefault.RemoteTempDir
	for i := 0; i < tryTimes; i++ {
		if _, err = utils.Execute(cmd, sshClient); err == nil {
			break
		}
		errMsg = fmt.Sprintf("try to delete remote dir(%s) failed: %s", config.GDefault.RemoteTempDir, err.Error())
	}

	// clean local temp dir
	for i := 0; i < tryTimes; i++ {
		if err = os.RemoveAll(configTempDir); err == nil {
			break
		}
		if len(errMsg) > 0 {
			errMsg = errMsg + ", "
		}
		errMsg = fmt.Sprintf("%s try to delete dir(%s) failed: %s", errMsg, configTempDir, err.Error())
	}
	if len(errMsg) > 0 {
		return fmt.Errorf(errMsg)
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
			log.Printf("remove older temp directory - %s", waitRemoveDir)
			if err = os.RemoveAll(waitRemoveDir); err != nil {
				log.Printf("remove dir - %s failed: %s", info.Name(), err.Error())
			}
		}
	}

	return nil
}

func (n *Node) exitIfRecreate(progress string, syncProgress bool) {
	if config.GDefault.ServiceType == "module" {
		// in module running type, just return
		return
	}

	logPrefix := fmt.Sprintf("[ %s ][ %s ][ exitIfRecreate ]", n.ClusterName, n.HostIP)

	inner, found := GetNode(n.ClusterName, n.HostIP)
	if !found {
		log.Printf("%s node already remove", logPrefix)
		runtime.Goexit()
	}

	if n.CreateTime != inner.CreateTime {
		log.Printf("%s node already recreate", logPrefix)
		runtime.Goexit()
	}

	if !syncProgress {
		return
	}
	n.Progress = progress
	n.saveStatusIfExist()
}

func (n *Node) setNodeLabel() error {
	tryTimes := 60

	client, err := NewK8sClient(fmt.Sprintf("https://%s:%d", n.MasterIP, 6443), []byte(n.APIClientCert), []byte(n.APIClientKey), time.Second * 30)
	if err != nil {
		return err
	}

	for i:=0;i<tryTimes;i++ {
		if v1Node, err := client.CoreV1().Nodes().Get(n.HostName, meta_v1.GetOptions{}); err == nil && v1Node != nil {
			v1Node.Labels["node-role.kubernetes.io/node"] = "true"

			_, err = client.CoreV1().Nodes().Update(v1Node)
			if err == nil {
				break
			}
		}

		time.Sleep(time.Second)
	}

	return err
}

// saveStatus check node status and write to etcd
func (n *Node) SaveStatus() {
	nLock.Lock()
	defer nLock.Unlock()

	nodeData, err := json.Marshal(&n)
	if err != nil {
		log.Printf(err.Error())
	}

	key := strings.Join([]string{common.ClusterKey, n.ClusterName, common.NodeKey, n.HostIP}, common.Sep)
	err = base.Put(key, string(nodeData))
	if err != nil {
		log.Printf("[ %s ][ %s ][ saveStatus ] Parser node data error: %s", n.ClusterName, n.HostIP, err.Error())
	}
}

// saveStatusIfExist Save status if node in etcd. But just return false when
//     1. node not in etcd
//     2. node is already installed
func (n *Node) saveStatusIfExist() bool {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ Etcd Node saveStatusIfExist ]", n.ClusterName, n.HostIP)

	nLock.Lock()
	defer nLock.Unlock()

	key := strings.Join([]string{common.ClusterKey, n.ClusterName, common.NodeKey, n.HostIP}, common.Sep)

	if config.GDefault.ServiceType != "module" {
		exist, _ := base.IsExist(key)
		if !exist {
			return false
		}

		/* if node already recreate, return false */
		v := Node{}
		key = strings.Join([]string{common.ClusterKey, n.ClusterName, common.NodeKey, n.HostIP}, common.Sep)
		values, err := base.Get(key)
		for i := 0; i < 3 && err != nil; i++ {
			log.Printf("%s retry (get %s) %d time, because: %s", logPrefix, key, i+1, err.Error())
			values, err = base.Get(key)
		}
		if err == nil && values != nil && values[key] != "" {
			err = json.Unmarshal([]byte(values[key]), &v)
			if err == nil && v.CreateTime != n.CreateTime {
				log.Printf("%s already recreate.", logPrefix)
				return false
			}
			if err != nil {
				log.Printf("%s parse node info from json info failed", logPrefix)
			}
		} else {
			log.Printf("%s get node info from etcd failed", logPrefix)
		}
	}

	nodeData, err := json.Marshal(&n)
	if err != nil {
		log.Printf(err.Error())
	}

	key = strings.Join([]string{common.ClusterKey, n.ClusterName, common.NodeKey, n.HostIP}, common.Sep)
	err = base.Put(key, string(nodeData))
	if err != nil {
		log.Printf("%s Parser node data error: %s", n.HostIP, err.Error())
	}
	return true
}