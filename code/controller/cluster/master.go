package cluster

import (
	"sync"
	"log"
	"strconv"
	"fmt"
	"golang.org/x/crypto/ssh"
	"ufleet/launcher/code/config/manifests"
	"ufleet/launcher/code/utils"
	"ufleet/launcher/code/model/common"
	"ufleet/launcher/code/model/base"
	"ufleet/launcher/code/config"
	"ufleet/launcher/code/utils/certs"
	"os"
	"io/ioutil"
	"strings"
	"time"
	"net"
	"runtime"
	"encoding/json"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	mLock = new(sync.Mutex) //mLock master lock
)

// genApiserverCert 生成apiserver 等证书，密钥
func (m *Master) genApiserverCert(vip, tempDir string) error {
	cfg := &certs.MasterConfiguration{}

	cfg.API.AdvertiseAddress = vip
	cfg.API.BindPort = 6443

	cfg.CertificatesDir = tempDir + "/" + m.HostIP + common.GlobalConfigCertPath
	cfg.Networking.PodSubnet = common.GlobalDefaultNetPodSubnet
	cfg.Networking.ServiceSubnet = common.GlobalDefaultNetServiceSubnet

	// 生成证书目录
	if err := os.MkdirAll(cfg.CertificatesDir, 0700); err != nil {
		return err
	}

	// 生成ca等证书及密钥
	cluInfo := GetClusterInfo(m.ClusterName)
	if cluInfo == nil || len(cluInfo.JoinToken) == 0 ||
		len(cluInfo.CaCert) == 0 || len(cluInfo.APIClientCert) == 0 ||
		len(cluInfo.APIClientKey) == 0 || len(cluInfo.Vip) == 0 {
		return fmt.Errorf("get cluster info failed")
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/ca.crt", []byte(cluInfo.CaCert), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/ca.key", []byte(cluInfo.CaKey), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/apiserver-kubelet-client.crt", []byte(cluInfo.APIClientCert), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/apiserver-kubelet-client.key", []byte(cluInfo.APIClientKey), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/front-proxy-ca.crt", []byte(cluInfo.FrontProxyCert), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/front-proxy-ca.key", []byte(cluInfo.FrontProxyKey), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/front-proxy-client.crt", []byte(cluInfo.FrPxyCliCert), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/front-proxy-client.key", []byte(cluInfo.FrPxyCliKey), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/sa.pub", []byte(cluInfo.SaPub), 600); err != nil {
		return err
	}
	if err := ioutil.WriteFile(cfg.CertificatesDir+"/sa.key", []byte(cluInfo.SaKey), 600); err != nil {
		return err
	}

	// 生成apiserver证书
	cfg.APIServerCertSAN = m.HostIP
	err := certs.CreatePKIAssetsJustApiserver(cfg, m.HostName)

	return err
}

/*
 * 多主机配置文件生成
 * 存放位置：
 * 	      {{tempDir}}/#type/#hostip/#file
 *   Like:{{tempDir}}/etcd/192.168.5.15/etcdStart.sh
 */
func (m *Master) genConfig(clu *Cluster, tempDir string) error {
	var err error

	/* 拷贝script 到 tempDir */
	destDirMaps := make(map[string]string)
	if err := utils.CopyDir(config.GDefault.CurrentDir+"/script", tempDir+"/"+m.HostIP+"/script"); err != nil {
		return fmt.Errorf("copy " + config.GDefault.CurrentDir + "/script to " + tempDir + "/" + m.HostIP + "/script failed. " + err.Error())
	}
	destDirMaps[m.HostIP] = tempDir + "/" + m.HostIP + "/script"

	/* 获取需要的变量值 */
	etcdCluster := ""
	etcdEndpoints := ""
	calicoEtcdCluster := ""
	etcdPeerPort := strconv.Itoa(config.GK8sDefault.EtcdPeerPort)
	etcdListenPort := strconv.Itoa(config.GK8sDefault.EtcdListenPort)

	etcdDir := tempDir + common.GlobalConfigEtcdPath
	//vespaceDir := tempDir + common.GlobalConfigVespacePath

	for i, master := range clu.Masters {
		if i == 0 {
			etcdCluster = fmt.Sprintf("infra-%s=http://%s:%d", master.HostIP, master.HostIP, config.GK8sDefault.EtcdPeerPort)
			etcdEndpoints = fmt.Sprintf("- http://%s:%d", master.HostIP, config.GK8sDefault.EtcdListenPort)
			calicoEtcdCluster = fmt.Sprintf("http://%s:%d", master.HostIP, config.GK8sDefault.EtcdListenPort)
		} else {
			etcdCluster = fmt.Sprintf("%s,infra-%s=http://%s:%d", etcdCluster, master.HostIP, master.HostIP, config.GK8sDefault.EtcdPeerPort)
			etcdEndpoints = fmt.Sprintf("%s\n  - http://%s:%d", etcdEndpoints, master.HostIP, config.GK8sDefault.EtcdListenPort)
			calicoEtcdCluster = fmt.Sprintf("%s,http://%s:%d", calicoEtcdCluster, master.HostIP, config.GK8sDefault.EtcdListenPort)
		}
	}

	// image array to map
	imageMaps := imageArray2Map(clu.Images)
	imageEtcdstart, found := imageMaps["etcd_amd64"]
	if !found { return fmt.Errorf("can't find image: %s", "etcd_amd64") }
	imageEtcdstart = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageEtcdstart)

	imageNtp, found := imageMaps["ntp"]
	if !found { return fmt.Errorf("can't find image: %s", "ntp") }
	imageNtp = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageNtp)

	imageKeepalived, found := imageMaps["keepalived"]
	if !found { return fmt.Errorf("can't find image: %s", "keepalived") }
	imageKeepalived = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKeepalived)

	imagePrometheusNodeExporter, found := imageMaps["prometheus_node_exporter"]
	if !found { return fmt.Errorf("can't find image: %s", "prometheus_node_exporter") }
	imagePrometheusNodeExporter = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imagePrometheusNodeExporter)

	imageKubelet, found := imageMaps["kubelet"]
	if !found { return fmt.Errorf("can't find image: %s", "kubelet") }
	imageKubelet = fmt.Sprintf("%s/%s", config.GDefault.BaseRegistory, imageKubelet)

	/* 生成所需的全部配置文件 */
	// etcdstart.sh
	etcdExistObject := struct {
		Hostip      string
		EtcdCluster string
		PeerPort    string
		ListenPort  string
		Token       string
		NtpdHost    string
		ImageNtp    string
		ImageEtcdAmd64 string
	}{
		m.HostIP,
		etcdCluster,
		etcdPeerPort,
		etcdListenPort,
		clu.Name,
		config.GDefault.NtpdHost,
		imageNtp,
		imageEtcdstart,
	}
	if err = utils.TmplReplaceByObject(etcdDir+"/"+m.HostIP+"/etcdStart.sh", manifests.GetEtcdstartSh(), etcdExistObject, 0777); err != nil {
		return err
	}

	// kubeadm.yaml
	kubeadmObject := struct {
		Hostip string
		Hostname string
		EtcdEndpoints string
		PodSubnet  string
		K8sVersion string
		JoinToken  string
		ServiceSubnet string
	}{
		clu.Vip,
		m.HostName,
		etcdEndpoints,
		m.PodNetwork,
		config.GK8sDefault.K8sVersion,
		clu.JoinToken,
		clu.Masters[0].PodNetwork,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[m.HostIP]+"/master/kubeadm.yaml", manifests.GetKubeadmYaml(), kubeadmObject, 0666); err != nil {
		return err
	}

	// keepalived
	keepalivedObject := struct {
		VirtualIP string
		VirtualRouterID string
		Interface string
		KeepalivedName string
		ImageKeepalived string
	}{
		clu.Vip,
		utils.GetIPField(clu.Vip, 4),
		"",
		strings.Replace(m.HostIP, ".", "-", -1),
		imageKeepalived,
	}
	netcard, err := m.getNetworkCardName()
	if err != nil {
		return err
	}
	keepalivedObject.Interface = netcard
	if err = utils.TmplReplaceByObject(destDirMaps[m.HostIP]+"/daemonset/keepalived.yaml", manifests.GetKeepalivedYaml(), keepalivedObject, 0666); err != nil {
		return err
	}

	// exporter.yaml
	exporterObject := struct {
		ImagePrometheusNodeExporter string
	}{
		imagePrometheusNodeExporter,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[clu.Masters[0].HostIP]+"/addon/conf/exporter.yaml", manifests.GetExporterYaml(), exporterObject, 0666); err != nil {
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
		m.HostName,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[m.HostIP]+"/common/kubernetes.conf", manifests.GetKubernetesConf(), kubernetesObject, 0666); err != nil {
		return err
	}

	// kubelet.sh
	kubeletObject := struct {
		K8sVersion string
		Hostip     string
		Hostname   string
		ImageKubelet string
	}{
		clu.K8sVersion,
		m.HostIP,
		m.HostName,
		imageKubelet,
	}
	if err = utils.TmplReplaceByObject(destDirMaps[m.HostIP]+"/common/kubelet.sh", manifests.GetKubeletSh(), kubeletObject, 0777); err != nil {
		return err
	}

	return nil
}

// releaseSSHClient release ssh client if exist.
func (m *Master) releaseSSHClient() error {
	if m.sshClient != nil {
		if err := m.sshClient.Close(); err != nil {
			return err
		}
		m.sshClient = nil
	}
	return nil
}

// getHostName get hostname
func (m *Master) getHostName() string {
	cmd := `hostname`
	return cmd
}

// callInstallScript 调用 install.sh 脚本
func (m *Master) callInstallScript() string {
	logDir := config.GDefault.RemoteLogDir + "/master"
	cmd := fmt.Sprintf("mkdir -p %s && /bin/bash %s/script/master/install.sh -logID %s > %s/install.log 2>&1",
		logDir, config.GDefault.RemoteTempDir, m.HostIP, logDir)
	return cmd
}

// callUninstallScript 调用 uninstall.sh 脚本
func (m *Master) callUninstallScript() string {
	logDir := config.GDefault.RemoteLogDir + "/master"
	cmd := fmt.Sprintf("mkdir -p %s && /bin/bash %s/script/master/uninstall.sh -logID %s > %s/uninstall.log 2>&1",
		logDir, config.GDefault.RemoteTempDir, m.HostIP, logDir)
	return cmd
}

// sendInstallFiles send cluster docker images, ca certs, cluster config files to remote host
func (m *Master) sendInstallFiles(sshClient *ssh.Client, configTempDir string, isAlone bool, isLeader bool) error {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ sendInstallFiles ]", m.ClusterName, m.HostIP)
	var dir map[string]string

	/* common */
	dir = map[string]string{
		config.GDefault.CurrentDir + "/ssl/ca.crt":							 config.GDefault.RemoteTempDir + "/script/ssl/",
		configTempDir + "/" + m.HostIP + "/script/":                 config.GDefault.RemoteTempDir + "/script/",
		configTempDir + "/" + m.HostIP + common.GlobalConfigCertPath + "/": "/etc/kubernetes/pki/",
		configTempDir + common.GlobalConfigEtcdPath + "/" + m.HostIP + "/": config.GDefault.RemoteTempDir + "/etcd/",
	}
	found, _ := utils.Exists(configTempDir + common.GlobalConfigVespacePath + "/" + m.HostIP + "/")
	if found {
		dir[configTempDir+common.GlobalConfigVespacePath+"/"+m.HostIP+"/"] = config.GDefault.RemoteTempDir + common.GlobalConfigVespacePath + "/"
	}

	if !isAlone {
		dir[configTempDir+"/"+m.HostIP+"/script/daemonset/"] = "/etc/kubernetes/manifests/"
	}

	// clean dir for later copy
	cmd := fmt.Sprintf("rm -rf %s", config.GDefault.RemoteTempDir)
	ret, err := utils.Execute(cmd, sshClient)
	if err != nil {
		log.Printf("%s clean send dest dir failed: %s, %s", logPrefix, ret, err.Error())
	}

	err = utils.SendToRemote(sshClient, dir)
	return err
}

func (m *Master) getAdminConfig(sshClient *ssh.Client) (string, error) {
	adminConfig, err := utils.Execute("cat /etc/kubernetes/admin.conf", sshClient)
	if err != nil {
		return "", err
	}

	return adminConfig, nil
}

// GetApiserverKey get apiserver certs, apiserver key
func (m *Master) getCertAndKeys(sshClient *ssh.Client) error {
	serverCert, err := utils.Execute("cat /etc/kubernetes/pki/apiserver.crt", sshClient)
	if err != nil || !strings.Contains(serverCert, "BEGIN CERTIFICATE") {
		return fmt.Errorf("get apiserver.crt failed: apiserver.crt: %s, err: %v", serverCert, err)
	}
	serverKey, err := utils.Execute("cat /etc/kubernetes/pki/apiserver.key", sshClient)
	if err != nil || !strings.Contains(serverKey, "BEGIN RSA PRIVATE KEY") {
		return fmt.Errorf("get apiserver.key failed: apiserver.key: %s, err: %v", serverKey, err)
	}

	m.APIServerCert = serverCert
	m.APIServerKey = serverKey

	return nil
}

// checkKubeAndAPIStatus check apiserver status
func (m *Master) checkKubeAndAPIStatus() error {
	logPrefix := fmt.Sprintf("[ %s ][ checkKubeAndAPIStatus ]", m.HostIP)
	if m.Status != common.GlobalNodeStatusRunning {
		log.Printf("%s Master status is failed", logPrefix)
		return fmt.Errorf("master status is failed")
	}
	// check apiserver status and install addons and check component status
	err := APICheck(m.CaCert, m.APIClientCert, m.APIClientKey, "https://"+m.HostIP+":6443", 10)
	if err != nil {
		return fmt.Errorf("check master by apiserver api, but failed. " + err.Error())
	}
	return nil
}

// sendUninstallFiles send uninstall depended files to remote machine.
func (m *Master) sendUninstallFiles(sshClient *ssh.Client) error {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ sendInstallFiles ]", m.ClusterName, m.HostIP)
	dir := map[string]string{
		config.GDefault.CurrentDir + "/script/": config.GDefault.RemoteTempDir + "/script/",
	}
	// clean dir for later copy
	cmd := fmt.Sprintf("rm -rf %s", config.GDefault.RemoteTempDir)
	ret, err := utils.Execute(cmd, sshClient)
	if err != nil {
		log.Printf("%s clean send dest dir failed: %s, %s", logPrefix, ret, err.Error())
	}

	err = utils.SendToRemote(sshClient, dir)
	return err
}

// installEssentialAddons install the kubernetes essential addons
func (m *Master) installEssentialAddons(sshClient *ssh.Client) error {
	logDir := config.GDefault.RemoteLogDir + "/addon"
	cmd := fmt.Sprintf("mkdir -p %s && cd %s/script/addon/ && ./addonctl -a install -m %s -logID %s > %s/addonctl.log 2>&1",
		logDir, config.GDefault.RemoteTempDir, m.HostIP, m.HostIP, logDir)

	_, err := utils.Execute(cmd, sshClient)
	if err != nil {
		return err
	}
	return nil
}

// installVespaceStrategy install vespace strategy
func (m *Master) installVespaceStrategy(sshClient *ssh.Client) error {
	logDir := config.GDefault.RemoteTempDir + common.GlobalConfigVespacePath
	cmd := fmt.Sprintf("mkdir -p %s && cd %s%s && /bin/bash vespace.sh -m %s -logID %s > %s/strategy.log 2>&1",
		logDir, config.GDefault.RemoteTempDir, common.GlobalConfigVespacePath, m.HostIP, m.HostIP, logDir)

	_, err := utils.Execute(cmd, sshClient)
	if err != nil {
		return err
	}
	return nil
}

// RemoveMaster remote master
func (m *Master) RemoveMaster() {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ RemoveMaster ]", m.ClusterName, m.HostIP)
	cmd := ""

	log.Printf("%s Begin to remove master node", logPrefix)

	log.Printf("%s check configure and set to default value", logPrefix)
	if m.HostSSHNetwork == "" {
		m.HostSSHNetwork = "tcp"
	}
	if m.HostSSHPort == "" {
		m.HostSSHPort = "22"
	}

	log.Printf("%s check ssh connect", logPrefix)
	sshClient, err := SSHClient(m.Host)
	if err != nil {
		log.Printf("%s Connect Failed! ErrorMsg: %s", logPrefix, err.Error())
		goto DELETEMASTER
	}
	defer sshClient.Close()

	log.Printf("%s Step 01: Send remove depended files", logPrefix)
	err = m.sendUninstallFiles(sshClient)
	if err != nil {
		log.Printf("%s Send remove depended files Failed! ErrorMsg: %s", logPrefix, err.Error())
		goto DELETEMASTER
	}

	log.Printf("%s Step 02: Run uninstall.sh", logPrefix)
	cmd = m.callUninstallScript()
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		log.Printf("%s Failed, uninstall.sh failed! ErrorMsg: %s", logPrefix, err.Error())
		goto DELETEMASTER
	}

DELETEMASTER:
	log.Printf("%s Done", logPrefix)
}

// setHostname 主机名规则： m + 集群名称（提取有效字符：字母、数字、-） + 节点IP
func (m *Master) setHostname(sshClient *ssh.Client) error {
	// Step : 设置主机名
	m.HostName = strings.ToLower("m-" + utils.GetValidCh(m.ClusterName) + "-" + strings.Replace(m.HostIP, ".", "-", -1)) // 转换成小写字母
	m.HostName = strings.Replace(m.HostName, "\n", "", -1)                                                               // 去掉换行符

	// Step : 修改主机名
	if config.GK8sDefault.ChangeHostname == "true" {
		cmd := fmt.Sprintf("hostnamectl set-hostname %s; hostname %s;sed -i '/127.0.1.1/d' /etc/hosts;sed -i '2 i 127.0.1.1       %s' /etc/hosts",
			m.HostName, m.HostName, m.HostName)
		_, err := utils.Execute(cmd, sshClient)
		if err != nil {
			return err
		}
	}

	return nil
}

// setDNS 设置dns
func (m *Master) setDNS(sshClient *ssh.Client) error {
	cmd := `sed  '/^$/d' /etc/hosts && sed -i '/ufleet.io/d' /etc/hosts`
	cmd  = fmt.Sprintf("%s && echo -e \"\n%s  ufleet.io\" >> /etc/hosts", cmd, config.GDefault.RegistryIp)
	_, err := utils.Execute(cmd, sshClient)
	return err
}

// restartDocker 重启docker
func (m *Master) restartDocker(sshClient *ssh.Client) error {
	cmd := `service docker restart`
	_, err := utils.Execute(cmd, sshClient)
	return err
}

// waitForDockerReady 等待docker准备好
func (m *Master) waitForDockerReady(sshClient *ssh.Client, timeout int) (err error) {
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

// importRegistryCa 导入内置的镜像仓库ca证书
func (m *Master) importRegistryCa(sshClient *ssh.Client) error {
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
	err = m.restartDocker(sshClient)
	if err != nil {
		return err
	}

	// 空等 5秒，保证重启docker命令已经下发
	time.Sleep(5 * time.Second)

	// 等待docker 完成
	err = m.waitForDockerReady(sshClient, 180) // wait 3 minutes

	return err
}

// LoadModprobe load mode probe
func (m *Master) LoadModprobe() error {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ LoadModprobe ]", m.ClusterName, m.HostIP)
	log.Printf("%s check ssh connect", logPrefix)
	sshClient, err := m.GetSSHClient()
	if err != nil {
		return fmt.Errorf("get ssh client failed")
	}

	cmd := "modprobe ip_vs"
	_, err = utils.Execute(cmd, sshClient)
	if err != nil {
		// 加载内核模块失败
		return err
	}
	return nil
}

// saveStatusWithMsgIfExist save status with msg
func (m *Master) saveStatusWithMsgIfExist(status string, msg string) {
	m.ErrorMsg = msg
	m.Status = status
	m.saveStatusIfExist()
}

// cleanEnv clean install depended files after install
func (m *Master) cleanEnv(sshClient *ssh.Client) error {
	cmd := "rm -rf " + config.GDefault.RemoteTempDir
	_, err := utils.Execute(cmd, sshClient)
	return err
}

//
func (m *Master) sameSubnetwork(vipStr string) (bool, error) {
	// 检查VIP，以及masters是否在同一个网段，不在同一个网段，VIP切换会造成无法访问服务的错误
	var vip net.IP
	var mVip net.IP // 经过掩码处理后的vip
	vip = net.ParseIP(vipStr)
	if vip == nil {
		return false, fmt.Errorf("parse vip failed")
	}

	ips := make([]net.IP, 0, 2)

	// 处理ip
	cmd := fmt.Sprintf("ip addr | sed 's/\\ /\\n/g' | grep %s", m.HostIP)
	sshClient, err := m.GetSSHClient()
	if err != nil {
		return false, fmt.Errorf("get ssh client failed, maybe master already lost")
	}
	resp, err := utils.Execute(cmd, sshClient)
	if err != nil {
		return false, fmt.Errorf("get ip/mask failed, maybe master already lost")
	}
	resp = strings.Replace(resp, "\n", "", -1)
	ip, ipNet, err := net.ParseCIDR(resp) // resp 必须满足： 192.168.4.12/24 的格式
	if err != nil {
		return false, fmt.Errorf("parse cidr failed: %s", err.Error())
	}
	realIp := ip.Mask(ipNet.Mask)
	if realIp == nil {
		return false, fmt.Errorf("parse mask ip failed: ip -- %s, mask -- %s", ip.String(), ipNet.Mask.String())
	}
	ips = append(ips, realIp)

	// 处理vip
	mVip = vip.Mask(ipNet.Mask)
	if mVip == nil {
		return false, fmt.Errorf("parse mask ip failed: ip -- %s, mask -- %s", mVip.String(), ipNet.Mask.String())
	}
	ips = append(ips, mVip)

	// 比较经过掩码处理后的ip是否相等
	for i := 0; i < len(ips); i++ {
		if mVip.Equal(ips[i]) {
			continue
		}
		return false, nil
	}

	return true, nil
}

// exitIfRecreate if already recreate, this goroutine go exit directly.
//                if not, and syncProgress is true, will record the progress of install steps(如果未删除，则记录当前进度)
func (m *Master) exitIfRecreate(progress string, syncProgress bool) {
	if config.GDefault.ServiceType == "module" {
		// in module running type, just return
		return
	}

	logPrefix := fmt.Sprintf("[ %s ][ %s ][ exitIfRecreate ]", m.ClusterName, m.HostIP)

	inner, found := GetMaster(m.ClusterName, m.HostIP)
	if !found {
		log.Printf("%s master already remove", logPrefix)
		runtime.Goexit()
	}

	if m.CreateTime != inner.CreateTime {
		log.Printf("%s master already recreate", logPrefix)
		runtime.Goexit()
	}

	if !syncProgress {
		return
	}
	m.Progress = progress
	m.saveStatusIfExist()
}

// setMasterLabel set =true
func (m *Master) setMasterLabel() error {
	tryTimes := 60

	client, err := NewK8sClient(fmt.Sprintf("https://%s:%d", m.HostIP, 6443), []byte(m.APIClientCert), []byte(m.APIClientKey), time.Second * 30)
	if err != nil {
		return err
	}

	for i:=0;i<tryTimes;i++ {
		if v1Node, err := client.CoreV1().Nodes().Get(m.HostName, meta_v1.GetOptions{}); err == nil && v1Node != nil{
			v1Node.Labels["node-role.kubernetes.io/master"] = "true"
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

// saveStatus save master status to etcd
func (m *Master) SaveStatus() {
	mLock.Lock()
	defer mLock.Unlock()

	masterData, err := json.Marshal(&m)
	if err != nil {
		log.Printf(err.Error())
	}

	key := strings.Join([]string{common.ClusterKey, m.ClusterName, common.MasterKey, m.HostIP}, common.Sep)
	err = base.Put(key, string(masterData))
	if err != nil {
		log.Printf("%s Parser master data error: %s", m.HostIP, err.Error())
	}
}

// saveStatusIfExist Save status if master in etcd. But just return false when:
//     1. master not in etcd
//     2. master is already installed.
func (m *Master) saveStatusIfExist() bool {
	logPrefix := fmt.Sprintf("[ %s ][ %s ][ Etcd Master saveStatusIfExist ]", m.ClusterName, m.HostIP)

	mLock.Lock()
	defer mLock.Unlock()

	key := strings.Join([]string{common.ClusterKey, m.ClusterName, common.MasterKey, m.HostIP}, common.Sep)

	if config.GDefault.ServiceType != "module" {
		exist, _ := base.IsExist(key)
		if !exist {
			return false
		}

		/* if master already recreate, return false */
		v := Master{}
		key = strings.Join([]string{common.ClusterKey, m.ClusterName, common.MasterKey, m.HostIP}, common.Sep)
		values, err := base.Get(key)
		for i := 0; i < 3 && err != nil; i++ {
			log.Printf("%s retry (get %s) %d time, because: %s", logPrefix, key, i+1, err.Error())
			values, err = base.Get(key)
		}
		if err == nil && len(values) > 0 {
			err = json.Unmarshal([]byte(values[key]), &v)
			if err == nil && v.CreateTime != m.CreateTime {
				log.Printf("%s master already recreate.", logPrefix)
				return false
			}
			if err != nil {
				log.Printf("%s parse master info from json info failed", logPrefix)
			}
		} else {
			log.Printf("%s get master info from etcd failed", logPrefix)
		}
	}

	masterData, err := json.Marshal(&m)
	if err != nil {
		log.Printf(err.Error())
		return true
	}

	err = base.Put(key, string(masterData))
	if err != nil {
		log.Printf("%s Parser master data error: %s", logPrefix, err.Error())
	}

	return true
}
