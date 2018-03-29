package config

import (
	"os"
	"log"
)

var (
	k8sConfigFile="/config/kubernetes.conf"
	launcherConfigFile="/config/launcher.conf"
)

func init(){
	var err error

	GDefault.CurrentDir, err = os.Getwd()
	if err != nil {
		log.Fatal("Can't get current dir by os.Getwd(). ErrorMsg: " + err.Error())
	}

	GDefault.NtpdHost = os.Getenv("NTPD_HOST")
	if len(GDefault.NtpdHost) == 0 {
		log.Fatal("can't get NTPD_HOST variable")
	}
	GDefault.RegistryIp = os.Getenv("REGISTRY_IP")
	if len(GDefault.RegistryIp) == 0 {
		log.Fatal("can't get REGISTRY_IP variable")
	}
	GDefault.HostIP = os.Getenv("CURRENT_HOST")
	if len(GDefault.HostIP) == 0 {
		log.Fatal("env variable CURRENT_HOST not found.")
	}
}
